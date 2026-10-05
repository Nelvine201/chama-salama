package multisig

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
 )

var (
	ErrInvalidRole = errors.New("approver role must be ADMIN or TREASURER")
	ErrDuplicateApproval = errors.New("approver has already signed this withdrawal")
	ErrWithdrawalMissing = errors.New("withdrawal request not found")
 )

type Store interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

type Service struct {
	db Store
}

func NewService(db Store) *Service {
	return &Service{db: db}
}

// RecordApproval records one role-based signature. A withdrawal is marked
// DISBURSED only after both an ADMIN and a TREASURER have signed it.
func (s *Service) RecordApproval(ctx context.Context, withdrawalID, userID int64, role string) error {
	role = strings.ToUpper(strings.TrimSpace(role))
	if role != "ADMIN" && role != "TREASURER" {
		return ErrInvalidRole
	}
	if withdrawalID <= 0 || userID <= 0 {
		return fmt.Errorf("withdrawalID and userID must be positive")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil { return err }
	defer tx.Rollback()
	var exists int
	err = tx.QueryRowContext(ctx, "SELECT 1 FROM withdrawals WHERE id = ? LIMIT 1", withdrawalID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) { return ErrWithdrawalMissing }
	if err != nil { return err }
	var existing int
	err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM withdrawal_approvals WHERE withdrawal_request_id = ? AND approver_id = ?", withdrawalID, userID).Scan(&existing)
	if err != nil { return err }
	if existing > 0 { return ErrDuplicateApproval }
	_, err = tx.ExecContext(ctx, "INSERT INTO withdrawal_approvals (withdrawal_request_id, approver_id, approver_role, status, signed_at) VALUES (?, ?, ?, 'SIGNED', CURRENT_TIMESTAMP)", withdrawalID, userID, role)
	if err != nil { return err }
	var adminSigned, treasurerSigned int
	err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM withdrawal_approvals WHERE withdrawal_request_id = ? AND approver_role = 'ADMIN' AND status = 'SIGNED'), EXISTS(SELECT 1 FROM withdrawal_approvals WHERE withdrawal_request_id = ? AND approver_role = 'TREASURER' AND status = 'SIGNED')", withdrawalID, withdrawalID).Scan(&adminSigned, &treasurerSigned)
	if err != nil { return err }
	if adminSigned == 1 && treasurerSigned == 1 {
		_, err = tx.ExecContext(ctx, "UPDATE withdrawals SET status = 'DISBURSED' WHERE id = ? AND status <> 'DISBURSED'", withdrawalID)
		if err != nil { return err }
	}
	return tx.Commit()
}
