package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// ---------------------------------------------------------------------------
// Admin users
// ---------------------------------------------------------------------------

// Admin is a dashboard account.
type Admin struct {
	ID        int64
	Email     string
	Hash      string
	CreatedAt time.Time
}

// CountAdmins is used to decide whether to seed the first account.
func (db *DB) CountAdmins() (int, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM admins`).Scan(&n)
	return n, err
}

// CreateAdmin hashes the password with bcrypt and stores the account.
func (db *DB) CreateAdmin(email, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO admins (email, password_hash, created_at) VALUES (?,?,?)`,
		strings.ToLower(strings.TrimSpace(email)), string(hash), time.Now().UTC().Format(time.RFC3339))
	return err
}

// AdminByEmail loads an account for login.
func (db *DB) AdminByEmail(email string) (Admin, error) {
	var a Admin
	var created string
	err := db.QueryRow(`SELECT id, email, password_hash, created_at FROM admins WHERE email = ?`,
		strings.ToLower(strings.TrimSpace(email))).Scan(&a.ID, &a.Email, &a.Hash, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return a, ErrNotFound
	}
	a.CreatedAt, _ = time.Parse(time.RFC3339, created)
	return a, err
}

// UpdateAdminPassword re-hashes and stores a new password.
func (db *DB) UpdateAdminPassword(id int64, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE admins SET password_hash = ? WHERE id = ?`, string(hash), id)
	return err
}

// ---------------------------------------------------------------------------
// Settings
// ---------------------------------------------------------------------------

// Settings are the business details shown on the site and used for
// notifications. Stored as key/value rows so new fields are easy to add.
type Settings struct {
	BusinessName string
	WhatsApp     string // digits only, international format
	Phone        string
	Email        string
	Address      string
	Hours        string
	NotifyEmail  bool
}

// DefaultSettings are used until the owner saves their own.
func DefaultSettings() Settings {
	return Settings{
		BusinessName: "Gojias Collections",
		WhatsApp:     "254724715509",
		Phone:        "+254 724 715 509",
		Email:        "hellenonyango340@gmail.com",
		Address:      "Kisumu, Kenya",
		Hours:        "Mon – Fri: 9am – 6pm · Saturday: 9am – 4pm",
		NotifyEmail:  true,
	}
}

// LoadSettings reads the settings table, falling back to defaults per key.
func (db *DB) LoadSettings() (Settings, error) {
	s := DefaultSettings()
	rows, err := db.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return s, err
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return s, err
		}
		switch k {
		case "business_name":
			s.BusinessName = v
		case "whatsapp":
			s.WhatsApp = v
		case "phone":
			s.Phone = v
		case "email":
			s.Email = v
		case "address":
			s.Address = v
		case "hours":
			s.Hours = v
		case "notify_email":
			s.NotifyEmail = v == "1"
		}
	}
	return s, rows.Err()
}

// SaveSettings upserts every key in one transaction.
func (db *DB) SaveSettings(s Settings) error {
	notify := "0"
	if s.NotifyEmail {
		notify = "1"
	}
	pairs := map[string]string{
		"business_name": s.BusinessName,
		"whatsapp":      s.WhatsApp,
		"phone":         s.Phone,
		"email":         s.Email,
		"address":       s.Address,
		"hours":         s.Hours,
		"notify_email":  notify,
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for k, v := range pairs {
		if _, err := tx.Exec(`INSERT INTO settings (key, value) VALUES (?,?)
            ON CONFLICT(key) DO UPDATE SET value = excluded.value`, k, v); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ---------------------------------------------------------------------------
// Order administration
// ---------------------------------------------------------------------------

// AdminOrderFilter mirrors the order table controls.
type AdminOrderFilter struct {
	Query  string
	Status string
	From   string
	To     string
}

// AdminListOrders returns matching orders, newest first (capped at 200).
func (db *DB) AdminListOrders(f AdminOrderFilter) ([]Order, error) {
	var clauses []string
	var args []any
	if q := strings.TrimSpace(f.Query); q != "" {
		like := "%" + strings.ToLower(q) + "%"
		clauses = append(clauses, `(lower(ref) LIKE ? OR lower(customer_name) LIKE ? OR phone_norm LIKE ?)`)
		args = append(args, like, like, "%"+NormalisePhone(q)+"%")
	}
	if f.Status != "" {
		clauses = append(clauses, `status = ?`)
		args = append(args, f.Status)
	}
	if f.From != "" {
		clauses = append(clauses, `created_at >= ?`)
		args = append(args, f.From+"T00:00:00Z")
	}
	if f.To != "" {
		clauses = append(clauses, `created_at <= ?`)
		args = append(args, f.To+"T23:59:59Z")
	}
	where := ""
	if len(clauses) > 0 {
		where = "WHERE " + strings.Join(clauses, " AND ")
	}
	rows, err := db.Query(`SELECT `+orderCols+` FROM orders `+where+`
        ORDER BY created_at DESC, id DESC LIMIT 200`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectOrders(rows)
}

// RecentOrders powers the dashboard table.
func (db *DB) RecentOrders(limit int) ([]Order, error) {
	rows, err := db.Query(`SELECT `+orderCols+` FROM orders ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectOrders(rows)
}

