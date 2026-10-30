package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"gojias/internal/measure"
	"gojias/internal/store"
)

// ---------------------------------------------------------------------------
// Sessions
// ---------------------------------------------------------------------------

// adminSession is a logged-in dashboard user.
type adminSession struct {
	Email   string
	CSRF    string
	Expires time.Time
}

// sessionStore keeps sessions in memory: a restart logs the owner out, which is
// acceptable for a single-server shop and avoids a session table.
type sessionStore struct {
	mu       sync.Mutex
	sessions map[string]*adminSession
}

func newSessionStore() *sessionStore {
	return &sessionStore{sessions: map[string]*adminSession{}}
}

func (s *sessionStore) create(email string) (string, *adminSession) {
	token := randomHex(32)
	csrf := randomHex(16)
	sess := &adminSession{Email: email, CSRF: csrf, Expires: time.Now().Add(12 * time.Hour)}

	s.mu.Lock()
	defer s.mu.Unlock()
	// Opportunistically drop expired sessions.
	now := time.Now()
	for k, v := range s.sessions {
		if v.Expires.Before(now) {
			delete(s.sessions, k)
		}
	}
	s.sessions[token] = sess
	return token, sess
}

func (s *sessionStore) get(token string) (*adminSession, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[token]
	if !ok || sess.Expires.Before(time.Now()) {
		delete(s.sessions, token)
		return nil, false
	}
	return sess, true
}

func (s *sessionStore) destroy(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, token)
}

const sessionCookie = "gc_admin"

func (a *App) setSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/admin",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
		MaxAge:   int((12 * time.Hour).Seconds()),
	})
}

func (a *App) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/admin", HttpOnly: true, MaxAge: -1,
	})
}

// adminHandler is a handler that already has a valid session.
type adminHandler func(w http.ResponseWriter, r *http.Request, sess *adminSession)

// admin wraps a handler with session authentication and CSRF checks on POST.
func (a *App) admin(next adminHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil {
			a.redirectToLogin(w, r)
			return
		}
		sess, ok := a.sessions.get(cookie.Value)
		if !ok {
			a.clearSessionCookie(w)
			a.redirectToLogin(w, r)
			return
		}
		if r.Method == http.MethodPost {
			// Upload forms are multipart; ParseForm alone would leave their
			// fields (including the CSRF token) empty.
			if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
				if err := r.ParseMultipartForm(16 << 20); err != nil {
					http.Error(w, "Bad request", http.StatusBadRequest)
					return
				}
			} else if err := r.ParseForm(); err != nil {
				http.Error(w, "Bad request", http.StatusBadRequest)
				return
			}
			if subtle.ConstantTimeCompare([]byte(r.PostFormValue("csrf")), []byte(sess.CSRF)) != 1 {
				http.Error(w, "Your session expired. Please sign in again.", http.StatusForbidden)
				return
			}
		}
		next(w, r, sess)
	}
}

func (a *App) redirectToLogin(w http.ResponseWriter, r *http.Request) {
	next := r.URL.Path
	if !strings.HasPrefix(next, "/admin") || strings.HasPrefix(next, "//") {
		next = "/admin"
	}
	http.Redirect(w, r, "/admin/login?next="+url.QueryEscape(next), http.StatusSeeOther)
}

// ---------------------------------------------------------------------------
// Admin page model
// ---------------------------------------------------------------------------

// adminPage is the payload for admin templates.
type adminPage struct {
	Title  string
	Active string
	Email  string
	CSRF   string
	Flash  string
	Error  string
	Year   int
	Data   any
}

// renderAdminSet executes the template whose name matches the set's shell.
func (a *App) writeAdmin(w http.ResponseWriter, status int, set, name string, p adminPage) {
	t, ok := a.adminPages[set]
	if !ok {
		http.Error(w, "unknown admin page", http.StatusInternalServerError)
		return
	}
	p.Year = time.Now().Year()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := t.ExecuteTemplate(w, name, p); err != nil {
		log.Printf("render admin %s: %v", set, err)
	}
}

func newAdminPage(r *http.Request, sess *adminSession) adminPage {
	return adminPage{
		Email: sess.Email,
		CSRF:  sess.CSRF,
		Flash: r.URL.Query().Get("flash"),
		Year:  time.Now().Year(),
	}
}

// ---------------------------------------------------------------------------
// Login
// ---------------------------------------------------------------------------

