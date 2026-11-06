package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"gojias/internal/measure"
	"gojias/internal/pdf"
	"gojias/internal/store"
)

// maxUpload is the biggest inspiration photo we accept.
const maxUpload = 5 << 20 // 5 MB

// ---------------------------------------------------------------------------
// Wizard view models
// ---------------------------------------------------------------------------

// measureGroup renders one category's measurement inputs. All groups are
// rendered; JavaScript shows the one matching the chosen design.
type measureGroup struct {
	Category string
	Label    string
	Note     string
	Fields   []measure.Field
}

// wizardData is the payload for the order form.
type wizardData struct {
	Designs        []store.Design
	Fabrics        []store.Fabric
	Groups         []measureGroup
	Categories     []string
	SelectedDesign string
	SelectedFabric string
	Category       string // category of the preselected design
	Values         map[string]string
	Errors         map[string]string
	ErrorStep      int
	Catalog        template.JS
	MinDate        string
	Today          string
}

type designJS struct {
	Slug     string  `json:"slug"`
	Name     string  `json:"name"`
	Category string  `json:"category"`
	Style    string  `json:"style"`
	Price    float64 `json:"price"`
	Days     int     `json:"days"`
	Yards    float64 `json:"yards"`
}

type fabricJS struct {
	Slug  string  `json:"slug"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
	Unit  string  `json:"unit"`
}

// ---------------------------------------------------------------------------
// GET /order
// ---------------------------------------------------------------------------

func (a *App) orderForm(w http.ResponseWriter, r *http.Request) {
	data, err := a.buildWizardData()
	if err != nil {
		log.Printf("order form: %v", err)
		http.Error(w, "Could not load the order form", http.StatusInternalServerError)
		return
	}

	// A fabric or design chosen on the catalogue pages is carried in.
	if slug := r.URL.Query().Get("design"); slug != "" {
		for _, d := range data.Designs {
			if d.Slug == slug {
				data.SelectedDesign = d.Slug
				data.Category = d.Category
				data.Values["design"] = d.Slug
				data.Values["yardage"] = trimNumber(estimateYardage(d))
				break
			}
		}
	}
	if slug := r.URL.Query().Get("fabric"); slug != "" {
		for _, f := range data.Fabrics {
			if f.Slug == slug && f.InStock {
				data.SelectedFabric = f.Slug
				data.Values["fabric"] = f.Slug
				break
			}
		}
	}

	a.render(w, http.StatusOK, "order", page{
		Title:  "Start your order",
		Desc:   "Order a made-to-measure kitenge outfit in five steps: design, fabric, measurements, details and review.",
		Active: "order",
		Data:   data,
	})
}

// buildWizardData loads the catalogue, measurement groups and the JSON blob the
// client-side estimate uses.
func (a *App) buildWizardData() (*wizardData, error) {
	designs, _, err := a.db.ListDesigns(store.DesignFilter{PerPage: 100, Sort: "name"})
	if err != nil {
		return nil, err
	}
	fabrics, _, err := a.db.ListFabrics(store.FabricFilter{PerPage: 100, Sort: "name"})
	if err != nil {
		return nil, err
	}

	groups := make([]measureGroup, 0, len(measure.Categories))
	for _, c := range measure.Categories {
		groups = append(groups, measureGroup{
			Category: c,
			Label:    wizardCategoryLabel(c),
			Note:     measure.Note(c),
			Fields:   measure.Fields(c),
		})
	}

	designPayload := make([]designJS, 0, len(designs))
	for _, d := range designs {
		designPayload = append(designPayload, designJS{
			Slug: d.Slug, Name: d.Name, Category: d.Category, Style: d.Style,
			Price: d.PriceFrom, Days: d.ProductionDays, Yards: estimateYardage(d),
		})
	}
	fabricPayload := make([]fabricJS, 0, len(fabrics))
	for _, f := range fabrics {
		fabricPayload = append(fabricPayload, fabricJS{Slug: f.Slug, Name: f.Name, Price: f.Price, Unit: f.Unit})
	}
	raw, err := json.Marshal(map[string]any{"designs": designPayload, "fabrics": fabricPayload})
	if err != nil {
		return nil, err
	}

	now := time.Now()
	return &wizardData{
		Designs:    designs,
		Fabrics:    fabrics,
		Groups:     groups,
		Categories: measure.Categories,
		Values:     map[string]string{"unit": "cm", "design": "", "fabric": ""},
		Errors:     map[string]string{},
		Catalog:    template.JS(raw),
		MinDate:    now.AddDate(0, 0, 3).Format("2006-01-02"),
		Today:      now.Format("2006-01-02"),
	}, nil
}

// ---------------------------------------------------------------------------
// POST /order
// ---------------------------------------------------------------------------

func (a *App) orderSubmit(w http.ResponseWriter, r *http.Request) {
	if !a.limiter.allow(clientIP(r)) {
		w.Header().Set("Retry-After", "600")
		http.Error(w, "Too many order attempts. Please wait a few minutes and try again, or message us on WhatsApp.", http.StatusTooManyRequests)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUpload+4<<20)
	if err := r.ParseMultipartForm(maxUpload); err != nil {
		log.Printf("parse order form: %v", err)
		http.Error(w, "That submission was too large. Keep the inspiration photo under 5 MB.", http.StatusBadRequest)
		return
	}

	data, err := a.buildWizardData()
	if err != nil {
		log.Printf("order submit: %v", err)
		http.Error(w, "Could not load the order form", http.StatusInternalServerError)
		return
	}

	form := r.MultipartForm.Value
	field := func(key string) string {
		if v, ok := form[key]; ok && len(v) > 0 {
			return strings.TrimSpace(v[0])
		}
		return ""
	}

	// Preserve what the customer typed so nothing is lost on an error.
	for _, key := range []string{"design", "fabric", "category", "yardage", "unit",
		"name", "phone", "email", "delivery", "address", "preferred_date", "notes",
		"custom_description"} {
		data.Values[key] = field(key)
	}
	unit := data.Values["unit"]
	if !measure.ValidUnit(unit) {
		unit = "cm"
		data.Values["unit"] = "cm"
	}
	if data.Values["delivery"] == "" {
		data.Values["delivery"] = "pickup"
	}

	errorsOut := map[string]string{}
	designSlug := data.Values["design"]
	custom := designSlug == "custom"

	// --- Step 1: design -----------------------------------------------------
	var design store.Design
	category := ""
	switch {
	case custom:
		category = field("category")
		if !isMeasureCategory(category) {
			errorsOut["category"] = "Tell us who the outfit is for."
		}
		if len([]rune(field("custom_description"))) < 10 && len(r.MultipartForm.File["inspiration"]) == 0 {
			errorsOut["custom_description"] = "Describe your idea in a sentence or two, or attach an inspiration photo."
		}
	case designSlug == "":
		errorsOut["design"] = "Choose a design to start with."
	default:
		found := false
		for _, d := range data.Designs {
			if d.Slug == designSlug {
				design, found = d, true
				break
			}
		}
		if !found {
			errorsOut["design"] = "That design is no longer available. Pick another below."
		} else {
			category = design.Category
			data.Category = design.Category
		}
	}

	// --- Step 1b: optional inspiration photo --------------------------------
	var inspirationPath string
	if a.hasUpload(r) {
		if msg := a.validateUpload(r); msg != "" {
			errorsOut["inspiration"] = msg
		}
	}

	// --- Step 2: fabric -----------------------------------------------------
	var fabric store.Fabric
	bringOwn := false
	fabricSlug := data.Values["fabric"]
	if fabricSlug == "" {
		errorsOut["fabric"] = "Choose a fabric, or bring your own."
	} else if fabricSlug == "own" {
		bringOwn = true
	} else {
		found := false
		for _, f := range data.Fabrics {
			if f.Slug == fabricSlug {
				fabric, found = f, true
				break
			}
		}
		if !found {
			errorsOut["fabric"] = "That fabric is no longer listed. Please choose another."
		} else if !fabric.InStock {
			errorsOut["fabric"] = fabric.Name + " is out of stock. Choose another fabric, or ask us to reorder it on WhatsApp."
		}
	}

	yards := parseFloat(data.Values["yardage"])
	if yards <= 0 {
		if custom {
			yards = 4
		} else {
			yards = estimateYardage(design)
		}
	}
	if yards < 0.5 || yards > 40 {
		errorsOut["yardage"] = "Enter a yardage between 0.5 and 40."
	}
	if data.Values["yardage"] == "" {
		data.Values["yardage"] = trimNumber(yards)
	}

	// --- Step 3: measurements ----------------------------------------------
	raw := map[string]string{}
	for _, f := range measure.Fields(category) {
		raw[f.Key] = field("measure_" + f.Key)
	}
	measurements, mErr := measure.Validate(raw, category, unit)
	for k, v := range mErr {
		errorsOut[k] = v
	}

	notes := truncate(field("notes"), 2000)

	// --- Step 4: customer details ------------------------------------------
	name := truncate(field("name"), 80)
	phone := truncate(field("phone"), 30)
	email := truncate(field("email"), 120)
	delivery := data.Values["delivery"]
	address := truncate(field("address"), 300)
	preferred := data.Values["preferred_date"]

	if len([]rune(name)) < 2 {
		errorsOut["name"] = "Tell us your name so we know who is collecting."
	}
	if digits := store.NormalisePhone(phone); len(digits) < 9 || len(digits) > 13 {
		errorsOut["phone"] = "Enter a phone number we can reach you on, e.g. 0712 345 678."
	}
	if email != "" && !emailPattern.MatchString(email) {
		errorsOut["email"] = "That email address looks incomplete."
	}
	if delivery != "pickup" && delivery != "delivery" {
		errorsOut["delivery"] = "Choose pickup or delivery."
	}
	if delivery == "delivery" && len([]rune(address)) < 8 {
		errorsOut["address"] = "Add the delivery address, including the estate or town."
	}
	if preferred == "" {
		errorsOut["preferred_date"] = "Pick the date you would like the outfit ready."
	} else if d, err := time.Parse("2006-01-02", preferred); err != nil {
		errorsOut["preferred_date"] = "Use the date picker to choose a completion date."
	} else if d.Before(time.Now().AddDate(0, 0, 3)) {
		errorsOut["preferred_date"] = "We need at least three days. Choose a later date, or message us about a rush order."
	}

	// --- Step 5: terms -----------------------------------------------------
	if field("terms") != "1" {
		errorsOut["terms"] = "Please accept the terms so we can start sewing."
	}

	if len(errorsOut) > 0 {
		data.Errors = errorsOut
		data.ErrorStep = errorStep(errorsOut)
		a.render(w, http.StatusBadRequest, "order", page{
			Title:  "Start your order",
			Desc:   "Order a made-to-measure kitenge outfit in five steps.",
			Active: "order",
			Data:   data,
		})
		return
	}

	// --- Save the inspiration photo (only once everything is valid) --------
	if a.hasUpload(r) {
		saved, msg := a.saveUpload(r)
		if msg != "" {
			data.Errors = map[string]string{"inspiration": msg}
			data.ErrorStep = 1
			a.render(w, http.StatusBadRequest, "order", page{
				Title: "Start your order", Active: "order", Data: data,
			})
			return
		}
		inspirationPath = saved
	}

	// --- Price and timeline (computed server-side, never trusted from JS) --
	price := 0.0
	productionDays := design.ProductionDays
	if custom {
		productionDays = 14
	} else {
		price = design.PriceFrom
		if !bringOwn && fabric.ID != 0 {
			price += fabric.Price * yards
		}
	}
	if productionDays <= 0 {
		productionDays = 10
	}

	designName := "Custom design"
	designSlugOut := ""
	if !custom {
		designName = design.Name
		designSlugOut = design.Slug
	}

	order := store.Order{
		Status:         "new",
		DesignSlug:     designSlugOut,
		DesignName:     designName,
		DesignCategory: category,
		CustomDesign:   custom,
		InspirationImg: inspirationPath,
		InspirationTxt: truncate(field("custom_description"), 600),
		BringOwnFabric: bringOwn,
		Yardage:        yards,
		Unit:           unit,
		Measurements:   measurements,
		Notes:          notes,
		CustomerName:   name,
		Phone:          phone,
		Email:          email,
		Delivery:       delivery,
		Address:        address,
		PreferredDate:  preferred,
		PriceEstimate:  math.Round(price),
		ProductionDays: productionDays,
	}
	if !bringOwn && fabric.ID != 0 {
		order.FabricSlug = fabric.Slug
		order.FabricName = fabric.Name
	}

	if err := a.db.CreateOrder(&order); err != nil {
		log.Printf("create order: %v", err)
		http.Error(w, "We could not save your order. Please try again or message us on WhatsApp.", http.StatusInternalServerError)
		return
	}

	a.notifyOrder(order, custom)
	http.Redirect(w, r, "/orders/"+order.Ref+"?t="+order.PublicToken, http.StatusSeeOther)
}

// notifyOrder tells the workshop about the new order.
func (a *App) notifyOrder(o store.Order, custom bool) {
	fabric := o.FabricName
	if o.BringOwnFabric {
		fabric = "customer's own fabric"
	}
	design := o.DesignName
	if custom {
		design = "Custom design: " + o.InspirationTxt
	}
	body := strings.Join([]string{
		"New order " + o.Ref,
		"",
		"Customer: " + o.CustomerName,
		"Phone: " + o.Phone,
		"Email: " + o.Email,
		"Delivery: " + o.Delivery,
		"Address: " + o.Address,
		"Ready by: " + o.PreferredDate,
		"",
		"Design: " + design,
		"Fabric: " + fabric,
		"Yardage: " + trimNumber(o.Yardage),
		"Measurements: " + strconv.Itoa(len(o.Measurements)) + " recorded (see the order sheet)",
		"Estimate: KES " + moneyPlain(o.PriceEstimate),
		"Production: " + strconv.Itoa(o.ProductionDays) + " days",
		"",
		"Notes: " + o.Notes,
	}, "\n")
	if err := a.mailer.SendOrder("New order "+o.Ref+" — "+o.CustomerName, body); err != nil {
		log.Printf("notify order %s: %v", o.Ref, err)
	}
}

// ---------------------------------------------------------------------------
// Confirmation and PDF
// ---------------------------------------------------------------------------

// measurementRow is one label/value pair for the confirmation and PDF.
type measurementRow struct {
	Label string
	Value string
}

func measurementRows(o store.Order) []measurementRow {
	rows := make([]measurementRow, 0, len(o.Measurements))
	for _, f := range measure.Fields(o.DesignCategory) {
		v, ok := o.Measurements[f.Key]
		if !ok {
			continue
		}
		value := trimNumber(v) + " cm"
		if f.Age {
			value = trimNumber(v) + " years"
		}
		rows = append(rows, measurementRow{Label: f.Label, Value: value})
	}
	return rows
}

func (a *App) orderConfirmation(w http.ResponseWriter, r *http.Request) {
	o, err := a.db.GetOrderByToken(r.PathValue("ref"), r.URL.Query().Get("t"))
	if err != nil {
		a.notFound(w, r)
		return
	}
	a.render(w, http.StatusOK, "confirmation", page{
		Title:  "Order " + o.Ref + " received",
		Desc:   "Your Gojias Collections order " + o.Ref + " has been received.",
		Active: "order",
		Data: map[string]any{
			"Order":      o,
			"Token":      o.PublicToken,
			"Rows":       measurementRows(o),
			"StatusText": store.StatusLabel(o.Status),
			"Custom":     o.CustomDesign,
		},
	})
}

func (a *App) orderSummaryPDF(w http.ResponseWriter, r *http.Request) {
	o, err := a.db.GetOrderByToken(r.PathValue("ref"), r.URL.Query().Get("t"))
	if err != nil {
		a.notFound(w, r)
		return
	}

	fabric := o.FabricName
	if o.BringOwnFabric {
		fabric = "Customer's own fabric"
	}
	if fabric == "" {
		fabric = "To be confirmed"
	}

	lines := []pdf.Line{
		{Text: "Gojias Collections", Size: 20, Bold: true},
		{Text: "Kitenge and Ankara attire, made to measure", Size: 10},
		{Text: "", Space: 6},
		{Text: "Order summary  " + o.Ref, Size: 14, Bold: true},
		{Text: "Received " + date(o.CreatedAt) + "  ·  Status: " + store.StatusLabel(o.Status), Size: 10},
		{Text: "", Space: 8},
		{Text: "Customer", Size: 12, Bold: true},
		{Text: o.CustomerName, Size: 11},
		{Text: "Phone: " + o.Phone, Size: 11},
	}
	if o.Email != "" {
		lines = append(lines, pdf.Line{Text: "Email: " + o.Email, Size: 11})
	}
	lines = append(lines,
		pdf.Line{Text: "Collection: " + label(o.Delivery) + " by " + o.PreferredDate, Size: 11},
	)
	if o.Address != "" {
		lines = append(lines, pdf.Line{Text: "Address: " + o.Address, Size: 11})
	}

	lines = append(lines,
		pdf.Line{Text: "", Space: 8},
		pdf.Line{Text: "Outfit", Size: 12, Bold: true},
		pdf.Line{Text: "Design: " + o.DesignName, Size: 11},
		pdf.Line{Text: "Fabric: " + fabric + " — estimated " + trimNumber(o.Yardage), Size: 11},
		pdf.Line{Text: "Production time: " + strconv.Itoa(o.ProductionDays) + " days", Size: 11},
		pdf.Line{Text: "Estimated price: KES " + moneyPlain(o.PriceEstimate), Size: 11},
		pdf.Line{Text: "", Space: 8},
		pdf.Line{Text: "Measurements", Size: 12, Bold: true},
	)
	for _, row := range measurementRows(o) {
		lines = append(lines, pdf.Line{Text: row.Label + ": " + row.Value, Size: 11})
	}
	if o.Notes != "" {
		lines = append(lines,
			pdf.Line{Text: "", Space: 8},
			pdf.Line{Text: "Notes", Size: 12, Bold: true},
			pdf.Line{Text: o.Notes, Size: 10},
		)
	}
	lines = append(lines,
		pdf.Line{Text: "", Space: 10},
		pdf.Line{Text: "This is an estimate, not a receipt. The workshop confirms the final price before cutting.", Size: 9},
		pdf.Line{Text: "Track this order at /track using " + o.Ref + " and your phone number.", Size: 9},
	)

	doc := pdf.Build(lines)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="`+o.Ref+`-summary.pdf"`)
	w.Header().Set("Content-Length", strconv.Itoa(len(doc)))
	_, _ = w.Write(doc)
}

