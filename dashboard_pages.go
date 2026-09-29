package main

import (
	"database/sql"
	"html/template"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type RotationRow struct {
	Round int
	Name string
	Date string
	Amount float64
	Status string
}

type HistoryRow struct {
	Event string
	User string
	Details string
	CreatedAt string
}

type LedgerRow struct {
	Date string
	Member string
	Type string
	Amount float64
	Receipt string
	Status string
}

type RuleData struct {
	Amount float64
	Frequency string
	GracePeriod int
	PenaltyType string
	PenaltyAmount float64
	ApprovalThreshold int
	WelfareReserve float64
	PayoutMethod string
}

func dashboardMemberID(w http.ResponseWriter, r *http.Request, db *sql.DB) (int64, bool) {
	id, err := getLoggedInMemberID(r, db)
	if err != nil {
		http.Redirect(w, r, "/login-page", http.StatusSeeOther)
		return 0, false
	}
	return id, true
}

func activeChamaForRequest(db *sql.DB, r *http.Request, memberID int64) (int64, string, bool) {
	id, err := strconv.ParseInt(r.URL.Query().Get("chama_id"), 10, 64)
	if err != nil || id <= 0 {
		chamas, err := GetMemberChamas(db, memberID)
		if err != nil || len(chamas) == 0 {
			return 0, "", false
		}
		return chamas[0].ChamaID, chamas[0].ChamaName, true
	}
	var name string
	err = db.QueryRow(
		"SELECT c.name FROM chamas c JOIN chama_members cm ON cm.chama_id=c.id WHERE c.id=? AND cm.member_id=? AND cm.status='active'",
		id, memberID,
	).Scan(&name)
	return id, name, err == nil
}

func registerDashboardSubpageRoutes(db *sql.DB) {
	http.HandleFunc("/profile/edit", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := dashboardMemberID(w, r, db); !ok { return }
		http.Redirect(w, r, "/profile-page?edit=true", http.StatusSeeOther)
	})

	http.HandleFunc("/settings", func(w http.ResponseWriter, r *http.Request) {
		memberID, ok := dashboardMemberID(w, r, db)
		if !ok { return }
		profile, err := GetMemberProfile(db, memberID)
		if err != nil { http.Error(w, "Failed to load settings", http.StatusInternalServerError); return }
		tmpl := templateMust("settings.html")
		tmpl.Execute(w, profile)
	})

	http.HandleFunc("/rotation", func(w http.ResponseWriter, r *http.Request) {
		memberID, ok := dashboardMemberID(w, r, db)
		if !ok { return }
		chamaID, chamaName, ok := activeChamaForRequest(db, r, memberID)
		if !ok { http.Redirect(w, r, "/chama/my-chamas", http.StatusSeeOther); return }

		amount, frequency, _ := GetGroupSettingsForChama(db, chamaID)
		rows, err := db.Query(`SELECT c.cycle_number, COALESCE(m.name, 'Unassigned'), c.due_date
			FROM cycles c LEFT JOIN members m ON m.id=c.recipient_member_id
			WHERE c.chama_id=? ORDER BY c.cycle_number`, chamaID)
		if err != nil { http.Error(w, "Failed to load rotation", 500); return }
		defer rows.Close()

		var rotation []RotationRow
		now := time.Now()
		for rows.Next() {
			var round int
			var name, date string
			if err := rows.Scan(&round, &name, &date); err != nil { continue }
			status := "Upcoming"
			if parsed, err := time.Parse("2006-01-02", date); err == nil {
				if parsed.Before(now.Truncate(24*time.Hour)) { status = "Completed" }
				if parsed.Year() == now.Year() && parsed.YearDay() == now.YearDay() { status = "In Progress" }
			}
			rotation = append(rotation, RotationRow{round, name, date, amount, status})
		}

		tmpl := templateMust("rotation.html")
		tmpl.Execute(w, struct {
			ChamaID int64
			ChamaName string
			Frequency string
			Rows []RotationRow
		}{chamaID, chamaName, frequency, rotation})
	})

	http.HandleFunc("/history", func(w http.ResponseWriter, r *http.Request) {
		memberID, ok := dashboardMemberID(w, r, db)
		if !ok { return }
		chamaID, chamaName, ok := activeChamaForRequest(db, r, memberID)
		if !ok { http.Redirect(w, r, "/chama/my-chamas", http.StatusSeeOther); return }

		var history []HistoryRow
		rows, err := db.Query(`SELECT 'Member joined', m.name, 'Joined the Chama', cm.joined_at
			FROM chama_members cm JOIN members m ON m.id=cm.member_id
			WHERE cm.chama_id=? AND cm.status='active'
			UNION ALL
			SELECT 'Payout authorization', m.name, 'Withdrawal approval recorded', wa.approved_at
			FROM withdrawal_approvals wa JOIN withdrawals w ON w.id=wa.withdrawal_id
			JOIN members m ON m.id=wa.member_id
			WHERE w.chama_id=?
			UNION ALL
			SELECT 'Join request reviewed', m.name, 'Membership request was ' || jr.status, jr.reviewed_at
			FROM chama_join_requests jr JOIN members m ON m.id=jr.member_id
			WHERE jr.chama_id=? AND jr.reviewed_at IS NOT NULL
			UNION ALL
			SELECT ae.event, COALESCE(m.name, 'System'), ae.details, ae.created_at
			FROM audit_events ae LEFT JOIN members m ON m.id=ae.member_id
			WHERE ae.chama_id=?
			ORDER BY 4 DESC`, chamaID, chamaID, chamaID, chamaID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var e, user, details, at sql.NullString
				if rows.Scan(&e, &user, &details, &at) == nil {
					history = append(history, HistoryRow{e.String, user.String, details.String, at.String})
				}
			}
		}

		tmpl := templateMust("history.html")
		tmpl.Execute(w, struct { ChamaID int64; ChamaName string; Rows []HistoryRow }{chamaID, chamaName, history})
	})

	http.HandleFunc("/ledger", func(w http.ResponseWriter, r *http.Request) {
		memberID, ok := dashboardMemberID(w, r, db)
		if !ok { return }
		chamaID, chamaName, ok := activeChamaForRequest(db, r, memberID)
		if !ok { http.Redirect(w, r, "/chama/my-chamas", http.StatusSeeOther); return }
		search := strings.TrimSpace(r.URL.Query().Get("q"))
		rows := loadLedgerRows(db, chamaID, search)
		tmpl := templateMust("ledger.html")
		tmpl.Execute(w, struct {
			ChamaID int64
			ChamaName string
			Search string
			Rows []LedgerRow
		}{chamaID, chamaName, search, rows})
	})

	http.HandleFunc("/ledger/pdf", func(w http.ResponseWriter, r *http.Request) {
		memberID, ok := dashboardMemberID(w, r, db)
		if !ok { return }
		chamaID, chamaName, ok := activeChamaForRequest(db, r, memberID)
		if !ok { http.Error(w, "Invalid Chama", 400); return }
		pdf := makeLedgerPDF(chamaName, loadLedgerRows(db, chamaID, ""))
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", "attachment; filename=\"chama-statement.pdf\"")
		w.Write(pdf)
	})

	http.HandleFunc("/rules", func(w http.ResponseWriter, r *http.Request) {
		memberID, ok := dashboardMemberID(w, r, db)
		if !ok { return }
		chamaID, chamaName, ok := activeChamaForRequest(db, r, memberID)
		if !ok { http.Redirect(w, r, "/chama/my-chamas", http.StatusSeeOther); return }
		var d RuleData
		db.QueryRow(`SELECT contribution_amount, frequency, grace_period_days,
			late_penalty_type, late_penalty_amount, approval_threshold,
			welfare_reserve_amount, payout_method
			FROM group_settings WHERE chama_id=? LIMIT 1`, chamaID).Scan(
			&d.Amount, &d.Frequency, &d.GracePeriod, &d.PenaltyType,
			&d.PenaltyAmount, &d.ApprovalThreshold, &d.WelfareReserve, &d.PayoutMethod)
		tmpl := templateMust("rules.html")
		tmpl.Execute(w, struct { ChamaID int64; ChamaName string; Rules RuleData }{chamaID, chamaName, d})
	})
}

