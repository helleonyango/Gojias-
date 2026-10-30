// Package store owns the SQLite connection, the schema and every query the
// site needs. Keeping SQL here means handlers stay small and read like prose.
package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	// Pure-Go SQLite driver: no CGO, so `go build` works anywhere.
	_ "modernc.org/sqlite"
)

// ---------------------------------------------------------------------------
// Models
// ---------------------------------------------------------------------------

// Fabric is one cloth a customer can order.
type Fabric struct {
	ID          int64
	Slug        string
	Name        string
	FabricType  string // kitenge, ankara, lace, plain-cotton ...
	Description string
	Colours     string   // comma separated, as stored
	ColourList  []string // split for rendering colour dots
	Care        string
	Width       string
	Image       string   // main image path (relative to the site root)
	Images      []string // gallery, main image first
	Similar     []Fabric // same-type fabrics, filled by GetFabric
	Unit        string   // "yard" or "metre"
	Price       float64  // price per Unit, in KES
	InStock     bool
	Featured    bool
	CreatedAt   time.Time
}

// TypeLabel is the human friendly version of FabricType ("plain-cotton" -> "Plain Cotton").
func (f Fabric) TypeLabel() string { return humanise(f.FabricType) }

// Design is one ready-made outfit style. A design recommends fabrics and lists
// the measurements a tailor needs before sewing it.
type Design struct {
	ID                   int64
	Slug                 string
	Name                 string
	Category             string // women, men, children, couples, bridal
	Style                string // gown, skirt-and-top, kaftan, shirt, suit ...
	Occasion             string
	Description          string
	Image                string
	Images               []string
	PriceFrom            float64 // starting price in KES
	ProductionDays       int
	Measurements         string   // comma separated keys, as stored
	RequiredMeasurements []string // split for rendering
	Featured             bool
	CreatedAt            time.Time
	Fabrics              []Fabric // recommended fabrics, filled by GetDesign
	FabricIDs            []int64  // recommended fabric ids, filled by DesignByID
}

// CategoryLabel maps stored slugs to the labels shown in the tab bar.
func (d Design) CategoryLabel() string {
	switch d.Category {
	case "couples":
		return "Couples/Family"
	case "bridal":
		return "Bridal & Occasion"
	default:
		return humanise(d.Category)
	}
}

// ---------------------------------------------------------------------------
// Filters
// ---------------------------------------------------------------------------

// FabricFilter mirrors the controls on the Fabrics page.
type FabricFilter struct {
	Query       string
	Type        string
	Colour      string
	MaxPrice    float64
	InStockOnly bool
	Sort        string // newest | price-asc | price-desc | name
	Page        int
	PerPage     int
}

// DesignFilter mirrors the controls on the Designs page.
type DesignFilter struct {
	Query    string
	Category string
	Style    string
	MaxPrice float64
	Sort     string
	Page     int
	PerPage  int
}

// ---------------------------------------------------------------------------
// Connection
// ---------------------------------------------------------------------------

// DB wraps *sql.DB so callers never touch SQL directly.
type DB struct{ *sql.DB }

// Open creates (or opens) the database file, applies the schema and seeds
// sample content the first time it runs.
func Open(path string) (*DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", path)
	raw, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := raw.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}
	db := &DB{raw}
	if err := db.migrate(); err != nil {
		return nil, err
	}
	if err := db.migrateOrders(); err != nil {
		return nil, err
	}
	if err := db.seed(); err != nil {
		return nil, err
	}
	return db, nil
}