func (a *App) adminLoginForm(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if _, ok := a.sessions.get(c.Value); ok {
			http.Redirect(w, r, "/admin", http.StatusSeeOther)
			return
		}
	}
	a.writeAdmin(w, http.StatusOK, "login", "base_login.html", adminPage{
		Title: "Sign in", Error: r.URL.Query().Get("error"), Year: time.Now().Year(),
		Data: map[string]any{"Next": r.URL.Query().Get("next")},
	})
}

func (a *App) adminLogin(w http.ResponseWriter, r *http.Request) {
	if !a.limiter.allow("login:" + clientIP(r)) {
		a.writeAdmin(w, http.StatusTooManyRequests, "login", "base_login.html", adminPage{
			Title: "Sign in", Error: "Too many attempts. Please wait a few minutes.", Year: time.Now().Year(),
			Data: map[string]any{"Next": "/admin"},
		})
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	email := strings.TrimSpace(r.PostFormValue("email"))
	password := r.PostFormValue("password")
	next := r.PostFormValue("next")
	if !strings.HasPrefix(next, "/admin") || strings.HasPrefix(next, "//") {
		next = "/admin"
	}

	admin, err := a.db.AdminByEmail(email)
	// Compare even when the account is missing, so timing does not reveal it.
	hash := admin.Hash
	if err != nil || hash == "" {
		hash = dummyHash
	}
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		log.Printf("failed admin login for %q from %s", email, clientIP(r))
		a.writeAdmin(w, http.StatusUnauthorized, "login", "base_login.html", adminPage{
			Title: "Sign in",
			Error: "Email or password is incorrect.",
			Year:  time.Now().Year(),
			Data:  map[string]any{"Next": next, "Email": email},
		})
		return
	}

	token, _ := a.sessions.create(admin.Email)
	a.setSessionCookie(w, r, token)
	http.Redirect(w, r, next, http.StatusSeeOther)
}

// dummyHash is a real bcrypt hash used for constant-ish time comparisons.
const dummyHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

func (a *App) adminLogout(w http.ResponseWriter, r *http.Request, sess *adminSession) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		a.sessions.destroy(c.Value)
	}
	a.clearSessionCookie(w)
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