// ---------------------------------------------------------------------------
// Tracking
// ---------------------------------------------------------------------------

// timelineStep is one node of the tracking timeline.
type timelineStep struct {
	Key     string
	Label   string
	Date    string
	Done    bool
	Current bool
}

func buildTimeline(o store.Order) []timelineStep {
	current := store.StatusIndex(o.Status)
	steps := make([]timelineStep, 0, len(store.Statuses))
	for i, s := range store.Statuses {
		step := timelineStep{Key: s.Key, Label: s.Label, Done: i <= current, Current: i == current}
		if e, ok := o.EventFor(s.Key); ok {
			step.Date = e.CreatedAt.Format("2 Jan 2006, 3:04 pm")
		}
		steps = append(steps, step)
	}
	return steps
}

func (a *App) trackForm(w http.ResponseWriter, r *http.Request) {
	a.render(w, http.StatusOK, "track", page{
		Title:  "Track your order",
		Desc:   "Enter your order reference and phone number to see where your Gojias Collections outfit is.",
		Active: "track",
		Data: map[string]any{
			"Ref": r.URL.Query().Get("ref"),
		},
	})
}

func (a *App) trackLookup(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	ref := strings.TrimSpace(r.PostFormValue("ref"))
	phone := strings.TrimSpace(r.PostFormValue("phone"))

	data := map[string]any{"Ref": ref}

	if ref == "" || phone == "" {
		data["Error"] = "Enter both the order reference and the phone number on the order."
		a.render(w, http.StatusBadRequest, "track", page{Title: "Track your order", Active: "track", Data: data})
		return
	}

	o, err := a.db.GetOrderByRefAndPhone(ref, phone)
	if err != nil {
		// Deliberately vague: do not reveal which of the two was wrong.
		data["Error"] = "We could not match that reference and phone number. Check the reference on your confirmation and try again."
		a.render(w, http.StatusNotFound, "track", page{Title: "Track your order", Active: "track", Data: data})
		return
	}

	data["Order"] = o
	data["Timeline"] = buildTimeline(o)
	data["StatusText"] = store.StatusLabel(o.Status)
	data["FabricText"] = fabricText(o)
	a.render(w, http.StatusOK, "track", page{
		Title:  "Order " + o.Ref,
		Desc:   "Progress on order " + o.Ref + ".",
		Active: "track",
		Data:   data,
	})
}

