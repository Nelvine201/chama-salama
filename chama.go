package main

import (
	"database/sql"
	"fmt"
	"time"
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

func GetPendingInvitations(db *sql.DB, memberID int64) (string, []PendingInvitation, error) {
	var memberName string
	db.QueryRow("SELECT name FROM members WHERE id = ?", memberID).Scan(&memberName)

	rows, err := db.Query(
		`SELECT c.id, c.name, cm.role
		 FROM chama_members cm
		 JOIN chamas c ON cm.chama_id = c.id
		 WHERE cm.member_id = ? AND cm.status = 'pending'`,
		memberID,
	)
	if err != nil {
		return memberName, nil, err
	}
	defer rows.Close()

	var invitations []PendingInvitation
	for rows.Next() {
		var i PendingInvitation
		if err := rows.Scan(&i.ChamaID, &i.ChamaName, &i.Role); err != nil {
			return memberName, nil, err
		}
		invitations = append(invitations, i)
	}
	return memberName, invitations, nil
}

func GetMemberRoleInChama(db *sql.DB, chamaID, memberID int64) (string, error) {
	var role string
	err := db.QueryRow(
		"SELECT role FROM chama_members WHERE chama_id = ? AND member_id = ? AND status = 'active'",
		chamaID, memberID,
	).Scan(&role)
	if err != nil {
		return "", err
	}
	return role, nil
}

type ChamaCard struct {
	ChamaID            int64
	ChamaName          string
	Role               string
	MemberCount        int
	QueuePosition      int
	NextRecipientName  string
	NextPayoutDate     string
	ContributionPaid   bool
	ContributionAmount float64
	PendingInviteCount int
}

func GetMemberChamaCards(db *sql.DB, memberID int64) ([]ChamaCard, error) {
	memberships, err := GetMemberChamas(db, memberID)
	if err != nil {
		return nil, err
	}

	var cards []ChamaCard
	for _, m := range memberships {
		var c ChamaCard
		c.ChamaID = m.ChamaID
		c.ChamaName = m.ChamaName
		c.Role = m.Role

		db.QueryRow("SELECT COUNT(*) FROM chama_members WHERE chama_id = ? AND status = 'active'", m.ChamaID).Scan(&c.MemberCount)

		var amount float64
		var position int
		var nextDate sql.NullString
		db.QueryRow("SELECT contribution_amount, payout_position, next_payout_date FROM group_settings WHERE chama_id = ? LIMIT 1", m.ChamaID).
			Scan(&amount, &position, &nextDate)
		c.ContributionAmount = amount
		c.NextPayoutDate = nextDate.String

		db.QueryRow(
			`SELECT name FROM members WHERE id = (
				SELECT member_id FROM chama_members WHERE chama_id = ? AND status = 'active' ORDER BY joined_at LIMIT 1 OFFSET ?
			)`, m.ChamaID, position,
		).Scan(&c.NextRecipientName)

		db.QueryRow(
			`SELECT COUNT(*) + 1 FROM chama_members WHERE chama_id = ? AND status = 'active' AND joined_at < (
				SELECT joined_at FROM chama_members WHERE chama_id = ? AND member_id = ?
			)`, m.ChamaID, m.ChamaID, memberID,
		).Scan(&c.QueuePosition)

		var paidCount int
		db.QueryRow("SELECT COUNT(*) FROM contributions WHERE chama_id = ? AND member_id = ? AND status = 'synced'", m.ChamaID, memberID).Scan(&paidCount)
		c.ContributionPaid = paidCount > 0

		if m.Role == "admin" {
			db.QueryRow("SELECT COUNT(*) FROM chama_members WHERE chama_id = ? AND status = 'pending'", m.ChamaID).Scan(&c.PendingInviteCount)
		}

		cards = append(cards, c)
	}
	return cards, nil
}

func GetGroupBalanceForChama(db *sql.DB, chamaID int64) (float64, error) {
	var total float64
	err := db.QueryRow("SELECT COALESCE(SUM(amount), 0) FROM contributions WHERE chama_id = ? AND status = 'synced'", chamaID).Scan(&total)
	return total, err
}

func GetGroupSettingsForChama(db *sql.DB, chamaID int64) (float64, string, error) {
	var amount float64
	var frequency string
	err := db.QueryRow("SELECT contribution_amount, frequency FROM group_settings WHERE chama_id = ? LIMIT 1", chamaID).Scan(&amount, &frequency)
	return amount, frequency, err
}

func GetRecentContributionsForChama(db *sql.DB, chamaID int64, limit int) ([]ContributionWithMember, error) {
	rows, err := db.Query(
		`SELECT c.id, m.name, c.amount, c.paid_on, c.status
		 FROM contributions c JOIN members m ON c.member_id = m.id
		 WHERE c.chama_id = ? ORDER BY c.id DESC LIMIT ?`, chamaID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []ContributionWithMember
	for rows.Next() {
		var c ContributionWithMember
		if err := rows.Scan(&c.ID, &c.MemberName, &c.Amount, &c.PaidOn, &c.Status); err != nil {
			return nil, err
		}
		results = append(results, c)
	}
	return results, nil
}

func GetPayoutQueueInfo(db *sql.DB, chamaID, memberID int64) (recipientName string, position int, queuePos int) {
	db.QueryRow("SELECT payout_position FROM group_settings WHERE chama_id = ? LIMIT 1", chamaID).Scan(&position)
	db.QueryRow(
		`SELECT name FROM members WHERE id = (
			SELECT member_id FROM chama_members WHERE chama_id = ? AND status = 'active' ORDER BY joined_at LIMIT 1 OFFSET ?
		)`, chamaID, position,
	).Scan(&recipientName)
	db.QueryRow(
		`SELECT COUNT(*) + 1 FROM chama_members WHERE chama_id = ? AND status = 'active' AND joined_at < (
			SELECT joined_at FROM chama_members WHERE chama_id = ? AND member_id = ?
		)`, chamaID, chamaID, memberID,
	).Scan(&queuePos)
	return
}

type CycleSummary struct {
	CycleNumber       int
	DueDate           string
	TargetAmount      float64
	TotalExpectedPool float64
	TotalCollected    float64
	RecipientName     string
	MembersPaidCount  int
	TotalActive       int
	PercentPaid       int
}

type UserCycleSummary struct {
	HasPaid            bool
	QueuePosition      int
	TotalActive        int
	ExpectedLumpSum    float64
	ExpectedPayoutDate string
}

func GetOrCreateCurrentCycle(db *sql.DB, chamaID int64) (int64, int, string, error) {
	var cycleID int64
	var cycleNumber int
	var dueDate string
	err := db.QueryRow(
		"SELECT id, cycle_number, due_date FROM cycles WHERE chama_id = ? ORDER BY cycle_number DESC LIMIT 1",
		chamaID,
	).Scan(&cycleID, &cycleNumber, &dueDate)

	if err == sql.ErrNoRows {
		dueDate = time.Now().AddDate(0, 0, 7).Format("2006-01-02")
		result, insertErr := db.Exec(
			"INSERT INTO cycles (chama_id, cycle_number, due_date) VALUES (?, 1, ?)",
			chamaID, dueDate,
		)
		if insertErr != nil {
			return 0, 0, "", insertErr
		}
		cycleID, _ = result.LastInsertId()
		return cycleID, 1, dueDate, nil
	}
	if err != nil {
		return 0, 0, "", err
	}
	return cycleID, cycleNumber, dueDate, nil
}

func GetCycleSummary(db *sql.DB, chamaID int64) (*CycleSummary, error) {
	cycleID, cycleNumber, dueDate, err := GetOrCreateCurrentCycle(db, chamaID)
	if err != nil {
		return nil, err
	}

	var totalActive int
	db.QueryRow("SELECT COUNT(*) FROM chama_members WHERE chama_id = ? AND status = 'active'", chamaID).Scan(&totalActive)

	var targetAmount float64
	db.QueryRow("SELECT contribution_amount FROM group_settings WHERE chama_id = ? LIMIT 1", chamaID).Scan(&targetAmount)

	var totalCollected float64
	db.QueryRow(
		"SELECT COALESCE(SUM(amount), 0) FROM contributions WHERE chama_id = ? AND cycle_id = ? AND status = 'synced'",
		chamaID, cycleID,
	).Scan(&totalCollected)

	var membersPaid int
	db.QueryRow(
		"SELECT COUNT(DISTINCT member_id) FROM contributions WHERE chama_id = ? AND cycle_id = ? AND status = 'synced'",
		chamaID, cycleID,
	).Scan(&membersPaid)

	var recipientName sql.NullString
	db.QueryRow(
		`SELECT m.name FROM cycles c JOIN members m ON c.recipient_member_id = m.id WHERE c.id = ?`,
		cycleID,
	).Scan(&recipientName)

	percentPaid := 0
	if totalActive > 0 {
		percentPaid = (membersPaid * 100) / totalActive
	}

	return &CycleSummary{
		CycleNumber:       cycleNumber,
		DueDate:           dueDate,
		TargetAmount:      targetAmount,
		TotalExpectedPool: targetAmount * float64(totalActive),
		TotalCollected:    totalCollected,
		RecipientName:     recipientName.String,
		MembersPaidCount:  membersPaid,
		TotalActive:       totalActive,
		PercentPaid:       percentPaid,
	}, nil
}

func GetUserCycleSummary(db *sql.DB, chamaID, memberID int64) (*UserCycleSummary, error) {
	cycleID, _, _, err := GetOrCreateCurrentCycle(db, chamaID)
	if err != nil {
		return nil, err
	}

	var paidCount int
	db.QueryRow(
		"SELECT COUNT(*) FROM contributions WHERE chama_id = ? AND cycle_id = ? AND member_id = ? AND status = 'synced'",
		chamaID, cycleID, memberID,
	).Scan(&paidCount)

	_, position, totalActive := GetPayoutQueueInfo(db, chamaID, memberID)

	var amount float64
	db.QueryRow("SELECT contribution_amount FROM group_settings WHERE chama_id = ? LIMIT 1", chamaID).Scan(&amount)

	var count int
	db.QueryRow("SELECT COUNT(*) FROM chama_members WHERE chama_id = ? AND status = 'active'", chamaID).Scan(&count)

	return &UserCycleSummary{
		HasPaid:            paidCount > 0,
		QueuePosition:      position,
		TotalActive:        count,
		ExpectedLumpSum:    amount * float64(count),
		ExpectedPayoutDate: time.Now().AddDate(0, 0, 7*totalActive).Format("2006-01-02"),
	}, nil
}
