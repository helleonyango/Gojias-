// Package measure is the single source of truth for tailoring measurements:
// which fields each design category needs, the guidance tip shown beside every
// field, the plausible range we accept, and validation/conversion helpers.
package measure

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Field describes one measurement input.
type Field struct {
	Key   string  // stable key, also the suffix of the form input name
	Label string  // shown to the customer
	Tip   string  // the "?" guidance text
	Min   float64 // in cm (or years for Age)
	Max   float64
	Age   bool // age is entered in years and is never converted to cm
}

// Unit labels for the range hint.
func (f Field) RangeHint() string {
	if f.Age {
		return fmt.Sprintf("%.0f–%.0f years", f.Min, f.Max)
	}
	return fmt.Sprintf("%.0f–%.0f cm", f.Min, f.Max)
}

// women is used by the women, bridal and couples categories. Couples orders
// collect the second partner's numbers during the fitting.
var women = []Field{
	{"bust", "Bust", "Around the fullest part of your bust, tape level under the arms.", 60, 160, false},
	{"underbust", "Underbust", "Snug around your ribcage, directly beneath the bust.", 55, 150, false},
	{"waist", "Waist", "At the narrowest part of your waist, usually just above the navel.", 45, 150, false},
	{"hips", "Hips", "Around the fullest part of your hips, about 20 cm below the waist.", 60, 175, false},
	{"shoulder", "Shoulder width", "Across your back, from one shoulder bone to the other.", 25, 60, false},
	{"sleeve", "Sleeve length", "From the shoulder point to the wrist, arm slightly bent.", 15, 75, false},
	{"armhole", "Armhole", "Around the arm at the shoulder joint, tape passing under the armpit.", 20, 70, false},
	{"bicep", "Bicep", "Around the fullest part of the upper arm, arm relaxed.", 15, 60, false},
	{"neck", "Neck", "Around the base of the neck where a collar would sit.", 25, 60, false},
	{"thigh", "Thigh", "Around the fullest part of the thigh, just below the crotch.", 30, 95, false},
	{"length", "Dress / skirt / gown length", "From the shoulder to where you want the hem to fall.", 40, 160, false},
	{"height", "Height", "Standing straight, without shoes.", 90, 220, false},
}

var men = []Field{
	{"neck", "Neck", "Around the base of the neck where a collar would sit, one finger of ease.", 25, 60, false},
	{"chest", "Chest", "Around the fullest part of the chest, under the arms.", 60, 180, false},
	{"waist", "Waist", "Around the natural waist, at or just above the navel.", 50, 180, false},
	{"hips", "Hips", "Around the fullest part of the seat.", 60, 180, false},
	{"shoulder", "Shoulder width", "Across your back, from one shoulder bone to the other.", 30, 65, false},
	{"sleeve", "Sleeve length", "From the shoulder point to the wrist, arm slightly bent.", 30, 85, false},
	{"shirt-length", "Shirt length", "From the shoulder seam down to where you want the shirt hem.", 45, 110, false},
	{"kaftan-length", "Kaftan length", "From the shoulder down to the hem, usually mid-calf.", 70, 160, false},
	{"trouser-length", "Trouser length", "From the waistband to the shoe, on the side.", 70, 135, false},
	{"inseam", "Inseam", "From the crotch to the ankle bone, along the inside leg.", 40, 100, false},
	{"thigh", "Thigh", "Around the fullest part of the thigh, just below the crotch.", 30, 100, false},
	{"height", "Height", "Standing straight, without shoes.", 90, 220, false},
}

var children = []Field{
	{"age", "Age", "In whole years. We allow a little growing room at the seams.", 1, 17, true},
	{"height", "Height", "Standing straight, without shoes, heels against a wall.", 60, 185, false},
	{"chest", "Chest", "Around the fullest part of the chest, under the arms.", 35, 110, false},
	{"waist", "Waist", "Around the natural waist. Keep the tape snug, not tight.", 30, 110, false},
	{"hips", "Hips", "Around the fullest part of the hips.", 35, 120, false},
	{"length", "Dress / outfit length", "From the shoulder down to where you want the hem.", 30, 150, false},
}

