package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
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

	startServer(db)
}
