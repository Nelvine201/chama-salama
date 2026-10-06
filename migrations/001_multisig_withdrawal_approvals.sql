-- Multi-signature withdrawal approval columns.
-- The existing withdrawal_approvals table already uses withdrawal_id and member_id.
-- This migration only adds the fields needed for role-based signatures.

ALTER TABLE withdrawal_approvals ADD COLUMN approver_role TEXT DEFAULT 'MEMBER';
ALTER TABLE withdrawal_approvals ADD COLUMN status TEXT DEFAULT 'SIGNED';
ALTER TABLE withdrawal_approvals ADD COLUMN signed_at DATETIME;

-- Backfill roles for existing approval rows where the Chama membership role is known.
UPDATE withdrawal_approvals
SET approver_role = COALESCE(
    (
        SELECT UPPER(cm.role)
        FROM withdrawals w
        JOIN chama_members cm
          ON cm.chama_id = w.chama_id
         AND cm.member_id = withdrawal_approvals.member_id
        WHERE w.id = withdrawal_approvals.withdrawal_id
        LIMIT 1
    ),
    'MEMBER'
)
WHERE approver_role = 'MEMBER';

CREATE INDEX IF NOT EXISTS idx_withdrawal_approvals_withdrawal
ON withdrawal_approvals (withdrawal_id);

CREATE INDEX IF NOT EXISTS idx_withdrawal_approvals_role
ON withdrawal_approvals (withdrawal_id, approver_role, status);