var catalog = map[string][]Field{
	"women":    women,
	"bridal":   women,
	"couples":  women,
	"men":      men,
	"children": children,
}

// Categories lists the category slugs in display order.
var Categories = []string{"women", "men", "children", "couples", "bridal"}

// Fields returns the measurement fields for a category, defaulting to women.
func Fields(category string) []Field {
	if f, ok := catalog[category]; ok {
		return f
	}
	return women
}

// FieldFor finds one field definition inside a category.
func FieldFor(category, key string) (Field, bool) {
	for _, f := range Fields(category) {
		if f.Key == key {
			return f, true
		}
	}
	return Field{}, false
}

// Note explains anything unusual about a category's measurement set.
func Note(category string) string {
	switch category {
	case "couples":
		return "We take your partner's measurements at the first fitting, or you can add them in the notes box below."
	case "bridal":
		return "Bridal gowns are measured over the underwear you plan to wear on the day. We book a fitting to confirm every number."
	case "children":
		return "Children grow. We add a little room at the side and shoulder seams so the outfit lasts a season longer."
	default:
		return ""
	}
}

// ValidUnit reports whether a unit choice is supported.
func ValidUnit(unit string) bool { return unit == "cm" || unit == "in" }

// ToCM converts a value entered in the given unit into centimetres.
func ToCM(v float64, unit string) float64 {
	if unit == "in" {
		return v * 2.54
	}
	return v
}

// FromCM converts stored centimetres into the requested display unit.
func FromCM(cm float64, unit string) float64 {
	if unit == "in" {
		return cm / 2.54
	}
	return cm
}

// Validate parses the submitted measurement strings for a category.
//
// It returns values in centimetres plus a map of friendly, per-field errors
// keyed by the form input name (e.g. "measure_bust"), ready for the template.
func Validate(raw map[string]string, category, unit string) (map[string]float64, map[string]string) {
	if !ValidUnit(unit) {
		unit = "cm"
	}
	values := make(map[string]float64, len(raw))
	errs := map[string]string{}

	for _, f := range Fields(category) {
		field := "measure_" + f.Key
		input := strings.TrimSpace(raw[f.Key])
		if input == "" {
			errs[field] = "Add your " + strings.ToLower(f.Label) + " so the fit is right."
			continue
		}
		n, err := strconv.ParseFloat(strings.Replace(strings.Replace(input, ",", ".", 1), " ", "", -1), 64)
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
			errs[field] = "Use numbers only — for example 92 or 36.5."
			continue
		}
		if f.Age {
			if n < f.Min || n > f.Max {
				errs[field] = fmt.Sprintf("%s should be between %.0f and %.0f years.", f.Label, f.Min, f.Max)
				continue
			}
			values[f.Key] = n
			continue
		}
		cm := ToCM(n, unit)
		if cm < f.Min || cm > f.Max {
			if unit == "in" {
				errs[field] = fmt.Sprintf("That looks unusual for %s. Please check — we expect roughly %.0f–%.0f inches.", strings.ToLower(f.Label), f.Min/2.54, f.Max/2.54)
			} else {
				errs[field] = fmt.Sprintf("That looks unusual for %s. Please check — we expect %.0f–%.0f cm.", strings.ToLower(f.Label), f.Min, f.Max)
			}
			continue
		}
		values[f.Key] = math.Round(cm*10) / 10
	}
	return values, errs
}

// ---------------------------------------------------------------------------
// How-to-measure garment guides (shown on /how-to-measure, in inches).
// Each guide lists only the measurements relevant to that garment so customers
// never see unrelated fields (e.g. no hips for a shirt, no chest for a dress).
// ---------------------------------------------------------------------------

// GuideItem is one measurement step in a garment guide.
type GuideItem struct {
	Key       string // matches a validation field key where applicable, else descriptive
	Label     string
	How       string // plain-language instruction
	InchRange string // typical range shown in inches
	Tip       string // extra tip, optional
}

// GarmentGuide describes which measurements to take for one type of garment.
type GarmentGuide struct {
	Slug  string
	Label string
	Blurb string
	Items []GuideItem
}

func inchRange(cmMin, cmMax float64) string {
	return fmt.Sprintf("%.0f–%.0f in", math.Round(cmMin/2.54), math.Round(cmMax/2.54))
}

