package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
	_ "modernc.org/sqlite"
)

func main() {
	key, secret := getDarajaCredentials()
	token, err := getAccessToken(key, secret)
	if err != nil {
		fmt.Println("Error getting access token:", err)
	} else {
		fmt.Println("Token:", token)
	}

	var db *sql.DB
	tursoURL := os.Getenv("TURSO_DATABASE_URL")
	tursoToken := os.Getenv("TURSO_AUTH_TOKEN")

	if tursoURL != "" {
		dbURL := tursoURL + "?authToken=" + tursoToken
		db, err = sql.Open("libsql", dbURL)
		fmt.Println("Using Turso database")
	} else {
		db, err = sql.Open("sqlite", "chama.db")
		fmt.Println("Using local SQLite database")
	}
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	fmt.Println("Successfully connected to database!")

	schema, err := os.ReadFile("schema.sql")
	if err != nil {
		log.Fatal("Failed to read schema.sql:", err)
	}
	_, err = db.Exec(string(schema))
	if err != nil {
		log.Fatal("Failed to apply schema:", err)
	}
	fmt.Println("Database schema ensured.")

	newColumns := []string{
		"ALTER TABLE members ADD COLUMN avatar_url TEXT",
		"ALTER TABLE members ADD COLUMN preferred_first_name TEXT",
		"ALTER TABLE members ADD COLUMN default_payout_method TEXT",
		"ALTER TABLE members ADD COLUMN next_of_kin_relationship TEXT",
		"ALTER TABLE members ADD COLUMN next_of_kin_id_number TEXT",
		"ALTER TABLE members ADD COLUMN next_of_kin_phone TEXT",
		"ALTER TABLE members ADD COLUMN notify_sms INTEGER DEFAULT 1",
		"ALTER TABLE members ADD COLUMN notify_email INTEGER DEFAULT 1",

		"ALTER TABLE chamas ADD COLUMN description TEXT",
		"ALTER TABLE chamas ADD COLUMN start_date TEXT",
		"ALTER TABLE chamas ADD COLUMN max_participants INTEGER",

		"ALTER TABLE chama_members ADD COLUMN status TEXT DEFAULT 'active'",

		"ALTER TABLE contributions ADD COLUMN chama_id INTEGER",
		"ALTER TABLE contributions ADD COLUMN checkout_request_id TEXT",
		"ALTER TABLE contributions ADD COLUMN cycle_id INTEGER",

		"ALTER TABLE withdrawals ADD COLUMN chama_id INTEGER",

		"ALTER TABLE group_settings ADD COLUMN chama_id INTEGER",
		"ALTER TABLE group_settings ADD COLUMN payout_position INTEGER DEFAULT 0",
		"ALTER TABLE group_settings ADD COLUMN next_payout_date TEXT",
		"ALTER TABLE group_settings ADD COLUMN number_of_rounds INTEGER DEFAULT 1",
		"ALTER TABLE group_settings ADD COLUMN payout_method TEXT DEFAULT 'fixed_rotation'",
		"ALTER TABLE group_settings ADD COLUMN grace_period_days INTEGER DEFAULT 0",
		"ALTER TABLE group_settings ADD COLUMN late_penalty_type TEXT DEFAULT 'none'",
		"ALTER TABLE group_settings ADD COLUMN late_penalty_amount REAL DEFAULT 0",
		"ALTER TABLE group_settings ADD COLUMN welfare_reserve_amount REAL DEFAULT 0",
		"ALTER TABLE group_settings ADD COLUMN approval_threshold INTEGER DEFAULT 1",
		"ALTER TABLE group_settings ADD COLUMN payout_destination_type TEXT",
		"ALTER TABLE group_settings ADD COLUMN paybill_number TEXT",
		"ALTER TABLE group_settings ADD COLUMN till_number TEXT",
		"ALTER TABLE group_settings ADD COLUMN treasurer_phone TEXT",
		"ALTER TABLE group_settings ADD COLUMN treasurer_account_name TEXT",
	}
	for _, stmt := range newColumns {
		_, err := db.Exec(stmt)
		if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			log.Fatal("Failed to add column:", err)
		}
	}
	fmt.Println("Profile system columns ensured.")

	var chamaCount int
	db.QueryRow("SELECT COUNT(*) FROM chamas").Scan(&chamaCount)
	if chamaCount == 0 {
		var firstMemberID int64
		err := db.QueryRow("SELECT id FROM members ORDER BY id LIMIT 1").Scan(&firstMemberID)
		if err == nil {
			result, err := db.Exec("INSERT INTO chamas (name, created_by) VALUES (?, ?)", "Chama Salama", firstMemberID)
			if err == nil {
				chamaID, _ := result.LastInsertId()
				db.Exec(
					`INSERT INTO chama_members (chama_id, member_id, role)
					 SELECT ?, id, role FROM members WHERE id NOT IN (SELECT member_id FROM chama_members WHERE chama_id = ?)`,
					chamaID, chamaID,
				)
				fmt.Println("Default chama created and existing members attached.")
			}
		}
	}
	var defaultChamaID int64
	db.QueryRow("SELECT id FROM chamas ORDER BY id LIMIT 1").Scan(&defaultChamaID)
	if defaultChamaID != 0 {
		db.Exec("UPDATE contributions SET chama_id = ? WHERE chama_id IS NULL", defaultChamaID)
		db.Exec("UPDATE group_settings SET chama_id = ? WHERE chama_id IS NULL", defaultChamaID)
		db.Exec("UPDATE withdrawals SET chama_id = ? WHERE chama_id IS NULL", defaultChamaID)
		fmt.Println("Backfilled chama_id for existing records.")
	}

	startServer(db)
}
