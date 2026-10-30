package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrNotFound is returned when a lookup matches no row.
var ErrNotFound = errors.New("not found")

// Status is one stage in the order lifecycle, in the order customers see them.
type Status struct {
	Key   string
	Label string
}

// Statuses is the canonical order lifecycle.
var Statuses = []Status{
	{"new", "New"},
	{"confirmed", "Confirmed"},
	{"cutting", "Cutting"},
	{"sewing", "Sewing"},
	{"ready", "Ready"},
	{"delivered", "Delivered"},
}

// StatusLabel returns the human label for a status key.
func StatusLabel(key string) string {
	for _, s := range Statuses {
		if s.Key == key {
			return s.Label
		}
	}
	return key
}

// StatusIndex gives the position of a status in the lifecycle (-1 if unknown).
func StatusIndex(key string) int {
	for i, s := range Statuses {
		if s.Key == key {
			return i
		}
	}
	return -1
}

// StatusEvent is one recorded change, shown on the tracking timeline.
type StatusEvent struct {
	Status    string
	Note      string
	CreatedAt time.Time
}

// Order is a customer order captured by the wizard.
type Order struct {
	ID             int64
	Ref            string
	PublicToken    string
	Status         string
	DesignSlug     string
	DesignName     string
	DesignCategory string
	CustomDesign   bool
	InspirationImg string
	InspirationTxt string
	FabricSlug     string
	FabricName     string
	BringOwnFabric bool
	Yardage        float64
	Unit           string // unit the customer measured in ("cm" or "in")
	Measurements   map[string]float64
	Notes          string
	CustomerName   string
	Phone          string
	PhoneNorm      string
	Email          string
	Delivery       string // "pickup" or "delivery"
	Address        string
	PreferredDate  string
	PriceEstimate  float64
	ProductionDays int
	InternalNotes  string
	Deposit        float64
	PaymentNote    string
	CreatedAt      time.Time
	History        []StatusEvent
}

// EventFor returns the first recorded event with the given status.
func (o Order) EventFor(status string) (StatusEvent, bool) {
	for _, e := range o.History {
		if e.Status == status {
			return e, true
		}
	}
	return StatusEvent{}, false
}

