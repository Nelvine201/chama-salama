package main

import (
	"database/sql"
	"golang.org/x/crypto/bcrypt"
)
type Member struct {
	
	ID  int64
	Name string
	Phone string
	Email string
	Role string
}
func CreateMember(db *sql.DB, name, phone, email, password, role string) (int64, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}

	result, err := db.Exec(
		"INSERT INTO members (name, phone, email, password_hash, role) VALUES (?, ?, ?, ?, ?)",
		name, phone, email, string(hashedPassword), role,
	)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return id, nil
}