// StatusCounts returns how many orders sit in each status.
func (db *DB) StatusCounts() (map[string]int, error) {
	rows, err := db.Query(`SELECT status, COUNT(*) FROM orders GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var s string
		var n int
		if err := rows.Scan(&s, &n); err != nil {
			return nil, err
		}
		out[s] = n
	}
	return out, rows.Err()
}

// DeliveredSince counts orders that reached "delivered" after a timestamp.
func (db *DB) DeliveredSince(since time.Time) (int, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(DISTINCT order_id) FROM order_events
        WHERE status = 'delivered' AND created_at >= ?`,
		since.UTC().Format(time.RFC3339)).Scan(&n)
	return n, err
}

// OutOfStockFabrics feeds the low-stock alert.
func (db *DB) OutOfStockFabrics(limit int) ([]Fabric, error) {
	rows, err := db.Query(`SELECT `+fabricCols+` FROM fabrics f
        WHERE f.in_stock = 0 ORDER BY f.name LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectFabrics(rows)
}

// UpdateOrderAdmin stores internal notes and the payment record.
func (db *DB) UpdateOrderAdmin(ref, internalNotes string, deposit float64, paymentNote string) error {
	_, err := db.Exec(`UPDATE orders SET internal_notes = ?, deposit = ?, payment_note = ?, updated_at = ?
        WHERE ref = ?`, internalNotes, deposit, paymentNote,
		time.Now().UTC().Format(time.RFC3339), strings.ToUpper(strings.TrimSpace(ref)))
	return err
}

func collectOrders(rows *sql.Rows) ([]Order, error) {
	var out []Order
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Catalogue administration
// ---------------------------------------------------------------------------

// ImageRow is one uploaded gallery image.
type ImageRow struct {
	ID  int64
	URL string
}

// FabricImages lists a fabric's gallery with row IDs (for deletion).
func (db *DB) FabricImages(fabricID int64) ([]ImageRow, error) {
	rows, err := db.Query(`SELECT id, url FROM fabric_images WHERE fabric_id = ? ORDER BY sort, id`, fabricID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ImageRow
	for rows.Next() {
		var r ImageRow
		if err := rows.Scan(&r.ID, &r.URL); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DesignImages lists a design's gallery with row IDs.
func (db *DB) DesignImages(designID int64) ([]ImageRow, error) {
	rows, err := db.Query(`SELECT id, url FROM design_images WHERE design_id = ? ORDER BY sort, id`, designID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ImageRow
	for rows.Next() {
		var r ImageRow
		if err := rows.Scan(&r.ID, &r.URL); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// FabricByID loads one fabric for editing.
func (db *DB) FabricByID(id int64) (Fabric, error) {
	f, err := scanFabric(db.QueryRow(`SELECT `+fabricCols+` FROM fabrics f WHERE f.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return f, ErrNotFound
	}
	return f, err
}

