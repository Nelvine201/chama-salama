package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strconv"
	"strings"
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
		_, err := getLoggedInMemberID(r, db)
		if err != nil {
			http.Redirect(w, r, "/login-page", http.StatusSeeOther)
			return
		}
		tmpl := template.Must(template.ParseFiles("contribute.html"))
		tmpl.Execute(w, nil)
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

		fmt.Fprintln(w, "Callback received")
	})
	http.HandleFunc("/stk-push", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			fmt.Fprintln(w, "Please submit this form using POST")
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

		result, err := sendStkPush(token, phone, amount)
		if err != nil {
			fmt.Fprintln(w, "STK push failed:", err)
			return
		}

		fmt.Fprintln(w, "STK push response:", result)
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

		balance, err := GetGroupBalance(db)
		if err != nil {
			fmt.Fprintln(w, "Failed to load dashboard:", err)
			return
		}

		settings, err := GetGroupSettings(db)
		if err != nil {
			fmt.Fprintln(w, "Failed to load group settings:", err)
			return
		}

		contributions, err := GetRecentContributions(db, 10)
		if err != nil {
			fmt.Fprintln(w, "Failed to load contributions:", err)
			return
		}

		data := struct {
			FirstName     string
			Balance       float64
			Frequency     string
			Amount        float64
			Contributions []ContributionWithMember
		}{
			FirstName:     firstName,
			Balance:       balance,
			Frequency:     settings.Frequency,
			Amount:        settings.ContributionAmount,
			Contributions: contributions,
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

		chamas, err := GetMemberChamas(db, memberID)
		if err != nil {
			fmt.Fprintln(w, "Failed to load chamas:", err)
			return
		}

		data := struct {
			Chamas []ChamaMembership
		}{
			Chamas: chamas,
		}

		tmpl := template.Must(template.ParseFiles("create-chama.html"))
		tmpl.Execute(w, data)
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
		_, err := getLoggedInMemberID(r, db)
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

		for _, m := range roster {
			fmt.Fprintln(w, m.MemberID, m.Name, m.Role)
		}
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

		invitations, err := GetPendingInvitations(db, memberID)
		if err != nil {
			fmt.Fprintln(w, "Failed to load invitations:", err)
			return
		}

		for _, inv := range invitations {
			fmt.Fprintln(w, inv.ChamaID, inv.ChamaName, inv.Role)
		}
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
