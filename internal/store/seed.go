package store

// Sample catalogue. All images are generated SVG placeholders in
// static/img/fabrics and static/img/designs so the site looks complete before
// real photography exists.
//
// TO REPLACE WITH REAL PHOTOS:
//   1. Drop the files into the /uploads folder (served at /uploads/...).
//   2. Update the Image / Images fields below, or edit the row from the admin
//      dashboard once stage 3 is built.
// Every fabric shows three gallery views, every design three as well.

type seedFabric struct {
	Slug        string
	Name        string
	Type        string
	Description string
	Colours     []string
	Care        string
	Width       string
	Unit        string
	Price       float64
	InStock     bool
	Featured    bool
	Image       string
	Images      []string
}

type seedDesign struct {
	Slug           string
	Name           string
	Category       string
	Style          string
	Occasion       string
	Description    string
	Measurements   []string
	Image          string
	Images         []string
	PriceFrom      float64
	ProductionDays int
	Featured       bool
	FabricSlugs    []string
}

var seedFabrics = []seedFabric{
	{
		Slug: "kitenge-royal-bloom", Name: "Kitenge Royal Bloom", Type: "kitenge",
		Description: "Our signature kitenge: large hand-drawn blooms in terracotta and gold on a deep indigo ground. Printed on a soft cotton wax cloth that holds its colour wash after wash — ideal for wrap dresses, gowns and statement sleeves.",
		Colours:     []string{"Orange", "Gold", "Indigo"},
		Care:        "Hand wash cold or dry clean. Wash separately for the first three washes, dry in shade and iron on the reverse.",
		Width:       "115 cm", Unit: "yard", Price: 1800, InStock: true, Featured: true,
		Image:  "/static/img/fabrics/kitenge-royal-bloom-1.svg",
		Images: []string{"/static/img/fabrics/kitenge-royal-bloom-1.svg", "/static/img/fabrics/kitenge-royal-bloom-2.svg", "/static/img/fabrics/kitenge-royal-bloom-3.svg"},
	},
	{
		Slug: "ankara-indigo-wave", Name: "Ankara Indigo Wave", Type: "ankara",
		Description: "A flowing wave print in indigo and cream, printed on durable Ankara cotton. It drapes beautifully for kaftans and men's sets, and the repeat is small enough to cut economically for children's wear.",
		Colours:     []string{"Indigo", "Cream"},
		Care:        "Machine wash at 30°C on a gentle cycle. Do not bleach. Cool iron on the reverse.",
		Width:       "115 cm", Unit: "yard", Price: 1500, InStock: true, Featured: true,
		Image:  "/static/img/fabrics/ankara-indigo-wave-1.svg",
		Images: []string{"/static/img/fabrics/ankara-indigo-wave-1.svg", "/static/img/fabrics/ankara-indigo-wave-2.svg", "/static/img/fabrics/ankara-indigo-wave-3.svg"},
	},
	{
		Slug: "terracotta-lace", Name: "Terracotta Lace", Type: "lace",
		Description: "Corded French-style lace in burnt terracotta with a scalloped selvedge. We use it as an overlay on satin for occasion wear, or on its own for a lined bodice.",
		Colours:     []string{"Terracotta", "Cream"},
		Care:        "Dry clean only. Steam rather than iron to keep the corded motifs raised.",
		Width:       "130 cm", Unit: "metre", Price: 3200, InStock: true, Featured: false,
		Image:  "/static/img/fabrics/terracotta-lace-1.svg",
		Images: []string{"/static/img/fabrics/terracotta-lace-1.svg", "/static/img/fabrics/terracotta-lace-2.svg", "/static/img/fabrics/terracotta-lace-3.svg"},
	},
	{
		Slug: "plain-cotton-sand", Name: "Plain Cotton Sand", Type: "plain-cotton",
		Description: "A dependable medium-weight cotton in warm sand. Perfect as a lining, for children's school sets or as the base cloth under a printed accent panel.",
		Colours:     []string{"Sand", "Cream"},
		Care:        "Machine wash at 40°C. Tumble dry low. Iron while slightly damp.",
		Width:       "150 cm", Unit: "metre", Price: 900, InStock: true, Featured: false,
		Image:  "/static/img/fabrics/plain-cotton-sand-1.svg",
		Images: []string{"/static/img/fabrics/plain-cotton-sand-1.svg", "/static/img/fabrics/plain-cotton-sand-2.svg", "/static/img/fabrics/plain-cotton-sand-3.svg"},
	},
	{
		Slug: "kitenge-sunburst", Name: "Kitenge Sunburst", Type: "kitenge",
		Description: "Golden sunburst motifs on maroon wax cloth — a bold print that photographs beautifully for wedding-guest gowns. Each yard carries a slightly different repeat, which is part of its charm.",
		Colours:     []string{"Gold", "Maroon", "Orange"},
		Care:        "Hand wash cold. Dry in shade. Iron on the reverse at medium heat.",
		Width:       "115 cm", Unit: "yard", Price: 1900, InStock: true, Featured: true,
		Image:  "/static/img/fabrics/kitenge-sunburst-1.svg",
		Images: []string{"/static/img/fabrics/kitenge-sunburst-1.svg", "/static/img/fabrics/kitenge-sunburst-2.svg", "/static/img/fabrics/kitenge-sunburst-3.svg"},
	},
	{
		Slug: "ankara-earth-grid", Name: "Ankara Earth Grid", Type: "ankara",
		Description: "A quiet geometric grid in earthy brown and gold. Softer in tone than a classic Ankara, so it works for office shirts, pencil skirts and family sets that need to look coordinated without shouting.",
		Colours:     []string{"Brown", "Gold"},
		Care:        "Machine wash at 30°C. Wash with similar colours. Warm iron.",
		Width:       "115 cm", Unit: "yard", Price: 1400, InStock: true, Featured: false,
		Image:  "/static/img/fabrics/ankara-earth-grid-1.svg",
		Images: []string{"/static/img/fabrics/ankara-earth-grid-1.svg", "/static/img/fabrics/ankara-earth-grid-2.svg", "/static/img/fabrics/ankara-earth-grid-3.svg"},
	},
	{
		Slug: "bridal-chantilly-ivory", Name: "Bridal Chantilly Ivory", Type: "lace",
		Description: "Fine Chantilly lace in ivory with a delicate floral trail. Ordered per metre from a limited bolt — we hold a small quantity for bridal fittings and reorder on request.",
		Colours:     []string{"Ivory", "Gold"},
		Care:        "Dry clean only. Store rolled, never folded, to avoid creasing the net.",
		Width:       "140 cm", Unit: "metre", Price: 4500, InStock: false, Featured: true,
		Image:  "/static/img/fabrics/bridal-chantilly-ivory-1.svg",
		Images: []string{"/static/img/fabrics/bridal-chantilly-ivory-1.svg", "/static/img/fabrics/bridal-chantilly-ivory-2.svg", "/static/img/fabrics/bridal-chantilly-ivory-3.svg"},
	},
	{
		Slug: "plain-cotton-indigo", Name: "Plain Cotton Indigo", Type: "plain-cotton",
		Description: "A deep indigo cotton with a crisp hand and almost no sheen. Our most requested cloth for men's shirts and children's uniforms-style sets.",
		Colours:     []string{"Indigo"},
		Care:        "Machine wash at 40°C. Expect a little colour release for the first two washes.",
		Width:       "150 cm", Unit: "metre", Price: 850, InStock: true, Featured: false,
		Image:  "/static/img/fabrics/plain-cotton-indigo-1.svg",
		Images: []string{"/static/img/fabrics/plain-cotton-indigo-1.svg", "/static/img/fabrics/plain-cotton-indigo-2.svg", "/static/img/fabrics/plain-cotton-indigo-3.svg"},
	},
}

