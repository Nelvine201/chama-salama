package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

func main() {
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

	id, err := CreateMember(db, "Jane Wanjiru", "0712345678", "jane@example.com", "mypassword123", "member")
	if err != nil {
		log.Fatal("Failed to create member:", err)
	}
	fmt.Println("Created member with ID:", id)
}