// fieldTip returns the tip for a known measurement key, falling back to def.
func fieldTip(category, key, def string) string {
	if f, ok := FieldFor(category, key); ok && f.Tip != "" {
		return f.Tip
	}
	return def
}

func itemFrom(category, key, label, how string) GuideItem {
	f, _ := FieldFor(category, key)
	r := ""
	if f.Key != "" {
		r = inchRange(f.Min, f.Max)
	}
	tip := ""
	if f.Key != "" {
		tip = f.Tip
	}
	return GuideItem{Key: key, Label: label, How: how, InchRange: r, Tip: tip}
}

// Guides returns garment-specific measuring guides in display order.
func Guides() []GarmentGuide {
	return []GarmentGuide{
		{
			Slug:  "dress-gown",
			Label: "Dress / Gown",
			Blurb: "For fitted and flared dresses, gowns and wrap styles. We take bust (not men's chest), waist, hips and the full length so the dress hangs right.",
			Items: []GuideItem{
				itemFrom("women", "shoulder", "Shoulder", "Measure across your back from the edge of one shoulder bone to the other, following the natural shoulder line."),
				itemFrom("women", "bust", "Bust", "Wrap the tape around the fullest part of your bust, keep it level under the arms and relaxed across the back."),
				itemFrom("women", "waist", "Waist", "Find the narrowest part of your torso (usually just above the navel) and measure around it, tape snug but not tight."),
				{Key: "bodice", Label: "Bodice length", How: "From the top of your shoulder (near the base of the neck) down over the bust to your natural waistline. This controls where the waist seam sits.", InchRange: "13–20 in", Tip: "Stand straight and keep the tape following your front body curve."},
				itemFrom("women", "hips", "Hips", "Measure around the fullest part of your hips and seat, about 8 in (20 cm) below the waist, keeping the tape level."),
				itemFrom("women", "sleeve", "Sleeve length", "From the shoulder point down the arm to where you want the sleeve to end (cap, short, elbow or wrist), arm slightly bent."),
				itemFrom("women", "armhole", "Armhole", "Wrap the tape around the arm at the shoulder joint, passing under the armpit — comfortably snug, not digging in."),
				itemFrom("women", "length", "Full length", "From the top of the shoulder down the front to where you want the hem (knee, midi or floor). Wear the shoes/heels you plan to use if length matters."),
			},
		},
		{
			Slug:  "skirt",
			Label: "Skirt",
			Blurb: "For pencil, A-line, wrap and pleated skirts. Hips and waist set the fit; full length sets the hem.",
			Items: []GuideItem{
				itemFrom("women", "waist", "Waist", "Measure around your natural waistline where the skirt waistband will sit. Keep one finger of ease under the tape."),
				itemFrom("women", "hips", "Hips", "Measure around the fullest part of your hips and seat, tape level all the way around."),
				{Key: "skirt-length", Label: "Full length (skirt)", How: "From your waistline down the side to where you want the hem to fall (above knee, knee, midi or maxi).", InchRange: "14–45 in", Tip: "Measure while standing straight; note the shoe height you’ll wear."},
				itemFrom("women", "thigh", "Thigh (for fitted styles)", "Around the fullest part of the thigh, just below the crotch. Helpful for pencil or fitted skirts."),
			},
		},
		{
			Slug:  "blouse-top",
			Label: "Blouse / Top",
			Blurb: "For blouses, peplum tops and fitted tops. Uses bust and upper-body measurements — no hips or full dress length needed.",
			Items: []GuideItem{
				itemFrom("women", "shoulder", "Shoulder", "Across the back from shoulder bone to shoulder bone."),
				itemFrom("women", "bust", "Bust", "Around the fullest part of the bust, tape level and not tight."),
				itemFrom("women", "waist", "Waist", "Around the natural waist — important for fitted or peplum tops."),
				{Key: "bodice", Label: "Bodice length", How: "From the shoulder (near neck base) over the bust to the waistline, or to where you want the top to end for a peplum/crop.", InchRange: "13–24 in", Tip: "Tell us if you want the top to end at the waist, hip, or below."},
				itemFrom("women", "sleeve", "Sleeve length", "From shoulder point to the sleeve end you want."),
				itemFrom("women", "armhole", "Armhole", "Around the arm at the shoulder joint, under the armpit."),
			},
		},
		{
			Slug:  "shirt",
			Label: "Shirt",
			Blurb: "For men's and unisex shirts. Uses chest, shoulder, sleeve and shirt length — no hips, no bust/bodice dress measurements.",
			Items: []GuideItem{
				itemFrom("men", "neck", "Neck", "Around the base of the neck where a collar sits, leaving one finger of ease inside the tape."),
				itemFrom("men", "chest", "Chest", "Around the fullest part of the chest, under the arms, tape level across the shoulder blades. Breathe normally."),
				itemFrom("men", "shoulder", "Shoulder", "Across the back from one shoulder bone to the other."),
				itemFrom("men", "sleeve", "Sleeve length", "From the shoulder point to the wrist bone, with the arm slightly bent."),
				itemFrom("men", "shirt-length", "Shirt length", "From the base of the neck/shoulder down the front to where you want the hem (hip or tucked length)."),
				itemFrom("men", "waist", "Waist (optional, for a fitted shirt)", "Around the natural waist if you want a tapered/slim fit."),
			},
		},
		{
			Slug:  "kaftan",
			Label: "Kaftan",
			Blurb: "For relaxed kaftans and boubous. The key measurements are shoulder, bust/chest, sleeve and full length so it drapes comfortably.",
			Items: []GuideItem{
				itemFrom("women", "shoulder", "Shoulder", "Across the back from shoulder bone to shoulder bone — this sets how the kaftan hangs."),
				itemFrom("women", "bust", "Bust / Chest", "Around the fullest part of the bust/chest, relaxed. Kaftans need ease, so don’t pull the tape tight."),
				itemFrom("men", "sleeve", "Sleeve length", "From the shoulder point to where you want the sleeve to end."),
				itemFrom("men", "kaftan-length", "Full length", "From the shoulder down to the hem (knee, mid-calf or ankle)."),
				itemFrom("women", "hips", "Hips (optional)", "Around the fullest part of the hips if you want the kaftan shaped rather than fully free-flowing."),
			},
		},
		{
			Slug:  "trousers",
			Label: "Trousers",
			Blurb: "For trousers and pants. Uses waist, hips, thigh and leg lengths — no bust, chest, bodice or sleeve measurements.",
			Items: []GuideItem{
				itemFrom("men", "waist", "Waist", "Around where you wear your waistband (natural waist or lower), tape comfortable."),
				itemFrom("men", "hips", "Hips / Seat", "Around the fullest part of the hips and seat, tape level."),
				itemFrom("men", "thigh", "Thigh", "Around the fullest part of the thigh, just below the crotch."),
				itemFrom("men", "trouser-length", "Trouser length (outseam)", "From the waistband down the outside of the leg to the shoe/hem point."),
				itemFrom("men", "inseam", "Inseam", "From the crotch seam down the inside of the leg to the ankle/hem. Easier with a helper or using a pair of trousers that fit well."),
			},
		},
		{
			Slug:  "children",
			Label: "Children’s outfit",
			Blurb: "For children’s dresses, sets and tops. We keep it simple and add a little growing room at the seams.",
			Items: []GuideItem{
				itemFrom("children", "age", "Age", "The child’s age in years — helps us allow growing room."),
				itemFrom("children", "height", "Height", "Child standing straight against a wall, no shoes, measured from floor to top of head."),
				itemFrom("children", "chest", "Chest", "Around the fullest part of the chest, under the arms, tape comfortably snug."),
				itemFrom("children", "waist", "Waist", "Around the natural waist."),
				itemFrom("children", "hips", "Hips (for dresses/skirts)", "Around the fullest part of the hips — include this for dresses and skirts; not needed for a simple top."),
				itemFrom("children", "length", "Outfit length", "From the shoulder down to the hem you want."),
			},
		},
	}
}

// GuideBySlug returns one garment guide, or the first if not found.
func GuideBySlug(slug string) GarmentGuide {
	gs := Guides()
	for _, g := range gs {
		if g.Slug == slug {
			return g
		}
	}
	return gs[0]
}
