CREATE TABLE IF NOT EXISTS members (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    phone TEXT,
    email TEXT,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'member',
    national_id TEXT,
    location TEXT,
    next_of_kin TEXT,
    terms_version TEXT,
    terms_accepted_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS contributions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    member_id INTEGER NOT NULL,
    amount REAL NOT NULL,
    paid_on DATE NOT NULL,
    source TEXT NOT NULL DEFAULT 'manual',
    status TEXT NOT NULL DEFAULT 'pending',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (member_id) REFERENCES members(id)
);
CREATE TABLE IF NOT EXISTS sync_queue (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    contribution_id INTEGER NOT NULL,
    sync_status TEXT NOT NULL DEFAULT 'pending',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (contribution_id) REFERENCES contributions(id)
);
CREATE TABLE IF NOT EXISTS group_settings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chama_id INTEGER NOT NULL,
    contribution_amount REAL NOT NULL,
    frequency TEXT NOT NULL,
    payout_order TEXT,
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
    treasurer_account_name TEXT,
    payout_position INTEGER DEFAULT 0,
    next_payout_date TEXT,
    FOREIGN KEY (chama_id) REFERENCES chamas(id)
);
CREATE TABLE IF NOT EXISTS withdrawals (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    requested_by INTEGER NOT NULL,
    amount REAL NOT NULL,
    reason TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (requested_by) REFERENCES members(id)
);
CREATE TABLE IF NOT EXISTS withdrawal_approvals (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    withdrawal_id INTEGER NOT NULL,
    member_id INTEGER NOT NULL,
    approved_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (withdrawal_id) REFERENCES withdrawals(id),
    FOREIGN KEY (member_id) REFERENCES members(id)
);
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    member_id INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (member_id) REFERENCES members(id)
);
CREATE TABLE IF NOT EXISTS chamas (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    description TEXT,
    start_date TEXT,
    max_participants INTEGER,
    created_by INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (created_by) REFERENCES members(id)
);

CREATE TABLE IF NOT EXISTS chama_members (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chama_id INTEGER NOT NULL,
    member_id INTEGER NOT NULL,
    role TEXT NOT NULL DEFAULT 'member',
    joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (chama_id) REFERENCES chamas(id),
    FOREIGN KEY (member_id) REFERENCES members(id)
);
CREATE TABLE IF NOT EXISTS cycles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chama_id INTEGER NOT NULL,
    cycle_number INTEGER NOT NULL,
    due_date TEXT NOT NULL,
    recipient_member_id INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (chama_id) REFERENCES chamas(id)
);