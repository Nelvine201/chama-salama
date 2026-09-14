package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

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

	db, err := sql.Open("sqlite", "chama.db")
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	fmt.Println("Successfully connected to chama.db!")

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