var seedDesigns = []seedDesign{
	{
		Slug: "amani-wrap-dress", Name: "Amani Wrap Dress", Category: "women",
		Style: "gown", Occasion: "wedding-guest",
		Description:  "A true wrap dress with a deep V neckline, self-tie waist and a gently flared midi skirt. Cut on the bias so the print falls in soft folds. We finish the neckline with a matching fabric-covered button at the waist.",
		Measurements: []string{"bust", "underbust", "waist", "hips", "shoulder", "sleeve", "armhole", "bicep", "neck", "length", "height"},
		Image:        "/static/img/designs/amani-wrap-dress-1.svg",
		Images:       []string{"/static/img/designs/amani-wrap-dress-1.svg", "/static/img/designs/amani-wrap-dress-2.svg", "/static/img/designs/amani-wrap-dress-3.svg"},
		PriceFrom:    6500, ProductionDays: 10, Featured: true,
		FabricSlugs: []string{"kitenge-royal-bloom", "kitenge-sunburst", "terracotta-lace"},
	},
	{
		Slug: "zawadi-kaftan", Name: "Zawadi Kaftan", Category: "women",
		Style: "kaftan", Occasion: "everyday",
		Description:  "An easy A-line kaftan with side slits and a mandarin collar. One size is not one size here — we cut it to your shoulder and hip so it skims rather than sacks. Comes with a matching sash.",
		Measurements: []string{"bust", "waist", "hips", "shoulder", "sleeve", "length", "height"},
		Image:        "/static/img/designs/zawadi-kaftan-1.svg",
		Images:       []string{"/static/img/designs/zawadi-kaftan-1.svg", "/static/img/designs/zawadi-kaftan-2.svg", "/static/img/designs/zawadi-kaftan-3.svg"},
		PriceFrom:    4800, ProductionDays: 7, Featured: true,
		FabricSlugs: []string{"ankara-indigo-wave", "ankara-earth-grid", "plain-cotton-sand"},
	},
	{
		Slug: "baraka-shirt", Name: "Baraka Shirt", Category: "men",
		Style: "shirt", Occasion: "casual",
		Description:  "A relaxed camp-collar shirt with a straight hem and a single chest pocket. Cut slightly long so it sits well untucked over trousers. The collar is interfaced to hold its shape through the wash.",
		Measurements: []string{"neck", "chest", "waist", "hips", "shoulder", "sleeve", "shirt-length", "height"},
		Image:        "/static/img/designs/baraka-shirt-1.svg",
		Images:       []string{"/static/img/designs/baraka-shirt-1.svg", "/static/img/designs/baraka-shirt-2.svg", "/static/img/designs/baraka-shirt-3.svg"},
		PriceFrom:    3500, ProductionDays: 6, Featured: true,
		FabricSlugs: []string{"ankara-earth-grid", "plain-cotton-indigo", "plain-cotton-sand"},
	},
	{
		Slug: "jabari-kaftan-set", Name: "Jabari Kaftan Set", Category: "men",
		Style: "kaftan", Occasion: "occasion",
		Description:  "A full kaftan set: long embroidered kaftan worn over straight-leg trousers. Gold thread along the placket is stitched by hand, so allow an extra few days before a big event.",
		Measurements: []string{"neck", "chest", "waist", "hips", "shoulder", "sleeve", "kaftan-length", "trouser-length", "inseam", "thigh", "height"},
		Image:        "/static/img/designs/jabari-kaftan-set-1.svg",
		Images:       []string{"/static/img/designs/jabari-kaftan-set-1.svg", "/static/img/designs/jabari-kaftan-set-2.svg", "/static/img/designs/jabari-kaftan-set-3.svg"},
		PriceFrom:    7200, ProductionDays: 12, Featured: false,
		FabricSlugs: []string{"kitenge-royal-bloom", "ankara-indigo-wave"},
	},
	{
		Slug: "rehema-girls-dress", Name: "Rehema Girls' Dress", Category: "children",
		Style: "dress", Occasion: "everyday",
		Description:  "A swing dress with a piped waistband and a half-bow at the back. Room to grow at the side seams, because children rarely stay the size you measured.",
		Measurements: []string{"age", "height", "chest", "waist", "hips", "length"},
		Image:        "/static/img/designs/rehema-girls-dress-1.svg",
		Images:       []string{"/static/img/designs/rehema-girls-dress-1.svg", "/static/img/designs/rehema-girls-dress-2.svg", "/static/img/designs/rehema-girls-dress-3.svg"},
		PriceFrom:    2800, ProductionDays: 5, Featured: true,
		FabricSlugs: []string{"ankara-indigo-wave", "plain-cotton-sand", "ankara-earth-grid"},
	},
	{
		Slug: "pendo-couples-set", Name: "Pendo Couples Set", Category: "couples",
		Style: "set", Occasion: "wedding-guest",
		Description:  "A matched pair: her wrap gown, his camp-collar shirt and trousers, cut from the same print. We plan the cutting layout so both garments come from one bolt and the pattern lines up at the photographs.",
		Measurements: []string{"bust", "waist", "hips", "shoulder", "sleeve", "length", "neck", "chest", "trouser-length", "height"},
		Image:        "/static/img/designs/pendo-couples-set-1.svg",
		Images:       []string{"/static/img/designs/pendo-couples-set-1.svg", "/static/img/designs/pendo-couples-set-2.svg", "/static/img/designs/pendo-couples-set-3.svg"},
		PriceFrom:    14000, ProductionDays: 16, Featured: true,
		FabricSlugs: []string{"kitenge-royal-bloom", "kitenge-sunburst"},
	},
	{
		Slug: "neema-bridal-gown", Name: "Neema Bridal Gown", Category: "bridal",
		Style: "gown", Occasion: "wedding",
		Description:  "A fitted mermaid gown with a corseted back and a detachable lace train. Made over four fittings: toile, structure, overlay, finish. Booking at least eight weeks before the wedding is strongly advised.",
		Measurements: []string{"bust", "underbust", "waist", "hips", "shoulder", "sleeve", "armhole", "bicep", "neck", "length", "height"},
		Image:        "/static/img/designs/neema-bridal-gown-1.svg",
		Images:       []string{"/static/img/designs/neema-bridal-gown-1.svg", "/static/img/designs/neema-bridal-gown-2.svg", "/static/img/designs/neema-bridal-gown-3.svg"},
		PriceFrom:    28000, ProductionDays: 45, Featured: true,
		FabricSlugs: []string{"bridal-chantilly-ivory", "terracotta-lace"},
	},
}