// SaveFabric inserts or updates a fabric, then applies gallery changes.
func (db *DB) SaveFabric(f *Fabric, newImages []string, removeImageIDs []int64) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	slug, err := uniqueSlug(tx, "fabrics", f.Slug, f.ID)
	if err != nil {
		return 0, err
	}
	f.Slug = slug
	now := time.Now().UTC().Format(time.RFC3339)

	if f.ID == 0 {
		res, err := tx.Exec(`INSERT INTO fabrics
            (slug, name, fabric_type, description, colours, care, width, image, unit, price, in_stock, featured, created_at)
            VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			f.Slug, f.Name, f.FabricType, f.Description, f.Colours, f.Care, f.Width,
			"", f.Unit, f.Price, boolToInt(f.InStock), boolToInt(f.Featured), now)
		if err != nil {
			return 0, err
		}
		f.ID, err = res.LastInsertId()
		if err != nil {
			return 0, err
		}
	} else {
		if _, err := tx.Exec(`UPDATE fabrics SET slug=?, name=?, fabric_type=?, description=?,
            colours=?, care=?, width=?, unit=?, price=?, in_stock=?, featured=? WHERE id=?`,
			f.Slug, f.Name, f.FabricType, f.Description, f.Colours, f.Care, f.Width,
			f.Unit, f.Price, boolToInt(f.InStock), boolToInt(f.Featured), f.ID); err != nil {
			return 0, err
		}
	}

	for _, id := range removeImageIDs {
		if _, err := tx.Exec(`DELETE FROM fabric_images WHERE id = ? AND fabric_id = ?`, id, f.ID); err != nil {
			return 0, err
		}
	}
	for _, url := range newImages {
		if _, err := tx.Exec(`INSERT INTO fabric_images (fabric_id, url, sort)
            VALUES (?,?, (SELECT COALESCE(MAX(sort),0)+1 FROM fabric_images WHERE fabric_id = ?))`,
			f.ID, url, f.ID); err != nil {
			return 0, err
		}
	}
	// Keep fabrics.image pointing at the first gallery image.
	if _, err := tx.Exec(`UPDATE fabrics SET image = COALESCE(
            (SELECT url FROM fabric_images WHERE fabric_id = ? ORDER BY sort, id LIMIT 1), '')
        WHERE id = ?`, f.ID, f.ID); err != nil {
		return 0, err
	}
	return f.ID, tx.Commit()
}

// DeleteFabric removes a fabric and its gallery/recommendations.
func (db *DB) DeleteFabric(id int64) error {
	_, err := db.Exec(`DELETE FROM fabrics WHERE id = ?`, id)
	return err
}

// DesignByID loads one design for editing.
func (db *DB) DesignByID(id int64) (Design, error) {
	d, err := scanDesign(db.QueryRow(`SELECT `+designCols+` FROM designs d WHERE d.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return d, ErrNotFound
	}
	if err != nil {
		return d, err
	}
	rows, err := db.Query(`SELECT fabric_id FROM design_fabrics WHERE design_id = ?`, id)
	if err != nil {
		return d, err
	}
	defer rows.Close()
	for rows.Next() {
		var fid int64
		if err := rows.Scan(&fid); err == nil {
			d.FabricIDs = append(d.FabricIDs, fid)
		}
	}
	return d, rows.Err()
}

