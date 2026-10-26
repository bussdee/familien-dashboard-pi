package wishes

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"family-dashboard/backend/internal/auth"
	"family-dashboard/backend/internal/points"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// Die Wunschbörse.
//
//	Wunsch (offen) ──Angebot von B──▶ Angebot (offen)
//	Angebot ──A nimmt an──▶ angenommen   (Wunsch: vergeben, Punkte zurückgelegt)
//	angenommen ──B meldet "erledigt"──▶ gemeldet
//	angenommen/gemeldet ──A bestätigt──▶ erledigt   (A zahlt an B, Wunsch: erfüllt)
//	offen/angenommen/gemeldet ──A lehnt ab / B zieht zurück──▶ abgelehnt / zurückgezogen
//
// Gezahlt wird erst bei der Bestätigung. Bis dahin sind die Punkte des
// Wünschenden zurückgelegt, damit er sie nicht ein zweites Mal ausgibt.

const (
	AngebotOffen          = "offen"
	AngebotAngenommen     = "angenommen"
	AngebotGemeldet       = "gemeldet"
	AngebotErledigt       = "erledigt"
	AngebotAbgelehnt      = "abgelehnt"
	AngebotZurueckgezogen = "zurueckgezogen"

	maxAngebote = 10
)

// Offer ist ein Angebot, einen Wunsch zu erfüllen.
type Offer struct {
	ID           int        `json:"id"`
	WishID       int        `json:"wish_id"`
	OffererID    int        `json:"offerer_id"`
	OffererName  string     `json:"offerer_name"`
	OffererEmoji string     `json:"offerer_emoji"`
	Points       int        `json:"points"`
	Note         string     `json:"note"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	DecidedAt    *time.Time `json:"decided_at,omitempty"`
}

// angebotenZuordnen hängt jedem Wunsch die Angebote an, die die Person sehen
// darf: der Wünschende alle auf seine Wünsche, jeder seine eigenen, die Eltern
// alle.
func (s *Service) angebotenZuordnen(wuensche []Wish, me int, eltern bool) {
	if len(wuensche) == 0 {
		return
	}
	index := make(map[int]*Wish, len(wuensche))
	for i := range wuensche {
		index[wuensche[i].ID] = &wuensche[i]
	}
	rows, err := s.db.Query(`
		SELECT o.id, o.wish_id, o.offerer_id, COALESCE(u.name, ''), COALESCE(u.avatar_emoji, ''),
		       o.points, o.note, o.status, o.created_at, o.decided_at
		FROM wish_offers o LEFT JOIN users u ON u.id = o.offerer_id
		WHERE o.status NOT IN ('abgelehnt', 'zurueckgezogen')
		ORDER BY o.created_at, o.id`)
	if err != nil {
		log.Error().Err(err).Msg("Angebote konnten nicht gelesen werden")
		return
	}
	defer rows.Close()
	for rows.Next() {
		var o Offer
		var decided sql.NullTime
		if err := rows.Scan(&o.ID, &o.WishID, &o.OffererID, &o.OffererName, &o.OffererEmoji,
			&o.Points, &o.Note, &o.Status, &o.CreatedAt, &decided); err != nil {
			return
		}
		w, ok := index[o.WishID]
		if !ok {
			continue
		}
		if decided.Valid {
			t := decided.Time
			o.DecidedAt = &t
		}
		if eltern || w.UserID == me || o.OffererID == me {
			w.Offers = append(w.Offers, o)
		}
	}
}

type angebotAnfrage struct {
	Points int    `json:"points"`
	Note   string `json:"note"`
}

// Offer macht ein Angebot für einen fremden Wunsch. Wer schon ein offenes
// Angebot für diesen Wunsch hat, ändert damit seinen Preis.
func (s *Service) Offer(w http.ResponseWriter, r *http.Request) {
	me, ok := auth.GetUserID(r)
	if !ok {
		auth.HTTPError(w, http.StatusForbidden, "Zum Erfüllen bitte anmelden")
		return
	}
	wishID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige ID")
		return
	}
	var req angebotAnfrage
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&req); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	req.Note = strings.TrimSpace(req.Note)
	switch {
	case req.Points < 0:
		auth.HTTPError(w, http.StatusBadRequest, "Ein Angebot kann nicht weniger als 0 Punkte kosten")
		return
	case req.Points > maxCost:
		auth.HTTPError(w, http.StatusBadRequest, "So viele Punkte sammelt niemand")
		return
	case utf8.RuneCountInString(req.Note) > maxNote:
		auth.HTTPError(w, http.StatusBadRequest, "Der Hinweis ist zu lang (höchstens 200 Zeichen)")
		return
	}

	tx, err := s.db.Begin()
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	defer func() { _ = tx.Rollback() }()

	var owner int
	var status string
	err = tx.QueryRow(`SELECT user_id, status FROM wishes WHERE id = ?`, wishID).Scan(&owner, &status)
	if errors.Is(err, sql.ErrNoRows) {
		auth.HTTPError(w, http.StatusNotFound, "Wunsch nicht gefunden")
		return
	}
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if owner == me {
		auth.HTTPError(w, http.StatusConflict, "Den eigenen Wunsch kannst du nicht selbst erfüllen")
		return
	}
	if status != StatusOffen {
		auth.HTTPError(w, http.StatusConflict, "Für diesen Wunsch gibt es keine Angebote mehr")
		return
	}

	var bisher int
	err = tx.QueryRow(`SELECT id FROM wish_offers WHERE wish_id = ? AND offerer_id = ? AND status = 'offen'`,
		wishID, me).Scan(&bisher)
	switch {
	case err == nil:
		_, err = tx.Exec(`UPDATE wish_offers SET points = ?, note = ? WHERE id = ?`, req.Points, req.Note, bisher)
	case errors.Is(err, sql.ErrNoRows):
		var offene int
		_ = tx.QueryRow(`SELECT COUNT(*) FROM wish_offers WHERE wish_id = ? AND status = 'offen'`, wishID).Scan(&offene)
		if offene >= maxAngebote {
			auth.HTTPError(w, http.StatusConflict, "Für diesen Wunsch gibt es schon genug Angebote")
			return
		}
		_, err = tx.Exec(`INSERT INTO wish_offers (wish_id, offerer_id, points, note) VALUES (?, ?, ?, ?)`,
			wishID, me, req.Points, req.Note)
	}
	if err != nil {
		log.Error().Err(err).Msg("Angebot konnte nicht gespeichert werden")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	if err := tx.Commit(); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	w.WriteHeader(http.StatusCreated)
	auth.WriteJSON(w, map[string]any{"wish_id": wishID, "points": req.Points})
}

// ladeAngebot liest Angebot und Wunsch in einer Transaktion.
type angebotZeile struct {
	id, wishID, offerer, points int
	status                      string
	wishOwner                   int
	wishStatus                  string
	cost                        sql.NullInt64
	wishTitle                   string
}

func ladeAngebot(tx *sql.Tx, wishID, offerID int) (angebotZeile, error) {
	var a angebotZeile
	err := tx.QueryRow(`
		SELECT o.id, o.wish_id, o.offerer_id, o.points, o.status,
		       w.user_id, w.status, w.cost, w.title
		FROM wish_offers o JOIN wishes w ON w.id = o.wish_id
		WHERE o.id = ? AND o.wish_id = ?`, offerID, wishID).
		Scan(&a.id, &a.wishID, &a.offerer, &a.points, &a.status,
			&a.wishOwner, &a.wishStatus, &a.cost, &a.wishTitle)
	return a, err
}

// aktion führt eine der Änderungen an einem Angebot aus. wer prüft, ob die
// Person es darf; aendern macht die Änderung innerhalb der Transaktion.
func (s *Service) aktion(w http.ResponseWriter, r *http.Request,
	wer func(a angebotZeile, me int) (bool, string),
	aendern func(tx *sql.Tx, a angebotZeile, me int) (int, string),
) {
	me, ok := auth.GetUserID(r)
	if !ok {
		auth.HTTPError(w, http.StatusForbidden, "Dafür bitte anmelden")
		return
	}
	wishID, err1 := strconv.Atoi(chi.URLParam(r, "id"))
	offerID, err2 := strconv.Atoi(chi.URLParam(r, "oid"))
	if err1 != nil || err2 != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige ID")
		return
	}
	tx, err := s.db.Begin()
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	defer func() { _ = tx.Rollback() }()

	a, err := ladeAngebot(tx, wishID, offerID)
	if errors.Is(err, sql.ErrNoRows) {
		auth.HTTPError(w, http.StatusNotFound, "Angebot nicht gefunden")
		return
	}
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	// Wer nichts damit zu tun hat, erfährt nicht einmal, dass es das Angebot gibt.
	if a.wishOwner != me && a.offerer != me {
		auth.HTTPError(w, http.StatusNotFound, "Angebot nicht gefunden")
		return
	}
	if erlaubt, grund := wer(a, me); !erlaubt {
		auth.HTTPError(w, http.StatusForbidden, grund)
		return
	}
	if code, msg := aendern(tx, a, me); code != 0 {
		auth.HTTPError(w, code, msg)
		return
	}
	if err := tx.Commit(); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func istWuenschender(a angebotZeile, me int) (bool, string) {
	return a.wishOwner == me, "Das entscheidet, wer sich etwas wünscht"
}

func istAnbieter(a angebotZeile, me int) (bool, string) {
	return a.offerer == me, "Das kann nur machen, wer das Angebot gemacht hat"
}

// wunschZurueck stellt den Wunsch nach einer geplatzten Vergabe wieder her.
func wunschZurueck(tx *sql.Tx, a angebotZeile) error {
	neu := StatusOffen
	if a.cost.Valid {
		neu = StatusBepreist
	}
	_, err := tx.Exec(`UPDATE wishes SET status = ?, fulfilled_by = NULL WHERE id = ? AND status = 'vergeben'`, neu, a.wishID)
	return err
}

// Accept nimmt ein Angebot an. Die Punkte werden zurückgelegt, bezahlt wird
// erst, wenn der Wunsch erfüllt ist.
func (s *Service) Accept(w http.ResponseWriter, r *http.Request) {
	s.aktion(w, r, istWuenschender, func(tx *sql.Tx, a angebotZeile, me int) (int, string) {
		if a.status != AngebotOffen || a.wishStatus != StatusOffen {
			return http.StatusConflict, "Dieses Angebot lässt sich nicht mehr annehmen"
		}
		bal, err := points.Balance(tx, me)
		if err != nil {
			return http.StatusInternalServerError, "Server-Fehler"
		}
		if bal < a.points {
			return http.StatusConflict, "Dafür fehlen noch " + strconv.Itoa(a.points-bal) + " Punkte"
		}
		if _, err := tx.Exec(`UPDATE wish_offers SET status = 'angenommen', decided_at = CURRENT_TIMESTAMP WHERE id = ?`, a.id); err != nil {
			return http.StatusInternalServerError, "Server-Fehler"
		}
		// Die anderen Angebote erledigen sich damit.
		if _, err := tx.Exec(`UPDATE wish_offers SET status = 'abgelehnt', decided_at = CURRENT_TIMESTAMP
			WHERE wish_id = ? AND id != ? AND status = 'offen'`, a.wishID, a.id); err != nil {
			return http.StatusInternalServerError, "Server-Fehler"
		}
		if _, err := tx.Exec(`UPDATE wishes SET status = 'vergeben', fulfilled_by = ?, decided_at = CURRENT_TIMESTAMP WHERE id = ?`,
			a.offerer, a.wishID); err != nil {
			return http.StatusInternalServerError, "Server-Fehler"
		}
		return 0, ""
	})
}

// Decline lehnt ein offenes Angebot ab oder nimmt ein angenommenes zurück,
// solange noch nicht gezahlt ist.
func (s *Service) Decline(w http.ResponseWriter, r *http.Request) {
	s.aktion(w, r, istWuenschender, func(tx *sql.Tx, a angebotZeile, me int) (int, string) {
		switch a.status {
		case AngebotOffen:
		case AngebotAngenommen, AngebotGemeldet:
			if err := wunschZurueck(tx, a); err != nil {
				return http.StatusInternalServerError, "Server-Fehler"
			}
		default:
			return http.StatusConflict, "Dieses Angebot ist schon entschieden"
		}
		if _, err := tx.Exec(`UPDATE wish_offers SET status = 'abgelehnt', decided_at = CURRENT_TIMESTAMP WHERE id = ?`, a.id); err != nil {
			return http.StatusInternalServerError, "Server-Fehler"
		}
		return 0, ""
	})
}

// Withdraw zieht das eigene Angebot zurück, auch ein angenommenes — solange
// noch nicht gezahlt ist.
func (s *Service) WithdrawOffer(w http.ResponseWriter, r *http.Request) {
	s.aktion(w, r, istAnbieter, func(tx *sql.Tx, a angebotZeile, me int) (int, string) {
		switch a.status {
		case AngebotOffen:
		case AngebotAngenommen, AngebotGemeldet:
			if err := wunschZurueck(tx, a); err != nil {
				return http.StatusInternalServerError, "Server-Fehler"
			}
		default:
			return http.StatusConflict, "Dieses Angebot ist schon entschieden"
		}
		if _, err := tx.Exec(`UPDATE wish_offers SET status = 'zurueckgezogen', decided_at = CURRENT_TIMESTAMP WHERE id = ?`, a.id); err != nil {
			return http.StatusInternalServerError, "Server-Fehler"
		}
		return 0, ""
	})
}

// Done meldet, dass der Wunsch erfüllt ist. Gezahlt wird erst, wenn der
// Wünschende das bestätigt.
func (s *Service) Done(w http.ResponseWriter, r *http.Request) {
	s.aktion(w, r, istAnbieter, func(tx *sql.Tx, a angebotZeile, me int) (int, string) {
		if a.status != AngebotAngenommen {
			return http.StatusConflict, "Dieses Angebot ist nicht angenommen"
		}
		if _, err := tx.Exec(`UPDATE wish_offers SET status = 'gemeldet' WHERE id = ?`, a.id); err != nil {
			return http.StatusInternalServerError, "Server-Fehler"
		}
		return 0, ""
	})
}

// Confirm bestätigt die Erfüllung und zahlt: Die Punkte gehen vom Wünschenden
// an den, der erfüllt hat. Level und Rangliste bleiben unberührt.
func (s *Service) Confirm(w http.ResponseWriter, r *http.Request) {
	s.aktion(w, r, istWuenschender, func(tx *sql.Tx, a angebotZeile, me int) (int, string) {
		if a.status != AngebotAngenommen && a.status != AngebotGemeldet {
			return http.StatusConflict, "Dieses Angebot ist nicht angenommen"
		}
		// Erst die Rücklage auflösen, dann prüfen: Das Guthaben ohne dieses
		// Angebot muss noch reichen (Eltern können zwischendurch Punkte
		// abgezogen haben).
		if _, err := tx.Exec(`UPDATE wish_offers SET status = 'erledigt', decided_at = CURRENT_TIMESTAMP WHERE id = ?`, a.id); err != nil {
			return http.StatusInternalServerError, "Server-Fehler"
		}
		bal, err := points.Balance(tx, me)
		if err != nil {
			return http.StatusInternalServerError, "Server-Fehler"
		}
		if bal < a.points {
			return http.StatusConflict, "Dafür fehlen noch " + strconv.Itoa(a.points-bal) + " Punkte"
		}
		if _, err := tx.Exec(`INSERT INTO point_transfers (from_user, to_user, points, wish_id, offer_id) VALUES (?, ?, ?, ?, ?)`,
			me, a.offerer, a.points, a.wishID, a.id); err != nil {
			log.Error().Err(err).Msg("Übertragung konnte nicht gebucht werden")
			return http.StatusInternalServerError, "Server-Fehler"
		}
		if _, err := tx.Exec(`UPDATE wishes SET status = 'erfuellt', fulfilled_by = ?, decided_at = CURRENT_TIMESTAMP WHERE id = ?`,
			a.offerer, a.wishID); err != nil {
			return http.StatusInternalServerError, "Server-Fehler"
		}
		return 0, ""
	})
}
