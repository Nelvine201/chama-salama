package multisig

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	_ "modernc.org/sqlite"
)

func setupMultisigTest(t *testing.T) (*sql.DB, *Service) {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	// Keep the in-memory database on one connection.
	db.SetMaxOpenConns(1)
	t.Cleanup(func() {
		db.Close()
	})

	schema := `
	CREATE TABLE withdrawals (
		id INTEGER PRIMARY KEY,
		requested_by INTEGER NOT NULL,
		chama_id INTEGER NOT NULL,
		amount REAL NOT NULL DEFAULT 100,
		status TEXT NOT NULL
	);

	CREATE TABLE chama_members (
		chama_id INTEGER NOT NULL,
		member_id INTEGER NOT NULL,
		role TEXT NOT NULL,
		status TEXT NOT NULL
	);

	CREATE TABLE withdrawal_approvals (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		withdrawal_id INTEGER NOT NULL,
		member_id INTEGER NOT NULL,
		approver_role TEXT NOT NULL,
		status TEXT NOT NULL,
		signed_at DATETIME,
		approved_at DATETIME
	);

	INSERT INTO withdrawals (id, requested_by, chama_id, amount, status)
	VALUES (1, 10, 100, 500, 'pending');

	INSERT INTO chama_members (chama_id, member_id, role, status)
	VALUES
		(100, 20, 'admin', 'active'),
		(100, 30, 'treasurer', 'active'),
		(100, 40, 'member', 'active');
	`

	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}

	return db, NewService(db)
}

func withdrawalStatus(t *testing.T, db *sql.DB, id int64) string {
	t.Helper()

	var status string
	if err := db.QueryRow(
		"SELECT status FROM withdrawals WHERE id = ?", id,
	).Scan(&status); err != nil {
		t.Fatal(err)
	}

	return status
}

func TestRecordApprovalRequiresBothSignatures(t *testing.T) {
	db, service := setupMultisigTest(t)
	ctx := context.Background()

	// First signature: admin.
	if err := service.RecordApproval(ctx, 1, 20, "admin"); err != nil {
		t.Fatalf("admin approval failed: %v", err)
	}

	if got := withdrawalStatus(t, db, 1); got != "pending" {
		t.Fatalf("after admin signature, status = %q; want pending", got)
	}

	// The same approver cannot sign twice.
	err := service.RecordApproval(ctx, 1, 20, "ADMIN")
	if !errors.Is(err, ErrDuplicateApproval) {
		t.Fatalf("duplicate approval error = %v; want ErrDuplicateApproval", err)
	}

	// Second signature: treasurer.
	if err := service.RecordApproval(ctx, 1, 30, "treasurer"); err != nil {
		t.Fatalf("treasurer approval failed: %v", err)
	}

	if got := withdrawalStatus(t, db, 1); got != "DISBURSED" {
		t.Fatalf("after both signatures, status = %q; want DISBURSED", got)
	}

	var count int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM withdrawal_approvals WHERE withdrawal_id = 1 AND status = 'SIGNED'",
	).Scan(&count); err != nil {
		t.Fatal(err)
	}

	if count != 2 {
		t.Fatalf("signed approvals = %d; want 2", count)
	}
}

func TestRecordApprovalRejectsInvalidApprovers(t *testing.T) {
	_, service := setupMultisigTest(t)
	ctx := context.Background()

	tests := []struct {
		name    string
		userID  int64
		role    string
		wantErr error
	}{
		{
			name:    "requester cannot approve own withdrawal",
			userID:  10,
			role:    "ADMIN",
			wantErr: ErrSelfApproval,
		},
		{
			name:    "member cannot sign as admin",
			userID:  40,
			role:    "ADMIN",
			wantErr: ErrRoleMismatch,
		},
		{
			name:    "admin cannot sign as treasurer",
			userID:  20,
			role:    "TREASURER",
			wantErr: ErrRoleMismatch,
		},
		{
			name:    "invalid role",
			userID:  20,
			role:    "MEMBER",
			wantErr: ErrInvalidRole,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.RecordApproval(ctx, 1, tt.userID, tt.role)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v; want %v", err, tt.wantErr)
			}
		})
	}
}

func TestRecordApprovalRejectsMissingWithdrawal(t *testing.T) {
	_, service := setupMultisigTest(t)

	err := service.RecordApproval(context.Background(), 999, 20, "ADMIN")
	if !errors.Is(err, ErrWithdrawalMissing) {
		t.Fatalf("error = %v; want ErrWithdrawalMissing", err)
	}
}