func templateMust(name string) *template.Template {
	return template.Must(template.ParseFiles(name))
}

func loadLedgerRows(db *sql.DB, chamaID int64, search string) []LedgerRow {
	var result []LedgerRow
	like := "%" + search + "%"
	rows, err := db.Query(`SELECT c.paid_on, m.name, 'Contribution', c.amount,
		COALESCE(c.checkout_request_id, '-'), c.status
		FROM contributions c JOIN members m ON m.id=c.member_id
		WHERE c.chama_id=? AND (m.name LIKE ? OR c.paid_on LIKE ? OR c.status LIKE ?)
		UNION ALL
		SELECT w.created_at, m.name, 'Withdrawal', -w.amount, '-', w.status
		FROM withdrawals w JOIN members m ON m.id=w.requested_by
		WHERE w.chama_id=? AND (m.name LIKE ? OR w.created_at LIKE ? OR w.status LIKE ?)
		ORDER BY 1 DESC`, chamaID, like, like, like, chamaID, like, like, like)
	if err != nil { return result }
	defer rows.Close()
	for rows.Next() {
		var x LedgerRow
		if rows.Scan(&x.Date, &x.Member, &x.Type, &x.Amount, &x.Receipt, &x.Status) == nil {
			if x.Status == "synced" { x.Status = "Synced" }
			result = append(result, x)
		}
	}
	return result
}

