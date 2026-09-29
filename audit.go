package main

import "database/sql"

func RecordAuditEvent(db *sql.DB, chamaID, memberID int64, event, details string) error {
	_, err := db.Exec(
		"INSERT INTO audit_events (chama_id, member_id, event, details) VALUES (?, ?, ?, ?)",
		chamaID, memberID, event, details,
	)
	return err
}
