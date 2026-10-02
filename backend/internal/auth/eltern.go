package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/alexedwards/argon2id"
	"github.com/rs/zerolog/log"
)

// Die Eltern-Freigabe am Wandgerät.
//
// Im Familien-Modus ist niemand angemeldet — das ist der Sinn der Sache. Ein
// paar Handgriffe gehören aber den Eltern: Punkte von Hand vergeben, eine
// erledigte Aufgabe bestätigen. Dafür musste man sich bisher am Tablet richtig
// anmelden und hinterher wieder abmelden; wer das vergass, liess das Tablet als
// Papa angemeldet im Flur hängen.
//
// Jetzt schickt das Wandgerät für genau diese Wege die Kennung eines
// Elternteils und dessen PIN mit. Geprüft wird bei jeder Anfrage, mit
// derselben Sperre nach fünf Fehlversuchen wie bei der Anmeldung. Es entsteht
// keine Sitzung: Die Oberfläche hält die PIN zwei Minuten im Arbeitsspeicher
// und vergisst sie dann.
const (
	ElternIDHeader  = "X-Eltern-Id"
	ElternPINHeader = "X-Eltern-Pin"
)

var (
	errElternUnbekannt = errors.New("unbekannt")
	errElternKeinAdmin = errors.New("kein elternteil")
	errElternPIN       = errors.New("falsche pin")
)

// pruefeEltern sieht nach, ob die PIN zu einem Administrator passt.
func (s *Service) pruefeEltern(userID int, pin string) (gesperrtBis string, err error) {
	if userID <= 0 || !validPIN(pin) {
		return "", errElternPIN
	}
	if locked, until := s.isLocked(userID); locked {
		return until.Format("15:04"), nil
	}

	var role, hash string
	err = s.db.QueryRow(`SELECT role, pin_hash FROM users WHERE id = ?`, userID).Scan(&role, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errElternUnbekannt
	}
	if err != nil {
		return "", err
	}
	if role != "admin" {
		return "", errElternKeinAdmin
	}

	match, err := argon2id.ComparePasswordAndHash(pin, hash)
	if err != nil || !match {
		s.recordFailure(userID)
		return "", errElternPIN
	}
	s.clearFailures(userID)
	return "", nil
}

// ElternMiddleware lässt einen Administrator durch — und am Wandgerät auch
// eine Anfrage, die eine gültige Eltern-PIN mitbringt. Der Handler dahinter
// sieht dann den Elternteil als angemeldete Person: Buchungen stehen unter
// seinem Namen, nicht unter „Wandgerät".
func (s *Service) ElternMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if role, ok := GetUserRole(r); ok && role == "admin" && !IsDevice(r) {
			next.ServeHTTP(w, r)
			return
		}
		if !IsDevice(r) {
			httpError(w, http.StatusForbidden, "Nur für Eltern")
			return
		}

		id, _ := strconv.Atoi(strings.TrimSpace(r.Header.Get(ElternIDHeader)))
		pin := strings.TrimSpace(r.Header.Get(ElternPINHeader))
		if id == 0 && pin == "" {
			// 403 und nicht 401: Ein 401 schickte die Oberfläche auf den
			// Anmeldebildschirm — am Wandgerät genau das Falsche.
			httpError(w, http.StatusForbidden, "Dafür braucht es die PIN eines Elternteils")
			return
		}

		gesperrt, err := s.pruefeEltern(id, pin)
		switch {
		case gesperrt != "":
			httpError(w, http.StatusTooManyRequests,
				fmt.Sprintf("Zu viele Fehlversuche. Gesperrt bis %s", gesperrt))
			return
		case errors.Is(err, errElternKeinAdmin):
			httpError(w, http.StatusForbidden, "Das geht nur mit der PIN eines Elternteils")
			return
		case errors.Is(err, errElternPIN), errors.Is(err, errElternUnbekannt):
			httpError(w, http.StatusForbidden, "Die PIN stimmt nicht")
			return
		case err != nil:
			log.Error().Err(err).Msg("Eltern-PIN konnte nicht geprüft werden")
			httpError(w, http.StatusInternalServerError, "Server-Fehler")
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, id)
		ctx = context.WithValue(ctx, userRoleKey, "admin")
		ctx = context.WithValue(ctx, deviceKey, false)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ElternPruefen beantwortet nur: passt die PIN? Die Oberfläche fragt einmal
// und hält die Freigabe danach zwei Minuten lang, statt bei jedem Tipp
// erneut nach der PIN zu fragen.
func (s *Service) ElternPruefen(w http.ResponseWriter, r *http.Request) {
	id, _ := GetUserID(r)
	var name string
	_ = s.db.QueryRow(`SELECT name FROM users WHERE id = ?`, id).Scan(&name)
	writeJSON(w, map[string]any{"id": id, "name": name})
}
