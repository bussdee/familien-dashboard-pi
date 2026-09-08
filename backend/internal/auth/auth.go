package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog/log"
)

const (
	CookieName     = "auth_token"
	maxPINFailures = 5
	lockoutWindow  = 5 * time.Minute
)

// PreferenceStore is the slice of the store this service needs for per-person
// settings such as the dashboard layout.
type PreferenceStore interface {
	UserSetting(userID int, key string) (string, bool, error)
	SetUserSetting(userID int, key, value string) error
}

type Service struct {
	db           *sql.DB
	prefs        PreferenceStore
	jwtSecret    []byte
	tokenTTL     time.Duration
	secureCookie bool
}

type User struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Color        string `json:"color"`
	Role         string `json:"role"`
	AvatarEmoji  string `json:"avatar_emoji"`
	PINIsDefault bool   `json:"pin_is_default"`
}

type Claims struct {
	UserID int    `json:"uid"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type LoginRequest struct {
	UserID int    `json:"user_id"`
	PIN    string `json:"pin"`
}

type LoginResponse struct {
	User User `json:"user"`
}

func NewService(db *sql.DB, prefs PreferenceStore, jwtSecret string, tokenTTL time.Duration, secureCookie bool) *Service {
	return &Service{
		db:           db,
		prefs:        prefs,
		jwtSecret:    []byte(jwtSecret),
		tokenTTL:     tokenTTL,
		secureCookie: secureCookie,
	}
}

// PublicUsers powers the login screen: names and avatars only, never hashes.
func (s *Service) PublicUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.listUsers()
	if err != nil {
		log.Error().Err(err).Msg("Failed to list users")
		httpError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	writeJSON(w, users)
}

func (s *Service) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}

	if !validPIN(req.PIN) {
		httpError(w, http.StatusUnauthorized, "PIN muss 4-stellig sein")
		return
	}
	if req.UserID <= 0 {
		httpError(w, http.StatusBadRequest, "Kein Benutzer gewählt")
		return
	}

	if locked, until := s.isLocked(req.UserID); locked {
		w.Header().Set("Retry-After", strconv.Itoa(int(time.Until(until).Seconds())+1))
		httpError(w, http.StatusTooManyRequests,
			fmt.Sprintf("Zu viele Fehlversuche. Gesperrt bis %s", until.Format("15:04:05")))
		return
	}

	var user User
	var pinHash string
	err := s.db.QueryRow(
		`SELECT id, name, color, pin_hash, role, avatar_emoji, pin_is_default
		 FROM users WHERE id = ?`, req.UserID,
	).Scan(&user.ID, &user.Name, &user.Color, &pinHash, &user.Role, &user.AvatarEmoji, &user.PINIsDefault)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpError(w, http.StatusUnauthorized, "Ungültige PIN")
			return
		}
		log.Error().Err(err).Msg("DB error on login")
		httpError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}

	match, err := argon2id.ComparePasswordAndHash(req.PIN, pinHash)
	if err != nil || !match {
		s.recordFailure(req.UserID)
		httpError(w, http.StatusUnauthorized, "Ungültige PIN")
		return
	}
	s.clearFailures(req.UserID)

	token, err := s.generateToken(user.ID, user.Role)
	if err != nil {
		log.Error().Err(err).Msg("Token generation failed")
		httpError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}

	s.setCookie(w, token, int(s.tokenTTL.Seconds()))
	writeJSON(w, LoginResponse{User: user})
}

func (s *Service) Logout(w http.ResponseWriter, r *http.Request) {
	s.setCookie(w, "", -1)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserID(r)
	if !ok {
		httpError(w, http.StatusUnauthorized, "Nicht angemeldet")
		return
	}

	var user User
	err := s.db.QueryRow(
		`SELECT id, name, color, role, avatar_emoji, pin_is_default FROM users WHERE id = ?`, userID,
	).Scan(&user.ID, &user.Name, &user.Color, &user.Role, &user.AvatarEmoji, &user.PINIsDefault)
	if err != nil {
		httpError(w, http.StatusNotFound, "Benutzer nicht gefunden")
		return
	}
	writeJSON(w, user)
}

// ChangeOwnPIN lets any signed-in member rotate their own PIN.
func (s *Service) ChangeOwnPIN(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserID(r)
	if !ok {
		httpError(w, http.StatusUnauthorized, "Nicht angemeldet")
		return
	}

	var req struct {
		CurrentPIN string `json:"current_pin"`
		NewPIN     string `json:"new_pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	if !validPIN(req.NewPIN) {
		httpError(w, http.StatusBadRequest, "Neue PIN muss 4-stellig sein")
		return
	}

	var current string
	if err := s.db.QueryRow("SELECT pin_hash FROM users WHERE id = ?", userID).Scan(&current); err != nil {
		httpError(w, http.StatusNotFound, "Benutzer nicht gefunden")
		return
	}
	if match, _ := argon2id.ComparePasswordAndHash(req.CurrentPIN, current); !match {
		httpError(w, http.StatusForbidden, "Aktuelle PIN ist falsch")
		return
	}

	hash, err := argon2id.CreateHash(req.NewPIN, argon2id.DefaultParams)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if _, err := s.db.Exec(
		"UPDATE users SET pin_hash = ?, pin_is_default = 0, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		hash, userID,
	); err != nil {
		httpError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------- Eigenes Profil

var emojiPattern = regexp.MustCompile(`^\s*$`)

// UpdateOwnProfile lets anyone change their own display name and avatar. The
// role stays out of reach — that is an admin decision.
func (s *Service) UpdateOwnProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserID(r)
	if !ok {
		httpError(w, http.StatusUnauthorized, "Nicht angemeldet")
		return
	}

	var req struct {
		Name        string `json:"name"`
		Color       string `json:"color"`
		AvatarEmoji string `json:"avatar_emoji"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}

	sets := []string{"updated_at = CURRENT_TIMESTAMP"}
	args := []any{}

	if name := strings.TrimSpace(req.Name); name != "" {
		if len([]rune(name)) > 40 {
			httpError(w, http.StatusBadRequest, "Name darf höchstens 40 Zeichen haben")
			return
		}
		sets = append(sets, "name = ?")
		args = append(args, name)
	}
	if color := strings.TrimSpace(req.Color); color != "" {
		if !validHexColor(color) {
			httpError(w, http.StatusBadRequest, "Farbe muss ein Hex-Wert sein, z. B. #3b82f6")
			return
		}
		sets = append(sets, "color = ?")
		args = append(args, color)
	}
	if emoji := strings.TrimSpace(req.AvatarEmoji); emoji != "" {
		// One or two code points; anything longer is a pasted word, not an avatar.
		if emojiPattern.MatchString(emoji) || len([]rune(emoji)) > 4 {
			httpError(w, http.StatusBadRequest, "Avatar muss ein einzelnes Symbol sein")
			return
		}
		sets = append(sets, "avatar_emoji = ?")
		args = append(args, emoji)
	}

	if len(sets) == 1 {
		httpError(w, http.StatusBadRequest, "Nichts zu ändern")
		return
	}

	args = append(args, userID)
	if _, err := s.db.Exec("UPDATE users SET "+strings.Join(sets, ", ")+" WHERE id = ?", args...); err != nil {
		log.Error().Err(err).Msg("Failed to update own profile")
		httpError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}

	s.Me(w, r)
}

func validHexColor(value string) bool {
	if len(value) != 7 || value[0] != '#' {
		return false
	}
	for _, c := range value[1:] {
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f', c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}

// GetPreference / SetPreference back the per-person dashboard layout. The value
// is opaque JSON: the server does not care what the UI stores in it.
func (s *Service) GetPreference(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserID(r)
	if !ok || s.prefs == nil {
		httpError(w, http.StatusUnauthorized, "Nicht angemeldet")
		return
	}

	key := chi.URLParam(r, "key")
	if !allowedPreference(key) {
		httpError(w, http.StatusBadRequest, "Unbekannte Einstellung")
		return
	}

	value, found, err := s.prefs.UserSetting(userID, key)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if !found {
		writeJSON(w, map[string]any{"key": key, "value": nil})
		return
	}

	var parsed any
	if err := json.Unmarshal([]byte(value), &parsed); err != nil {
		parsed = value
	}
	writeJSON(w, map[string]any{"key": key, "value": parsed})
}

func (s *Service) SetPreference(w http.ResponseWriter, r *http.Request) {
	userID, ok := GetUserID(r)
	if !ok || s.prefs == nil {
		httpError(w, http.StatusUnauthorized, "Nicht angemeldet")
		return
	}

	key := chi.URLParam(r, "key")
	if !allowedPreference(key) {
		httpError(w, http.StatusBadRequest, "Unbekannte Einstellung")
		return
	}

	var body struct {
		Value any `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}

	encoded, err := json.Marshal(body.Value)
	if err != nil {
		httpError(w, http.StatusBadRequest, "Wert lässt sich nicht speichern")
		return
	}
	// A runaway value would bloat the row; the layout is a few hundred bytes.
	if len(encoded) > 8192 {
		httpError(w, http.StatusRequestEntityTooLarge, "Einstellung ist zu groß")
		return
	}

	if err := s.prefs.SetUserSetting(userID, key, string(encoded)); err != nil {
		log.Error().Err(err).Msg("Failed to save preference")
		httpError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// allowedPreference keeps the key space closed, so this cannot become an
// arbitrary per-user key/value store reachable from the browser.
func allowedPreference(key string) bool {
	switch key {
	case "dashboard.layout":
		return true
	}
	return false
}

func (s *Service) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(CookieName)
		if err != nil {
			httpError(w, http.StatusUnauthorized, "Nicht angemeldet")
			return
		}

		claims, err := s.validateToken(cookie.Value)
		if err != nil {
			s.setCookie(w, "", -1)
			httpError(w, http.StatusUnauthorized, "Sitzung abgelaufen")
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
		ctx = context.WithValue(ctx, userRoleKey, claims.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Service) AdminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if role, ok := GetUserRole(r); !ok || role != "admin" {
			httpError(w, http.StatusForbidden, "Nur für Admins")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Service) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.listUsers()
	if err != nil {
		httpError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	writeJSON(w, users)
}

type userPayload struct {
	Name        string `json:"name"`
	Color       string `json:"color"`
	PIN         string `json:"pin"`
	Role        string `json:"role"`
	AvatarEmoji string `json:"avatar_emoji"`
}

func (s *Service) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req userPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		httpError(w, http.StatusBadRequest, "Name erforderlich")
		return
	}
	if !validPIN(req.PIN) {
		httpError(w, http.StatusBadRequest, "PIN muss 4-stellig sein")
		return
	}
	if req.Role != "admin" {
		req.Role = "member"
	}
	if req.Color == "" {
		req.Color = "#64748b"
	}
	if req.AvatarEmoji == "" {
		req.AvatarEmoji = "👤"
	}

	hash, err := argon2id.CreateHash(req.PIN, argon2id.DefaultParams)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}

	res, err := s.db.Exec(
		`INSERT INTO users (name, color, pin_hash, role, avatar_emoji, pin_is_default)
		 VALUES (?, ?, ?, ?, ?, 0)`,
		strings.TrimSpace(req.Name), req.Color, hash, req.Role, req.AvatarEmoji,
	)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create user")
		httpError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}

	id, _ := res.LastInsertId()
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, User{
		ID: int(id), Name: strings.TrimSpace(req.Name), Color: req.Color,
		Role: req.Role, AvatarEmoji: req.AvatarEmoji,
	})
}

