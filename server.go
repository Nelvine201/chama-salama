package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strconv"
	"strings"
	"encoding/json"
)

func startServer(db *sql.DB) {
	http.Handle("/", http.FileServer(http.Dir("docs")))

	http.HandleFunc("/register-page", func(w http.ResponseWriter, r *http.Request) {
		tmpl := template.Must(template.ParseFiles("register.html"))
		tmpl.Execute(w, nil)
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
		tmpl.Execute(w, nil)
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
		if err != nil {
			fmt.Fprintln(w, "Failed to load chamas:", err)
			return
		}
		if len(chamas) == 0 {
			http.Redirect(w, r, "/chama/my-chamas", http.StatusSeeOther)
			return
		}

		chamaIDStr := r.URL.Query().Get("chama_id")
		var activeChamaID int64
		if chamaIDStr != "" {
			activeChamaID, _ = strconv.ParseInt(chamaIDStr, 10, 64)
		}
		found := false
		for _, c := range chamas {
			if activeChamaID == 0 || c.ChamaID == activeChamaID {
				activeChamaID = c.ChamaID
				found = true
				break
			}
		}
		if !found {
			activeChamaID = chamas[0].ChamaID
		}

		amount, _, _ := GetGroupSettingsForChama(db, activeChamaID)

		var phone string
		if profile, err := GetMemberProfile(db, memberID); err == nil && profile.Phone.Valid {
			phone = profile.Phone.String
		}

		var amountVal any
		if amount > 0 {
			if amount == float64(int64(amount)) {
				amountVal = int64(amount)
			} else {
				amountVal = amount
			}
		}

		data := struct {
			ChamaID int64
			Amount  any
			Phone   string
			Message string
		}{
			ChamaID: activeChamaID,
			Amount:  amountVal,
			Phone:   phone,
			Message: r.URL.Query().Get("message"),
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
			http.Redirect(w, r, "/chama/my-chamas", http.StatusSeeOther)
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

		var paidCount int
		db.QueryRow("SELECT COUNT(*) FROM contributions WHERE chama_id = ? AND member_id = ? AND status = 'synced'", activeChamaID, memberID).Scan(&paidCount)
		personalPaid := paidCount > 0

		isAdminOrTreasurer := activeRole == "admin" || activeRole == "treasurer"

		data := struct {
			FirstName           string
			ActiveChamaID       int64
			ActiveChamaName     string
			ActiveRole          string
			Chamas              []ChamaMembership
			Balance             float64
			Amount              float64
			Frequency           string
			Contributions       []ContributionWithMember
			RecipientName       string
			QueuePosition       int
			PersonalPaid        bool
			IsAdminOrTreasurer  bool
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

	http.HandleFunc("/chama/create", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			fmt.Fprintln(w, "Please submit this form using POST")
			return
		}

		memberID, err := getLoggedInMemberID(r, db)
		if err != nil {
			http.Redirect(w, r, "/login-page", http.StatusSeeOther)
			return
		}

		name := r.FormValue("name")
		_, err = CreateChama(db, name, memberID)
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

	fmt.Println("Server starting on http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
