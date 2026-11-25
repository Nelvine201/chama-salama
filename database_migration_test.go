package main

import (
	"database/sql"
	"os"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigrateDatabasePreservesDataAndIsIdempotent(t *testing.T) {
	db, err := sql.Open(
		"sqlite",
		"file:migration_safety_test?mode=memory&cache=shared&_pragma=foreign_keys=on",
	)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	legacySchema := []string{
		`CREATE TABLE members (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			phone TEXT,
			email TEXT,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'member',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE chamas (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			created_by INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE chama_members (
			id INTEGER PRIMARY KEY,
			chama_id INTEGER NOT NULL,
			member_id INTEGER NOT NULL,
			role TEXT NOT NULL DEFAULT 'member',
			joined_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE contributions (
			id INTEGER PRIMARY KEY,
			member_id INTEGER NOT NULL,
			amount REAL NOT NULL,
			paid_on DATE NOT NULL,
			source TEXT NOT NULL DEFAULT 'manual',
			status TEXT NOT NULL DEFAULT 'pending',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE notifications (
			id INTEGER PRIMARY KEY,
			member_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			message TEXT NOT NULL,
			type TEXT NOT NULL DEFAULT 'system',
			read_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, statement := range legacySchema {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("create legacy schema: %v", err)
		}
	}

	seedData := []string{
		`INSERT INTO members (id, name, password_hash) VALUES (1, 'Existing member', 'hash')`,
		`INSERT INTO chamas (id, name, created_by) VALUES (7, 'Existing chama', 1)`,
		`INSERT INTO chama_members (id, chama_id, member_id, role) VALUES (11, 7, 1, 'admin')`,
		`INSERT INTO contributions (id, member_id, amount, paid_on, status) VALUES (13, 1, 250, '2026-01-01', 'paid')`,
		`INSERT INTO notifications (id, member_id, title, message) VALUES (17, 1, 'Existing notification', 'Keep this record')`,
	}
	for _, statement := range seedData {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("seed legacy data: %v", err)
		}
	}

	schema, err := os.ReadFile("schema.sql")
	if err != nil {
		t.Fatal(err)
	}

	for run := 1; run <= 2; run++ {
		if err := migrateDatabase(db, string(schema)); err != nil {
			t.Fatalf("migration run %d: %v", run, err)
		}
	}

	var memberName string
	if err := db.QueryRow("SELECT name FROM members WHERE id = 1").Scan(&memberName); err != nil {
		t.Fatal(err)
	}
	if memberName != "Existing member" {
		t.Fatalf("member data changed: got %q", memberName)
	}

	var chamaName string
	if err := db.QueryRow("SELECT name FROM chamas WHERE id = 7").Scan(&chamaName); err != nil {
		t.Fatal(err)
	}
	if chamaName != "Existing chama" {
		t.Fatalf("chama data changed: got %q", chamaName)
	}

	var amount, chamaID float64
	if err := db.QueryRow("SELECT amount, chama_id FROM contributions WHERE id = 13").Scan(&amount, &chamaID); err != nil {
		t.Fatal(err)
	}
	if amount != 250 || chamaID != 7 {
		t.Fatalf("contribution data changed: amount=%v chama_id=%v", amount, chamaID)
	}

	var notificationTitle string
	var referenceID sql.NullInt64
	if err := db.QueryRow(
		"SELECT title, reference_id FROM notifications WHERE id = 17",
	).Scan(&notificationTitle, &referenceID); err != nil {
		t.Fatal(err)
	}
	if notificationTitle != "Existing notification" || referenceID.Valid {
		t.Fatalf("notification data changed: title=%q reference_id=%v", notificationTitle, referenceID)
	}

	rows, err := db.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		var table string
		var rowID, parentID int64
		var constraintID int
		if err := rows.Scan(&table, &rowID, &parentID, &constraintID); err != nil {
			t.Fatal(err)
		}
		t.Fatalf("foreign key violation after migration: table=%s row=%d parent=%d constraint=%d", table, rowID, parentID, constraintID)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}