// migrateOrders creates the order tables. Called from Open after migrate().
func (db *DB) migrateOrders() error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS orders (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    ref              TEXT NOT NULL UNIQUE,
    public_token     TEXT NOT NULL UNIQUE,
    status           TEXT NOT NULL DEFAULT 'new',
    design_slug      TEXT NOT NULL DEFAULT '',
    design_name      TEXT NOT NULL DEFAULT '',
    design_category  TEXT NOT NULL DEFAULT '',
    custom_design    INTEGER NOT NULL DEFAULT 0,
    inspiration_img  TEXT NOT NULL DEFAULT '',
    inspiration_txt  TEXT NOT NULL DEFAULT '',
    fabric_slug      TEXT NOT NULL DEFAULT '',
    fabric_name      TEXT NOT NULL DEFAULT '',
    bring_own_fabric INTEGER NOT NULL DEFAULT 0,
    yardage          REAL NOT NULL DEFAULT 0,
    unit             TEXT NOT NULL DEFAULT 'cm',
    measurements     TEXT NOT NULL DEFAULT '{}',
    notes            TEXT NOT NULL DEFAULT '',
    customer_name    TEXT NOT NULL DEFAULT '',
    phone            TEXT NOT NULL DEFAULT '',
    phone_norm       TEXT NOT NULL DEFAULT '',
    email            TEXT NOT NULL DEFAULT '',
    delivery         TEXT NOT NULL DEFAULT 'pickup',
    address          TEXT NOT NULL DEFAULT '',
    preferred_date   TEXT NOT NULL DEFAULT '',
    price_estimate   REAL NOT NULL DEFAULT 0,
    production_days  INTEGER NOT NULL DEFAULT 0,
    internal_notes   TEXT NOT NULL DEFAULT '',
    deposit          REAL NOT NULL DEFAULT 0,
    payment_note     TEXT NOT NULL DEFAULT '',
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS order_events (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    order_id   INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    status     TEXT NOT NULL,
    note       TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS admins (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_orders_phone ON orders(phone_norm);
CREATE INDEX IF NOT EXISTS idx_order_events_order ON order_events(order_id);
`)
	if err != nil {
		return err
	}
	// Older databases predate the admin columns; add them in place.
	for _, col := range []struct{ name, ddl string }{
		{"internal_notes", "internal_notes TEXT NOT NULL DEFAULT ''"},
		{"deposit", "deposit REAL NOT NULL DEFAULT 0"},
		{"payment_note", "payment_note TEXT NOT NULL DEFAULT ''"},
	} {
		if err := db.ensureColumn("orders", col.name, col.ddl); err != nil {
			return err
		}
	}
	return nil
}

// ensureColumn adds a column when it is missing (SQLite has no IF NOT EXISTS
// for ALTER TABLE).
func (db *DB) ensureColumn(table, column, ddl string) error {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == column {
			found = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if found {
		return nil
	}
	_, err = db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + ddl)
	return err
}

// CreateOrder stores the order and its first "new" event in one transaction,
// allocating the next reference for the current year (GC-2026-0001).
func (db *DB) CreateOrder(o *Order) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	year := time.Now().Year()
	var last int
	prefix := fmt.Sprintf("GC-%d-", year)
	if err := tx.QueryRow(`SELECT COALESCE(MAX(CAST(substr(ref, 9) AS INTEGER)), 0)
        FROM orders WHERE ref LIKE ?`, prefix+"%").Scan(&last); err != nil {
		return err
	}
	o.Ref = fmt.Sprintf("%s%04d", prefix, last+1)

	token, err := randomToken()
	if err != nil {
		return err
	}
	o.PublicToken = token

	if o.Measurements == nil {
		o.Measurements = map[string]float64{}
	}
	measureJSON, err := json.Marshal(o.Measurements)
	if err != nil {
		return err
	}
	if o.Status == "" {
		o.Status = "new"
	}
	now := time.Now().UTC()
	o.CreatedAt = now

	res, err := tx.Exec(`INSERT INTO orders
        (ref, public_token, status, design_slug, design_name, design_category, custom_design,
         inspiration_img, inspiration_txt, fabric_slug, fabric_name, bring_own_fabric,
         yardage, unit, measurements, notes, customer_name, phone, phone_norm, email,
         delivery, address, preferred_date, price_estimate, production_days, created_at, updated_at)
        VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		o.Ref, o.PublicToken, o.Status, o.DesignSlug, o.DesignName, o.DesignCategory,
		boolToInt(o.CustomDesign), o.InspirationImg, o.InspirationTxt, o.FabricSlug, o.FabricName,
		boolToInt(o.BringOwnFabric), o.Yardage, o.Unit, string(measureJSON), o.Notes,
		o.CustomerName, o.Phone, NormalisePhone(o.Phone), o.Email, o.Delivery, o.Address,
		o.PreferredDate, o.PriceEstimate, o.ProductionDays,
		now.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		return err
	}
	o.ID, err = res.LastInsertId()
	if err != nil {
		return err
	}

	if _, err := tx.Exec(`INSERT INTO order_events (order_id, status, note, created_at) VALUES (?,?,?,?)`,
		o.ID, o.Status, "Order received through the website.", now.Format(time.RFC3339)); err != nil {
		return err
	}
	return tx.Commit()
}

const orderCols = `id, ref, public_token, status, design_slug, design_name, design_category,
    custom_design, inspiration_img, inspiration_txt, fabric_slug, fabric_name, bring_own_fabric,
    yardage, unit, measurements, notes, customer_name, phone, phone_norm, email, delivery,
    address, preferred_date, price_estimate, production_days, internal_notes, deposit,
    payment_note, created_at`

func scanOrder(s scanner) (Order, error) {
	var o Order
	var custom, own int
	var measureJSON, created string
	err := s.Scan(&o.ID, &o.Ref, &o.PublicToken, &o.Status, &o.DesignSlug, &o.DesignName,
		&o.DesignCategory, &custom, &o.InspirationImg, &o.InspirationTxt, &o.FabricSlug,
		&o.FabricName, &own, &o.Yardage, &o.Unit, &measureJSON, &o.Notes, &o.CustomerName,
		&o.Phone, &o.PhoneNorm, &o.Email, &o.Delivery, &o.Address, &o.PreferredDate,
		&o.PriceEstimate, &o.ProductionDays, &o.InternalNotes, &o.Deposit, &o.PaymentNote, &created)
	if err != nil {
		return o, err
	}
	o.CustomDesign = custom == 1
	o.BringOwnFabric = own == 1
	o.Measurements = map[string]float64{}
	_ = json.Unmarshal([]byte(measureJSON), &o.Measurements)
	o.CreatedAt, _ = time.Parse(time.RFC3339, created)
	return o, nil
}

// GetOrderByRef loads an order and its status history.
func (db *DB) GetOrderByRef(ref string) (Order, error) {
	o, err := scanOrder(db.QueryRow(`SELECT `+orderCols+` FROM orders WHERE ref = ?`,
		strings.ToUpper(strings.TrimSpace(ref))))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return o, ErrNotFound
		}
		return o, err
	}
	return o, db.loadHistory(&o)
}