func makeLedgerPDF(chamaName string, rows []LedgerRow) []byte {
	var lines []string
	lines = append(lines, "CHAMA SALAMA - GROUP LEDGER")
	lines = append(lines, chamaName)
	lines = append(lines, "Date        Member                  Type          Amount       Receipt")
	lines = append(lines, "--------------------------------------------------------------------------")
	for _, r := range rows {
		lines = append(lines, fmt.Sprintf("%-11s %-23s %-12s KES %-8.2f %s",
			r.Date, truncatePDF(r.Member, 23), r.Type, r.Amount, truncatePDF(r.Receipt, 20)))
	}
	if len(lines) == 4 { lines = append(lines, "No ledger entries found.") }

	var objects []string
	add := func(s string) int { objects = append(objects, s); return len(objects) }
	font := add("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	var pageIDs []int
	var contentIDs []int
	for pageStart := 0; pageStart < len(lines); pageStart += 42 {
		end := pageStart + 42
		if end > len(lines) { end = len(lines) }
		var stream strings.Builder
		stream.WriteString("BT /F1 9 Tf 40 770 Td 12 TL\n")
		for _, line := range lines[pageStart:end] {
			stream.WriteString("(" + pdfEscape(line) + ") Tj T*\n")
		}
		stream.WriteString("ET")
		content := add(fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream.String()), stream.String()))
		contentIDs = append(contentIDs, content)
		pageIDs = append(pageIDs, 0)
	}
	pages := add("<< /Type /Pages /Kids [] /Count 0 >>")
	for i, contentID := range contentIDs {
		pageIDs[i] = add(fmt.Sprintf("<< /Type /Page /Parent %d 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 %d 0 R >> >> /Contents %d 0 R >>", pages, font, contentID))
	}
	kids := make([]string, len(pageIDs))
	for i, id := range pageIDs { kids[i] = fmt.Sprintf("%d 0 R", id) }
	objects[pages-1] = fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(kids))
	catalog := add(fmt.Sprintf("<< /Type /Catalog /Pages %d 0 R >>", pages))

	var out strings.Builder
	out.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects)+1)
	for i, obj := range objects {
		offsets[i+1] = out.Len()
		out.WriteString(fmt.Sprintf("%d 0 obj\n%s\nendobj\n", i+1, obj))
	}
	xref := out.Len()
	out.WriteString(fmt.Sprintf("xref\n0 %d\n0000000000 65535 f \n", len(objects)+1))
	for i := 1; i <= len(objects); i++ { out.WriteString(fmt.Sprintf("%010d 00000 n \n", offsets[i])) }
	out.WriteString(fmt.Sprintf("trailer\n<< /Size %d /Root %d 0 R >>\nstartxref\n%d\n%%%%EOF", len(objects)+1, catalog, xref))
	return []byte(out.String())
}

func truncatePDF(s string, n int) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\n", " "), "\r", " ")
	if len(s) > n { return s[:n] }
	return s
}

func pdfEscape(s string) string {
	return strings.NewReplacer("\\", "\\\\", "(", "\\(", ")", "\\)").Replace(s)
}
