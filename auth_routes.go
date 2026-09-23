package main

import (
    "database/sql"
    "encoding/json"
    "net/http"
    "net/url"
    "strings"
    "fmt"
)

func registerAuthRoutes(db *sql.DB) {
    http.HandleFunc("/auth/status", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        _, err := getLoggedInMemberID(r, db)
        _ = json.NewEncoder(w).Encode(map[string]bool{"authenticated": err == nil})
    })

    http.HandleFunc("/auth/login", func(w http.ResponseWriter, r *http.Request) {
        if r.Method == http.MethodGet {
            next := safeNextPath(r.URL.Query().Get("next"))
            target := "/login-page"
            if next != "" {
                target += "?next=" + url.QueryEscape(next)
            }
            http.Redirect(w, r, target, http.StatusSeeOther)
            return
        }
        if r.Method != http.MethodPost {
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
            return
        }
        if err := requireSameOrigin(r); err != nil {
            http.Error(w, "Invalid request origin", http.StatusForbidden)
            return
        }

        identifier := strings.TrimSpace(r.FormValue("identifier"))
        password := r.FormValue("password")
        member, err := CheckLogin(db, identifier, password)
        if err != nil {
            http.Error(w, "Invalid credentials", http.StatusUnauthorized)
            return
        }

        token, err := createSession(db, member.ID)
        if err != nil {
            http.Error(w, "Failed to create session", http.StatusInternalServerError)
            return
        }
        setSessionCookie(w, r, token)

        next := safeNextPath(r.FormValue("next"))
        if next == "" {
            next = "/dashboard"
        }
        http.Redirect(w, r, next, http.StatusSeeOther)
    })

    http.HandleFunc("/auth/signup", func(w http.ResponseWriter, r *http.Request) {
        if r.Method == http.MethodGet {
            next := safeNextPath(r.URL.Query().Get("next"))
            target := "/register-page"
            if next != "" {
                target += "?next=" + url.QueryEscape(next)
            }
            http.Redirect(w, r, target, http.StatusSeeOther)
            return
        }
        if r.Method != http.MethodPost {
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
            return
        }
        if err := requireSameOrigin(r); err != nil {
            http.Error(w, "Invalid request origin", http.StatusForbidden)
            return
        }

        name := strings.TrimSpace(r.FormValue("name"))
        phone := strings.TrimSpace(r.FormValue("phone"))
        password := r.FormValue("password")
        if name == "" || phone == "" || password == "" {
            http.Error(w, "Full name, phone number and password are required", http.StatusBadRequest)
            return
        }

        memberID, err := CreateMember(db, name, phone, "", password, "member", true)
        if err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }

        token, err := createSession(db, memberID)
        if err != nil {
            http.Error(w, "Failed to create session", http.StatusInternalServerError)
            return
        }
        setSessionCookie(w, r, token)

        next := safeNextPath(r.FormValue("next"))
        if next == "" {
            next = "/dashboard"
        }
        http.Redirect(w, r, next, http.StatusSeeOther)
    })
}


func requireChamaRole(db *sql.DB, r *http.Request, chamaID int64, allowed ...string) (int64, error) {
    memberID, err := getLoggedInMemberID(r, db)
    if err != nil {
        return 0, err
    }
    var role string
    if err := db.QueryRow(
        "SELECT role FROM chama_members WHERE chama_id = ? AND member_id = ? AND status = 'active'",
        chamaID, memberID,
    ).Scan(&role); err != nil {
        return 0, err
    }
    for _, allowedRole := range allowed {
        if role == allowedRole {
            return memberID, nil
        }
    }
    return 0, fmt.Errorf("forbidden")
}