// SaveDesign inserts or updates a design with its fabrics and gallery.
func (db *DB) SaveDesign(d *Design, fabricIDs []int64, newImages []string, removeImageIDs []int64) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	slug, err := uniqueSlug(tx, "designs", d.Slug, d.ID)
	if err != nil {
		return 0, err
	}
	d.Slug = slug
	now := time.Now().UTC().Format(time.RFC3339)

	if d.ID == 0 {
		res, err := tx.Exec(`INSERT INTO designs
            (slug, name, category, style, occasion, description, measurements, image, price_from, production_days, featured, created_at)
            VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
			d.Slug, d.Name, d.Category, d.Style, d.Occasion, d.Description, d.Measurements,
			"", d.PriceFrom, d.ProductionDays, boolToInt(d.Featured), now)
		if err != nil {
			return 0, err
		}
		d.ID, err = res.LastInsertId()
		if err != nil {
			return 0, err
		}
	} else {
		if _, err := tx.Exec(`UPDATE designs SET slug=?, name=?, category=?, style=?, occasion=?,
            description=?, measurements=?, price_from=?, production_days=?, featured=? WHERE id=?`,
			d.Slug, d.Name, d.Category, d.Style, d.Occasion, d.Description, d.Measurements,
			d.PriceFrom, d.ProductionDays, boolToInt(d.Featured), d.ID); err != nil {
			return 0, err
		}
	}

	if _, err := tx.Exec(`DELETE FROM design_fabrics WHERE design_id = ?`, d.ID); err != nil {
		return 0, err
	}
	for _, fid := range fabricIDs {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO design_fabrics (design_id, fabric_id) VALUES (?,?)`,
			d.ID, fid); err != nil {
			return 0, err
		}
	}
	for _, id := range removeImageIDs {
		if _, err := tx.Exec(`DELETE FROM design_images WHERE id = ? AND design_id = ?`, id, d.ID); err != nil {
			return 0, err
		}
	}
	for _, url := range newImages {
		if _, err := tx.Exec(`INSERT INTO design_images (design_id, url, sort)
            VALUES (?,?, (SELECT COALESCE(MAX(sort),0)+1 FROM design_images WHERE design_id = ?))`,
			d.ID, url, d.ID); err != nil {
			return 0, err
		}
	}
	if _, err := tx.Exec(`UPDATE designs SET image = COALESCE(
            (SELECT url FROM design_images WHERE design_id = ? ORDER BY sort, id LIMIT 1), '')
        WHERE id = ?`, d.ID, d.ID); err != nil {
		return 0, err
	}
	return d.ID, tx.Commit()
}

// DeleteDesign removes a design and its gallery/recommendations.
func (db *DB) DeleteDesign(id int64) error {
	_, err := db.Exec(`DELETE FROM designs WHERE id = ?`, id)
	return err
}

// ToggleFabricFeatured flips the featured flag from the table view.
func (db *DB) ToggleFabricFeatured(id int64) error {
	_, err := db.Exec(`UPDATE fabrics SET featured = 1 - featured WHERE id = ?`, id)
	return err
}

// ToggleDesignFeatured flips the featured flag from the table view.
func (db *DB) ToggleDesignFeatured(id int64) error {
	_, err := db.Exec(`UPDATE designs SET featured = 1 - featured WHERE id = ?`, id)
	return err
}

// ToggleFabricStock flips availability from the table view.
func (db *DB) ToggleFabricStock(id int64) error {
	_, err := db.Exec(`UPDATE fabrics SET in_stock = 1 - in_stock WHERE id = ?`, id)
	return err
}

// uniqueSlug appends -2, -3 ... until the slug is free in the given table.
func uniqueSlug(tx *sql.Tx, table, slug string, excludeID int64) (string, error) {
	base := slugify(slug)
	if base == "" {
		base = "item"
	}
	candidate := base
	for i := 2; i < 100; i++ {
		var existing int64
		err := tx.QueryRow(`SELECT id FROM `+table+` WHERE slug = ?`, candidate).Scan(&existing)
		if errors.Is(err, sql.ErrNoRows) {
			return candidate, nil
		}
		if err != nil {
			return "", err
		}
		if existing == excludeID {
			return candidate, nil
		}
		candidate = base + "-" + itoa(i)
	}
	return base, nil
}

// Slugify builds a URL-safe slug from a name.
func Slugify(s string) string { return slugify(s) }

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	dash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case r == ' ' || r == '-' || r == '_' || r == '/' || r == '&':
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
