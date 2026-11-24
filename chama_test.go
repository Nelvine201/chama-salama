package main

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func setupChamaDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)

	statements := []string{
		`CREATE TABLE members (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL
		)`,
		`CREATE TABLE chamas (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT,
			start_date TEXT,
			max_participants INTEGER,
			created_by INTEGER NOT NULL,
			lifecycle_status TEXT NOT NULL DEFAULT 'forming'
		)`,
		`CREATE TABLE chama_members (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			chama_id INTEGER NOT NULL,
			member_id INTEGER NOT NULL,
			role TEXT NOT NULL DEFAULT 'member',
			status TEXT DEFAULT 'active'
		)`,
		`CREATE TABLE group_settings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			chama_id INTEGER NOT NULL,
			contribution_amount REAL NOT NULL,
			frequency TEXT NOT NULL,
			number_of_rounds INTEGER NOT NULL,
			payout_method TEXT NOT NULL,
			grace_period_days INTEGER NOT NULL DEFAULT 0,
			late_penalty_type TEXT NOT NULL DEFAULT 'none',
			late_penalty_amount REAL NOT NULL DEFAULT 0,
			welfare_reserve_amount REAL NOT NULL DEFAULT 0,
			approval_threshold INTEGER NOT NULL DEFAULT 1,
			payout_destination_type TEXT,
			paybill_number TEXT,
			till_number TEXT,
			treasurer_phone TEXT,
			treasurer_account_name TEXT
		)`,
		`INSERT INTO members (id, name) VALUES (1, 'Test Admin')`,
	}

	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			db.Close()
			t.Fatalf("setup database: %v", err)
		}
	}

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})

	return db
}

func TestCreateChama(t *testing.T) {
	db := setupChamaDB(t)

	chamaID, err := CreateChama(db, "Test Chama", 1)
	if err != nil {
		t.Fatalf("CreateChama returned an error: %v", err)
	}
	if chamaID <= 0 {
		t.Fatalf("expected a positive Chama ID, got %d", chamaID)
	}

	var name, lifecycleStatus string
	var createdBy int64
	err = db.QueryRow(
		`SELECT name, created_by, lifecycle_status FROM chamas WHERE id = ?`,
		chamaID,
	).Scan(&name, &createdBy, &lifecycleStatus)
	if err != nil {
		t.Fatalf("query created Chama: %v", err)
	}
	if name != "Test Chama" || createdBy != 1 || lifecycleStatus != "forming" {
		t.Fatalf("unexpected Chama values: name=%q createdBy=%d lifecycleStatus=%q",
			name, createdBy, lifecycleStatus)
	}

	var role string
	err = db.QueryRow(
		"SELECT role FROM chama_members WHERE chama_id = ? AND member_id = ?",
		chamaID, 1,
	).Scan(&role)
	if err != nil {
		t.Fatalf("query creator membership: %v", err)
	}
	if role != "admin" {
		t.Fatalf("expected creator role admin, got %q", role)
	}
}

func TestCreateChamaRejectsEmptyName(t *testing.T) {
	db := setupChamaDB(t)

	if _, err := CreateChama(db, "", 1); err == nil {
		t.Fatal("expected an error for an empty Chama name")
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM chamas").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected no Chama to be created, found %d", count)
	}
}

func TestCreateChamaRollsBackWhenCreatorMembershipFails(t *testing.T) {
	db := setupChamaDB(t)

	_, err := db.Exec(`
		CREATE TRIGGER reject_creator_membership
		BEFORE INSERT ON chama_members
		BEGIN
			SELECT RAISE(ABORT, 'membership insert rejected');
		END
	`)
	if err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}

	if _, err := CreateChama(db, "Orphan Prevention", 1); err == nil {
		t.Fatal("expected creator membership insertion to fail")
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM chamas").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected Chama insert to roll back, found %d Chama(s)", count)
	}
}

func TestCreateChamaWithSettings(t *testing.T) {
	db := setupChamaDB(t)

	config := ChamaConfig{
		Name:               "Configured Chama",
		Description:        "A test Chama",
		StartDate:          "2026-11-24",
		MaxParticipants:    5,
		ContributionAmount: 100,
		Frequency:          "monthly",
		NumberOfRounds:     5,
		PayoutMethod:       "fixed_rotation",
		GracePeriodDays:    2,
		LatePenaltyType:    "none",
		ApprovalThreshold:  1,
	}

	chamaID, err := CreateChamaWithSettings(db, config, 1)
	if err != nil {
		t.Fatalf("CreateChamaWithSettings returned an error: %v", err)
	}

	var lifecycleStatus string
	if err := db.QueryRow(
		"SELECT lifecycle_status FROM chamas WHERE id = ?", chamaID,
	).Scan(&lifecycleStatus); err != nil {
		t.Fatalf("query Chama lifecycle: %v", err)
	}
	if lifecycleStatus != "forming" {
		t.Fatalf("expected lifecycle status forming, got %q", lifecycleStatus)
	}

	var amount float64
	var frequency, payoutMethod string
	if err := db.QueryRow(
		`SELECT contribution_amount, frequency, payout_method
		 FROM group_settings WHERE chama_id = ?`,
		chamaID,
	).Scan(&amount, &frequency, &payoutMethod); err != nil {
		t.Fatalf("query group settings: %v", err)
	}
	if amount != 100 || frequency != "monthly" || payoutMethod != "fixed_rotation" {
		t.Fatalf("unexpected settings: amount=%.2f frequency=%q payoutMethod=%q",
			amount, frequency, payoutMethod)
	}

	var role, status string
	if err := db.QueryRow(
		"SELECT role, status FROM chama_members WHERE chama_id = ? AND member_id = ?",
		chamaID, 1,
	).Scan(&role, &status); err != nil {
		t.Fatalf("query creator membership: %v", err)
	}
	if role != "admin" || status != "active" {
		t.Fatalf("expected active admin membership, got role=%q status=%q", role, status)
	}
}
