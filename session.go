package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
)

func generateSessionToken() (string, error) {
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
func createSession(db *sql.DB, memberID int64) (string, error) {
	token, err := generateSessionToken()
	if err != nil {
		return "", err
	}

	_, err = db.Exec(
		"INSERT INTO sessions (id, member_id) VALUES (?, ?)",
		token, memberID,
	)
	if err != nil {
		return "", err
	}

	return token, nil
}

