package main

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"gojias/internal/measure"
	"gojias/internal/store"
)

// ---------------------------------------------------------------------------
// Fabrics
// ---------------------------------------------------------------------------

// fabricFormData backs the add/edit fabric form.
type fabricFormData struct {
	ID       int64
	Values   map[string]string
	InStock  bool
	Featured bool
	Images   []store.ImageRow
	Errors   map[string]string
}

func newFabricForm(f store.Fabric, images []store.ImageRow) *fabricFormData {
	price := ""
	if f.Price > 0 {
		price = trimNumber(f.Price)
	}
	unit := f.Unit
	if unit == "" {
		unit = "yard"
	}
	return &fabricFormData{
		ID: f.ID,
		Values: map[string]string{
			"name": f.Name, "slug": f.Slug, "type": f.FabricType, "colours": f.Colours,
			"description": f.Description, "care": f.Care, "width": f.Width, "unit": unit,
			"price": price,
		},
		InStock:  f.InStock,
		Featured: f.Featured,
		Images:   images,
		Errors:   map[string]string{},
	}
}

func (a *App) adminFabrics(w http.ResponseWriter, r *http.Request, sess *adminSession) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	fabrics, _, err := a.db.ListFabrics(store.FabricFilter{Query: q, PerPage: 200, Sort: "name"})
	if err != nil {
		log.Printf("admin fabrics: %v", err)
		http.Error(w, "Could not load fabrics", http.StatusInternalServerError)
		return
	}
	p := newAdminPage(r, sess)
	p.Title = "Fabrics"
	p.Active = "fabrics"
	p.Data = map[string]any{"Fabrics": fabrics, "Query": q, "Total": len(fabrics)}
	a.writeAdmin(w, http.StatusOK, "fabrics", "base.html", p)
}

func (a *App) adminFabricNew(w http.ResponseWriter, r *http.Request, sess *adminSession) {
	form := newFabricForm(store.Fabric{InStock: true}, nil)
	p := newAdminPage(r, sess)
	p.Title = "Add fabric"
	p.Active = "fabrics"
	p.Data = form
	a.writeAdmin(w, http.StatusOK, "fabric_form", "base.html", p)
}

func (a *App) adminFabricEdit(w http.ResponseWriter, r *http.Request, sess *adminSession) {
	id := atoi64(r.PathValue("id"))
	f, err := a.db.FabricByID(id)
	if err != nil {
		http.Error(w, "Fabric not found", http.StatusNotFound)
		return
	}
	images, err := a.db.FabricImages(id)
	if err != nil {
		log.Printf("fabric images: %v", err)
	}
	p := newAdminPage(r, sess)
	p.Title = "Edit " + f.Name
	p.Active = "fabrics"
	p.Data = newFabricForm(f, images)
	a.writeAdmin(w, http.StatusOK, "fabric_form", "base.html", p)
}

