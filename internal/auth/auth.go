// Package auth handles accounts, passwords and login sessions.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	bcryptCost  = 12
	minPassword = 8
	maxPassword = 72 // bcrypt only accepts 72 bytes

	cookieName = "woa_session"
	sessionTTL = 14 * 24 * time.Hour

	RoleAdmin  = "admin"
	RolePlayer = "player"
)

var (
	ErrInvalidLogin    = errors.New("invalid username or password")
	ErrPasswordInvalid = fmt.Errorf("password must be %d to %d bytes long", minPassword, maxPassword)
	ErrUsernameInvalid = errors.New("username must be 3 to 32 characters: letters, numbers, - or _")
	ErrUsernameTaken   = errors.New("that username is already taken")
	ErrLastAdmin       = errors.New("the last active admin can't be removed or disabled")
	ErrNotFound        = errors.New("user not found")

	usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{2,31}$`)
)

// User is an account. The password hash is never loaded onto it.
type User struct {
	ID         int64
	Username   string
	Role       string
	MustChange bool
	Disabled   bool
	CreatedAt  time.Time
}

func (u *User) IsAdmin() bool { return u.Role == RoleAdmin }

// Service provides account and session operations on the database.
type Service struct {
	DB      *sql.DB
	Secure  bool // set the Secure cookie flag (when served over TLS)
	Limiter *Limiter

	dummyHash []byte
}

// New builds a Service. Secure should be true whenever the site is on TLS.
func New(db *sql.DB, secure bool) (*Service, error) {
	dummy, err := bcrypt.GenerateFromPassword([]byte(randomToken(16)), bcryptCost)
	if err != nil {
		return nil, err
	}
	return &Service{DB: db, Secure: secure, Limiter: NewLimiter(), dummyHash: dummy}, nil
}

func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// GeneratePassword returns a random temporary password.
func GeneratePassword() string { return randomToken(12) }

// NormalizeUsername lowercases and trims a username. Paths are lowercased by the
// server, so usernames are only ever stored in lowercase.
func NormalizeUsername(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// ValidatePassword enforces the length limits, in bytes as bcrypt counts them.
func ValidatePassword(p string) error {
	if len(p) < minPassword || len(p) > maxPassword {
		return ErrPasswordInvalid
	}
	return nil
}

func hashPassword(p string) (string, error) {
	if err := ValidatePassword(p); err != nil {
		return "", err
	}
	h, err := bcrypt.GenerateFromPassword([]byte(p), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// EnsureAdmin creates the first admin account when none exists. It returns the
// generated password when one had to be made up, for printing once.
func (s *Service) EnsureAdmin(username, password string) (string, error) {
	var count int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM users WHERE role = ?`, RoleAdmin).Scan(&count); err != nil {
		return "", err
	}
	if count > 0 {
		return "", nil
	}
	if username == "" {
		username = "admin"
	}
	generated := ""
	if password == "" {
		generated = GeneratePassword()
		password = generated
	}
	if _, err := s.CreateUser(username, password, RoleAdmin); err != nil {
		return "", fmt.Errorf("create admin: %w", err)
	}
	return generated, nil
}

// CreateUser adds an account that must change its password at first login.
func (s *Service) CreateUser(username, password, role string) (int64, error) {
	username = NormalizeUsername(username)
	if !usernamePattern.MatchString(username) {
		return 0, ErrUsernameInvalid
	}
	if role != RoleAdmin && role != RolePlayer {
		return 0, errors.New("unknown role")
	}
	hash, err := hashPassword(password)
	if err != nil {
		return 0, err
	}
	res, err := s.DB.Exec(`INSERT INTO users (username, password_hash, role, must_change_password, created_at) VALUES (?, ?, ?, 1, ?)`,
		username, hash, role, time.Now().Unix())
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return 0, ErrUsernameTaken
		}
		return 0, err
	}
	return res.LastInsertId()
}

