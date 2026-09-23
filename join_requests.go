package main

import (
    "database/sql"
    "fmt"
)

type PublicChama struct {
    ID                 int64
    Name               string
    Description        string
    Status             string
    ContributionAmount float64
    Frequency          string
    ActiveMembers      int
    MaxParticipants    int
    NumberOfRounds     int
    PayoutMethod       string
    GracePeriodDays    int
    LatePenaltyType    string
    LatePenaltyAmount  float64
    WelfareReserve     float64
    ApprovalThreshold  int
}

type JoinRequest struct {
    ID          int64
    ChamaID     int64
    ChamaName   string
    MemberID    int64
    ApplicantName string
    Phone       string
    Note        string
    Status      string
    CreatedAt   string
}

func GetPublicChamas(db *sql.DB) ([]PublicChama, error) {
    rows, err := db.Query(`
        SELECT c.id, c.name, COALESCE(c.description, ''), 'Open',
               COALESCE(gs.contribution_amount, 0), COALESCE(gs.frequency, ''),
               COUNT(CASE WHEN cm.status = 'active' THEN 1 END),
               COALESCE(c.max_participants, 0), COALESCE(gs.number_of_rounds, 0),
               COALESCE(gs.payout_method, ''), COALESCE(gs.grace_period_days, 0),
               COALESCE(gs.late_penalty_type, 'none'), COALESCE(gs.late_penalty_amount, 0),
               COALESCE(gs.welfare_reserve_amount, 0), COALESCE(gs.approval_threshold, 1)
        FROM chamas c
        LEFT JOIN group_settings gs ON gs.chama_id = c.id
        LEFT JOIN chama_members cm ON cm.chama_id = c.id
        GROUP BY c.id, c.name, c.description, gs.contribution_amount, gs.frequency,
                 c.max_participants, gs.number_of_rounds, gs.payout_method,
                 gs.grace_period_days, gs.late_penalty_type, gs.late_penalty_amount,
                 gs.welfare_reserve_amount, gs.approval_threshold
        ORDER BY c.created_at DESC
    `)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var result []PublicChama
    for rows.Next() {
        var c PublicChama
        if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.Status,
            &c.ContributionAmount, &c.Frequency, &c.ActiveMembers, &c.MaxParticipants,
            &c.NumberOfRounds, &c.PayoutMethod, &c.GracePeriodDays, &c.LatePenaltyType,
            &c.LatePenaltyAmount, &c.WelfareReserve, &c.ApprovalThreshold); err != nil {
            return nil, err
        }
        if c.MaxParticipants > 0 && c.ActiveMembers >= c.MaxParticipants {
            c.Status = "Full"
        }
        result = append(result, c)
    }
    return result, rows.Err()
}

func GetPublicChama(db *sql.DB, chamaID int64) (*PublicChama, error) {
    chamas, err := GetPublicChamas(db)
    if err != nil {
        return nil, err
    }
    for _, c := range chamas {
        if c.ID == chamaID {
            return &c, nil
        }
    }
    return nil, fmt.Errorf("chama not found")
}

func CreateJoinRequest(db *sql.DB, chamaID, memberID int64, phone, note string, agreed bool) error {
    if phone == "" {
        return fmt.Errorf("phone number is required")
    }
    if !agreed {
        return fmt.Errorf("you must agree to the group rules")
    }

    var maxParticipants, activeMembers int
    err := db.QueryRow("SELECT COALESCE(max_participants, 0) FROM chamas WHERE id = ?", chamaID).Scan(&maxParticipants)
    if err != nil {
        return fmt.Errorf("chama not found")
    }
    db.QueryRow("SELECT COUNT(*) FROM chama_members WHERE chama_id = ? AND status = 'active'", chamaID).Scan(&activeMembers)
    if maxParticipants > 0 && activeMembers >= maxParticipants {
        return fmt.Errorf("this chama is full")
    }

    var existingStatus string
    err = db.QueryRow("SELECT status FROM chama_members WHERE chama_id = ? AND member_id = ?", chamaID, memberID).Scan(&existingStatus)
    if err == nil {
        if existingStatus == "active" {
            return fmt.Errorf("you are already a member of this chama")
        }
        if existingStatus == "pending" {
            return fmt.Errorf("you already have a pending membership")
        }
    } else if err != sql.ErrNoRows {
        return err
    }

    var requestStatus string
    err = db.QueryRow("SELECT status FROM chama_join_requests WHERE chama_id = ? AND member_id = ? ORDER BY id DESC LIMIT 1", chamaID, memberID).Scan(&requestStatus)
    if err == nil && requestStatus == "pending" {
        return fmt.Errorf("you already have a pending join request")
    }

    _, err = db.Exec(`INSERT INTO chama_join_requests
        (chama_id, member_id, phone, note, agreed_to_rules, status)
        VALUES (?, ?, ?, ?, 1, 'pending')`, chamaID, memberID, phone, note)
    return err
}