// GetOrderByToken loads an order only when the private confirmation token
// matches, so order summaries are not guessable from the reference alone.
func (db *DB) GetOrderByToken(ref, token string) (Order, error) {
	o, err := scanOrder(db.QueryRow(`SELECT `+orderCols+` FROM orders WHERE ref = ? AND public_token = ?`,
		strings.ToUpper(strings.TrimSpace(ref)), token))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return o, ErrNotFound
		}
		return o, err
	}
	return o, db.loadHistory(&o)
}

// GetOrderByRefAndPhone is the tracking lookup: reference plus the phone number
// used on the order.
func (db *DB) GetOrderByRefAndPhone(ref, phone string) (Order, error) {
	o, err := db.GetOrderByRef(ref)
	if err != nil {
		return o, err
	}
	want := NormalisePhone(phone)
	if want == "" || want != o.PhoneNorm {
		return Order{}, ErrNotFound
	}
	return o, nil
}

// AddStatus records a lifecycle change; the admin dashboard will drive this in
// stage 3.
func (db *DB) AddStatus(orderID int64, status, note string) error {
	if StatusIndex(status) < 0 {
		return fmt.Errorf("unknown status %q", status)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE orders SET status = ?, updated_at = ? WHERE id = ?`,
		status, now, orderID); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO order_events (order_id, status, note, created_at) VALUES (?,?,?,?)`,
		orderID, status, note, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (db *DB) loadHistory(o *Order) error {
	rows, err := db.Query(`SELECT status, note, created_at FROM order_events
        WHERE order_id = ? ORDER BY created_at, id`, o.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var e StatusEvent
		var created string
		if err := rows.Scan(&e.Status, &e.Note, &created); err != nil {
			return err
		}
		e.CreatedAt, _ = time.Parse(time.RFC3339, created)
		o.History = append(o.History, e)
	}
	return rows.Err()
}

// NormalisePhone reduces a phone number to digits so "0712 345 678" and
// "+254712345678" can be compared.
func NormalisePhone(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	digits := b.String()
	// Compare local and international forms of Kenyan numbers.
	if strings.HasPrefix(digits, "254") {
		if len(digits) > 9 {
			return digits[len(digits)-9:]
		}
	}
	if strings.HasPrefix(digits, "0") && len(digits) == 10 {
		return digits[1:]
	}
	return digits
}

func randomToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
