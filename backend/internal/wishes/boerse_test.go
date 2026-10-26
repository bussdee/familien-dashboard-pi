package wishes

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"family-dashboard/backend/internal/auth"
	"family-dashboard/backend/internal/points"
	"github.com/go-chi/chi/v5"
)

const mama = 2

func angebotAufruf(h http.HandlerFunc, wer int, rolle string, body, id, oid string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r = auth.AlsPerson(r, wer, rolle, false)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	if oid != "" {
		rctx.URLParams.Add("oid", oid)
	}
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	h(rec, r)
	return rec
}

func angebot(t *testing.T, s *Service, wer int, rolle string, wunsch int, body string) int {
	t.Helper()
	rec := angebotAufruf(s.Offer, wer, rolle, body, strconv.Itoa(wunsch), "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("Angebot: %d %s", rec.Code, rec.Body)
	}
	var id int
	if err := s.db.QueryRow(`SELECT MAX(id) FROM wish_offers`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func guthaben(t *testing.T, db *sql.DB, user int) int {
	t.Helper()
	b, err := points.Balance(db, user)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func gesamt(t *testing.T, db *sql.DB, user int) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COALESCE(SUM(points),0) FROM point_events WHERE user_id = ?`, user).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Der ganze Weg: Papa wünscht sich etwas, das Kind macht ein Angebot, Papa
// nimmt an, das Kind meldet "erledigt", Papa bestätigt — und zahlt an das Kind.
func TestEltern_wuenschen_Kind_erfuellt(t *testing.T) {
	db := testDB(t)
	s := NewService(db)
	punkte(t, db, papa, 100)

	wunsch := wunschAnlegen(t, s, papa, "admin", `{"title":"Frühstück ans Bett"}`)
	// Das Kind sieht den Wunsch der Eltern.
	if l := liste(t, s, kind, "member", false); len(l) != 1 || l[0].Mine {
		t.Fatalf("Kind sieht: %+v", l)
	}

	o := angebot(t, s, kind, "member", wunsch, `{"points":30,"note":"mit Saft"}`)
	id, oid := strconv.Itoa(wunsch), strconv.Itoa(o)

	// Das Angebot sieht Papa (der Wünschende) und das Kind (der Anbieter).
	if l := liste(t, s, papa, "admin", false); len(l[0].Offers) != 1 || l[0].Offers[0].Points != 30 || l[0].Offers[0].Note != "mit Saft" {
		t.Fatalf("Papa sieht: %+v", l[0])
	}
	if l := liste(t, s, kind, "member", false); len(l[0].Offers) != 1 {
		t.Fatalf("Anbieter sieht sein Angebot nicht: %+v", l[0])
	}

	// Nur der Wünschende nimmt an.
	if rec := angebotAufruf(s.Accept, kind, "member", "", id, oid); rec.Code != http.StatusForbidden {
		t.Errorf("Anbieter nimmt selbst an: %d", rec.Code)
	}
	if rec := angebotAufruf(s.Accept, papa, "admin", "", id, oid); rec.Code != http.StatusNoContent {
		t.Fatalf("Annehmen: %d %s", rec.Code, rec.Body)
	}
	if stand(t, db, wunsch) != StatusVergeben {
		t.Errorf("Wunsch sollte vergeben sein, war %s", stand(t, db, wunsch))
	}
	// Die Punkte sind zurückgelegt, aber noch nicht bezahlt.
	if guthaben(t, db, papa) != 70 || guthaben(t, db, kind) != 0 {
		t.Errorf("nach dem Annehmen: Papa %d, Kind %d", guthaben(t, db, papa), guthaben(t, db, kind))
	}

	if rec := angebotAufruf(s.Done, papa, "admin", "", id, oid); rec.Code != http.StatusForbidden {
		t.Errorf("Wünschender meldet Erfüllung: %d", rec.Code)
	}
	if rec := angebotAufruf(s.Done, kind, "member", "", id, oid); rec.Code != http.StatusNoContent {
		t.Fatalf("Melden: %d", rec.Code)
	}
	if rec := angebotAufruf(s.Confirm, kind, "member", "", id, oid); rec.Code != http.StatusForbidden {
		t.Errorf("Anbieter bestätigt selbst: %d", rec.Code)
	}
	if rec := angebotAufruf(s.Confirm, papa, "admin", "", id, oid); rec.Code != http.StatusNoContent {
		t.Fatalf("Bestätigen: %d %s", rec.Code, rec.Body)
	}

	if stand(t, db, wunsch) != StatusErfuellt {
		t.Errorf("Wunsch sollte erfüllt sein, war %s", stand(t, db, wunsch))
	}
	if guthaben(t, db, papa) != 70 || guthaben(t, db, kind) != 30 {
		t.Errorf("nach der Zahlung: Papa %d, Kind %d", guthaben(t, db, papa), guthaben(t, db, kind))
	}
	// Level und Rangliste bleiben, wie sie waren: Die Übertragung ist kein Verdienst.
	if gesamt(t, db, papa) != 100 || gesamt(t, db, kind) != 0 {
		t.Errorf("verdiente Punkte verändert: Papa %d, Kind %d", gesamt(t, db, papa), gesamt(t, db, kind))
	}
	// Ein zweites Bestätigen zahlt nicht noch einmal.
	if rec := angebotAufruf(s.Confirm, papa, "admin", "", id, oid); rec.Code != http.StatusConflict {
		t.Errorf("zweites Bestätigen: %d", rec.Code)
	}
	if guthaben(t, db, papa) != 70 {
		t.Error("doppelt bezahlt")
	}
}

func TestGuthabenReichtNicht(t *testing.T) {
	db := testDB(t)
	s := NewService(db)
	punkte(t, db, kind, 20)

	wunsch := wunschAnlegen(t, s, kind, "member", `{"title":"Kinoabend"}`)
	o := angebot(t, s, papa, "admin", wunsch, `{"points":50}`)
	id, oid := strconv.Itoa(wunsch), strconv.Itoa(o)

	rec := angebotAufruf(s.Accept, kind, "member", "", id, oid)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "fehlen noch 30") {
		t.Fatalf("erwartet 'fehlen noch 30', war %d %s", rec.Code, rec.Body)
	}
	if stand(t, db, wunsch) != StatusOffen {
		t.Error("der Wunsch muss offen bleiben")
	}

	// Ein günstigeres Angebot ersetzt das alte (ein offenes Angebot je Person).
	angebot(t, s, papa, "admin", wunsch, `{"points":15}`)
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM wish_offers WHERE wish_id = ?`, wunsch).Scan(&n)
	if n != 1 {
		t.Fatalf("erwartet 1 Angebot, waren %d", n)
	}
	if rec := angebotAufruf(s.Accept, kind, "member", "", id, oid); rec.Code != http.StatusNoContent {
		t.Fatalf("Annehmen: %d %s", rec.Code, rec.Body)
	}
}

// Rücklage: Wer ein Angebot angenommen hat, kann dieselben Punkte nicht noch
// einmal ausgeben — nicht für eine Belohnung, nicht für ein zweites Angebot.
func TestRuecklageSchuetztVorDoppeltemAusgeben(t *testing.T) {
	db := testDB(t)
	s := NewService(db)
	punkte(t, db, papa, 60)

	w1 := wunschAnlegen(t, s, papa, "admin", `{"title":"Eins"}`)
	w2 := wunschAnlegen(t, s, papa, "admin", `{"title":"Zwei"}`)
	o1 := angebot(t, s, kind, "member", w1, `{"points":40}`)
	o2 := angebot(t, s, mama, "admin", w2, `{"points":40}`)

	if rec := angebotAufruf(s.Accept, papa, "admin", "", strconv.Itoa(w1), strconv.Itoa(o1)); rec.Code != http.StatusNoContent {
		t.Fatalf("erstes Annehmen: %d", rec.Code)
	}
	if rec := angebotAufruf(s.Accept, papa, "admin", "", strconv.Itoa(w2), strconv.Itoa(o2)); rec.Code != http.StatusConflict {
		t.Errorf("zweites Annehmen sollte am Guthaben scheitern: %d %s", rec.Code, rec.Body)
	}
}

func TestZurueckziehenUndAblehnen(t *testing.T) {
	db := testDB(t)
	s := NewService(db)
	punkte(t, db, kind, 100)

	wunsch := wunschAnlegen(t, s, kind, "member", `{"title":"Eis"}`)
	id := strconv.Itoa(wunsch)

	// Ablehnen: nur der Wünschende.
	o := angebot(t, s, papa, "admin", wunsch, `{"points":10}`)
	if rec := angebotAufruf(s.Decline, papa, "admin", "", id, strconv.Itoa(o)); rec.Code != http.StatusForbidden {
		t.Errorf("Anbieter lehnt eigenes Angebot ab: %d", rec.Code)
	}
	if rec := angebotAufruf(s.Decline, kind, "member", "", id, strconv.Itoa(o)); rec.Code != http.StatusNoContent {
		t.Fatalf("Ablehnen: %d", rec.Code)
	}

	// Zurückziehen eines angenommenen Angebots stellt den Wunsch wieder her
	// und gibt die Rücklage frei.
	o = angebot(t, s, papa, "admin", wunsch, `{"points":40}`)
	if rec := angebotAufruf(s.Accept, kind, "member", "", id, strconv.Itoa(o)); rec.Code != http.StatusNoContent {
		t.Fatalf("Annehmen: %d", rec.Code)
	}
	if guthaben(t, db, kind) != 60 {
		t.Fatalf("Rücklage: %d", guthaben(t, db, kind))
	}
	if rec := angebotAufruf(s.WithdrawOffer, kind, "member", "", id, strconv.Itoa(o)); rec.Code != http.StatusForbidden {
		t.Errorf("Wünschender zieht fremdes Angebot zurück: %d", rec.Code)
	}
	if rec := angebotAufruf(s.WithdrawOffer, papa, "admin", "", id, strconv.Itoa(o)); rec.Code != http.StatusNoContent {
		t.Fatalf("Zurückziehen: %d", rec.Code)
	}
	if stand(t, db, wunsch) != StatusOffen || guthaben(t, db, kind) != 100 {
		t.Errorf("danach: %s, Guthaben %d", stand(t, db, wunsch), guthaben(t, db, kind))
	}

	// Ein bezahltes Angebot lässt sich nicht mehr zurückholen.
	o = angebot(t, s, papa, "admin", wunsch, `{"points":40}`)
	_ = angebotAufruf(s.Accept, kind, "member", "", id, strconv.Itoa(o))
	_ = angebotAufruf(s.Confirm, kind, "member", "", id, strconv.Itoa(o))
	if rec := angebotAufruf(s.WithdrawOffer, papa, "admin", "", id, strconv.Itoa(o)); rec.Code != http.StatusConflict {
		t.Errorf("bezahlt und zurückgezogen: %d", rec.Code)
	}
	if rec := angebotAufruf(s.Decline, kind, "member", "", id, strconv.Itoa(o)); rec.Code != http.StatusConflict {
		t.Errorf("bezahlt und abgelehnt: %d", rec.Code)
	}
	if guthaben(t, db, kind) != 60 || guthaben(t, db, papa) != 40 {
		t.Errorf("Guthaben: Kind %d, Papa %d", guthaben(t, db, kind), guthaben(t, db, papa))
	}
}

func TestAngebotRegeln(t *testing.T) {
	db := testDB(t)
	s := NewService(db)
	wunsch := wunschAnlegen(t, s, kind, "member", `{"title":"Roller"}`)
	id := strconv.Itoa(wunsch)

	for name, c := range map[string]struct {
		wer   int
		rolle string
		body  string
		code  int
	}{
		"eigener Wunsch":  {kind, "member", `{"points":5}`, http.StatusConflict},
		"negativ":         {papa, "admin", `{"points":-1}`, http.StatusBadRequest},
		"zu viel":         {papa, "admin", `{"points":999999}`, http.StatusBadRequest},
		"Hinweis zu lang": {papa, "admin", `{"points":5,"note":"` + strings.Repeat("x", 300) + `"}`, http.StatusBadRequest},
		"kein JSON":       {papa, "admin", `nein`, http.StatusBadRequest},
		"geschenkt (0)":   {papa, "admin", `{"points":0}`, http.StatusCreated},
	} {
		if rec := angebotAufruf(s.Offer, c.wer, c.rolle, c.body, id, ""); rec.Code != c.code {
			t.Errorf("%s: erwartet %d, war %d (%s)", name, c.code, rec.Code, rec.Body)
		}
	}
	if rec := angebotAufruf(s.Offer, papa, "admin", `{"points":5}`, "9999", ""); rec.Code != http.StatusNotFound {
		t.Errorf("unbekannter Wunsch: %d", rec.Code)
	}

	// Fremde erfahren nichts von einem Angebot, mit dem sie nichts zu tun haben.
	o := strconv.Itoa(angebotsID(t, db, wunsch))
	if rec := angebotAufruf(s.Accept, mama, "admin", "", id, o); rec.Code != http.StatusNotFound {
		t.Errorf("Fremde nimmt an: %d", rec.Code)
	}
	if rec := angebotAufruf(s.Accept, papa, "admin", "", "99999", o); rec.Code != http.StatusNotFound {
		t.Errorf("falscher Wunsch zum Angebot: %d", rec.Code)
	}
}

func angebotsID(t *testing.T, db *sql.DB, wunsch int) int {
	t.Helper()
	var id int
	if err := db.QueryRow(`SELECT id FROM wish_offers WHERE wish_id = ? ORDER BY id DESC LIMIT 1`, wunsch).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// Angebote sind nicht für jeden sichtbar: Der Wünschende und die Eltern sehen
// alle, ein Anbieter nur seine eigenen, Dritte nur die Zahl.
func TestAngeboteSichtbarkeit(t *testing.T) {
	db := testDB(t)
	s := NewService(db)
	if _, err := db.Exec(`INSERT INTO users (id, name, color, pin_hash, role, avatar_emoji) VALUES (4, 'Zweit', '#fff', 'x', 'member', '🙂')`); err != nil {
		t.Fatal(err)
	}
	wunsch := wunschAnlegen(t, s, kind, "member", `{"title":"Kino"}`)
	angebot(t, s, papa, "admin", wunsch, `{"points":10}`)
	angebot(t, s, 4, "member", wunsch, `{"points":5}`)

	sehen := func(wer int, rolle string) Wish {
		l := liste(t, s, wer, rolle, false)
		if len(l) != 1 {
			t.Fatalf("%d sieht %d Wünsche", wer, len(l))
		}
		return l[0]
	}
	if w := sehen(kind, "member"); len(w.Offers) != 2 {
		t.Errorf("der Wünschende sieht %d Angebote", len(w.Offers))
	}
	if w := sehen(4, "member"); len(w.Offers) != 1 || w.Offers[0].OffererID != 4 || w.OfferCount != 2 {
		t.Errorf("Anbieter sieht %+v (Zahl %d)", w.Offers, w.OfferCount)
	}
	if w := sehen(mama, "admin"); len(w.Offers) != 2 {
		t.Errorf("Eltern sehen %d Angebote", len(w.Offers))
	}
	// Das Wandgerät bekommt gar nichts.
	rec := httptest.NewRecorder()
	r := auth.AlsPerson(httptest.NewRequest(http.MethodGet, "/", nil), 0, "device", true)
	s.List(rec, r)
	var leer []Wish
	_ = json.Unmarshal(rec.Body.Bytes(), &leer)
	if len(leer) != 0 {
		t.Errorf("Wandgerät sieht %d Wünsche", len(leer))
	}
}

// Ein vergebener Wunsch lässt sich nicht mehr löschen oder neu bepreisen.
func TestVergebenerWunschIstGeschuetzt(t *testing.T) {
	db := testDB(t)
	s := NewService(db)
	punkte(t, db, kind, 50)
	wunsch := wunschAnlegen(t, s, kind, "member", `{"title":"Zelten"}`)
	o := angebot(t, s, papa, "admin", wunsch, `{"points":20}`)
	_ = angebotAufruf(s.Accept, kind, "member", "", strconv.Itoa(wunsch), strconv.Itoa(o))

	if rec := aufruf(s.Withdraw, kind, "member", false, "", strconv.Itoa(wunsch)); rec.Code != http.StatusConflict {
		t.Errorf("vergebenen Wunsch zurücknehmen: %d", rec.Code)
	}
	if rec := aufruf(s.Decide, papa, "admin", false, `{"cost":10}`, strconv.Itoa(wunsch)); rec.Code != http.StatusConflict {
		t.Errorf("vergebenen Wunsch bepreisen: %d", rec.Code)
	}
}

// Die Rangliste und das Guthaben-Konto rechnen dasselbe: Wer bezahlt hat,
// steht mit weniger Guthaben da, wer bekommen hat, mit mehr — Punkte und
// Platz ändern sich nicht.
func TestRanglisteStimmtMitBalanceUeberein(t *testing.T) {
	db := testDB(t)
	s := NewService(db)
	punkte(t, db, papa, 100)
	punkte(t, db, kind, 10)

	wunsch := wunschAnlegen(t, s, papa, "admin", `{"title":"Kaffee"}`)
	o := angebot(t, s, kind, "member", wunsch, `{"points":25}`)
	id, oid := strconv.Itoa(wunsch), strconv.Itoa(o)
	_ = angebotAufruf(s.Accept, papa, "admin", "", id, oid)

	vorher, err := points.NewService(db).Leaderboard()
	if err != nil {
		t.Fatal(err)
	}
	for _, sc := range vorher {
		if sc.ID == papa && (sc.Balance != 75 || sc.Spent != 25 || sc.TotalPoints != 100) {
			t.Errorf("Papa mit Rücklage: %+v", sc)
		}
	}

	_ = angebotAufruf(s.Confirm, papa, "admin", "", id, oid)
	nachher, _ := points.NewService(db).Leaderboard()
	for _, sc := range nachher {
		b, _ := points.Balance(db, sc.ID)
		if sc.Balance != b {
			t.Errorf("%s: Rangliste %d, Balance %d", sc.Name, sc.Balance, b)
		}
		switch sc.ID {
		case papa:
			if sc.TotalPoints != 100 || sc.Balance != 75 || sc.Rank != 1 {
				t.Errorf("Papa danach: %+v", sc)
			}
		case kind:
			if sc.TotalPoints != 10 || sc.Received != 25 || sc.Balance != 35 || sc.Rank != 2 {
				t.Errorf("Kind danach: %+v", sc)
			}
		}
	}
}
