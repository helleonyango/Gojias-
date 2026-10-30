// Command gojias runs the Gojias Collections website: a Go HTTP server that
// renders server-side templates over a SQLite catalogue of fabrics and designs.
//
// The public side covers Home, Fabrics, Designs, the five-step order wizard,
// confirmation with a PDF summary, and order tracking. The staff dashboard at
// /admin manages the catalogue, orders and business settings. The content pages
// (How to Measure, About, Contact) still show a placeholder.
package main

import (
	"bytes"
	"embed"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"gojias/internal/notify"
	"gojias/internal/store"
)

// Templates and static assets are embedded so the binary can be deployed as a
// single file. The uploads folder stays on disk because it holds customer art.
//
//go:embed templates static
var assets embed.FS

const brand = "Gojias Collections"

// categories drives the tab bar on the Designs page.
var categories = []struct{ Slug, Label string }{
	{"", "All"},
	{"women", "Women"},
	{"men", "Men"},
	{"children", "Children"},
	{"couples", "Couples/Family"},
	{"bridal", "Bridal & Occasion"},
}

// testimonials are hard-coded for stage 1; the admin dashboard will manage
// them once stage 3 is built.
var testimonials = []struct{ Name, Role, Quote string }{
	{"Wanjiru M.", "Bride, Kisumu", "They sewed my gown in kitenge and I have never felt more myself in a dress. Four fittings, and not one rushed."},
	{"Otieno K.", "Nakuru", "I sent my measurements from Nakuru and the kaftan fitted like it was pinned on me. The WhatsApp updates kept me calm."},
	{"Achieng' O.", "Family set, Kisumu", "Six outfits from one bolt, all matching for the photos. The children have already asked for the next set."},
}

// App holds the database connection, parsed templates, the mailer and the
// order-endpoint rate limiter.
type App struct {
	db         *store.DB
	pages      map[string]*template.Template
	adminPages map[string]*template.Template
	sessions   *sessionStore
	mailer     notify.Mailer
	limiter    *rateLimiter
	uploadDir  string
}

// page is the common shape every template receives.
type page struct {
	Title  string
	Desc   string // meta description
	Active string // nav item to highlight: home, fabrics, designs...
	Year   int
	Data   any
}

// ---------------------------------------------------------------------------
// Wiring
// ---------------------------------------------------------------------------

func main() {
	addr := flag.String("addr", ":8080", "address to listen on")
	dbPath := flag.String("db", "gojias.db", "path to the SQLite database file")
	flag.Parse()

	if err := run(*addr, *dbPath); err != nil {
		log.Fatal(err)
	}
}