// seedAdmin creates the first account, using ADMIN_EMAIL/ADMIN_PASSWORD when
// provided and printing a generated password otherwise.
func (a *App) seedAdmin() error {
	n, err := a.db.CountAdmins()
	if err != nil || n > 0 {
		return err
	}
	email := os.Getenv("ADMIN_EMAIL")
	if email == "" {
		email = "admin@gojiascollections.co.ke"
	}
	password := os.Getenv("ADMIN_PASSWORD")
	generated := password == ""
	if generated {
		password = randomHex(9)
	}
	if err := a.db.CreateAdmin(email, password); err != nil {
		return err
	}
	log.Printf("created admin account %s", email)
	if generated {
		log.Printf("TEMPORARY ADMIN PASSWORD: %s — change it in Settings after signing in", password)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Dashboard
// ---------------------------------------------------------------------------

func (a *App) adminDashboard(w http.ResponseWriter, r *http.Request, sess *adminSession) {
	counts, err := a.db.StatusCounts()
	if err != nil {
		log.Printf("status counts: %v", err)
	}
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	delivered, err := a.db.DeliveredSince(monthStart)
	if err != nil {
		log.Printf("delivered this month: %v", err)
	}
	recent, err := a.db.RecentOrders(8)
	if err != nil {
		log.Printf("recent orders: %v", err)
	}
	low, err := a.db.OutOfStockFabrics(6)
	if err != nil {
		log.Printf("low stock: %v", err)
	}

	inProgress := counts["confirmed"] + counts["cutting"] + counts["sewing"]

	p := newAdminPage(r, sess)
	p.Title = "Dashboard"
	p.Active = "dashboard"
	p.Data = map[string]any{
		"Counts":     counts,
		"New":        counts["new"],
		"Ready":      counts["ready"],
		"Delivered":  delivered,
		"InProgress": inProgress,
		"Recent":     recent,
		"LowStock":   low,
	}
	a.writeAdmin(w, http.StatusOK, "dashboard", "base.html", p)
}

// ---------------------------------------------------------------------------
// Orders
// ---------------------------------------------------------------------------

func (a *App) adminOrders(w http.ResponseWriter, r *http.Request, sess *adminSession) {
	q := r.URL.Query()
	filter := store.AdminOrderFilter{
		Query:  q.Get("q"),
		Status: q.Get("status"),
		From:   q.Get("from"),
		To:     q.Get("to"),
	}
	orders, err := a.db.AdminListOrders(filter)
	if err != nil {
		log.Printf("admin orders: %v", err)
		http.Error(w, "Could not load orders", http.StatusInternalServerError)
		return
	}
	p := newAdminPage(r, sess)
	p.Title = "Orders"
	p.Active = "orders"
	p.Data = map[string]any{
		"Orders":   orders,
		"Filter":   filter,
		"Statuses": store.Statuses,
		"Total":    len(orders),
	}
	a.writeAdmin(w, http.StatusOK, "orders", "base.html", p)
}

func (a *App) adminOrderDetail(w http.ResponseWriter, r *http.Request, sess *adminSession) {
	o, err := a.db.GetOrderByRef(r.PathValue("ref"))
	if err != nil {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}
	p := newAdminPage(r, sess)
	p.Title = "Order " + o.Ref
	p.Active = "orders"
	p.Data = map[string]any{
		"Order":      o,
		"Rows":       measurementRows(o),
		"Timeline":   buildTimeline(o),
		"Statuses":   store.Statuses,
		"FabricText": fabricText(o),
	}
	a.writeAdmin(w, http.StatusOK, "order_detail", "base.html", p)
}

func (a *App) adminOrderStatus(w http.ResponseWriter, r *http.Request, sess *adminSession) {
	ref := r.PathValue("ref")
	o, err := a.db.GetOrderByRef(ref)
	if err != nil {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}
	status := strings.TrimSpace(r.PostFormValue("status"))
	if store.StatusIndex(status) < 0 {
		a.redirectFlash(w, r, "/admin/orders/"+o.Ref, "Unknown status.")
		return
	}
	note := truncate(r.PostFormValue("note"), 500)
	if err := a.db.AddStatus(o.ID, status, note); err != nil {
		log.Printf("add status: %v", err)
		http.Error(w, "Could not update status", http.StatusInternalServerError)
		return
	}

	msg := "Status updated to " + store.StatusLabel(status) + "."
	if r.PostFormValue("notify") == "1" {
		if o.Email == "" {
			msg += " No customer email on file, so nothing was sent."
		} else if !a.mailer.Configured() {
			// Do not claim an email went out when SMTP is not set up.
			log.Printf("status notification for %s not emailed: SMTP not configured", o.Ref)
			msg += " No SMTP is configured on this server, so the notification was only logged."
		} else if err := a.mailer.SendTo(o.Email,
			"Update on your Gojias Collections order "+o.Ref,
			"Hello "+o.CustomerName+",\n\nYour order "+o.Ref+" is now: "+store.StatusLabel(status)+".\n\n"+
				"You can follow progress at /track using your reference and phone number.\n\n"+
				"Gojias Collections"); err != nil {
			log.Printf("notify customer: %v", err)
			msg += " The email could not be sent."
		} else {
			msg += " The customer was emailed."
		}
	}
	a.redirectFlash(w, r, "/admin/orders/"+o.Ref, msg)
}

func (a *App) adminOrderNotes(w http.ResponseWriter, r *http.Request, sess *adminSession) {
	o, err := a.db.GetOrderByRef(r.PathValue("ref"))
	if err != nil {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}
	notes := truncate(r.PostFormValue("internal_notes"), 2000)
	deposit := parseFloat(r.PostFormValue("deposit"))
	if deposit < 0 {
		deposit = 0
	}
	paymentNote := truncate(r.PostFormValue("payment_note"), 400)
	if err := a.db.UpdateOrderAdmin(o.Ref, notes, deposit, paymentNote); err != nil {
		log.Printf("update order admin: %v", err)
		http.Error(w, "Could not save", http.StatusInternalServerError)
		return
	}
	a.redirectFlash(w, r, "/admin/orders/"+o.Ref, "Order notes and payment saved.")
}

// adminMeasurementSheet renders a printable tailor's sheet.
func (a *App) adminMeasurementSheet(w http.ResponseWriter, r *http.Request, sess *adminSession) {
	o, err := a.db.GetOrderByRef(r.PathValue("ref"))
	if err != nil {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}
	p := adminPage{
		Title: "Measurement sheet " + o.Ref,
		Data: map[string]any{
			"Order":      o,
			"Rows":       measurementRows(o),
			"FabricText": fabricText(o),
			"Unit":       o.Unit,
		},
	}
	a.writeAdmin(w, http.StatusOK, "sheet", "sheet.html", p)
}

// ---------------------------------------------------------------------------
// Settings
// ---------------------------------------------------------------------------

func (a *App) adminSettings(w http.ResponseWriter, r *http.Request, sess *adminSession) {
	s, err := a.db.LoadSettings()
	if err != nil {
		log.Printf("load settings: %v", err)
		s = store.DefaultSettings()
	}
	p := newAdminPage(r, sess)
	p.Title = "Settings"
	p.Active = "settings"
	p.Data = map[string]any{"Settings": s}
	a.writeAdmin(w, http.StatusOK, "settings", "base.html", p)
}

func (a *App) adminSaveSettings(w http.ResponseWriter, r *http.Request, sess *adminSession) {
	s := store.Settings{
		BusinessName: truncate(r.PostFormValue("business_name"), 80),
		WhatsApp:     digitsOnly(r.PostFormValue("whatsapp")),
		Phone:        truncate(r.PostFormValue("phone"), 40),
		Email:        truncate(r.PostFormValue("email"), 120),
		Address:      truncate(r.PostFormValue("address"), 160),
		Hours:        truncate(r.PostFormValue("hours"), 160),
		NotifyEmail:  r.PostFormValue("notify_email") == "1",
	}
	var problems []string
	if len(s.BusinessName) < 2 {
		problems = append(problems, "Business name is too short.")
	}
	if len(s.WhatsApp) < 9 || len(s.WhatsApp) > 15 {
		problems = append(problems, "WhatsApp number must be 9–15 digits, international format.")
	}
	if !emailPattern.MatchString(s.Email) {
		problems = append(problems, "Email address looks invalid.")
	}
	if len(problems) > 0 {
		p := newAdminPage(r, sess)
		p.Title = "Settings"
		p.Active = "settings"
		p.Error = strings.Join(problems, " ")
		p.Data = map[string]any{"Settings": s}
		a.writeAdmin(w, http.StatusBadRequest, "settings", "base.html", p)
		return
	}
	if err := a.db.SaveSettings(s); err != nil {
		log.Printf("save settings: %v", err)
		http.Error(w, "Could not save settings", http.StatusInternalServerError)
		return
	}
	setSiteSettings(s)
	a.redirectFlash(w, r, "/admin/settings", "Settings saved. They are live on the site now.")
}

func (a *App) adminChangePassword(w http.ResponseWriter, r *http.Request, sess *adminSession) {
	current := r.PostFormValue("current_password")
	next := r.PostFormValue("new_password")
	confirm := r.PostFormValue("confirm_password")

	admin, err := a.db.AdminByEmail(sess.Email)
	if err != nil {
		http.Error(w, "Account not found", http.StatusInternalServerError)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(admin.Hash), []byte(current)) != nil {
		a.redirectFlash(w, r, "/admin/settings", "Current password is incorrect.")
		return
	}
	switch {
	case len(next) < 8:
		a.redirectFlash(w, r, "/admin/settings", "New password must be at least 8 characters.")
		return
	case next != confirm:
		a.redirectFlash(w, r, "/admin/settings", "New passwords do not match.")
		return
	case next == current:
		a.redirectFlash(w, r, "/admin/settings", "Choose a password different from the current one.")
		return
	}
	if err := a.db.UpdateAdminPassword(admin.ID, next); err != nil {
		log.Printf("update password: %v", err)
		http.Error(w, "Could not change password", http.StatusInternalServerError)
		return
	}
	a.redirectFlash(w, r, "/admin/settings", "Password changed.")
}

// ---------------------------------------------------------------------------
// Small helpers
// ---------------------------------------------------------------------------

func (a *App) redirectFlash(w http.ResponseWriter, r *http.Request, path, msg string) {
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	http.Redirect(w, r, path+sep+"flash="+url.QueryEscape(msg), http.StatusSeeOther)
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func randomHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		// Fall back to a time-derived value rather than returning empty.
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(buf)
}

// measurementOptions is the union of every category's fields, used by the
// design form's checkboxes.
type measurementOption struct {
	Key   string
	Label string
}

func allMeasurementOptions() []measurementOption {
	seen := map[string]bool{}
	var out []measurementOption
	for _, c := range measure.Categories {
		for _, f := range measure.Fields(c) {
			if seen[f.Key] {
				continue
			}
			seen[f.Key] = true
			out = append(out, measurementOption{Key: f.Key, Label: f.Label})
		}
	}
	return out
}