// Authenticate checks a username and password. A dummy compare runs for unknown
// usernames so response time doesn't reveal which usernames exist.
func (s *Service) Authenticate(username, password string) (*User, error) {
	var hash string
	u := &User{}
	var created int64
	err := s.DB.QueryRow(`SELECT id, username, password_hash, role, must_change_password, disabled, created_at FROM users WHERE username = ?`,
		NormalizeUsername(username)).Scan(&u.ID, &u.Username, &hash, &u.Role, &u.MustChange, &u.Disabled, &created)
	if errors.Is(err, sql.ErrNoRows) {
		bcrypt.CompareHashAndPassword(s.dummyHash, []byte(password))
		return nil, ErrInvalidLogin
	}
	if err != nil {
		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil || u.Disabled {
		return nil, ErrInvalidLogin
	}
	u.CreatedAt = time.Unix(created, 0)
	return u, nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// StartSession creates a session and sets its cookie. Only the token's hash is
// stored, so a leaked database can't be used to log in.
func (s *Service) StartSession(w http.ResponseWriter, userID int64) error {
	now := time.Now()
	if _, err := s.DB.Exec(`DELETE FROM sessions WHERE expires_at <= ?`, now.Unix()); err != nil {
		return err
	}
	token := randomToken(32)
	expires := now.Add(sessionTTL)
	if _, err := s.DB.Exec(`INSERT INTO sessions (token_hash, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		hashToken(token), userID, now.Unix(), expires.Unix()); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: token, Path: "/", Expires: expires,
		HttpOnly: true, Secure: s.Secure, SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// EndSession deletes the request's session and clears its cookie.
func (s *Service) EndSession(w http.ResponseWriter, r *http.Request) error {
	var err error
	if c, cerr := r.Cookie(cookieName); cerr == nil {
		_, err = s.DB.Exec(`DELETE FROM sessions WHERE token_hash = ?`, hashToken(c.Value))
	}
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: s.Secure, SameSite: http.SameSiteLaxMode,
	})
	return err
}

// UserFromRequest resolves the session cookie to an active user, or nil.
func (s *Service) UserFromRequest(r *http.Request) *User {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return nil
	}
	u := &User{}
	var created int64
	err = s.DB.QueryRow(`SELECT u.id, u.username, u.role, u.must_change_password, u.disabled, u.created_at
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = ? AND s.expires_at > ? AND u.disabled = 0`,
		hashToken(c.Value), time.Now().Unix()).Scan(&u.ID, &u.Username, &u.Role, &u.MustChange, &u.Disabled, &created)
	if err != nil {
		return nil
	}
	u.CreatedAt = time.Unix(created, 0)
	return u
}

// ChangePassword replaces a user's own password after checking the current one,
// and revokes all their sessions. The caller starts a fresh session afterwards.
func (s *Service) ChangePassword(userID int64, current, next string) error {
	var hash string
	if err := s.DB.QueryRow(`SELECT password_hash FROM users WHERE id = ?`, userID).Scan(&hash); err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(current)) != nil {
		return errors.New("your current password is incorrect")
	}
	if current == next {
		return errors.New("your new password must be different")
	}
	return s.setPassword(userID, next, false)
}

// ResetPassword sets a temporary password and revokes the user's sessions.
func (s *Service) ResetPassword(userID int64, password string) error {
	return s.setPassword(userID, password, true)
}

func (s *Service) setPassword(userID int64, password string, mustChange bool) error {
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`UPDATE users SET password_hash = ?, must_change_password = ? WHERE id = ?`, hash, mustChange, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if _, err := tx.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID); err != nil {
		return err
	}
	return tx.Commit()
}

// SetDisabled enables or disables an account. Disabling revokes its sessions.
func (s *Service) SetDisabled(userID int64, disabled bool) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if disabled {
		if err := guardLastAdmin(tx, userID); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID); err != nil {
			return err
		}
	}
	res, err := tx.Exec(`UPDATE users SET disabled = ? WHERE id = ?`, disabled, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

// DeleteUser removes an account. Its sessions and characters cascade with it.
func (s *Service) DeleteUser(userID int64) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := guardLastAdmin(tx, userID); err != nil {
		return err
	}
	res, err := tx.Exec(`DELETE FROM users WHERE id = ?`, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

func guardLastAdmin(tx *sql.Tx, userID int64) error {
	var others int
	err := tx.QueryRow(`SELECT COUNT(*) FROM users WHERE role = ? AND disabled = 0 AND id <> ?`, RoleAdmin, userID).Scan(&others)
	if err != nil {
		return err
	}
	var role string
	err = tx.QueryRow(`SELECT role FROM users WHERE id = ?`, userID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if role == RoleAdmin && others == 0 {
		return ErrLastAdmin
	}
	return nil
}

// GetUser loads one account.
func (s *Service) GetUser(id int64) (*User, error) {
	u := &User{}
	var created int64
	err := s.DB.QueryRow(`SELECT id, username, role, must_change_password, disabled, created_at FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.Username, &u.Role, &u.MustChange, &u.Disabled, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	u.CreatedAt = time.Unix(created, 0)
	return u, nil
}

// ListUsers returns every account, admins first.
func (s *Service) ListUsers() ([]User, error) {
	rows, err := s.DB.Query(`SELECT id, username, role, must_change_password, disabled, created_at FROM users ORDER BY role, username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		var created int64
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.MustChange, &u.Disabled, &created); err != nil {
			return nil, err
		}
		u.CreatedAt = time.Unix(created, 0)
		out = append(out, u)
	}
	return out, rows.Err()
}

type contextKey struct{}

// CurrentUser returns the user set by RequireLogin.
func CurrentUser(r *http.Request) *User {
	u, _ := r.Context().Value(contextKey{}).(*User)
	return u
}

// SafeNext only allows redirecting to a local path, never another site.
func SafeNext(next string) string {
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.HasPrefix(next, "/\\") {
		return "/sheets"
	}
	return next
}

// RequireLogin sends anonymous visitors to /login and users with a temporary
// password to /account. Every other route stays closed until it's been changed.
func (s *Service) RequireLogin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := s.UserFromRequest(r)
		if u == nil {
			target := "/login"
			if r.Method == http.MethodGet {
				target += "?next=" + url.QueryEscape(r.URL.RequestURI())
			}
			http.Redirect(w, r, target, http.StatusSeeOther)
			return
		}
		if u.MustChange && r.URL.Path != "/account" && r.URL.Path != "/logout" {
			http.Redirect(w, r, "/account", http.StatusSeeOther)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), contextKey{}, u)))
	}
}

// RequireAdmin is RequireLogin plus a 404 for anyone but an admin, so the admin
// area isn't advertised to players.
func (s *Service) RequireAdmin(notFound http.HandlerFunc, next http.HandlerFunc) http.HandlerFunc {
	return s.RequireLogin(func(w http.ResponseWriter, r *http.Request) {
		if !CurrentUser(r).IsAdmin() {
			notFound(w, r)
			return
		}
		next(w, r)
	})
}
