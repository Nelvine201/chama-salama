package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"io"
)

func startServer(db *sql.DB) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Chama Salama server is running")
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

		id, err := CreateMember(db, name, phone, email, password, role, termsAccepted)
		if err != nil {
			fmt.Fprintln(w, "Registration failed:", err)
			return
		}

		fmt.Fprintln(w, "Registered successfully! Member ID:", id)
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

		fmt.Fprintln(w, "Login successful! Welcome,", member.Name)
	})

	http.HandleFunc("/profile", func(w http.ResponseWriter, r *http.Request) {
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

		nationalID := r.FormValue("national_id")
		location := r.FormValue("location")
		nextOfKin := r.FormValue("next_of_kin")

		err = UpdateProfile(db, memberID, nationalID, location, nextOfKin)
		if err != nil {
			fmt.Fprintln(w, "Profile update failed:", err)
			return
		}

		fmt.Fprintln(w, "Profile updated successfully")
	})
	http.HandleFunc("/contribute", func(w http.ResponseWriter, r *http.Request) {
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

		id, err := RecordContribution(db, memberID, amount, paidOn, source)
		if err != nil {
			fmt.Fprintln(w, "Failed to record contribution:", err)
			return
		}

		fmt.Fprintln(w, "Contribution recorded successfully! ID:", id)
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

	fmt.Println("Server starting on http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}