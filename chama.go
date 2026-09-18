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
		 WHERE cm.member_id = ? AND cm.status = 'active'`,
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
		 WHERE cm.chama_id = ? AND cm.status = 'active'`,
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
func InviteMember(db *sql.DB, chamaID int64, identifier, role string) error {
	member, _, err := getMemberByIdentifier(db, identifier)
	if err != nil {
		return fmt.Errorf("no registered member found with that phone or email")
	}

	var existing int
	db.QueryRow(
		"SELECT COUNT(*) FROM chama_members WHERE chama_id = ? AND member_id = ?",
		chamaID, member.ID,
	).Scan(&existing)
	if existing > 0 {
		return fmt.Errorf("this member already has a membership or pending invite for this chama")
	}

	_, err = db.Exec(
		"INSERT INTO chama_members (chama_id, member_id, role, status) VALUES (?, ?, ?, ?)",
		chamaID, member.ID, role, "pending",
	)
	return err
}
func RespondToInvitation(db *sql.DB, chamaID, memberID int64, accept bool) error {
	status := "declined"
	if accept {
		status = "active"
	}

	result, err := db.Exec(
		"UPDATE chama_members SET status = ? WHERE chama_id = ? AND member_id = ? AND status = 'pending'",
		status, chamaID, memberID,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("no pending invitation found")
	}
	return nil
}
type PendingInvitation struct {
	ChamaID   int64
	ChamaName string
	Role      string
}

func GetPendingInvitations(db *sql.DB, memberID int64) ([]PendingInvitation, error) {
	rows, err := db.Query(
		`SELECT c.id, c.name, cm.role
		 FROM chama_members cm
		 JOIN chamas c ON cm.chama_id = c.id
		 WHERE cm.member_id = ? AND cm.status = 'pending'`,
		memberID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invitations []PendingInvitation
	for rows.Next() {
		var i PendingInvitation
		if err := rows.Scan(&i.ChamaID, &i.ChamaName, &i.Role); err != nil {
			return nil, err
		}
		invitations = append(invitations, i)
	}
	return invitations, nil
}