func fabricText(o store.Order) string {
	if o.BringOwnFabric {
		return "Your own fabric"
	}
	if o.FabricName == "" {
		return "To be confirmed"
	}
	return o.FabricName
}

// ---------------------------------------------------------------------------
// Uploads
// ---------------------------------------------------------------------------

func (a *App) hasUpload(r *http.Request) bool {
	if r.MultipartForm == nil {
		return false
	}
	for _, fh := range r.MultipartForm.File["inspiration"] {
		if fh.Size > 0 {
			return true
		}
	}
	return false
}

// validateUpload checks type and size without writing anything to disk.
func (a *App) validateUpload(r *http.Request) string {
	fh, _, err := r.FormFile("inspiration")
	if err != nil {
		return "We could not read that photo. Please try again."
	}
	defer fh.Close()
	if r.MultipartForm.File["inspiration"][0].Size > maxUpload {
		return "That image is larger than 5 MB. Please compress it and try again."
	}
	head := make([]byte, 512)
	n, _ := io.ReadFull(fh, head)
	ct := http.DetectContentType(head[:n])
	if !allowedImage(ct) {
		return "Please upload a JPG, PNG or WebP photo."
	}
	return ""
}

// saveUpload stores the validated photo under uploads/ and returns its URL.
func (a *App) saveUpload(r *http.Request) (string, string) {
	fh, _, err := r.FormFile("inspiration")
	if err != nil {
		return "", "We could not read that photo. Please try again."
	}
	defer fh.Close()

	head := make([]byte, 512)
	n, _ := io.ReadFull(fh, head)
	ct := http.DetectContentType(head[:n])
	exts := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp"}
	ext, ok := exts[ct]
	if !ok {
		return "", "Please upload a JPG, PNG or WebP photo."
	}

	name, err := randomName()
	if err != nil {
		return "", "We could not save that photo. Please try again."
	}
	if err := os.MkdirAll(a.uploadDir, 0o755); err != nil {
		log.Printf("create upload dir: %v", err)
		return "", "We could not save that photo. Please try again."
	}
	path := filepath.Join(a.uploadDir, name+ext)
	dst, err := os.Create(path)
	if err != nil {
		log.Printf("create upload: %v", err)
		return "", "We could not save that photo. Please try again."
	}
	defer dst.Close()

	body := io.LimitReader(io.MultiReader(bytes.NewReader(head[:n]), fh), maxUpload)
	if _, err := io.Copy(dst, body); err != nil {
		log.Printf("write upload: %v", err)
		_ = os.Remove(path)
		return "", "We could not save that photo. Please try again."
	}
	return "/uploads/" + filepath.Base(path), ""
}

