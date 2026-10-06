package multisig

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrInvalidRole       = errors.New("approver role must be ADMIN or TREASURER")
	ErrDuplicateApproval = errors.New("approver has already signed this withdrawal")
	ErrWithdrawalMissing = errors.New("withdrawal request not found")
	ErrSelfApproval      = errors.New("requester cannot approve their own withdrawal")
	ErrNotMember         = errors.New("approver is not an active member of this Chama")
	ErrRoleMismatch      = errors.New("approver role does not match their active Chama role")
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
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var requestedBy, chamaID int64
	err = tx.QueryRowContext(ctx, "SELECT requested_by, chama_id FROM withdrawals WHERE id = ? AND status = 'pending' LIMIT 1", withdrawalID).Scan(&requestedBy, &chamaID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrWithdrawalMissing
	}
	if err != nil {
		return err
	}
	if requestedBy == userID {
		return ErrSelfApproval
	}

	var memberRole string
	err = tx.QueryRowContext(ctx, "SELECT role FROM chama_members WHERE chama_id = ? AND member_id = ? AND status = 'active'", chamaID, userID).Scan(&memberRole)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotMember
	}
	if err != nil {
		return err
	}
	if strings.ToUpper(memberRole) != role {
		return ErrRoleMismatch
	}

	var existing int
	err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM withdrawal_approvals WHERE withdrawal_id = ? AND member_id = ?", withdrawalID, userID).Scan(&existing)
	if err != nil {
		return err
	}
	if existing > 0 {
		return ErrDuplicateApproval
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO withdrawal_approvals (withdrawal_id, member_id, approver_role, status, signed_at, approved_at) VALUES (?, ?, ?, 'SIGNED', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)", withdrawalID, userID, role, withdrawalID, userID)
	if err != nil {
		return err
	}
	var adminSigned, treasurerSigned int
	err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM withdrawal_approvals WHERE withdrawal_id = ? AND approver_role = 'ADMIN' AND status = 'SIGNED'), EXISTS(SELECT 1 FROM withdrawal_approvals WHERE withdrawal_id = ? AND approver_role = 'TREASURER' AND status = 'SIGNED')", withdrawalID, withdrawalID).Scan(&adminSigned, &treasurerSigned)
	if err != nil {
		return err
	}
	if adminSigned == 1 && treasurerSigned == 1 {
		_, err = tx.ExecContext(ctx, "UPDATE withdrawals SET status = 'DISBURSED' WHERE id = ? AND status <> 'DISBURSED'", withdrawalID)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
