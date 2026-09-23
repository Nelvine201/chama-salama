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