func GetMyJoinRequests(db *sql.DB, memberID int64) ([]JoinRequest, error) {
    rows, err := db.Query(`
        SELECT jr.id, jr.chama_id, c.name, jr.member_id, m.name,
               jr.phone, COALESCE(jr.note, ''), jr.status, jr.created_at
        FROM chama_join_requests jr
        JOIN chamas c ON c.id = jr.chama_id
        JOIN members m ON m.id = jr.member_id
        WHERE jr.member_id = ?
        ORDER BY jr.id DESC`, memberID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var requests []JoinRequest
    for rows.Next() {
        var j JoinRequest
        if err := rows.Scan(&j.ID, &j.ChamaID, &j.ChamaName, &j.MemberID,
            &j.ApplicantName, &j.Phone, &j.Note, &j.Status, &j.CreatedAt); err != nil {
            return nil, err
        }
        requests = append(requests, j)
    }
    return requests, rows.Err()
}

func GetAdminJoinRequests(db *sql.DB, chamaID int64) ([]JoinRequest, error) {
    rows, err := db.Query(`
        SELECT jr.id, jr.chama_id, c.name, jr.member_id, m.name,
               jr.phone, COALESCE(jr.note, ''), jr.status, jr.created_at
        FROM chama_join_requests jr
        JOIN chamas c ON c.id = jr.chama_id
        JOIN members m ON m.id = jr.member_id
        WHERE jr.chama_id = ? AND jr.status = 'pending'
        ORDER BY jr.id ASC`, chamaID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var requests []JoinRequest
    for rows.Next() {
        var j JoinRequest
        if err := rows.Scan(&j.ID, &j.ChamaID, &j.ChamaName, &j.MemberID,
            &j.ApplicantName, &j.Phone, &j.Note, &j.Status, &j.CreatedAt); err != nil {
            return nil, err
        }
        requests = append(requests, j)
    }
    return requests, rows.Err()
}

func ReviewJoinRequest(db *sql.DB, requestID, reviewerID int64, approve bool) error {
    tx, err := db.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback()

    var chamaID, applicantID int64
    var status string
    err = tx.QueryRow("SELECT chama_id, member_id, status FROM chama_join_requests WHERE id = ?", requestID).
        Scan(&chamaID, &applicantID, &status)
    if err != nil {
        return err
    }
    if status != "pending" {
        return fmt.Errorf("join request is no longer pending")
    }

    var role string
    err = tx.QueryRow("SELECT role FROM chama_members WHERE chama_id = ? AND member_id = ? AND status = 'active'", chamaID, reviewerID).Scan(&role)
    if err != nil || role != "admin" {
        return fmt.Errorf("only the chama admin can review join requests")
    }

    newStatus := "declined"
    title := "Chama join request declined"
    message := "Your request to join the chama was declined."
    if approve {
        var maxParticipants, activeMembers int
        tx.QueryRow("SELECT COALESCE(max_participants, 0) FROM chamas WHERE id = ?", chamaID).Scan(&maxParticipants)
        tx.QueryRow("SELECT COUNT(*) FROM chama_members WHERE chama_id = ? AND status = 'active'", chamaID).Scan(&activeMembers)
        if maxParticipants > 0 && activeMembers >= maxParticipants {
            return fmt.Errorf("the chama is now full")
        }
        newStatus = "approved"
        title = "You have been approved"
        message = "Your request to join the chama has been approved."
        _, err = tx.Exec(`INSERT INTO chama_members (chama_id, member_id, role, status)
            VALUES (?, ?, 'member', 'active')`, chamaID, applicantID)
        if err != nil {
            return err
        }
    }

    _, err = tx.Exec(`UPDATE chama_join_requests
        SET status = ?, reviewed_at = CURRENT_TIMESTAMP, reviewed_by = ?
        WHERE id = ? AND status = 'pending'`, newStatus, reviewerID, requestID)
    if err != nil {
        return err
    }

    _, err = tx.Exec("INSERT INTO notifications (member_id, title, message, type) VALUES (?, ?, ?, 'join_request')",
        applicantID, title, message)
    if err != nil {
        return err
    }

    return tx.Commit()
}
