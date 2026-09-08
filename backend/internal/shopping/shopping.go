package shopping

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"family-dashboard/backend/internal/auth"
	"family-dashboard/backend/internal/points"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 45 * time.Second
)

// The shopping list is deliberately shared: every family member sees and edits
// the same list. user_id only records who added an item.
type Item struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Quantity  string    `json:"quantity"`
	Category  string    `json:"category"`
	Checked   bool      `json:"checked"`
	UserID    *int      `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateRequest struct {
	Name     string `json:"name"`
	Quantity string `json:"quantity"`
	Category string `json:"category"`
}

type UpdateRequest struct {
	Name     *string `json:"name"`
	Quantity *string `json:"quantity"`
	Category *string `json:"category"`
	Checked  *bool   `json:"checked"`
}

type Service struct {
	db       *sql.DB
	hub      *Hub
	upgrader websocket.Upgrader
}

func NewService(db *sql.DB, allowedOrigins []string) *Service {
	return &Service{
		db:  db,
		hub: newHub(),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     originChecker(allowedOrigins),
		},
	}
}

// originChecker rejects cross-site WebSocket handshakes. The connection is
// authenticated by a cookie, so a foreign page must not be able to open one.
func originChecker(allowed []string) func(*http.Request) bool {
	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // non-browser client (curl, native app)
		}

		for _, a := range allowed {
			if a == "*" || strings.EqualFold(a, origin) {
				return true
			}
		}

		u, err := url.Parse(origin)
		if err != nil || u.Host == "" {
			log.Warn().Str("origin", origin).Msg("Rejected unparsable WebSocket origin")
			return false
		}

		// Go moves the Host header into r.Host, so r.Header.Get("Host") is
		// always empty here. Behind the reverse proxy the browser sees the
		// proxy's host, which nginx forwards verbatim (including the port).
		for _, host := range []string{r.Host, r.Header.Get("X-Forwarded-Host")} {
			if host != "" && strings.EqualFold(host, u.Host) {
				return true
			}
		}

		log.Warn().Str("origin", origin).Str("host", r.Host).Msg("Rejected WebSocket origin")
		return false
	}
}

func (s *Service) Start(ctx context.Context) {
	s.hub.run(ctx)
}

// ---------------------------------------------------------------- WebSocket

type Hub struct {
	mu         sync.RWMutex
	clients    map[*Client]struct{}
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
}

type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

func newHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]struct{}),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			h.mu.Lock()
			for c := range h.clients {
				close(c.send)
			}
			h.clients = map[*Client]struct{}{}
			h.mu.Unlock()
			return

		case c := <-h.register:
			h.mu.Lock()
			h.clients[c] = struct{}{}
			n := len(h.clients)
			h.mu.Unlock()
			log.Debug().Int("clients", n).Msg("Shopping WS client connected")

		case c := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			h.mu.RLock()
			targets := make([]*Client, 0, len(h.clients))
			for c := range h.clients {
				targets = append(targets, c)
			}
			h.mu.RUnlock()

			for _, c := range targets {
				select {
				case c.send <- msg:
				default:
					// Slow client: drop it rather than block the hub.
					h.unregister <- c
				}
			}
		}
	}
}

func (s *Service) WebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Debug().Err(err).Msg("WebSocket upgrade failed")
		return
	}

	client := &Client{hub: s.hub, conn: conn, send: make(chan []byte, 64)}
	s.hub.register <- client

	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(512)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Debug().Err(err).Msg("WebSocket read error")
			}
			return
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (s *Service) broadcast(action string, item Item) {
	msg, err := json.Marshal(map[string]any{"action": action, "item": item})
	if err != nil {
		return
	}
	select {
	case s.hub.broadcast <- msg:
	default:
		log.Warn().Msg("Shopping broadcast buffer full, dropping event")
	}
}

// ------------------------------------------------------------------ Handlers

const itemColumns = `id, name, quantity, category, checked, user_id, created_at, updated_at`

func (s *Service) List(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(
		`SELECT ` + itemColumns + ` FROM shopping_items ORDER BY checked ASC, created_at DESC`)
	if err != nil {
		log.Error().Err(err).Msg("DB error listing shopping items")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	defer rows.Close()

	items := []Item{}
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan shopping item")
			continue
		}
		items = append(items, item)
	}
	auth.WriteJSON(w, items)
}

func (s *Service) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r)
	if !ok {
		auth.HTTPError(w, http.StatusUnauthorized, "Nicht angemeldet")
		return
	}

	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		auth.HTTPError(w, http.StatusBadRequest, "Name erforderlich")
		return
	}

	res, err := s.db.Exec(
		`INSERT INTO shopping_items (name, quantity, category, user_id) VALUES (?, ?, ?, ?)`,
		req.Name, strings.TrimSpace(req.Quantity), strings.TrimSpace(req.Category), userID,
	)
	if err != nil {
		log.Error().Err(err).Msg("DB error creating shopping item")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}

	id, _ := res.LastInsertId()
	item, err := s.byID(int(id))
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}

	s.broadcast("created", item)
	w.WriteHeader(http.StatusCreated)
	auth.WriteJSON(w, item)
}

func (s *Service) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige ID")
		return
	}

	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige Anfrage")
		return
	}

	sets := []string{"updated_at = CURRENT_TIMESTAMP"}
	args := []any{}
	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		sets = append(sets, "name = ?")
		args = append(args, strings.TrimSpace(*req.Name))
	}
	if req.Quantity != nil {
		sets = append(sets, "quantity = ?")
		args = append(args, strings.TrimSpace(*req.Quantity))
	}
	if req.Category != nil {
		sets = append(sets, "category = ?")
		args = append(args, strings.TrimSpace(*req.Category))
	}
	if req.Checked != nil {
		sets = append(sets, "checked = ?")
		args = append(args, *req.Checked)
	}
	args = append(args, id)

	if _, err := s.db.Exec(
		"UPDATE shopping_items SET "+strings.Join(sets, ", ")+" WHERE id = ?", args...); err != nil {
		log.Error().Err(err).Msg("DB error updating shopping item")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}

	item, err := s.byID(id)
	if err != nil {
		auth.HTTPError(w, http.StatusNotFound, "Eintrag nicht gefunden")
		return
	}

	s.broadcast("updated", item)
	auth.WriteJSON(w, item)
}

func (s *Service) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		auth.HTTPError(w, http.StatusBadRequest, "Ungültige ID")
		return
	}

	if _, err := s.db.Exec("DELETE FROM shopping_items WHERE id = ?", id); err != nil {
		log.Error().Err(err).Msg("DB error deleting shopping item")
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}

	s.broadcast("deleted", Item{ID: id})
	w.WriteHeader(http.StatusNoContent)
}

// Shopping rewards: a flat amount for doing the run at all, plus a little per
// item, because a 30-item weekly shop is not the same errand as fetching milk.
const (
	shoppingBasePoints = 15
	shoppingPerItem    = 2
	shoppingMaxPoints  = 80
)

// ShoppingReward is what clearing the given number of items is worth.
func ShoppingReward(items int) int {
	if items <= 0 {
		return 0
	}
	reward := shoppingBasePoints + items*shoppingPerItem
	if reward > shoppingMaxPoints {
		return shoppingMaxPoints
	}
	return reward
}

// ClearChecked removes everything already ticked off — the usual "done
// shopping" gesture — and credits whoever did it.
func (s *Service) ClearChecked(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserID(r)
	if !ok {
		auth.HTTPError(w, http.StatusUnauthorized, "Nicht angemeldet")
		return
	}

	tx, err := s.db.Begin()
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.Exec("DELETE FROM shopping_items WHERE checked = 1")
	if err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	cleared, _ := res.RowsAffected()

	reward := ShoppingReward(int(cleared))
	if reward > 0 {
		note := fmt.Sprintf("Einkauf erledigt (%d Artikel)", cleared)
		if err := points.Award(tx, userID, points.SourceShopping, 0, reward, note); err != nil {
			auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}

	s.broadcast("cleared", Item{})
	auth.WriteJSON(w, map[string]any{
		"deleted":        cleared,
		"points_awarded": reward,
	})
}

// Reward lets the frontend show what the current list is worth before anyone
// commits to the shopping trip.
func (s *Service) Reward(w http.ResponseWriter, r *http.Request) {
	var checked int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM shopping_items WHERE checked = 1").Scan(&checked); err != nil {
		auth.HTTPError(w, http.StatusInternalServerError, "Server-Fehler")
		return
	}
	auth.WriteJSON(w, map[string]int{
		"checked_items":  checked,
		"points_awarded": ShoppingReward(checked),
	})
}

func (s *Service) byID(id int) (Item, error) {
	row := s.db.QueryRow(`SELECT `+itemColumns+` FROM shopping_items WHERE id = ?`, id)
	return scanItem(row)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanItem(row scanner) (Item, error) {
	var item Item
	var userID sql.NullInt64
	err := row.Scan(&item.ID, &item.Name, &item.Quantity, &item.Category,
		&item.Checked, &userID, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return Item{}, err
	}
	if userID.Valid {
		id := int(userID.Int64)
		item.UserID = &id
	}
	return item, nil
}