func (db *DB) migrate() error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS fabrics (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    slug        TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    fabric_type TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    colours     TEXT NOT NULL DEFAULT '',
    care        TEXT NOT NULL DEFAULT '',
    width       TEXT NOT NULL DEFAULT '',
    image       TEXT NOT NULL DEFAULT '',
    unit        TEXT NOT NULL DEFAULT 'yard',
    price       REAL NOT NULL DEFAULT 0,
    in_stock    INTEGER NOT NULL DEFAULT 1,
    featured    INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS fabric_images (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    fabric_id INTEGER NOT NULL REFERENCES fabrics(id) ON DELETE CASCADE,
    url       TEXT NOT NULL,
    sort      INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS designs (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    slug            TEXT NOT NULL UNIQUE,
    name            TEXT NOT NULL,
    category        TEXT NOT NULL,
    style           TEXT NOT NULL DEFAULT '',
    occasion        TEXT NOT NULL DEFAULT '',
    description     TEXT NOT NULL DEFAULT '',
    measurements    TEXT NOT NULL DEFAULT '',
    image           TEXT NOT NULL DEFAULT '',
    price_from      REAL NOT NULL DEFAULT 0,
    production_days INTEGER NOT NULL DEFAULT 0,
    featured        INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS design_images (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    design_id INTEGER NOT NULL REFERENCES designs(id) ON DELETE CASCADE,
    url       TEXT NOT NULL,
    sort      INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS design_fabrics (
    design_id INTEGER NOT NULL REFERENCES designs(id) ON DELETE CASCADE,
    fabric_id INTEGER NOT NULL REFERENCES fabrics(id) ON DELETE CASCADE,
    PRIMARY KEY (design_id, fabric_id)
);

CREATE INDEX IF NOT EXISTS idx_fabrics_type ON fabrics(fabric_type);
CREATE INDEX IF NOT EXISTS idx_designs_category ON designs(category);
`)
	return err
}

// seed inserts sample content only when the tables are empty, so restarting
// the server never duplicates rows.
func (db *DB) seed() error {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM fabrics`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().UTC().Format(time.RFC3339)
	for _, f := range seedFabrics {
		res, err := tx.Exec(`INSERT INTO fabrics
            (slug, name, fabric_type, description, colours, care, width, image, unit, price, in_stock, featured, created_at)
            VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			f.Slug, f.Name, f.Type, f.Description, strings.Join(f.Colours, ", "), f.Care,
			f.Width, f.Image, f.Unit, f.Price, boolToInt(f.InStock), boolToInt(f.Featured), now)
		if err != nil {
			return fmt.Errorf("seed fabric %s: %w", f.Slug, err)
		}
		id, _ := res.LastInsertId()
		// Every seeded fabric gets three gallery views built from the same
		// placeholder art; replace these SVG paths with real photos later.
		for j, url := range f.Images {
			if _, err := tx.Exec(`INSERT INTO fabric_images (fabric_id, url, sort) VALUES (?,?,?)`, id, url, j); err != nil {
				return err
			}
		}
		seedFabricIDs[f.Slug] = id
	}

	for _, d := range seedDesigns {
		res, err := tx.Exec(`INSERT INTO designs
            (slug, name, category, style, occasion, description, measurements, image, price_from, production_days, featured, created_at)
            VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
			d.Slug, d.Name, d.Category, d.Style, d.Occasion, d.Description,
			strings.Join(d.Measurements, ", "), d.Image, d.PriceFrom, d.ProductionDays, boolToInt(d.Featured), now)
		if err != nil {
			return fmt.Errorf("seed design %s: %w", d.Slug, err)
		}
		id, _ := res.LastInsertId()
		for j, url := range d.Images {
			if _, err := tx.Exec(`INSERT INTO design_images (design_id, url, sort) VALUES (?,?,?)`, id, url, j); err != nil {
				return err
			}
		}
		for _, fs := range d.FabricSlugs {
			fid, ok := seedFabricIDs[fs]
			if !ok {
				continue
			}
			if _, err := tx.Exec(`INSERT OR IGNORE INTO design_fabrics (design_id, fabric_id) VALUES (?,?)`, id, fid); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

var seedFabricIDs = map[string]int64{}

// ---------------------------------------------------------------------------
// Fabric queries
// ---------------------------------------------------------------------------

const fabricCols = `f.id, f.slug, f.name, f.fabric_type, f.description, f.colours,
    f.care, f.width, f.image, f.unit, f.price, f.in_stock, f.featured, f.created_at`

type scanner interface{ Scan(dest ...any) error }

func scanFabric(s scanner) (Fabric, error) {
	var f Fabric
	var inStock, featured int
	var created string
	err := s.Scan(&f.ID, &f.Slug, &f.Name, &f.FabricType, &f.Description, &f.Colours,
		&f.Care, &f.Width, &f.Image, &f.Unit, &f.Price, &inStock, &featured, &created)
	if err != nil {
		return f, err
	}
	f.InStock = inStock == 1
	f.Featured = featured == 1
	f.CreatedAt, _ = time.Parse(time.RFC3339, created)
	f.ColourList = splitList(f.Colours)
	f.Images = append(f.Images, f.Image)
	return f, nil
}

// ListFabrics returns one page of fabrics plus the total number of matches.
func (db *DB) ListFabrics(filter FabricFilter) ([]Fabric, int, error) {
	where, args := filter.sql()

	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM fabrics f `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	filter.Page = clampPage(filter.page(), filter.limit(), total)

	rows, err := db.Query(`SELECT `+fabricCols+` FROM fabrics f `+where+`
        ORDER BY `+sortSQL(filter.Sort, "f", "price")+` LIMIT ? OFFSET ?`,
		append(args, filter.limit(), (filter.page()-1)*filter.limit())...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	fabrics, err := collectFabrics(rows)
	return fabrics, total, err
}

// FeaturedFabrics powers the home page strip.
func (db *DB) FeaturedFabrics(limit int) ([]Fabric, error) {
	rows, err := db.Query(`SELECT `+fabricCols+` FROM fabrics f
        WHERE f.featured = 1 ORDER BY f.created_at DESC, f.id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectFabrics(rows)
}

// GetFabric loads one fabric with its gallery, similar fabrics and designs
// that recommend it.
func (db *DB) GetFabric(slug string) (Fabric, error) {
	f, err := scanFabric(db.QueryRow(`SELECT `+fabricCols+` FROM fabrics f WHERE f.slug = ?`, slug))
	if err != nil {
		return f, err
	}

	if rows, err := db.Query(`SELECT url FROM fabric_images WHERE fabric_id = ? ORDER BY sort, id`, f.ID); err == nil {
		f.Images = f.Images[:0]
		for rows.Next() {
			var url string
			if err := rows.Scan(&url); err == nil {
				f.Images = append(f.Images, url)
			}
		}
		rows.Close()
	}
	if len(f.Images) == 0 {
		f.Images = []string{f.Image}
	}

	// Similar: same family of cloth, never the fabric you are already looking at.
	if rows, err := db.Query(`SELECT `+fabricCols+` FROM fabrics f
        WHERE f.fabric_type = ? AND f.id <> ?
        ORDER BY f.featured DESC, f.id LIMIT 4`, f.FabricType, f.ID); err == nil {
		f.Similar = nil
		for rows.Next() {
			if s, err := scanFabric(rows); err == nil {
				f.Similar = append(f.Similar, s)
			}
		}
		rows.Close()
	}
	return f, nil
}

// DesignsForFabric returns designs that recommend this fabric (falls back to
// featured designs so the section is never empty).
func (db *DB) DesignsForFabric(fabricID int64, limit int) ([]Design, error) {
	rows, err := db.Query(`SELECT `+designCols+` FROM designs d
        JOIN design_fabrics df ON df.design_id = d.id
        WHERE df.fabric_id = ?
        ORDER BY d.featured DESC, d.id LIMIT ?`, fabricID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	designs, err := collectDesigns(rows)
	if err != nil {
		return nil, err
	}
	if len(designs) == 0 {
		return db.FeaturedDesigns(limit)
	}
	return designs, nil
}

// FabricFacets lists the filter options actually present in the catalogue.
func (db *DB) FabricFacets() (types []string, colours []string, maxPrice float64, err error) {
	if types, err = db.distinct(`SELECT DISTINCT fabric_type FROM fabrics ORDER BY fabric_type`); err != nil {
		return
	}
	var joined []string
	if joined, err = db.distinct(`SELECT colours FROM fabrics`); err != nil {
		return
	}
	seen := map[string]bool{}
	for _, row := range joined {
		for _, c := range splitList(row) {
			key := strings.ToLower(c)
			if c != "" && !seen[key] {
				seen[key] = true
				colours = append(colours, c)
			}
		}
	}
	err = db.QueryRow(`SELECT COALESCE(MAX(price), 0) FROM fabrics`).Scan(&maxPrice)
	return
}

func (db *DB) distinct(query string) ([]string, error) {
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err == nil && v != "" {
			out = append(out, v)
		}
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Design queries
// ---------------------------------------------------------------------------

const designCols = `d.id, d.slug, d.name, d.category, d.style, d.occasion, d.description,
    d.measurements, d.image, d.price_from, d.production_days, d.featured, d.created_at`

func scanDesign(s scanner) (Design, error) {
	var d Design
	var featured int
	var created string
	err := s.Scan(&d.ID, &d.Slug, &d.Name, &d.Category, &d.Style, &d.Occasion, &d.Description,
		&d.Measurements, &d.Image, &d.PriceFrom, &d.ProductionDays, &featured, &created)
	if err != nil {
		return d, err
	}
	d.Featured = featured == 1
	d.CreatedAt, _ = time.Parse(time.RFC3339, created)
	d.RequiredMeasurements = splitList(d.Measurements)
	d.Images = []string{d.Image}
	return d, nil
}

// ListDesigns returns one page of designs plus the total number of matches.
func (db *DB) ListDesigns(filter DesignFilter) ([]Design, int, error) {
	where, args := filter.sql()

	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM designs d `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	filter.Page = clampPage(filter.page(), filter.limit(), total)
	rows, err := db.Query(`SELECT `+designCols+` FROM designs d `+where+`
        ORDER BY `+sortSQL(filter.Sort, "d", "price_from")+` LIMIT ? OFFSET ?`,
		append(args, filter.limit(), (filter.page()-1)*filter.limit())...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	designs, err := collectDesigns(rows)
	return designs, total, err
}

// FeaturedDesigns powers the home page and design-picker galleries.
func (db *DB) FeaturedDesigns(limit int) ([]Design, error) {
	rows, err := db.Query(`SELECT `+designCols+` FROM designs d
        WHERE d.featured = 1 ORDER BY d.created_at DESC, d.id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectDesigns(rows)
}

// GetDesign loads one design with its gallery and recommended fabrics.
func (db *DB) GetDesign(slug string) (Design, error) {
	d, err := scanDesign(db.QueryRow(`SELECT `+designCols+` FROM designs d WHERE d.slug = ?`, slug))
	if err != nil {
		return d, err
	}

	if rows, err := db.Query(`SELECT url FROM design_images WHERE design_id = ? ORDER BY sort, id`, d.ID); err == nil {
		d.Images = d.Images[:0]
		for rows.Next() {
			var url string
			if err := rows.Scan(&url); err == nil {
				d.Images = append(d.Images, url)
			}
		}
		rows.Close()
	}
	if len(d.Images) == 0 {
		d.Images = []string{d.Image}
	}

	rows, err := db.Query(`SELECT `+fabricCols+` FROM fabrics f
        JOIN design_fabrics df ON df.fabric_id = f.id
        WHERE df.design_id = ? ORDER BY f.id`, d.ID)
	if err != nil {
		return d, err
	}
	defer rows.Close()
	for rows.Next() {
		if f, err := scanFabric(rows); err == nil {
			d.Fabrics = append(d.Fabrics, f)
		}
	}
	return d, rows.Err()
}

// DesignStyles lists style options present in the catalogue.
func (db *DB) DesignStyles() ([]string, error) {
	return db.distinct(`SELECT DISTINCT style FROM designs WHERE style <> '' ORDER BY style`)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (f FabricFilter) sql() (string, []any) {
	var clauses []string
	var args []any
	if q := strings.TrimSpace(f.Query); q != "" {
		like := "%" + strings.ToLower(q) + "%"
		clauses = append(clauses, `(lower(f.name) LIKE ? OR lower(f.description) LIKE ? OR lower(f.fabric_type) LIKE ?)`)
		args = append(args, like, like, like)
	}
	if f.Type != "" {
		clauses = append(clauses, `f.fabric_type = ?`)
		args = append(args, f.Type)
	}
	if f.Colour != "" {
		clauses = append(clauses, `lower(f.colours) LIKE ?`)
		args = append(args, "%"+strings.ToLower(f.Colour)+"%")
	}
	if f.MaxPrice > 0 {
		clauses = append(clauses, `f.price <= ?`)
		args = append(args, f.MaxPrice)
	}
	if f.InStockOnly {
		clauses = append(clauses, `f.in_stock = 1`)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

func (f FabricFilter) page() int { return max(1, f.Page) }
func (f FabricFilter) limit() int {
	if f.PerPage <= 0 {
		return 9
	}
	return f.PerPage
}

func (f DesignFilter) sql() (string, []any) {
	var clauses []string
	var args []any
	if q := strings.TrimSpace(f.Query); q != "" {
		like := "%" + strings.ToLower(q) + "%"
		clauses = append(clauses, `(lower(d.name) LIKE ? OR lower(d.description) LIKE ? OR lower(d.style) LIKE ?)`)
		args = append(args, like, like, like)
	}
	if f.Category != "" {
		clauses = append(clauses, `d.category = ?`)
		args = append(args, f.Category)
	}
	if f.Style != "" {
		clauses = append(clauses, `d.style = ?`)
		args = append(args, f.Style)
	}
	if f.MaxPrice > 0 {
		clauses = append(clauses, `d.price_from <= ?`)
		args = append(args, f.MaxPrice)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

func (f DesignFilter) page() int { return max(1, f.Page) }

func (f DesignFilter) limit() int {
	if f.PerPage <= 0 {
		return 6
	}
	return f.PerPage
}

// sortSQL whitelists sort options so the value can never reach SQL unchecked.
// clampPage keeps ?page=99 from showing an empty list when matches exist.
func clampPage(page, perPage, total int) int {
	pages := (total + perPage - 1) / perPage
	if pages > 0 && page > pages {
		return pages
	}
	return page
}

func sortSQL(sort, alias, priceCol string) string {
	switch sort {
	case "price-asc":
		return priceCol + " ASC"
	case "price-desc":
		return priceCol + " DESC"
	case "name":
		return alias + ".name COLLATE NOCASE ASC"
	default:
		return alias + ".created_at DESC, " + alias + ".id DESC"
	}
}

func collectFabrics(rows *sql.Rows) ([]Fabric, error) {
	var out []Fabric
	for rows.Next() {
		f, err := scanFabric(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func collectDesigns(rows *sql.Rows) ([]Design, error) {
	var out []Design
	for rows.Next() {
		d, err := scanDesign(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func humanise(s string) string {
	words := strings.FieldsFunc(s, func(r rune) bool { return r == '-' || r == '_' })
	for i, w := range words {
		if w == "" {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
