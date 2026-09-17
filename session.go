package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
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
func getMemberBySession(db *sql.DB, token string) (int64, error) {
	var memberID int64
	err := db.QueryRow(
		"SELECT member_id FROM sessions WHERE id = ?",
		token,
	).Scan(&memberID)
	if err != nil {
		return 0, err
	}
	return memberID, nil
}

func getLoggedInMemberID(r *http.Request, db *sql.DB) (int64, error) {
	cookie, err := r.Cookie("session_token")
	if err != nil {
		return 0, fmt.Errorf("not logged in")
	}

	memberID, err := getMemberBySession(db, cookie.Value)
	if err != nil {
		return 0, fmt.Errorf("invalid or expired session")
	}

	return memberID, nil
}
func deleteSession(db *sql.DB, token string) error {
	_, err := db.Exec("DELETE FROM sessions WHERE id = ?", token)
	return err
}
