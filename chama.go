package main

import (
	"database/sql"
	"fmt"
)

type Chama struct {
	ID        int64
	Name      string
	CreatedBy int64
}

func CreateChama(db *sql.DB, name string, createdBy int64) (int64, error) {
	if name == "" {
		return 0, fmt.Errorf("chama name is required")
	}

	result, err := db.Exec("INSERT INTO chamas (name, created_by) VALUES (?, ?)", name, createdBy)
	if err != nil {
		return 0, err
	}

	chamaID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	_, err = db.Exec(
		"INSERT INTO chama_members (chama_id, member_id, role) VALUES (?, ?, ?)",
		chamaID, createdBy, "admin",
	)
	if err != nil {
		return 0, err
	}

	return chamaID, nil
}
type ChamaMembership struct {
	ChamaID   int64
	ChamaName string
	Role      string
}

func GetMemberChamas(db *sql.DB, memberID int64) ([]ChamaMembership, error) {
	rows, err := db.Query(
		`SELECT c.id, c.name, cm.role
		 FROM chama_members cm
		 JOIN chamas c ON cm.chama_id = c.id
		 WHERE cm.member_id = ?`,
		memberID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memberships []ChamaMembership
	for rows.Next() {
		var m ChamaMembership
		if err := rows.Scan(&m.ChamaID, &m.ChamaName, &m.Role); err != nil {
			return nil, err
		}
		memberships = append(memberships, m)
	}
	return memberships, nil
}

type RosterMember struct {
	MemberID int64
	Name     string
	Role     string
}

func GetChamaRoster(db *sql.DB, chamaID int64) ([]RosterMember, error) {
	rows, err := db.Query(
		`SELECT m.id, m.name, cm.role
		 FROM chama_members cm
		 JOIN members m ON cm.member_id = m.id
		 WHERE cm.chama_id = ?`,
		chamaID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roster []RosterMember
	for rows.Next() {
		var r RosterMember
		if err := rows.Scan(&r.MemberID, &r.Name, &r.Role); err != nil {
			return nil, err
		}
		roster = append(roster, r)
	}
	return roster, nil
}