func allowedImage(contentType string) bool {
	switch contentType {
	case "image/jpeg", "image/png", "image/webp":
		return true
	}
	return false
}

func randomName() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "inspiration-" + hex.EncodeToString(buf), nil
}

// ---------------------------------------------------------------------------
// Validation helpers
// ---------------------------------------------------------------------------

var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]{2,}$`)

// estimateYardage is the starting point shown in step 2. Customers can edit it;
// the workshop confirms the real requirement before cutting.
func estimateYardage(d store.Design) float64 {
	base := map[string]float64{
		"gown": 5, "kaftan": 4, "skirt-and-top": 3.5, "shirt": 2.5,
		"dress": 3, "suit": 7, "set": 6,
	}
	yards, ok := base[d.Style]
	if !ok {
		yards = 4
	}
	switch d.Category {
	case "children":
		yards = math.Max(1.5, yards/2)
	case "couples":
		yards += 3
	}
	return math.Round(yards*2) / 2
}

func errorStep(errs map[string]string) int {
	step := 5
	for key := range errs {
		s := stepForField(key)
		if s < step {
			step = s
		}
	}
	return step
}

func stepForField(key string) int {
	switch {
	case key == "design" || key == "category" || key == "inspiration" || key == "custom_description":
		return 1
	case key == "fabric" || key == "yardage":
		return 2
	case strings.HasPrefix(key, "measure_") || key == "unit":
		return 3
	case key == "name" || key == "phone" || key == "email" || key == "delivery" || key == "address" || key == "preferred_date":
		return 4
	default:
		return 5
	}
}

func isMeasureCategory(c string) bool {
	for _, cat := range measure.Categories {
		if cat == c {
			return true
		}
	}
	return false
}

func wizardCategoryLabel(c string) string {
	switch c {
	case "couples":
		return "Couples/Family"
	case "bridal":
		return "Bridal & Occasion"
	default:
		return label(c)
	}
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len([]rune(s)) <= max {
		return s
	}
	return string([]rune(s)[:max])
}

// trimNumber formats a float without trailing zeros: 3 -> "3", 3.50 -> "3.5".
func trimNumber(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func parseFloat(s string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return f
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ---------------------------------------------------------------------------
// Rate limiting
// ---------------------------------------------------------------------------

// rateLimiter is a small in-memory sliding window limiter for the order
// endpoint. It is per-process, which is appropriate for a single-server shop.
type rateLimiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	limit  int
	window time.Duration
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{hits: map[string][]time.Time{}, limit: limit, window: window}
}

func (l *rateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := time.Now().Add(-l.window)
	recent := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	if len(recent) >= l.limit {
		l.hits[key] = recent
		return false
	}
	l.hits[key] = append(recent, time.Now())
	return true
}
