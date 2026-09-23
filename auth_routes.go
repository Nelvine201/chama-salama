package main

import (\n    "database/sql"
    "encoding/json"
    "net/http"
    "net/url"
    "strings"
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
