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
	var db *sql.DB
	var err error
	tursoURL := os.Getenv("TURSO_DATABASE_URL")
	tursoToken := os.Getenv("TURSO_AUTH_TOKEN")

	if tursoURL != "" {
		dbURL := tursoURL + "?authToken=" + tursoToken
		db, err = sql.Open("libsql", dbURL)
		fmt.Println("Using Turso database")
	} else {
		db, err = sql.Open("sqlite", "file:chama.db?_pragma=foreign_keys=on")
		fmt.Println("Using local SQLite database")
	}
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	fmt.Println("Successfully connected to database!")

	schema, err := os.ReadFile("schema.sql")
	if err != nil {
		log.Fatal("Failed to read schema.sql:", err)
	}
	if err := migrateDatabase(db, string(schema)); err != nil {
		log.Fatal("Failed to migrate database:", err)
	}
	fmt.Println("Database schema and migrations ensured.")

	startServer(db)
}