func (a *App) adminFabricSave(w http.ResponseWriter, r *http.Request, sess *adminSession) {
	id := atoi64(r.PathValue("id"))
	form := &fabricFormData{
		ID: id,
		Values: map[string]string{
			"name":        strings.TrimSpace(r.PostFormValue("name")),
			"slug":        strings.TrimSpace(r.PostFormValue("slug")),
			"type":        strings.TrimSpace(r.PostFormValue("type")),
			"colours":     strings.TrimSpace(r.PostFormValue("colours")),
			"description": strings.TrimSpace(r.PostFormValue("description")),
			"care":        strings.TrimSpace(r.PostFormValue("care")),
			"width":       strings.TrimSpace(r.PostFormValue("width")),
			"unit":        r.PostFormValue("unit"),
			"price":       strings.TrimSpace(r.PostFormValue("price")),
		},
		InStock:  r.PostFormValue("in_stock") == "1",
		Featured: r.PostFormValue("featured") == "1",
		Errors:   map[string]string{},
	}
	if form.Values["unit"] != "yard" && form.Values["unit"] != "metre" {
		form.Values["unit"] = "yard"
	}

	var existing []store.ImageRow
	removed := map[int64]bool{}
	if id != 0 {
		rows, err := a.db.FabricImages(id)
		if err != nil {
			log.Printf("fabric images: %v", err)
		}
		existing = rows
	}
	for _, v := range r.PostForm["remove_image"] {
		removed[atoi64(v)] = true
	}
	for _, img := range existing {
		if !removed[img.ID] {
			form.Images = append(form.Images, img)
		}
	}

	// --- validation ---
	if len(form.Values["name"]) < 2 {
		form.Errors["name"] = "Give the fabric a name."
	}
	if form.Values["type"] == "" {
		form.Errors["type"] = "Choose or type a fabric family (kitenge, ankara, lace…)."
	}
	price := parseFloat(form.Values["price"])
	if price < 0 {
		form.Errors["price"] = "Price cannot be negative."
	}
	if form.Values["price"] == "" {
		form.Errors["price"] = "Enter the price per unit in KES."
	}

	newImages, uploadErr := a.saveUploadedImages(r, "images", "fabric", 6)
	if uploadErr != "" {
		form.Errors["images"] = uploadErr
	}
	if len(form.Images)+len(newImages) == 0 {
		form.Errors["images"] = "Add at least one photo — the catalogue card needs one."
	}

	if len(form.Errors) > 0 {
		for _, f := range newImages {
			a.removeUpload(f)
		}
		p := newAdminPage(r, sess)
		p.Title = "Add fabric"
		if id != 0 {
			p.Title = "Edit fabric"
		}
		p.Active = "fabrics"
		p.Data = form
		p.Error = "Please fix the highlighted fields."
		a.writeAdmin(w, http.StatusBadRequest, "fabric_form", "base.html", p)
		return
	}

	fabric := store.Fabric{
		ID:          id,
		Slug:        form.Values["slug"],
		Name:        form.Values["name"],
		FabricType:  form.Values["type"],
		Description: truncate(form.Values["description"], 2000),
		Colours:     truncate(form.Values["colours"], 200),
		Care:        truncate(form.Values["care"], 600),
		Width:       truncate(form.Values["width"], 40),
		Unit:        form.Values["unit"],
		Price:       price,
		InStock:     form.InStock,
		Featured:    form.Featured,
	}
	if fabric.Slug == "" {
		fabric.Slug = store.Slugify(fabric.Name)
	}

	var removeIDs []int64
	for rid := range removed {
		removeIDs = append(removeIDs, rid)
	}
	if _, err := a.db.SaveFabric(&fabric, newImages, removeIDs); err != nil {
		log.Printf("save fabric: %v", err)
		http.Error(w, "Could not save the fabric", http.StatusInternalServerError)
		return
	}
	a.redirectFlash(w, r, "/admin/fabrics", "Fabric saved.")
}

func (a *App) adminFabricDelete(w http.ResponseWriter, r *http.Request, sess *adminSession) {
	id := atoi64(r.PathValue("id"))
	// Grab the gallery paths first so the files can be removed with the row.
	images, err := a.db.FabricImages(id)
	if err != nil {
		log.Printf("fabric images before delete: %v", err)
	}
	if err := a.db.DeleteFabric(id); err != nil {
		log.Printf("delete fabric: %v", err)
		http.Error(w, "Could not delete the fabric", http.StatusInternalServerError)
		return
	}
	for _, img := range images {
		a.removeUpload(img.URL)
	}
	a.redirectFlash(w, r, "/admin/fabrics", "Fabric deleted.")
}

func (a *App) adminFabricToggle(w http.ResponseWriter, r *http.Request, sess *adminSession) {
	id := atoi64(r.PathValue("id"))
	var err error
	if r.PostFormValue("field") == "stock" {
		err = a.db.ToggleFabricStock(id)
	} else {
		err = a.db.ToggleFabricFeatured(id)
	}
	if err != nil {
		log.Printf("toggle fabric: %v", err)
		http.Error(w, "Could not update the fabric", http.StatusInternalServerError)
		return
	}
	a.redirectFlash(w, r, "/admin/fabrics", "Fabric updated.")
}

// ---------------------------------------------------------------------------
// Designs
// ---------------------------------------------------------------------------

// designFormData backs the add/edit design form.
type designFormData struct {
	ID             int64
	Values         map[string]string
	Featured       bool
	Measurements   []string
	FabricIDs      []int64
	Images         []store.ImageRow
	Fabrics        []store.Fabric
	Categories     []string
	MeasureOptions []measurementOption
	Errors         map[string]string
}

