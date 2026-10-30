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
