package main

import "database/sql"

type Notification struct {
    ID        int64
    Title     string
    Message   string
    Type      string
    ReadAt    sql.NullString
    CreatedAt string
}

func GetNotifications(db *sql.DB, memberID int64) ([]Notification, error) {
    rows, err := db.Query(`SELECT id, title, message, type, read_at, created_at
        FROM notifications WHERE member_id = ? ORDER BY id DESC`, memberID)
    if err != nil { return nil, err }
    defer rows.Close()
    var result []Notification
    for rows.Next() {
        var n Notification
        if err := rows.Scan(&n.ID, &n.Title, &n.Message, &n.Type, &n.ReadAt, &n.CreatedAt); err != nil {
            return nil, err
        }
        result = append(result, n)
    }
    return result, rows.Err()
}

func CreateNotification(db *sql.DB, memberID int64, title, message, notificationType string) error {
	_, err := db.Exec("INSERT INTO notifications (member_id, title, message, type) VALUES (?, ?, ?, ?)", memberID, title, message, notificationType)
	return err
}

func CreateChamaNotification(db *sql.DB, chamaID int64, excludeMemberID int64, title, message, notificationType string) error {
	rows, err := db.Query("SELECT member_id FROM chama_members WHERE chama_id = ? AND status = 'active' AND member_id != ?", chamaID, excludeMemberID)
	if err != nil { return err }
	defer rows.Close()
	for rows.Next() {
		var memberID int64
		if err := rows.Scan(&memberID); err != nil { return err }
		if err := CreateNotification(db, memberID, title, message, notificationType); err != nil { return err }
	}
	return rows.Err()
}