func (a *App) newDesignForm(d store.Design, images []store.ImageRow) (*designFormData, error) {
	fabrics, _, err := a.db.ListFabrics(store.FabricFilter{PerPage: 200, Sort: "name"})
	if err != nil {
		return nil, err
	}
	price := ""
	if d.PriceFrom > 0 {
		price = trimNumber(d.PriceFrom)
	}
	days := ""
	if d.ProductionDays > 0 {
		days = strconv.Itoa(d.ProductionDays)
	}
	values := map[string]string{
		"name": d.Name, "slug": d.Slug, "category": d.Category, "style": d.Style,
		"occasion": d.Occasion, "description": d.Description, "price": price, "days": days,
	}
	if values["category"] == "" {
		values["category"] = "women"
	}
	return &designFormData{
		ID:             d.ID,
		Values:         values,
		Featured:       d.Featured,
		Measurements:   d.RequiredMeasurements,
		FabricIDs:      d.FabricIDs,
		Images:         images,
		Fabrics:        fabrics,
		Categories:     measure.Categories,
		MeasureOptions: allMeasurementOptions(),
		Errors:         map[string]string{},
	}, nil
}

func (a *App) adminDesigns(w http.ResponseWriter, r *http.Request, sess *adminSession) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	designs, _, err := a.db.ListDesigns(store.DesignFilter{Query: q, PerPage: 200, Sort: "name"})
	if err != nil {
		log.Printf("admin designs: %v", err)
		http.Error(w, "Could not load designs", http.StatusInternalServerError)
		return
	}
	p := newAdminPage(r, sess)
	p.Title = "Designs"
	p.Active = "designs"
	p.Data = map[string]any{"Designs": designs, "Query": q, "Total": len(designs)}
	a.writeAdmin(w, http.StatusOK, "designs", "base.html", p)
}

func (a *App) adminDesignNew(w http.ResponseWriter, r *http.Request, sess *adminSession) {
	form, err := a.newDesignForm(store.Design{Category: "women"}, nil)
	if err != nil {
		http.Error(w, "Could not load fabrics", http.StatusInternalServerError)
		return
	}
	p := newAdminPage(r, sess)
	p.Title = "Add design"
	p.Active = "designs"
	p.Data = form
	a.writeAdmin(w, http.StatusOK, "design_form", "base.html", p)
}

func (a *App) adminDesignEdit(w http.ResponseWriter, r *http.Request, sess *adminSession) {
	id := atoi64(r.PathValue("id"))
	d, err := a.db.DesignByID(id)
	if err != nil {
		http.Error(w, "Design not found", http.StatusNotFound)
		return
	}
	images, err := a.db.DesignImages(id)
	if err != nil {
		log.Printf("design images: %v", err)
	}
	form, err := a.newDesignForm(d, images)
	if err != nil {
		http.Error(w, "Could not load fabrics", http.StatusInternalServerError)
		return
	}
	p := newAdminPage(r, sess)
	p.Title = "Edit " + d.Name
	p.Active = "designs"
	p.Data = form
	a.writeAdmin(w, http.StatusOK, "design_form", "base.html", p)
}

