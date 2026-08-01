package main

import (
	"database/sql"
	"golang.org/x/crypto/bcrypt"
	"fmt"
)
type Member struct {
	
	ID  int64
	Name string
	Phone string
	Email string
	Role string
}
func CreateMember(db *sql.DB, name, phone, email, password, role string) (int64, error) {
	if err := validateMemberInput(name, phone, email, password); err != nil {
		return 0, err
	}

	if phone != "" {
		exists, err := phoneExists(db, phone)
		if err != nil {
			return 0, err
		}
		if exists {
			return 0, fmt.Errorf("phone number already registered")
		}
	}

	if email != "" {
		exists, err := emailExists(db, email)
		if err != nil {
			return 0, err
		}
		if exists {
			return 0, fmt.Errorf("email already registered")
		}
	}

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
func validateMemberInput(name, phone, email, password string) error {
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if phone == "" && email == "" {
		return fmt.Errorf("phone or email is required")
	}
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	return nil
}
func phoneExists(db *sql.DB, phone string) (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM members WHERE phone = ?", phone).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
func emailExists(db *sql.DB, email string) (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM members WHERE email = ?", email).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
func getMemberByIdentifier(db *sql.DB, identifier string) (*Member, string, error) {
	var m Member
	var passwordHash string

	err := db.QueryRow(
		"SELECT id, name, phone, email, role, password_hash FROM members WHERE phone = ? OR email = ?",
		identifier, identifier,
	).Scan(&m.ID, &m.Name, &m.Phone, &m.Email, &m.Role, &passwordHash)

	if err != nil {
		return nil, "", err
	}

	return &m, passwordHash, nil
}
func CheckLogin(db *sql.DB, identifier, password string) (*Member, error) {
	member, passwordHash, err := getMemberByIdentifier(db, identifier)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password))
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	return member, nil
}
