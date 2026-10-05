-- Multi-signature withdrawal approvals migration.
-- Non-destructive: preserves the existing approval rows and keeps the
-- legacy column names as compatibility fields while adding the new
-- role-based approval fields.
--
-- This migration targets the existing Chama Salama SQLite/Turso schema.
-- The application currently calls the withdrawal table "withdrawals";
-- withdrawal_request_id references withdrawals(id).

BEGIN;

ALTER TABLE withdrawal_approvals RENAME TO withdrawal_approvals_legacy;

CREATE TABLE withdrawal_approvals (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    withdrawal_request_id INTEGER NOT NULL,
    approver_id INTEGER NOT NULL,
    approver_role TEXT NOT NULL DEFAULT 'MEMBER',
    status TEXT NOT NULL DEFAULT 'SIGNED',
    signed_at DATETIME DEFAULT CURRENT_TIMESTAMP,

    -- Legacy compatibility fields. Existing approval code can continue to
    -- read/write these while the new multisig service uses the fields above.
    withdrawal_id INTEGER,
    member_id INTEGER,
    approved_at DATETIME,

    FOREIGN KEY (withdrawal_request_id) REFERENCES withdrawals(id),
    FOREIGN KEY (approver_id) REFERENCES members(id)
);

INSERT INTO withdrawal_approvals (
    withdrawal_request_id,
    approver_id,
    approver_role,
    status,
    signed_at,
    withdrawal_id,
    member_id,
    approved_at
)
SELECT
    legacy.withdrawal_id,
    legacy.member_id,
    UPPER(COALESCE(cm.role, m.role, 'MEMBER')),
    'SIGNED',
    legacy.approved_at,
    legacy.withdrawal_id,
    legacy.member_id,
    legacy.approved_at
FROM withdrawal_approvals_legacy AS legacy
LEFT JOIN withdrawals AS w
    ON w.id = legacy.withdrawal_id
LEFT JOIN chama_members AS cm
    ON cm.chama_id = w.chama_id
   AND cm.member_id = legacy.member_id
LEFT JOIN members AS m
    ON m.id = legacy.member_id;

CREATE UNIQUE INDEX IF NOT EXISTS idx_withdrawal_approvals_approver
ON withdrawal_approvals (withdrawal_request_id, approver_id);

CREATE INDEX IF NOT EXISTS idx_withdrawal_approvals_request
ON withdrawal_approvals (withdrawal_request_id);

COMMIT;
