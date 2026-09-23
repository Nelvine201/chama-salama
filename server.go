package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strconv"
	"strings"
	"net/url"
)

func startServer(db *sql.DB) {
	http.Handle("/", http.FileServer(http.Dir("docs")))
	registerAuthRoutes(db)

	http.HandleFunc("/register-page", func(w http.ResponseWriter, r *http.Request) {
		tmpl := template.Must(template.ParseFiles("register.html"))
		tmpl.Execute(w, struct{ Next string }{Next: safeNextPath(r.URL.Query().Get("next"))})
	})
	http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			fmt.Fprintln(w, "Please submit this form using POST")
			return
		}

		name := r.FormValue("name")
		phone := r.FormValue("phone")
		email := r.FormValue("email")
		password := r.FormValue("password")
		role := "member"
		termsAccepted := r.FormValue("terms_accepted") == "true"

		_, err := CreateMember(db, name, phone, email, password, role, termsAccepted)
		if err != nil {
			fmt.Fprintln(w, "Registration failed:", err)
			return
		}

		http.Redirect(w, r, "/login-page", http.StatusSeeOther)
	})
	http.HandleFunc("/login-page", func(w http.ResponseWriter, r *http.Request) {
		tmpl := template.Must(template.ParseFiles("login.html"))
		tmpl.Execute(w, struct{ Next string }{Next: safeNextPath(r.URL.Query().Get("next"))})
	})
	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			fmt.Fprintln(w, "Please submit this form using POST")
			return
		}

		identifier := r.FormValue("identifier")
		password := r.FormValue("password")

		member, err := CheckLogin(db, identifier, password)
		if err != nil {
			fmt.Fprintln(w, "Login failed:", err)
			return
		}

		token, err := createSession(db, member.ID)
		if err != nil {
			fmt.Fprintln(w, "Failed to create session:", err)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    token,
			HttpOnly: true,
			Path:     "/",
		})
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	})
	http.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_token")
		if err == nil {
			deleteSession(db, cookie.Value)
		}
		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    "",
			HttpOnly: true,
			Path:     "/",
			MaxAge:   -1,
		})
		http.Redirect(w, r, "/login-page", http.StatusSeeOther)
	})
	http.HandleFunc("/profile-page", func(w http.ResponseWriter, r *http.Request) {
		memberID, err := getLoggedInMemberID(r, db)
		if err != nil {
			http.Redirect(w, r, "/login-page", http.StatusSeeOther)
			return
		}

		profile, err := GetMemberProfile(db, memberID)
		if err != nil {
			fmt.Fprintln(w, "Failed to load profile:", err)
			return
		}

		saved := profile.NationalID.Valid && profile.NationalID.String != ""
		editing := r.URL.Query().Get("edit") == "true"

		data := struct {
			Saved                 bool
			Editing               bool
			Name                  string
			PreferredFirstName    string
			NationalID            string
			Location              string
			NextOfKin             string
			NextOfKinRelationship string
			NextOfKinIDNumber     string
			NextOfKinPhone        string
			DefaultPayoutMethod   string
			NotifySMS             bool
			NotifyEmail           bool
		}{
			Saved:                 saved,
			Editing:               editing,
			Name:                  profile.Name,
			PreferredFirstName:    profile.PreferredFirstName.String,
			NationalID:            profile.NationalID.String,
			Location:              profile.Location.String,
			NextOfKin:             profile.NextOfKin.String,
			NextOfKinRelationship: profile.NextOfKinRelationship.String,
			NextOfKinIDNumber:     profile.NextOfKinIDNumber.String,
			NextOfKinPhone:        profile.NextOfKinPhone.String,
			DefaultPayoutMethod:   profile.DefaultPayoutMethod.String,
			NotifySMS:             profile.NotifySMS,
			NotifyEmail:           profile.NotifyEmail,
		}

		tmpl := template.Must(template.ParseFiles("profile.html"))
		tmpl.Execute(w, data)
	})

	http.HandleFunc("/profile", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			fmt.Fprintln(w, "Please submit this form using POST")
			return
		}

		memberID, err := getLoggedInMemberID(r, db)
		if err != nil {
			http.Redirect(w, r, "/login-page", http.StatusSeeOther)
			return
		}

		nationalID := r.FormValue("national_id")
		location := r.FormValue("location")
		nextOfKin := r.FormValue("next_of_kin")
		nextOfKinRelationship := r.FormValue("next_of_kin_relationship")
		nextOfKinIDNumber := r.FormValue("next_of_kin_id_number")
		nextOfKinPhone := r.FormValue("next_of_kin_phone")
		preferredFirstName := r.FormValue("preferred_first_name")
		defaultPayoutMethod := r.FormValue("default_payout_method")
		notifySMS := r.FormValue("notify_sms") == "true"
		notifyEmail := r.FormValue("notify_email") == "true"

		err = UpdateProfile(db, memberID, nationalID, location, nextOfKin, nextOfKinRelationship,
			nextOfKinIDNumber, nextOfKinPhone, preferredFirstName, defaultPayoutMethod, notifySMS, notifyEmail)
		if err != nil {
			fmt.Fprintln(w, "Profile update failed:", err)
			return
		}

		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	})
	http.HandleFunc("/contribute-page", func(w http.ResponseWriter, r *http.Request) {
		memberID, err := getLoggedInMemberID(r, db)
		if err != nil {
			http.Redirect(w, r, "/login-page", http.StatusSeeOther)
			return
		}

		chamas, err := GetMemberChamas(db, memberID)
		if err != nil || len(chamas) == 0 {
			http.Redirect(w, r, "/chama/my-chamas", http.StatusSeeOther)
			return
		}
		chamaID := chamas[0].ChamaID
		if qid := r.URL.Query().Get("chama_id"); qid != "" {
			if parsed, err := strconv.ParseInt(qid, 10, 64); err == nil {
				chamaID = parsed
			}
		}

		amount, _, _ := GetGroupSettingsForChama(db, chamaID)
		profile, _ := GetMemberProfile(db, memberID)

		data := struct {
			ChamaID int64
			Amount  float64
			Phone   string
		}{
			ChamaID: chamaID,
			Amount:  amount,
			Phone:   profile.Phone.String,
		}

		tmpl := template.Must(template.ParseFiles("contribute.html"))
		tmpl.Execute(w, data)
	})

	http.HandleFunc("/contribute", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			fmt.Fprintln(w, "Please submit this form using POST")
			return
		}

		memberID, err := getLoggedInMemberID(r, db)
		if err != nil {
			http.Redirect(w, r, "/login-page", http.StatusSeeOther)
			return
		}

		amountStr := r.FormValue("amount")
		amount, err := strconv.ParseFloat(amountStr, 64)
		if err != nil {
			fmt.Fprintln(w, "Invalid amount")
			return
		}

		paidOn := r.FormValue("paid_on")
		source := r.FormValue("source")

		_, err = RecordContribution(db, memberID, amount, paidOn, source)
		if err != nil {
			fmt.Fprintln(w, "Failed to record contribution:", err)
			return
		}

		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	})
	http.HandleFunc("/contribute-offline", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			fmt.Fprintln(w, "Please submit this form using POST")
			return
		}

		memberIDStr := r.FormValue("member_id")
		memberID, err := strconv.ParseInt(memberIDStr, 10, 64)
		if err != nil {
			fmt.Fprintln(w, "Invalid member ID")
			return
		}

		amountStr := r.FormValue("amount")
		amount, err := strconv.ParseFloat(amountStr, 64)
		if err != nil {
			fmt.Fprintln(w, "Invalid amount")
			return
		}

		paidOn := r.FormValue("paid_on")
		source := r.FormValue("source")

		id, err := RecordContributionOffline(db, memberID, amount, paidOn, source)
		if err != nil {
			fmt.Fprintln(w, "Failed to record contribution:", err)
			return
		}

		fmt.Fprintln(w, "Contribution saved offline (pending sync). ID:", id)
	})

	http.HandleFunc("/sync", func(w http.ResponseWriter, r *http.Request) {
		count, err := ProcessSyncQueue(db)
		if err != nil {
			fmt.Fprintln(w, "Sync failed:", err)
			return
		}
		fmt.Fprintln(w, "Sync complete. Contributions synced:", count)
	})

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			fmt.Fprintln(w, "Failed to read callback")
			return
		}
		defer r.Body.Close()

		fmt.Println("Received Daraja callback:")
		fmt.Println(string(body))

		var payload struct {
			Body struct {
				StkCallback struct {
					CheckoutRequestID string `json:"CheckoutRequestID"`
					ResultCode        int    `json:"ResultCode"`
				} `json:"stkCallback"`
			} `json:"Body"`
		}

		if err := json.Unmarshal(body, &payload); err != nil {
			fmt.Println("Failed to parse callback:", err)
			fmt.Fprintln(w, "Callback received")
			return
		}

		checkoutID := payload.Body.StkCallback.CheckoutRequestID
		success := payload.Body.StkCallback.ResultCode == 0

		err = ConfirmSTKContribution(db, checkoutID, success)
		if err != nil {
			fmt.Println("Failed to update contribution:", err)
		}

		fmt.Fprintln(w, "Callback received")
	})
	http.HandleFunc("/stk-push", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			fmt.Fprintln(w, "Please submit this form using POST")
			return
		}

		memberID, err := getLoggedInMemberID(r, db)
		if err != nil {
			http.Redirect(w, r, "/login-page", http.StatusSeeOther)
			return
		}

		chamaIDStr := r.FormValue("chama_id")
		chamaID, err := strconv.ParseInt(chamaIDStr, 10, 64)
		if err != nil {
			fmt.Fprintln(w, "Invalid chama ID")
			return
		}

		phone := r.FormValue("phone")
		amountStr := r.FormValue("amount")
		amount, err := strconv.Atoi(amountStr)
		if err != nil {
			fmt.Fprintln(w, "Invalid amount")
			return
		}

		consumerKey, consumerSecret := getDarajaCredentials()
		token, err := getAccessToken(consumerKey, consumerSecret)
		if err != nil {
			fmt.Fprintln(w, "Failed to get access token:", err)
			return
		}

		stkResp, err := sendStkPush(token, phone, amount)
		if err != nil {
			fmt.Fprintln(w, "STK push failed:", err)
			return
		}

		if stkResp.ResponseCode != "0" {
			fmt.Fprintln(w, "STK push rejected:", stkResp.ResponseDescription)
			return
		}

		_, err = RecordPendingSTKContribution(db, memberID, chamaID, float64(amount), stkResp.CheckoutRequestID)
		if err != nil {
			fmt.Fprintln(w, "Failed to save pending contribution:", err)
			return
		}

		fmt.Fprintf(w, "Check your phone (%s) to complete the M-Pesa payment.", phone)
	})
	http.HandleFunc("/admin/set-settings", func(w http.ResponseWriter, r *http.Request) {
		chamaID, _ := strconv.ParseInt(r.FormValue("chama_id"), 10, 64)
		if chamaID == 0 { chamaID, _ = strconv.ParseInt(r.URL.Query().Get("chama_id"), 10, 64) }
		if _, err := requireChamaRole(db, r, chamaID, "admin"); err != nil {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodPost {
			fmt.Fprintln(w, "Please submit this form using POST")
			return
		}

		amountStr := r.FormValue("amount")
		amount, err := strconv.ParseFloat(amountStr, 64)
		if err != nil {
			fmt.Fprintln(w, "Invalid amount")
			return
		}

		frequency := r.FormValue("frequency")

		err = SetGroupSettings(db, amount, frequency)
		if err != nil {
			fmt.Fprintln(w, "Failed to set group settings:", err)
			return
		}

		fmt.Fprintln(w, "Group settings updated successfully")
	})

	http.HandleFunc("/admin/settings", func(w http.ResponseWriter, r *http.Request) {
		chamaID, _ := strconv.ParseInt(r.FormValue("chama_id"), 10, 64)
		if chamaID == 0 { chamaID, _ = strconv.ParseInt(r.URL.Query().Get("chama_id"), 10, 64) }
		if _, err := requireChamaRole(db, r, chamaID, "admin"); err != nil {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}
		settings, err := GetGroupSettings(db)
		if err != nil {
			fmt.Fprintln(w, "Failed to get group settings:", err)
			return
		}

		fmt.Fprintln(w, "Contribution amount:", settings.ContributionAmount)
		fmt.Fprintln(w, "Frequency:", settings.Frequency)

		if settings.PayoutOrder.Valid {
			fmt.Fprintln(w, "Payout order:", settings.PayoutOrder.String)
		} else {
			fmt.Fprintln(w, "Payout order: not set yet")
		}
	})
	http.HandleFunc("/admin/members", func(w http.ResponseWriter, r *http.Request) {
		chamaID, _ := strconv.ParseInt(r.FormValue("chama_id"), 10, 64)
		if chamaID == 0 { chamaID, _ = strconv.ParseInt(r.URL.Query().Get("chama_id"), 10, 64) }
		if _, err := requireChamaRole(db, r, chamaID, "admin"); err != nil {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}
		members, err := GetAllMembers(db)
		if err != nil {
			fmt.Fprintln(w, "Failed to get members:", err)
			return
		}

		for _, m := range members {
			fmt.Fprintln(w, m.ID, m.Name, m.Phone, m.Email, m.Role)
		}
	})
	http.HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
		memberID, err := getLoggedInMemberID(r, db)
		if err != nil {
			fmt.Fprintln(w, "Please log in to view the dashboard")
			return
		}

		profile, err := GetMemberProfile(db, memberID)
		if err != nil {
			fmt.Fprintln(w, "Failed to load profile:", err)
			return
		}
		firstName := strings.Fields(profile.Name)[0]

		chamas, err := GetMemberChamas(db, memberID)
		if err != nil {
			fmt.Fprintln(w, "Failed to load chamas:", err)
			return
		}
		if len(chamas) == 0 {
			tmpl := template.Must(template.ParseFiles("dashboard-empty.html"))
			tmpl.Execute(w, struct{ FirstName string }{FirstName: firstName})
			return
		}

		chamaIDStr := r.URL.Query().Get("chama_id")
		var activeChamaID int64
		var activeRole string
		if chamaIDStr != "" {
			activeChamaID, _ = strconv.ParseInt(chamaIDStr, 10, 64)
		}
		var activeChamaName string
		found := false
		for _, c := range chamas {
			if activeChamaID == 0 || c.ChamaID == activeChamaID {
				activeChamaID = c.ChamaID
				activeChamaName = c.ChamaName
				activeRole = c.Role
				found = true
				break
			}
		}
		if !found {
			http.Redirect(w, r, "/chama/my-chamas", http.StatusSeeOther)
			return
		}

		balance, _ := GetGroupBalanceForChama(db, activeChamaID)
		amount, frequency, _ := GetGroupSettingsForChama(db, activeChamaID)
		contributions, _ := GetRecentContributionsForChama(db, activeChamaID, 10)
		recipientName, _, queuePos := GetPayoutQueueInfo(db, activeChamaID, memberID)
		cycleSummary, _ := GetCycleSummary(db, activeChamaID)
		userSummary, _ := GetUserCycleSummary(db, activeChamaID, memberID)
		pendingRequests, _ := GetMyJoinRequests(db, memberID)

		var paidCount int
		db.QueryRow("SELECT COUNT(*) FROM contributions WHERE chama_id = ? AND member_id = ? AND status = 'synced'", activeChamaID, memberID).Scan(&paidCount)
		personalPaid := paidCount > 0

		isAdminOrTreasurer := activeRole == "admin" || activeRole == "treasurer"

		data := struct {
			FirstName          string
			ActiveChamaID      int64
			ActiveChamaName    string
			ActiveRole         string
			Chamas             []ChamaMembership
			Balance            float64
			Amount             float64
			Frequency          string
			Contributions      []ContributionWithMember
			RecipientName      string
			QueuePosition      int
			PersonalPaid       bool
			IsAdminOrTreasurer bool
			Cycle              *CycleSummary
			UserCycle          *UserCycleSummary
			PendingRequests    []JoinRequest
		}{
			FirstName:          firstName,
			ActiveChamaID:      activeChamaID,
			ActiveChamaName:    activeChamaName,
			ActiveRole:         activeRole,
			Chamas:             chamas,
			Balance:            balance,
			Amount:             amount,
			Frequency:          frequency,
			Contributions:      contributions,
			RecipientName:      recipientName,
			QueuePosition:      queuePos,
			PersonalPaid:       personalPaid,
			IsAdminOrTreasurer: isAdminOrTreasurer,
			Cycle:              cycleSummary,
			UserCycle:          userSummary,
			PendingRequests:    pendingRequests,
		}

		tmpl := template.Must(template.ParseFiles("dashboard.html"))
		tmpl.Execute(w, data)
	})
	http.HandleFunc("/withdraw/request-page", func(w http.ResponseWriter, r *http.Request) {
		_, err := getLoggedInMemberID(r, db)
		if err != nil {
			http.Redirect(w, r, "/login-page", http.StatusSeeOther)
			return
		}
		tmpl := template.Must(template.ParseFiles("withdraw-request.html"))
		tmpl.Execute(w, nil)
	})

	http.HandleFunc("/withdraw/request", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			fmt.Fprintln(w, "Please submit this form using POST")
			return
		}

		requestedBy, err := getLoggedInMemberID(r, db)
		if err != nil {
			http.Redirect(w, r, "/login-page", http.StatusSeeOther)
			return
		}

		amountStr := r.FormValue("amount")
		amount, err := strconv.ParseFloat(amountStr, 64)
		if err != nil {
			fmt.Fprintln(w, "Invalid amount")
			return
		}

		reason := r.FormValue("reason")

		_, err = CreateWithdrawal(db, requestedBy, amount, reason)
		if err != nil {
			fmt.Fprintln(w, "Failed to create withdrawal request:", err)
			return
		}

		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	})

	http.HandleFunc("/withdraw/approve-page", func(w http.ResponseWriter, r *http.Request) {
		_, err := getLoggedInMemberID(r, db)
		if err != nil {
			http.Redirect(w, r, "/login-page", http.StatusSeeOther)
			return
		}
		tmpl := template.Must(template.ParseFiles("withdraw-approve.html"))
		tmpl.Execute(w, nil)
	})

	http.HandleFunc("/withdraw/approve", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			fmt.Fprintln(w, "Please submit this form using POST")
			return
		}

		memberID, err := getLoggedInMemberID(r, db)
		if err != nil {
			http.Redirect(w, r, "/login-page", http.StatusSeeOther)
			return
		}

		withdrawalIDStr := r.FormValue("withdrawal_id")
		withdrawalID, err := strconv.ParseInt(withdrawalIDStr, 10, 64)
		if err != nil {
			fmt.Fprintln(w, "Invalid withdrawal ID")
			return
		}

		fullyApproved, err := ApproveWithdrawal(db, withdrawalID, memberID)
		if err != nil {
			fmt.Fprintln(w, "Approval failed:", err)
			return
		}

		if fullyApproved {
			fmt.Fprintln(w, "Approval recorded. Withdrawal is now fully approved!")
		} else {
			fmt.Fprintln(w, "Approval recorded. Waiting for more signatures.")
		}
	})



	http.HandleFunc("/notifications", func(w http.ResponseWriter, r *http.Request) {
		memberID, err := getLoggedInMemberID(r, db)
		if err != nil { http.Redirect(w, r, "/login-page", http.StatusSeeOther); return }
		notifications, err := GetNotifications(db, memberID)
		if err != nil { http.Error(w, "Failed to load notifications", http.StatusInternalServerError); return }
		tmpl := template.Must(template.ParseFiles("notifications.html"))
		tmpl.Execute(w, struct{ Notifications []Notification }{notifications})
	})

	http.HandleFunc("/chama/discover", func(w http.ResponseWriter, r *http.Request) {
		chamas, err := GetPublicChamas(db)
		if err != nil {
			http.Error(w, "Failed to load public chamas", http.StatusInternalServerError)
			return
		}
		phone := ""
		if memberID, sessionErr := getLoggedInMemberID(r, db); sessionErr == nil {
			if profile, profileErr := GetMemberProfile(db, memberID); profileErr == nil {
				phone = profile.Phone.String
			}
		}
		tmpl := template.Must(template.ParseFiles("chama-discover.html"))
		tmpl.Execute(w, struct {
			Chamas []PublicChama
			Phone  string
		}{Chamas: chamas, Phone: phone})
	})

	http.HandleFunc("/chama/join", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := requireSameOrigin(r); err != nil {
			http.Error(w, "Invalid request origin", http.StatusForbidden)
			return
		}
		memberID, err := getLoggedInMemberID(r, db)
		if err != nil {
			next := "/chama/discover"
			if id := r.FormValue("chama_id"); id != "" { next = "/chama/discover?chama_id=" + url.QueryEscape(id) }
			http.Redirect(w, r, "/login-page?next="+url.QueryEscape(next), http.StatusSeeOther)
			return
		}
		chamaID, err := strconv.ParseInt(r.FormValue("chama_id"), 10, 64)
		if err != nil {
			http.Error(w, "Invalid chama ID", http.StatusBadRequest)
			return
		}
		if err := CreateJoinRequest(db, chamaID, memberID, strings.TrimSpace(r.FormValue("phone")),
			strings.TrimSpace(r.FormValue("note")), r.FormValue("agreed_to_rules") == "on"); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Redirect(w, r, "/chama/pending-requests", http.StatusSeeOther)
	})

	http.HandleFunc("/chama/pending-requests", func(w http.ResponseWriter, r *http.Request) {
		memberID, err := getLoggedInMemberID(r, db)
		if err != nil {
			http.Redirect(w, r, "/login-page", http.StatusSeeOther)
			return
		}
		requests, err := GetMyJoinRequests(db, memberID)
		if err != nil {
			http.Error(w, "Failed to load requests", http.StatusInternalServerError)
			return
		}
		tmpl := template.Must(template.ParseFiles("chama-pending-requests.html"))
		tmpl.Execute(w, struct{ Requests []JoinRequest }{Requests: requests})
	})

	http.HandleFunc("/chama/member-requests", func(w http.ResponseWriter, r *http.Request) {
		memberID, err := getLoggedInMemberID(r, db)
		if err != nil {
			http.Redirect(w, r, "/login-page", http.StatusSeeOther)
			return
		}
		chamaID, err := strconv.ParseInt(r.URL.Query().Get("chama_id"), 10, 64)
		if err != nil {
			http.Error(w, "Invalid chama ID", http.StatusBadRequest)
			return
		}
		role, err := GetMemberRoleInChama(db, chamaID, memberID)
		if err != nil || role != "admin" {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}
		requests, err := GetAdminJoinRequests(db, chamaID)
		if err != nil {
			http.Error(w, "Failed to load member requests", http.StatusInternalServerError)
			return
		}
		var chamaName string
		db.QueryRow("SELECT name FROM chamas WHERE id = ?", chamaID).Scan(&chamaName)
		tmpl := template.Must(template.ParseFiles("chama-member-requests.html"))
		tmpl.Execute(w, struct {
			ChamaID int64
			ChamaName string
			Requests []JoinRequest
		}{chamaID, chamaName, requests})
	})

	http.HandleFunc("/chama/member-requests/review", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		memberID, err := getLoggedInMemberID(r, db)
		if err != nil {
			http.Redirect(w, r, "/login-page", http.StatusSeeOther)
			return
		}
		requestID, err := strconv.ParseInt(r.FormValue("request_id"), 10, 64)
		if err != nil {
			http.Error(w, "Invalid request ID", http.StatusBadRequest)
			return
		}
		chamaID := r.FormValue("chama_id")
		approve := r.FormValue("action") == "approve"
		if err := ReviewJoinRequest(db, requestID, memberID, approve); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Redirect(w, r, "/chama/member-requests?chama_id="+chamaID, http.StatusSeeOther)
	})

	http.HandleFunc("/chama/my-chamas", func(w http.ResponseWriter, r *http.Request) {
		memberID, err := getLoggedInMemberID(r, db)
		if err != nil {
			http.Redirect(w, r, "/login-page", http.StatusSeeOther)
			return
		}

		cards, err := GetMemberChamaCards(db, memberID)
		if err != nil {
			fmt.Fprintln(w, "Failed to load chamas:", err)
			return
		}

		data := struct{ Cards []ChamaCard }{Cards: cards}
		tmpl := template.Must(template.ParseFiles("create-chama.html"))
		tmpl.Execute(w, data)
	})
	http.HandleFunc("/chama/dashboard", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dashboard?chama_id="+r.URL.Query().Get("chama_id"), http.StatusSeeOther)
	})

	http.HandleFunc("/chama/create-page", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if _, err := getLoggedInMemberID(r, db); err != nil {
			http.Redirect(w, r, "/login-page?next="+url.QueryEscape("/chama/create-page"), http.StatusSeeOther)
			return
		}

		tmpl := template.Must(template.ParseFiles("chama-create-form.html"))
		tmpl.Execute(w, nil)
	})

	http.HandleFunc("/chama/create", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			fmt.Fprintln(w, "Please submit this form using POST")
			return
		}
		if err := requireSameOrigin(r); err != nil {
			http.Error(w, "Invalid request origin", http.StatusForbidden)
			return
		}

		memberID, err := getLoggedInMemberID(r, db)
		if err != nil {
			http.Redirect(w, r, "/login-page?next="+url.QueryEscape("/chama/create-page"), http.StatusSeeOther)
			return
		}

		maxParticipants, err := strconv.Atoi(r.FormValue("max_participants"))
		if err != nil {
			fmt.Fprintln(w, "Invalid maximum participants")
			return
		}

		numberOfRounds, err := strconv.Atoi(r.FormValue("number_of_rounds"))
		if err != nil {
			fmt.Fprintln(w, "Invalid number of rounds")
			return
		}

		gracePeriodDays, err := strconv.Atoi(r.FormValue("grace_period_days"))
		if err != nil {
			fmt.Fprintln(w, "Invalid grace period")
			return
		}

		approvalThreshold, err := strconv.Atoi(r.FormValue("approval_threshold"))
		if err != nil {
			fmt.Fprintln(w, "Invalid approval threshold")
			return
		}

		contributionAmount, err := strconv.ParseFloat(r.FormValue("contribution_amount"), 64)
		if err != nil {
			fmt.Fprintln(w, "Invalid contribution amount")
			return
		}

		latePenaltyAmount, err := strconv.ParseFloat(r.FormValue("late_penalty_amount"), 64)
		if err != nil {
			fmt.Fprintln(w, "Invalid late penalty amount")
			return
		}

		welfareReserveAmount, err := strconv.ParseFloat(r.FormValue("welfare_reserve_amount"), 64)
		if err != nil {
			fmt.Fprintln(w, "Invalid welfare reserve amount")
			return
		}

		config := ChamaConfig{
			Name:                  strings.TrimSpace(r.FormValue("name")),
			Description:           strings.TrimSpace(r.FormValue("description")),
			StartDate:             r.FormValue("start_date"),
			MaxParticipants:       maxParticipants,
			ContributionAmount:    contributionAmount,
			Frequency:             r.FormValue("frequency"),
			NumberOfRounds:        numberOfRounds,
			PayoutMethod:          r.FormValue("payout_method"),
			GracePeriodDays:       gracePeriodDays,
			LatePenaltyType:       r.FormValue("late_penalty_type"),
			LatePenaltyAmount:     latePenaltyAmount,
			WelfareReserveAmount:  welfareReserveAmount,
			ApprovalThreshold:     approvalThreshold,
			PayoutDestinationType: r.FormValue("payout_destination_type"),
			PaybillNumber:         strings.TrimSpace(r.FormValue("paybill_number")),
			TillNumber:            strings.TrimSpace(r.FormValue("till_number")),
			TreasurerPhone:        strings.TrimSpace(r.FormValue("treasurer_phone")),
			TreasurerAccountName:  strings.TrimSpace(r.FormValue("treasurer_account_name")),
		}

		_, err = CreateChamaWithSettings(db, config, memberID)
		if err != nil {
			fmt.Fprintln(w, "Failed to create chama:", err)
			return
		}

		http.Redirect(w, r, "/chama/my-chamas", http.StatusSeeOther)
	})

	http.HandleFunc("/chama/roster", func(w http.ResponseWriter, r *http.Request) {
		memberID, err := getLoggedInMemberID(r, db)
		if err != nil {
			http.Redirect(w, r, "/login-page", http.StatusSeeOther)
			return
		}

		chamaIDStr := r.URL.Query().Get("chama_id")
		chamaID, err := strconv.ParseInt(chamaIDStr, 10, 64)
		if err != nil {
			fmt.Fprintln(w, "Invalid chama ID")
			return
		}

		roster, err := GetChamaRoster(db, chamaID)
		if err != nil {
			fmt.Fprintln(w, "Failed to load roster:", err)
			return
		}

		role, _ := GetMemberRoleInChama(db, chamaID, memberID)
		isAdmin := role == "admin"

		var chamaName string
		db.QueryRow("SELECT name FROM chamas WHERE id = ?", chamaID).Scan(&chamaName)

		data := struct {
			ChamaID   int64
			ChamaName string
			Roster    []RosterMember
			IsAdmin   bool
		}{
			ChamaID:   chamaID,
			ChamaName: chamaName,
			Roster:    roster,
			IsAdmin:   isAdmin,
		}

		tmpl := template.Must(template.ParseFiles("chama-roster.html"))
		tmpl.Execute(w, data)
	})
	http.HandleFunc("/chama/invite", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			fmt.Fprintln(w, "Please submit this form using POST")
			return
		}

		_, err := getLoggedInMemberID(r, db)
		if err != nil {
			http.Redirect(w, r, "/login-page", http.StatusSeeOther)
			return
		}

		chamaIDStr := r.FormValue("chama_id")
		chamaID, err := strconv.ParseInt(chamaIDStr, 10, 64)
		if err != nil {
			fmt.Fprintln(w, "Invalid chama ID")
			return
		}

		identifier := r.FormValue("identifier")
		role := r.FormValue("role")
		if role == "" {
			role = "member"
		}

		err = InviteMember(db, chamaID, identifier, role)
		if err != nil {
			fmt.Fprintln(w, "Invite failed:", err)
			return
		}

		http.Redirect(w, r, "/chama/roster?chama_id="+chamaIDStr, http.StatusSeeOther)
	})
	http.HandleFunc("/chama/invitations", func(w http.ResponseWriter, r *http.Request) {
		memberID, err := getLoggedInMemberID(r, db)
		if err != nil {
			http.Redirect(w, r, "/login-page", http.StatusSeeOther)
			return
		}

		memberName, invitations, err := GetPendingInvitations(db, memberID)
		if err != nil {
			fmt.Fprintln(w, "Failed to load invitations:", err)
			return
		}

		data := struct {
			MemberName  string
			Invitations []PendingInvitation
		}{
			MemberName:  memberName,
			Invitations: invitations,
		}

		tmpl := template.Must(template.ParseFiles("invitations.html"))
		tmpl.Execute(w, data)
	})

	http.HandleFunc("/chama/respond", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			fmt.Fprintln(w, "Please submit this form using POST")
			return
		}

		memberID, err := getLoggedInMemberID(r, db)
		if err != nil {
			http.Redirect(w, r, "/login-page", http.StatusSeeOther)
			return
		}

		chamaIDStr := r.FormValue("chama_id")
		chamaID, err := strconv.ParseInt(chamaIDStr, 10, 64)
		if err != nil {
			fmt.Fprintln(w, "Invalid chama ID")
			return
		}

		accept := r.FormValue("accept") == "true"

		err = RespondToInvitation(db, chamaID, memberID, accept)
		if err != nil {
			fmt.Fprintln(w, "Failed to respond:", err)
			return
		}

		http.Redirect(w, r, "/chama/my-chamas", http.StatusSeeOther)
	})
	http.HandleFunc("/api/chama/summary", func(w http.ResponseWriter, r *http.Request) {
		memberID, err := getLoggedInMemberID(r, db)
		if err != nil {
			http.Error(w, `{"error":"not logged in"}`, http.StatusUnauthorized)
			return
		}

		chamaIDStr := r.URL.Query().Get("chama_id")
		chamaID, err := strconv.ParseInt(chamaIDStr, 10, 64)
		if err != nil {
			http.Error(w, `{"error":"invalid chama id"}`, http.StatusBadRequest)
			return
		}

		var chamaName string
		db.QueryRow("SELECT name FROM chamas WHERE id = ?", chamaID).Scan(&chamaName)

		cycle, err := GetCycleSummary(db, chamaID)
		if err != nil {
			http.Error(w, `{"error":"failed to load cycle"}`, http.StatusInternalServerError)

			return
		}
		user, err := GetUserCycleSummary(db, chamaID, memberID)
		if err != nil {
			http.Error(w, `{"error":"failed to load user summary"}`, http.StatusInternalServerError)
			return
		}

		response := map[string]interface{}{
			"chama_id":   chamaID,
			"chama_name": chamaName,
			"current_cycle": map[string]interface{}{
				"cycle_number":             cycle.CycleNumber,
				"due_date":                 cycle.DueDate,
				"target_amount_per_member": cycle.TargetAmount,
				"total_expected_pool":      cycle.TotalExpectedPool,
				"total_collected_pool":     cycle.TotalCollected,
				"active_recipient_name":    cycle.RecipientName,
				"members_paid_count":       cycle.MembersPaidCount,
				"total_active_members":     cycle.TotalActive,
			},
			"user_summary": map[string]interface{}{
				"user_id":                memberID,
				"has_paid_current_cycle": user.HasPaid,
				"queue_position":         user.QueuePosition,
				"expected_payout_date":   user.ExpectedPayoutDate,
				"estimated_lump_sum":     user.ExpectedLumpSum,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	fmt.Println("Server starting on http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