func (a *App) adminDesignSave(w http.ResponseWriter, r *http.Request, sess *adminSession) {
	id := atoi64(r.PathValue("id"))
	form := &designFormData{
		ID: id,
		Values: map[string]string{
			"name":        strings.TrimSpace(r.PostFormValue("name")),
			"slug":        strings.TrimSpace(r.PostFormValue("slug")),
			"category":    r.PostFormValue("category"),
			"style":       strings.TrimSpace(r.PostFormValue("style")),
			"occasion":    strings.TrimSpace(r.PostFormValue("occasion")),
			"description": strings.TrimSpace(r.PostFormValue("description")),
			"price":       strings.TrimSpace(r.PostFormValue("price")),
			"days":        strings.TrimSpace(r.PostFormValue("days")),
		},
		Featured:       r.PostFormValue("featured") == "1",
		Measurements:   r.PostForm["measurements"],
		Categories:     measure.Categories,
		MeasureOptions: allMeasurementOptions(),
		Errors:         map[string]string{},
	}
	for _, v := range r.PostForm["fabrics"] {
		if fid := atoi64(v); fid > 0 {
			form.FabricIDs = append(form.FabricIDs, fid)
		}
	}

	fabrics, _, err := a.db.ListFabrics(store.FabricFilter{PerPage: 200, Sort: "name"})
	if err != nil {
		http.Error(w, "Could not load fabrics", http.StatusInternalServerError)
		return
	}
	form.Fabrics = fabrics

	var existing []store.ImageRow
	removed := map[int64]bool{}
	if id != 0 {
		rows, err := a.db.DesignImages(id)
		if err != nil {
			log.Printf("design images: %v", err)
		}
		existing = rows
	}
	for _, v := range r.PostForm["remove_image"] {
		removed[atoi64(v)] = true
	}
	for _, img := range existing {
		if !removed[img.ID] {
			form.Images = append(form.Images, img)
		}
	}

	if len(form.Values["name"]) < 2 {
		form.Errors["name"] = "Give the design a name."
	}
	if !isMeasureCategory(form.Values["category"]) {
		form.Errors["category"] = "Choose a category."
	}
	price := parseFloat(form.Values["price"])
	if price < 0 {
		form.Errors["price"] = "Starting price cannot be negative."
	}
	days := atoi(form.Values["days"])
	if days < 0 || days > 120 {
		form.Errors["days"] = "Production time should be between 0 and 120 days."
	}
	if len(form.Measurements) == 0 {
		form.Errors["measurements"] = "Tick the measurements this design needs."
	}

	newImages, uploadErr := a.saveUploadedImages(r, "images", "design", 6)
	if uploadErr != "" {
		form.Errors["images"] = uploadErr
	}
	if len(form.Images)+len(newImages) == 0 {
		form.Errors["images"] = "Add at least one photo — the catalogue card needs one."
	}

	if len(form.Errors) > 0 {
		for _, f := range newImages {
			a.removeUpload(f)
		}
		p := newAdminPage(r, sess)
		p.Title = "Add design"
		if id != 0 {
			p.Title = "Edit design"
		}
		p.Active = "designs"
		p.Data = form
		p.Error = "Please fix the highlighted fields."
		a.writeAdmin(w, http.StatusBadRequest, "design_form", "base.html", p)
		return
	}

	design := store.Design{
		ID:             id,
		Slug:           form.Values["slug"],
		Name:           form.Values["name"],
		Category:       form.Values["category"],
		Style:          truncate(form.Values["style"], 60),
		Occasion:       truncate(form.Values["occasion"], 60),
		Description:    truncate(form.Values["description"], 2000),
		Measurements:   strings.Join(uniqueStrings(form.Measurements), ", "),
		PriceFrom:      price,
		ProductionDays: days,
		Featured:       form.Featured,
	}
	if design.Slug == "" {
		design.Slug = store.Slugify(design.Name)
	}

	var removeIDs []int64
	for rid := range removed {
		removeIDs = append(removeIDs, rid)
	}
	if _, err := a.db.SaveDesign(&design, form.FabricIDs, newImages, removeIDs); err != nil {
		log.Printf("save design: %v", err)
		http.Error(w, "Could not save the design", http.StatusInternalServerError)
		return
	}
	a.redirectFlash(w, r, "/admin/designs", "Design saved.")
}

func (a *App) adminDesignDelete(w http.ResponseWriter, r *http.Request, sess *adminSession) {
	id := atoi64(r.PathValue("id"))
	images, err := a.db.DesignImages(id)
	if err != nil {
		log.Printf("design images before delete: %v", err)
	}
	if err := a.db.DeleteDesign(id); err != nil {
		log.Printf("delete design: %v", err)
		http.Error(w, "Could not delete the design", http.StatusInternalServerError)
		return
	}
	for _, img := range images {
		a.removeUpload(img.URL)
	}
	a.redirectFlash(w, r, "/admin/designs", "Design deleted.")
}

func (a *App) adminDesignToggle(w http.ResponseWriter, r *http.Request, sess *adminSession) {
	id := atoi64(r.PathValue("id"))
	if err := a.db.ToggleDesignFeatured(id); err != nil {
		log.Printf("toggle design: %v", err)
		http.Error(w, "Could not update the design", http.StatusInternalServerError)
		return
	}
	a.redirectFlash(w, r, "/admin/designs", "Design updated.")
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func atoi64(s string) int64 {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}