func (s *Service) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		httpError(w, http.StatusBadRequest, "Ungültige ID")
		return
	}

	var req userPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}

	sets := []string{"updated_at = CURRENT_TIMESTAMP"}
	args := []any{}
	if strings.TrimSpace(req.Name) != "" {
		sets = append(sets, "name = ?")
		args = append(args, strings.TrimSpace(req.Name))
	}
	if req.Color != "" {
		sets = append(sets, "color = ?")
		args = append(args, req.Color)
	}
	if req.AvatarEmoji != "" {
		sets = append(sets, "avatar_emoji = ?")
		args = append(args, req.AvatarEmoji)
	}
	if req.Role == "admin" || req.Role == "member" {
		if req.Role == "member" {
			ok, err := s.hasOtherAdmin(id)
			if err != nil {
				httpError(w, http.StatusInternalServerError, "Server-Fehler")
				return
			}
			if !ok {
				httpError(w, http.StatusConflict, "Der letzte Admin kann nicht herabgestuft werden")
				return
			}
		}
		sets = append(sets, "role = ?")
		args = append(args, req.Role)
	}
	if req.PIN != "" {
		if !validPIN(req.PIN) {
			httpError(w, http.StatusBadRequest, "PIN muss 4-stellig sein")
			return
		}
		hash, err := argon2id.CreateHash(req.PIN, argon2id.DefaultParams)
		if err != nil {
			httpError(w, http.StatusInternalServerError, "Server-Fehler")
			return
		}
		sets = append(sets, "pin_hash = ?", "pin_is_default = 0")
		args = append(args, hash)
	}

	args = append(args, id)
	if _, err := s.db.Exec("UPDATE users SET "+strings.Join(sets, ", ")+" WHERE id = ?", args...); err != nil {
		log.Error().Err(err).Msg("Failed to update user")
		httpError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	s.clearFailures(id)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		httpError(w, http.StatusBadRequest, "Ungültige ID")
		return
	}
	if self, _ := GetUserID(r); self == id {
		httpError(w, http.StatusConflict, "Du kannst dich nicht selbst löschen")
		return
	}
	ok, err := s.hasOtherAdmin(id)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if !ok {
		httpError(w, http.StatusConflict, "Der letzte Admin kann nicht gelöscht werden")
		return
	}

	if _, err := s.db.Exec("DELETE FROM users WHERE id = ?", id); err != nil {
		httpError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) listUsers() ([]User, error) {
	rows, err := s.db.Query(
		`SELECT id, name, color, role, avatar_emoji, pin_is_default FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.Color, &u.Role, &u.AvatarEmoji, &u.PINIsDefault); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// hasOtherAdmin reports whether an admin other than excludeID exists, so the
// last admin can never be deleted or demoted.
func (s *Service) hasOtherAdmin(excludeID int) (bool, error) {
	var n int
	err := s.db.QueryRow(
		"SELECT COUNT(*) FROM users WHERE role = 'admin' AND id != ?", excludeID).Scan(&n)
	return n > 0, err
}

func (s *Service) isLocked(userID int) (bool, time.Time) {
	var until sql.NullTime
	err := s.db.QueryRow("SELECT locked_until FROM login_attempts WHERE user_id = ?", userID).Scan(&until)
	if err != nil || !until.Valid {
		return false, time.Time{}
	}
	if time.Now().Before(until.Time) {
		return true, until.Time
	}
	return false, time.Time{}
}

func (s *Service) recordFailure(userID int) {
	_, err := s.db.Exec(`
		INSERT INTO login_attempts (user_id, failures, locked_until)
		VALUES (?, 1, NULL)
		ON CONFLICT(user_id) DO UPDATE SET
			failures = login_attempts.failures + 1,
			locked_until = CASE
				WHEN login_attempts.failures + 1 >= ? THEN ?
				ELSE NULL
			END`,
		userID, maxPINFailures, time.Now().Add(lockoutWindow))
	if err != nil {
		log.Error().Err(err).Msg("Failed to record login failure")
	}
}

func (s *Service) clearFailures(userID int) {
	if _, err := s.db.Exec("DELETE FROM login_attempts WHERE user_id = ?", userID); err != nil {
		log.Debug().Err(err).Msg("Failed to clear login failures")
	}
}

func (s *Service) setCookie(w http.ResponseWriter, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.secureCookie,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
}

func (s *Service) generateToken(userID int, role string) (string, error) {
	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.tokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
}

func (s *Service) validateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{},
		func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return s.jwtSecret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}

func validPIN(pin string) bool {
	if len(pin) != 4 {
		return false
	}
	for _, c := range pin {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

type contextKey string

const (
	userIDKey   contextKey = "user_id"
	userRoleKey contextKey = "user_role"
)

func GetUserID(r *http.Request) (int, bool) {
	id, ok := r.Context().Value(userIDKey).(int)
	return id, ok
}

func GetUserRole(r *http.Request) (string, bool) {
	role, ok := r.Context().Value(userRoleKey).(string)
	return role, ok
}

// writeJSON and httpError keep every handler on one response shape, so the
// frontend can always read {"message": "..."} from a failure.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Error().Err(err).Msg("Failed to encode response")
	}
}

func httpError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": message})
}

// WriteJSON / HTTPError are the exported helpers other packages reuse.
func WriteJSON(w http.ResponseWriter, v any)                  { writeJSON(w, v) }
func HTTPError(w http.ResponseWriter, status int, msg string) { httpError(w, status, msg) }
