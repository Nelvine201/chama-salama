package main

import (
	"database/sql"
	"golang.org/x/crypto/bcrypt"
	"fmt"
)
type Member struct {
	
	ID  int64
	Name string
	Phone string
	Email string
	Role string
}
func CreateMember(db *sql.DB, name, phone, email, password, role string, termsAccepted bool,) (int64, error) {
	if err := validateMemberInput(name, phone, email, password); err != nil {
		return 0, err
	}

	if !termsAccepted {
		return 0, fmt.Errorf("you must accept the terms and conditions to register")
	}


	if phone != "" {
		exists, err := phoneExists(db, phone)
		if err != nil {
			return 0, err
		}
		if exists {
			return 0, fmt.Errorf("phone number already registered")
		}
	}

	if email != "" {
		exists, err := emailExists(db, email)
		if err != nil {
			return 0, err
		}
		if exists {
			return 0, fmt.Errorf("email already registered")
		}
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}

	const currentTermsVersion = "v1"

	result, err := db.Exec(
		"INSERT INTO members (name, phone, email, password_hash, role, terms_version, terms_accepted_at) VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)",
		name, phone, email, string(hashedPassword), role, currentTermsVersion,
	)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return id, nil
}
func validateMemberInput(name, phone, email, password string) error {
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if phone == "" && email == "" {
		return fmt.Errorf("phone or email is required")
	}
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	return nil
}
func phoneExists(db *sql.DB, phone string) (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM members WHERE phone = ?", phone).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
func emailExists(db *sql.DB, email string) (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM members WHERE email = ?", email).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
func getMemberByIdentifier(db *sql.DB, identifier string) (*Member, string, error) {
	var m Member
	var passwordHash string

	err := db.QueryRow(
		"SELECT id, name, phone, email, role, password_hash FROM members WHERE phone = ? OR email = ?",
		identifier, identifier,
	).Scan(&m.ID, &m.Name, &m.Phone, &m.Email, &m.Role, &passwordHash)

	if err != nil {
		return nil, "", err
	}

	return &m, passwordHash, nil
}
func GetAllMembers(db *sql.DB) ([]Member, error) {
	rows, err := db.Query("SELECT id, name, phone, email, role FROM members")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []Member
	for rows.Next() {
		var m Member
		err := rows.Scan(&m.ID, &m.Name, &m.Phone, &m.Email, &m.Role)
		if err != nil {
			return nil, err
		}
		members = append(members, m)
	}

	return members, nil
}
func CheckLogin(db *sql.DB, identifier, password string) (*Member, error) {
	member, passwordHash, err := getMemberByIdentifier(db, identifier)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password))
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	return member, nil
}
type MemberProfile struct {
	Name                  string
	PreferredFirstName    sql.NullString
	AvatarURL             sql.NullString
	Phone                 sql.NullString
	Email                 sql.NullString
	NationalID            sql.NullString
	Location              sql.NullString
	DefaultPayoutMethod   sql.NullString
	NextOfKin             sql.NullString
	NextOfKinRelationship sql.NullString
	NextOfKinIDNumber     sql.NullString
	NextOfKinPhone        sql.NullString
	NotifySMS             bool
	NotifyEmail           bool
}

func GetMemberProfile(db *sql.DB, memberID int64) (*MemberProfile, error) {
	var p MemberProfile
	err := db.QueryRow(
		`SELECT name, preferred_first_name, avatar_url, phone, email, national_id, location,
		 default_payout_method, next_of_kin, next_of_kin_relationship, next_of_kin_id_number,
		 next_of_kin_phone, notify_sms, notify_email
		 FROM members WHERE id = ?`,
		memberID,
	).Scan(&p.Name, &p.PreferredFirstName, &p.AvatarURL, &p.Phone, &p.Email, &p.NationalID,
		&p.Location, &p.DefaultPayoutMethod, &p.NextOfKin, &p.NextOfKinRelationship,
		&p.NextOfKinIDNumber, &p.NextOfKinPhone, &p.NotifySMS, &p.NotifyEmail)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func UpdateProfile(db *sql.DB, memberID int64, nationalID, location, nextOfKin, nextOfKinRelationship, nextOfKinIDNumber, nextOfKinPhone, preferredFirstName, defaultPayoutMethod string, notifySMS, notifyEmail bool) error {
	_, err := db.Exec(
		`UPDATE members SET national_id = ?, location = ?, next_of_kin = ?, next_of_kin_relationship = ?,
		 next_of_kin_id_number = ?, next_of_kin_phone = ?, preferred_first_name = ?, default_payout_method = ?,
		 notify_sms = ?, notify_email = ? WHERE id = ?`,
		nationalID, location, nextOfKin, nextOfKinRelationship, nextOfKinIDNumber, nextOfKinPhone,
		preferredFirstName, defaultPayoutMethod, notifySMS, notifyEmail, memberID,
	)
	return err
}