func run(addr, dbPath string) error {
	db, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	pages, err := loadTemplates()
	if err != nil {
		return err
	}
	adminPages, err := loadAdminTemplates()
	if err != nil {
		return err
	}
	if err := os.MkdirAll("uploads", 0o755); err != nil {
		return fmt.Errorf("create uploads dir: %w", err)
	}

	// Business details live in the database so the owner can edit them.
	settings, err := db.LoadSettings()
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}
	setSiteSettings(settings)

	app := &App{
		db:         db,
		pages:      pages,
		adminPages: adminPages,
		sessions:   newSessionStore(),
		mailer:     notify.FromEnv(),
		limiter:    newRateLimiter(6, 10*time.Minute),
		uploadDir:  "uploads",
	}
	if err := app.seedAdmin(); err != nil {
		return fmt.Errorf("seed admin: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(mustSub(assets, "static")))))
	mux.Handle("GET /uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))))

	mux.HandleFunc("GET /{$}", app.home)
	mux.HandleFunc("GET /fabrics", app.fabrics)
	mux.HandleFunc("GET /fabrics/{slug}", app.fabricDetail)
	mux.HandleFunc("GET /designs", app.designs)
	mux.HandleFunc("GET /designs/{slug}", app.designDetail)

	// Order wizard, confirmation, PDF summary and tracking.
	mux.HandleFunc("GET /order", app.orderForm)
	mux.HandleFunc("POST /order", app.orderSubmit)
	mux.HandleFunc("GET /orders/{ref}", app.orderConfirmation)
	mux.HandleFunc("GET /orders/{ref}/summary.pdf", app.orderSummaryPDF)
	mux.HandleFunc("GET /track", app.trackForm)
	mux.HandleFunc("POST /track", app.trackLookup)

	// Admin dashboard. Everything except the login form is session-protected and
	// POSTs additionally require the session's CSRF token.
	mux.HandleFunc("GET /admin/login", app.adminLoginForm)
	mux.HandleFunc("POST /admin/login", app.adminLogin)
	mux.HandleFunc("POST /admin/logout", app.admin(app.adminLogout))
	mux.HandleFunc("GET /admin", app.admin(app.adminDashboard))

	mux.HandleFunc("GET /admin/fabrics", app.admin(app.adminFabrics))
	mux.HandleFunc("GET /admin/fabrics/new", app.admin(app.adminFabricNew))
	mux.HandleFunc("POST /admin/fabrics", app.admin(app.adminFabricSave))
	mux.HandleFunc("GET /admin/fabrics/{id}/edit", app.admin(app.adminFabricEdit))
	mux.HandleFunc("POST /admin/fabrics/{id}", app.admin(app.adminFabricSave))
	mux.HandleFunc("POST /admin/fabrics/{id}/delete", app.admin(app.adminFabricDelete))
	mux.HandleFunc("POST /admin/fabrics/{id}/toggle", app.admin(app.adminFabricToggle))

	mux.HandleFunc("GET /admin/designs", app.admin(app.adminDesigns))
	mux.HandleFunc("GET /admin/designs/new", app.admin(app.adminDesignNew))
	mux.HandleFunc("POST /admin/designs", app.admin(app.adminDesignSave))
	mux.HandleFunc("GET /admin/designs/{id}/edit", app.admin(app.adminDesignEdit))
	mux.HandleFunc("POST /admin/designs/{id}", app.admin(app.adminDesignSave))
	mux.HandleFunc("POST /admin/designs/{id}/delete", app.admin(app.adminDesignDelete))
	mux.HandleFunc("POST /admin/designs/{id}/toggle", app.admin(app.adminDesignToggle))

	mux.HandleFunc("GET /admin/orders", app.admin(app.adminOrders))
	mux.HandleFunc("GET /admin/orders/{ref}", app.admin(app.adminOrderDetail))
	mux.HandleFunc("GET /admin/orders/{ref}/sheet", app.admin(app.adminMeasurementSheet))
	mux.HandleFunc("POST /admin/orders/{ref}/status", app.admin(app.adminOrderStatus))
	mux.HandleFunc("POST /admin/orders/{ref}/notes", app.admin(app.adminOrderNotes))

	mux.HandleFunc("GET /admin/settings", app.admin(app.adminSettings))
	mux.HandleFunc("POST /admin/settings", app.admin(app.adminSaveSettings))
	mux.HandleFunc("POST /admin/settings/password", app.admin(app.adminChangePassword))

	// Content pages that are still to be written.
	comingSoon := map[string]string{
		"/how-to-measure": "How to Measure",
		"/about":          "About Gojias Collections",
		"/contact":        "Contact & Visit",
	}
	for route, title := range comingSoon {
		route, title := route, title
		mux.HandleFunc("GET "+route, func(w http.ResponseWriter, r *http.Request) {
			app.render(w, http.StatusOK, "coming-soon", page{
				Title:  title,
				Desc:   title + " at Gojias Collections.",
				Active: activeFor(r.URL.Path),
				Data:   map[string]any{"Heading": title},
			})
		})
	}
	mux.HandleFunc("/", app.notFound)

	srv := &http.Server{
		Addr:              addr,
		Handler:           logRequests(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("%s listening on http://localhost%s", brand, addr)
	return srv.ListenAndServe()
}

func mustSub(fsys embed.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}

func activeFor(path string) string {
	switch {
	case path == "/":
		return "home"
	case strings.HasPrefix(path, "/fabrics"):
		return "fabrics"
	case strings.HasPrefix(path, "/designs"):
		return "designs"
	default:
		return ""
	}
}

// logRequests is a deliberately small access log: method, path, status, time.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Millisecond))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

func (a *App) home(w http.ResponseWriter, r *http.Request) {
	designs, err := a.db.FeaturedDesigns(6)
	if err != nil {
		log.Printf("featured designs: %v", err)
	}
	fabrics, err := a.db.FeaturedFabrics(4)
	if err != nil {
		log.Printf("featured fabrics: %v", err)
	}
	a.render(w, http.StatusOK, "home", page{
		Title:  "Kitenge & Ankara tailoring, made to measure",
		Desc:   "Gojias Collections designs and sews African attire inspired by kitenge. Browse fabrics, choose a design, send your measurements and we sew and deliver.",
		Active: "home",
		Data: map[string]any{
			"Designs":      designs,
			"Fabrics":      fabrics,
			"Testimonials": testimonials,
		},
	})
}

func (a *App) fabrics(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	perPage := 6
	filter := store.FabricFilter{
		Query:       q.Get("q"),
		Type:        q.Get("type"),
		Colour:      q.Get("colour"),
		MaxPrice:    atof(q.Get("max")),
		InStockOnly: q.Get("stock") == "in",
		Sort:        q.Get("sort"),
		Page:        max(1, atoi(q.Get("page"))),
		PerPage:     perPage,
	}

	fabrics, total, err := a.db.ListFabrics(filter)
	if err != nil {
		log.Printf("list fabrics: %v", err)
		http.Error(w, "Could not load fabrics", http.StatusInternalServerError)
		return
	}
	types, colours, maxPrice, err := a.db.FabricFacets()
	if err != nil {
		log.Printf("fabric facets: %v", err)
	}

	title := "Fabrics"
	if filter.Type != "" {
		title = label(filter.Type) + " fabrics"
	}
	a.render(w, http.StatusOK, "fabrics", page{
		Title:  title,
		Desc:   "Browse kitenge, Ankara, lace and plain cotton fabrics at Gojias Collections. Prices per yard or metre in Kenyan shillings.",
		Active: "fabrics",
		Data: map[string]any{
			"Fabrics":  fabrics,
			"Filter":   filter,
			"Types":    types,
			"Colours":  colours,
			"MaxPrice": maxPrice,
			"Pager":    buildPager(r, filter.Page, perPage, total),
			"Total":    total,
			"QuerySome": filter.Query != "" || filter.Type != "" || filter.Colour != "" ||
				filter.MaxPrice > 0 || filter.InStockOnly,
		},
	})
}

func (a *App) fabricDetail(w http.ResponseWriter, r *http.Request) {
	fabric, err := a.db.GetFabric(r.PathValue("slug"))
	if err != nil {
		a.notFound(w, r)
		return
	}
	designs, err := a.db.DesignsForFabric(fabric.ID, 3)
	if err != nil {
		log.Printf("designs for fabric: %v", err)
	}
	a.render(w, http.StatusOK, "fabric", page{
		Title:  fabric.Name,
		Desc:   fabric.Name + " — " + fabric.TypeLabel() + " fabric at KES " + moneyPlain(fabric.Price) + " per " + fabric.Unit + ".",
		Active: "fabrics",
		Data: map[string]any{
			"Fabric":  fabric,
			"Designs": designs,
		},
	})
}

func (a *App) designs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	perPage := 6
	filter := store.DesignFilter{
		Query:    q.Get("q"),
		Category: q.Get("category"),
		Style:    q.Get("style"),
		MaxPrice: atof(q.Get("max")),
		Sort:     q.Get("sort"),
		Page:     max(1, atoi(q.Get("page"))),
		PerPage:  perPage,
	}

	designs, total, err := a.db.ListDesigns(filter)
	if err != nil {
		log.Printf("list designs: %v", err)
		http.Error(w, "Could not load designs", http.StatusInternalServerError)
		return
	}
	styles, err := a.db.DesignStyles()
	if err != nil {
		log.Printf("design styles: %v", err)
	}

	a.render(w, http.StatusOK, "designs", page{
		Title:  "Ready-made designs",
		Desc:   "Women's, men's, children's and bridal designs made to measure from kitenge and Ankara. Starting prices in Kenyan shillings.",
		Active: "designs",
		Data: map[string]any{
			"Designs":    designs,
			"Filter":     filter,
			"Categories": categories,
			"Styles":     styles,
			"Pager":      buildPager(r, filter.Page, perPage, total),
			"Total":      total,
			"QuerySome":  filter.Query != "" || filter.Category != "" || filter.Style != "" || filter.MaxPrice > 0,
		},
	})
}

func (a *App) designDetail(w http.ResponseWriter, r *http.Request) {
	design, err := a.db.GetDesign(r.PathValue("slug"))
	if err != nil {
		a.notFound(w, r)
		return
	}
	// Hide the chosen design's own recommended fabrics from nothing: they are
	// already filtered in the query. Fall back to featured fabrics when a
	// design has no recommendations configured.
	a.render(w, http.StatusOK, "design", page{
		Title:  design.Name,
		Desc:   design.Name + " — a made-to-measure " + design.CategoryLabel() + " design by Gojias Collections, from KES " + moneyPlain(design.PriceFrom) + ".",
		Active: "designs",
		Data:   map[string]any{"Design": design},
	})
}

func (a *App) notFound(w http.ResponseWriter, r *http.Request) {
	a.render(w, http.StatusNotFound, "notfound", page{
		Title: "Page not found",
		Desc:  "That page is not here. Browse our fabrics or designs instead.",
		Data:  nil,
	})
}

// ---------------------------------------------------------------------------
// Rendering
// ---------------------------------------------------------------------------

func loadTemplates() (map[string]*template.Template, error) {
	names := []string{"home", "fabrics", "fabric", "designs", "design", "coming-soon", "notfound",
		"order", "confirmation", "track"}
	funcs := template.FuncMap{
		"money":      money,
		"date":       date,
		"wa":         wa,
		"label":      label,
		"swatch":     swatch,
		"get":        func(m map[string]string, key string) string { return m[key] },
		"num":        trimNumber,
		"fabricText": fabricText,
	}
	// Settings-backed helpers (also used by the public footer).
	funcs["bizName"] = bizName
	funcs["contactPhone"] = contactPhone
	funcs["contactEmail"] = contactEmail
	funcs["contactAddress"] = contactAddress
	funcs["contactHours"] = contactHours
	funcs["statusLabel"] = store.StatusLabel

	out := make(map[string]*template.Template, len(names))
	for _, name := range names {
		t, err := template.New("base").Funcs(funcs).ParseFS(assets,
			"templates/base.html",
			"templates/partials/*.html",
			"templates/pages/"+name+".html",
		)
		if err != nil {
			return nil, fmt.Errorf("parse templates for %s: %w", name, err)
		}
		out[name] = t
	}
	return out, nil
}

func (a *App) render(w http.ResponseWriter, status int, name string, p page) {
	t, ok := a.pages[name]
	if !ok {
		http.Error(w, "unknown page", http.StatusInternalServerError)
		return
	}
	p.Year = time.Now().Year()
	var buf bytes.Buffer
	// base.html holds the document shell at top level, so it parses into the
	// "base.html" template; every page file contributes "content".
	if err := t.ExecuteTemplate(&buf, "base.html", p); err != nil {
		log.Printf("render %s: %v", name, err)
		http.Error(w, "Could not render page", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

// ---------------------------------------------------------------------------
// Pagination
// ---------------------------------------------------------------------------

// PageLink is one numbered button in the pager.
type PageLink struct {
	Num    int
	URL    string
	Active bool
}

// Pager carries everything the pager partial needs.
type Pager struct {
	Page, TotalPages, Total, From, To int
	Links                             []PageLink
	Prev, Next                        string
	HasPrev, HasNext                  bool
}

func buildPager(r *http.Request, current, perPage, total int) Pager {
	totalPages := int(math.Ceil(float64(total) / float64(perPage)))
	p := Pager{Page: current, TotalPages: totalPages, Total: total}
	if totalPages == 0 {
		return p
	}
	if p.Page > totalPages {
		p.Page = totalPages
	}
	p.From = (p.Page-1)*perPage + 1
	p.To = min(p.Page*perPage, total)

	link := func(n int) string {
		q := r.URL.Query()
		q.Set("page", strconv.Itoa(n))
		return r.URL.Path + "?" + q.Encode()
	}

	// At most five numbered links, centred on the current page.
	start := max(1, p.Page-2)
	end := min(totalPages, start+4)
	start = max(1, end-4)
	for n := start; n <= end; n++ {
		p.Links = append(p.Links, PageLink{Num: n, URL: link(n), Active: n == p.Page})
	}
	if p.Page > 1 {
		p.HasPrev = true
		p.Prev = link(p.Page - 1)
	}
	if p.Page < totalPages {
		p.HasNext = true
		p.Next = link(p.Page + 1)
	}
	return p
}

// ---------------------------------------------------------------------------
// Template helpers
// ---------------------------------------------------------------------------

// money renders "KES 1,250" (whole shillings, comma grouped).
func money(v float64) string { return "KES " + moneyPlain(v) }

func moneyPlain(v float64) string {
	rounded := math.Round(v)
	neg := rounded < 0
	if neg {
		rounded = -rounded
	}
	s := strconv.FormatInt(int64(rounded), 10)
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}

// date avoids printing a zero date when a row is malformed.
func date(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2 Jan 2006")
}

// wa builds a WhatsApp deep link with a pre-filled message, using the number
// saved in Settings.
func wa(msg string) string {
	return "https://wa.me/" + currentSettings().WhatsApp + "?text=" + url.QueryEscape(msg)
}

// label turns a stored measurement key into a readable name.
func label(key string) string {
	words := strings.FieldsFunc(key, func(r rune) bool { return r == '-' || r == '_' })
	for i, w := range words {
		if w != "" {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// swatch maps the colour names used in the catalogue to hex values so cards can
// show real colour dots. Unknown names fall back to the brand gold.
func swatch(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "orange":
		return "#D2691E"
	case "terracotta":
		return "#B84A2E"
	case "gold":
		return "#C8973B"
	case "indigo":
		return "#22355B"
	case "cream":
		return "#F3E8D5"
	case "ivory":
		return "#FFF8EC"
	case "sand":
		return "#D9C7A7"
	case "maroon":
		return "#6B2B2B"
	case "brown":
		return "#6B4A34"
	default:
		return "#C8973B"
	}
}

func atoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

func atof(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

// ---------------------------------------------------------------------------
// Site settings (editable in the dashboard)
// ---------------------------------------------------------------------------

var (
	settingsMu   sync.RWMutex
	siteSettings = store.DefaultSettings()
)

// currentSettings returns a copy of the live settings.
func currentSettings() store.Settings {
	settingsMu.RLock()
	defer settingsMu.RUnlock()
	return siteSettings
}

// setSiteSettings swaps the live settings after the owner saves them.
func setSiteSettings(s store.Settings) {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	siteSettings = s
}

func bizName() string        { return currentSettings().BusinessName }
func contactPhone() string   { return currentSettings().Phone }
func contactEmail() string   { return currentSettings().Email }
func contactAddress() string { return currentSettings().Address }
func contactHours() string   { return currentSettings().Hours }

// ---------------------------------------------------------------------------
// Admin templates
// ---------------------------------------------------------------------------

// adminFuncs mirrors the public helpers the dashboard templates need.
func adminFuncs() template.FuncMap {
	return template.FuncMap{
		"money":          money,
		"date":           date,
		"wa":             wa,
		"label":          label,
		"num":            trimNumber,
		"statusLabel":    store.StatusLabel,
		"bizName":        bizName,
		"contactPhone":   contactPhone,
		"contactEmail":   contactEmail,
		"contactAddress": contactAddress,
		"contactHours":   contactHours,
		"get":            func(m map[string]string, key string) string { return m[key] },
	}
}

// loadAdminTemplates parses each dashboard page together with its shell. The
// login form and the printable sheet use their own standalone shells.
func loadAdminTemplates() (map[string]*template.Template, error) {
	funcs := adminFuncs()
	out := map[string]*template.Template{}

	for _, name := range []string{"dashboard", "fabrics", "fabric_form", "designs",
		"design_form", "orders", "order_detail", "settings"} {
		t, err := template.New("admin").Funcs(funcs).ParseFS(assets,
			"templates/admin/base.html",
			"templates/admin/pages/"+name+".html",
		)
		if err != nil {
			return nil, fmt.Errorf("parse admin templates for %s: %w", name, err)
		}
		out[name] = t
	}

	login, err := template.New("login").Funcs(funcs).ParseFS(assets,
		"templates/admin/base_login.html", "templates/admin/pages/login.html")
	if err != nil {
		return nil, fmt.Errorf("parse admin login: %w", err)
	}
	out["login"] = login

	sheet, err := template.New("sheet").Funcs(funcs).ParseFS(assets, "templates/admin/sheet.html")
	if err != nil {
		return nil, fmt.Errorf("parse measurement sheet: %w", err)
	}
	out["sheet"] = sheet

	return out, nil
}
