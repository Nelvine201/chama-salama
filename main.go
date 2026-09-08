package main

import (
	"database/sql"
	"fmt"
	"log"
	_"modernc.org/sqlite"
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

	
	startServer(db)
	
}


