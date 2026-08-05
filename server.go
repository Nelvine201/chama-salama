package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
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


	fmt.Println("Server starting on http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}