package store

// Sample catalogue.
//
// All photography is real, imported from the "Kitenge Materials" folder:
//   - Fabrics: 14 cloth photos in static/img/fabrics/ (13 listed bolts; one
//     extra fold shown as a second gallery image on Kitenge Paprika Wave).
//   - Designs: 17 outfit photos in static/img/designs/.
//
// There are no generated placeholder images in the project. The 7 original
// design slugs are preserved so existing demo orders still resolve; their
// photos and copy have been updated to match the real garments.
//
// TO REPLACE OR ADD PHOTOS:
//   1. Drop files into /uploads (served at /uploads/...), or add them under
//      static/img/ and reference the path below.
//   2. Update Image / Images here, or edit the row from the admin dashboard
//      (/admin). Seeding only runs on an empty database, so for a running site
//      use the dashboard.

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
		Description: "Our signature kitenge: paisley blooms in amber and tangerine, outlined in navy on a warm orange ground. Printed on a soft cotton wax cloth that holds its colour wash after wash — ideal for wrap dresses, gowns and statement sleeves.",
		Colours:     []string{"Orange", "Indigo", "Gold"},
		Care:        "Hand wash cold with a mild detergent, or dry clean. Wash separately for the first three washes, dry in shade and iron on the reverse.",
		Width:       "115 cm", Unit: "yard", Price: 1800, InStock: true, Featured: true,
		Image:  "/static/img/fabrics/kitenge-royal-bloom.jpg",
		Images: []string{"/static/img/fabrics/kitenge-royal-bloom.jpg"},
	},
	{
		Slug: "kitenge-paprika-wave", Name: "Kitenge Paprika Wave", Type: "kitenge",
		Description: "Broad waves of paprika red, tangerine and cream, with a fine dotted trail running through the black ground. A firm wax cloth that stands up on its own — cut it for kaftans, wrap skirts and men's sets that need to hold a line.",
		Colours:     []string{"Red", "Maroon", "Orange", "Cream"},
		Care:        "Hand wash cold, inside out. Dry in shade rather than direct sun. Medium iron on the reverse.",
		Width:       "115 cm", Unit: "yard", Price: 2000, InStock: true, Featured: true,
		Image:  "/static/img/fabrics/kitenge-paprika-wave.jpg",
		Images: []string{"/static/img/fabrics/kitenge-paprika-wave.jpg", "/static/img/fabrics/kitenge-paprika-wave-2.jpg"},
	},
	{
		Slug: "ankara-lilac-bloom", Name: "Ankara Lilac Bloom", Type: "ankara",
		Description: "Deep magenta and maroon sunflowers on a soft lilac ground, ringed with a fine black dot print. Lighter in the hand than a classic wax cloth, so it presses beautifully for dresses, blouses and children's wear.",
		Colours:     []string{"Lilac", "Magenta", "Maroon"},
		Care:        "Machine wash at 30°C on a gentle cycle. Wash with similar colours. Cool iron on the reverse.",
		Width:       "115 cm", Unit: "yard", Price: 1700, InStock: true, Featured: false,
		Image:  "/static/img/fabrics/ankara-lilac-bloom.jpg",
		Images: []string{"/static/img/fabrics/ankara-lilac-bloom.jpg"},
	},
	{
		Slug: "kitenge-midnight-orb", Name: "Kitenge Midnight Orb", Type: "kitenge",
		Description: "Red and tangerine orbs scattered across a black ground, dusted with white dot work. Dramatic enough for evening wear, and a favourite for couples' sets where both outfits have to read from across a room.",
		Colours:     []string{"Black", "Red", "Orange"},
		Care:        "Hand wash cold. Do not bleach. Iron on the reverse while slightly damp.",
		Width:       "115 cm", Unit: "yard", Price: 1750, InStock: false, Featured: true,
		Image:  "/static/img/fabrics/kitenge-midnight-orb.jpg",
		Images: []string{"/static/img/fabrics/kitenge-midnight-orb.jpg"},
	},
	{
		Slug: "kitenge-amber-sunburst", Name: "Kitenge Amber Sunburst", Type: "kitenge",
		Description: "Sunburst medallions in amber, maroon and yellow on a cream ground, in the bold scale you see on market-day kitenge. Each yard carries a slightly different repeat, which is part of its charm — we lay out the pattern with you before cutting.",
		Colours:     []string{"Gold", "Maroon", "Cream"},
		Care:        "Hand wash cold. Dry in shade. Iron on the reverse at medium heat.",
		Width:       "115 cm", Unit: "yard", Price: 1900, InStock: true, Featured: true,
		Image:  "/static/img/fabrics/kitenge-amber-sunburst.jpg",
		Images: []string{"/static/img/fabrics/kitenge-amber-sunburst.jpg"},
	},
	{
		Slug: "ankara-earth-diamond", Name: "Ankara Earth Diamond", Type: "ankara",
		Description: "Rust diamonds and fine bead-like motifs on deep navy, framed by beige pinstripes. Quieter in tone than a classic Ankara, so it works for office shirts, pencil skirts and family sets that look coordinated without shouting.",
		Colours:     []string{"Indigo", "Brown", "Sand"},
		Care:        "Machine wash at 30°C. Wash with similar colours. Warm iron on the reverse.",
		Width:       "115 cm", Unit: "yard", Price: 1400, InStock: true, Featured: false,
		Image:  "/static/img/fabrics/ankara-earth-diamond.jpg",
		Images: []string{"/static/img/fabrics/ankara-earth-diamond.jpg"},
	},
	{
		Slug: "kitenge-crimson-dot", Name: "Kitenge Crimson Dot", Type: "kitenge",
		Description: "A dense crimson and black ground scattered with white dotted scrollwork. The repeat is small, so it cuts economically — our pick for children's sets, trim panels and anything that needs to read as texture from a distance.",
		Colours:     []string{"Red", "Black", "Cream"},
		Care:        "Hand wash cold, separately. Dry in shade. Iron on the reverse.",
		Width:       "115 cm", Unit: "yard", Price: 1800, InStock: true, Featured: false,
		Image:  "/static/img/fabrics/kitenge-crimson-dot.jpg",
		Images: []string{"/static/img/fabrics/kitenge-crimson-dot.jpg"},
	},
	{
		Slug: "ankara-amber-fan", Name: "Ankara Amber Fan", Type: "ankara",
		Description: "Overlapping fan shells in amber and tangerine with navy outlines, sold with a matching headwrap piece. A crisp, dry-hand Ankara that keeps pleats, collars and cuffs sharp through a long day.",
		Colours:     []string{"Orange", "Indigo", "Cream"},
		Care:        "Machine wash at 30°C. Cool iron. Colourfast, but wash separately the first time.",
		Width:       "115 cm", Unit: "yard", Price: 1650, InStock: true, Featured: true,
		Image:  "/static/img/fabrics/ankara-amber-fan.jpg",
		Images: []string{"/static/img/fabrics/ankara-amber-fan.jpg"},
	},
	{
		Slug: "ankara-indigo-orbit", Name: "Ankara Indigo Orbit", Type: "ankara",
		Description: "Indigo roundels on a black ground of fine white dots — polka dots from a few paces away, concentric rings up close. A crisp, low-stretch Ankara that holds a peplum or a pleated skirt without help.",
		Colours:     []string{"Indigo", "Black", "Cream"},
		Care:        "Machine wash at 30°C on a gentle cycle. Cool iron on the reverse.",
		Width:       "115 cm", Unit: "yard", Price: 1550, InStock: true, Featured: false,
		Image:  "/static/img/fabrics/ankara-indigo-orbit.jpg",
		Images: []string{"/static/img/fabrics/ankara-indigo-orbit.jpg"},
	},
	{
		Slug: "kitenge-crimson-spiral", Name: "Kitenge Crimson Spiral", Type: "kitenge",
		Description: "Tight spirals of crimson and chestnut on a cream ground. The repeat is small, so it cuts with almost no waste — our go-to for trim panels, pocket linings and matching family sets.",
		Colours:     []string{"Red", "Brown", "Cream"},
		Care:        "Hand wash cold. Dry in shade. Iron on the reverse.",
		Width:       "115 cm", Unit: "yard", Price: 1500, InStock: true, Featured: false,
		Image:  "/static/img/fabrics/kitenge-crimson-spiral.jpg",
		Images: []string{"/static/img/fabrics/kitenge-crimson-spiral.jpg"},
	},
	{
		Slug: "ankara-meadow-disc", Name: "Ankara Meadow Disc", Type: "ankara",
		Description: "Sage and chestnut discs trailing across an off-white ground, crossed by fine dotted lines. Gentle enough for office wear and children's dresses, and it presses flat without shine.",
		Colours:     []string{"Green", "Brown", "Cream"},
		Care:        "Machine wash at 30°C. Warm iron on the reverse.",
		Width:       "115 cm", Unit: "yard", Price: 1450, InStock: true, Featured: false,
		Image:  "/static/img/fabrics/ankara-meadow-disc.jpg",
		Images: []string{"/static/img/fabrics/ankara-meadow-disc.jpg"},
	},
	{
		Slug: "kitenge-ember-mandala", Name: "Kitenge Ember Mandala", Type: "kitenge",
		Description: "Hand-drawn mandalas in ember red and gold on a wine-black ground. The motif is large, so we lay the pattern out with you before cutting — use it whole on a gown or centred on a kaftan front.",
		Colours:     []string{"Red", "Maroon", "Gold"},
		Care:        "Hand wash cold, separately. Dry in shade. Iron on the reverse at medium heat.",
		Width:       "115 cm", Unit: "yard", Price: 1950, InStock: true, Featured: true,
		Image:  "/static/img/fabrics/kitenge-ember-mandala.jpg",
		Images: []string{"/static/img/fabrics/kitenge-ember-mandala.jpg"},
	},
	{
		Slug: "ankara-cobalt-triangle", Name: "Ankara Cobalt Triangle", Type: "ankara",
		Description: "Cobalt and tangerine triangles stacked into a woven-looking lattice, with a fine red pinstripe running through. Graphic and modern — it suits tunics, jumpsuits and men's shirts cut on the grain.",
		Colours:     []string{"Blue", "Orange", "Cream"},
		Care:        "Machine wash at 30°C. Wash with similar colours. Cool iron.",
		Width:       "115 cm", Unit: "yard", Price: 1600, InStock: true, Featured: true,
		Image:  "/static/img/fabrics/ankara-cobalt-triangle.jpg",
		Images: []string{"/static/img/fabrics/ankara-cobalt-triangle.jpg"},
	},
}

var seedDesigns = []seedDesign{
	// The 7 original slugs are kept so existing orders (e.g. GC-2026-0001,
	// Amani Wrap Dress) still resolve. Photos and details use the real garments.
	{
		Slug: "amani-wrap-dress", Name: "Amani Wrap Dress", Category: "women",
		Style: "dress", Occasion: "everyday",
		Description:  "A flowing A-line dress in a bold kitenge print with a scoop neckline and short sleeves. Cut to skim the body and fall to midi length — easy to wear, easy to move in. Shown here in our flame-print kitenge.",
		Measurements: []string{"bust", "underbust", "waist", "hips", "shoulder", "sleeve", "armhole", "bicep", "neck", "length", "height"},
		Image:        "/static/img/designs/img-20230208-131029-794.jpg",
		Images:       []string{"/static/img/designs/img-20230208-131029-794.jpg", "/static/img/designs/img-20211026-141702-071.jpg"},
		PriceFrom:    4500, ProductionDays: 10, Featured: true,
		FabricSlugs: []string{"kitenge-paprika-wave", "kitenge-royal-bloom", "kitenge-ember-mandala"},
	},
	{
		Slug: "zawadi-kaftan", Name: "Zawadi Kaftan", Category: "women",
		Style: "kaftan", Occasion: "everyday",
		Description:  "A royal-blue bodice paired with a flared floral skirt and double-tier ruffle cuffs. The neckline is softly scooped and the skirt falls in easy folds — made to your shoulder, bust and length for a clean drape.",
		Measurements: []string{"bust", "waist", "hips", "shoulder", "sleeve", "length", "height"},
		Image:        "/static/img/designs/img-20230419-091743-581.jpg",
		Images:       []string{"/static/img/designs/img-20230419-091743-581.jpg", "/static/img/designs/img-20230729-123010-530.jpg"},
		PriceFrom:    5200, ProductionDays: 7, Featured: true,
		FabricSlugs: []string{"ankara-lilac-bloom", "ankara-cobalt-triangle", "ankara-amber-fan"},
	},
	{
		Slug: "baraka-shirt", Name: "Baraka Shirt", Category: "men",
		Style: "shirt", Occasion: "casual",
		Description:  "A crisp short-sleeve top with a contrasting collar and cuff detail, cut straight through the body. Works as a relaxed shirt or a smart-casual top — we tailor the shoulder and sleeve to sit cleanly.",
		Measurements: []string{"neck", "chest", "waist", "hips", "shoulder", "sleeve", "shirt-length", "height"},
		Image:        "/static/img/designs/img-20230217-094507-694.jpg",
		Images:       []string{"/static/img/designs/img-20230217-094507-694.jpg"},
		PriceFrom:    3500, ProductionDays: 6, Featured: true,
		FabricSlugs: []string{"ankara-earth-diamond", "ankara-indigo-orbit", "ankara-meadow-disc"},
	},
	{
		Slug: "jabari-kaftan-set", Name: "Jabari Kaftan Set", Category: "men",
		Style: "kaftan", Occasion: "occasion",
		Description:  "A statement African-print outfit tailored for occasion wear. Cut to your measurements with clean finishing at the neckline, cuffs and hem — a confident choice for celebrations and family events.",
		Measurements: []string{"neck", "chest", "waist", "hips", "shoulder", "sleeve", "kaftan-length", "trouser-length", "inseam", "thigh", "height"},
		Image:        "/static/img/designs/img-20211130-120742-828.jpg",
		Images:       []string{"/static/img/designs/img-20211130-120742-828.jpg", "/static/img/designs/img-20211130-115717-358.jpg"},
		PriceFrom:    7200, ProductionDays: 12, Featured: false,
		FabricSlugs: []string{"kitenge-crimson-dot", "kitenge-royal-bloom", "kitenge-crimson-spiral"},
	},
	{
		Slug: "rehema-girls-dress", Name: "Rehema Girls' Dress", Category: "children",
		Style: "dress", Occasion: "everyday",
		Description:  "A bright Ankara bodice with a flared peplum over a layered white tulle skirt, finished with a matching print border at the hem. Room to grow at the side seams, because children rarely stay the size you measured.",
		Measurements: []string{"age", "height", "chest", "waist", "hips", "length"},
		Image:        "/static/img/designs/img-20221222-153611-201.jpg",
		Images:       []string{"/static/img/designs/img-20221222-153611-201.jpg"},
		PriceFrom:    2800, ProductionDays: 5, Featured: true,
		FabricSlugs: []string{"ankara-meadow-disc", "ankara-lilac-bloom", "kitenge-crimson-spiral"},
	},
	{
		Slug: "pendo-couples-set", Name: "Pendo Couples Set", Category: "couples",
		Style: "set", Occasion: "wedding-guest",
		Description:  "A coordinated couple's look cut from matching or complementary prints. We plan the layout so the garments read as a pair in photographs and tailor each piece to its own measurements.",
		Measurements: []string{"bust", "waist", "hips", "shoulder", "sleeve", "length", "neck", "chest", "trouser-length", "height"},
		Image:        "/static/img/designs/img-20211129-110253-390.jpg",
		Images:       []string{"/static/img/designs/img-20211129-110253-390.jpg", "/static/img/designs/img-20211129-110606-999.jpg"},
		PriceFrom:    14000, ProductionDays: 16, Featured: true,
		FabricSlugs: []string{"kitenge-midnight-orb", "kitenge-ember-mandala", "kitenge-amber-sunburst"},
	},
	{
		Slug: "neema-bridal-gown", Name: "Neema Bridal Gown", Category: "bridal",
		Style: "gown", Occasion: "wedding",
		Description:  "A white lace gown with a sparkling embellished neckline and waist, long sleeves and a flared layered skirt. Made to measure over fittings — book ahead so we have time to get the structure and finish right.",
		Measurements: []string{"bust", "underbust", "waist", "hips", "shoulder", "sleeve", "armhole", "bicep", "neck", "length", "height"},
		Image:        "/static/img/designs/img-20221115-124443-607.jpg",
		Images:       []string{"/static/img/designs/img-20221115-124443-607.jpg"},
		PriceFrom:    28000, ProductionDays: 45, Featured: true,
		FabricSlugs: []string{"kitenge-ember-mandala", "kitenge-crimson-dot", "ankara-amber-fan"},
	},
	// Additional designs from the owner's photo collection.
	{
		Slug: "safi-ankara-dress", Name: "Safi Ankara Dress", Category: "women",
		Style: "dress", Occasion: "everyday",
		Description:  "A royal-blue dress trimmed with a bold African-print yoke, sleeve bands and hem, finished with a matching print front tab. A clean, flattering silhouette that works for church, office or a daytime event.",
		Measurements: []string{"bust", "waist", "hips", "shoulder", "sleeve", "armhole", "bicep", "length", "height"},
		Image:        "/static/img/designs/img-20211026-141702-071.jpg",
		Images:       []string{"/static/img/designs/img-20211026-141702-071.jpg"},
		PriceFrom:    4800, ProductionDays: 9, Featured: true,
		FabricSlugs: []string{"ankara-cobalt-triangle", "ankara-indigo-orbit", "kitenge-royal-bloom"},
	},
	{
		Slug: "nuru-magenta-dress", Name: "Nuru Magenta Dress", Category: "women",
		Style: "dress", Occasion: "occasion",
		Description:  "A fitted V-neck dress in a vivid magenta geometric print with dark contrast cuffs and a flared skirt. Cut close through the bodice and released at the hip for a sharp, feminine shape.",
		Measurements: []string{"bust", "underbust", "waist", "hips", "shoulder", "sleeve", "length", "height"},
		Image:        "/static/img/designs/screenshot-20220410-091051.jpg",
		Images:       []string{"/static/img/designs/screenshot-20220410-091051.jpg"},
		PriceFrom:    5500, ProductionDays: 10, Featured: false,
		FabricSlugs: []string{"ankara-lilac-bloom", "kitenge-crimson-spiral", "kitenge-crimson-dot"},
	},
	{
		Slug: "imani-panel-dress", Name: "Imani Panel Dress", Category: "women",
		Style: "dress", Occasion: "casual",
		Description:  "A navy dress with colourful Ankara print inserts at the neckline, sleeves, front panel and flounced hem. The contrast panels do the work — a simple shape that still stands out.",
		Measurements: []string{"bust", "waist", "hips", "shoulder", "sleeve", "armhole", "length", "height"},
		Image:        "/static/img/designs/img-20220606-132623-191.jpg",
		Images:       []string{"/static/img/designs/img-20220606-132623-191.jpg"},
		PriceFrom:    4200, ProductionDays: 8, Featured: false,
		FabricSlugs: []string{"ankara-earth-diamond", "kitenge-amber-sunburst", "ankara-amber-fan"},
	},
	{
		Slug: "tumaini-occasion-set", Name: "Tumaini Occasion Set", Category: "women",
		Style: "set", Occasion: "occasion",
		Description:  "A tailored African-print occasion outfit, finished for events where you want colour and structure in equal measure. We cut each piece to your measurements and match the print placement across seams.",
		Measurements: []string{"bust", "waist", "hips", "shoulder", "sleeve", "length", "height"},
		Image:        "/static/img/designs/img-20211129-110606-999.jpg",
		Images:       []string{"/static/img/designs/img-20211129-110606-999.jpg", "/static/img/designs/img-20211130-115717-358.jpg"},
		PriceFrom:    6800, ProductionDays: 11, Featured: false,
		FabricSlugs: []string{"kitenge-ember-mandala", "ankara-indigo-orbit", "kitenge-midnight-orb"},
	},
	{
		Slug: "faraja-print-dress", Name: "Faraja Print Dress", Category: "women",
		Style: "dress", Occasion: "everyday",
		Description:  "A made-to-measure print dress with a comfortable everyday fit. Bring your preferred fabric from our collection or your own — we will advise on yardage and cut it to flatter.",
		Measurements: []string{"bust", "waist", "hips", "shoulder", "sleeve", "armhole", "length", "height"},
		Image:        "/static/img/designs/img-20210722-145704-11.jpg",
		Images:       []string{"/static/img/designs/img-20210722-145704-11.jpg"},
		PriceFrom:    4000, ProductionDays: 8, Featured: false,
		FabricSlugs: []string{"kitenge-royal-bloom", "ankara-amber-fan", "kitenge-paprika-wave"},
	},
	{
		Slug: "bahati-print-dress", Name: "Bahati Print Dress", Category: "women",
		Style: "dress", Occasion: "everyday",
		Description:  "A tailored print dress sewn to your measurements in the fabric of your choice. Simple lines, clean finishing, and a fit that is yours rather than off the rack.",
		Measurements: []string{"bust", "waist", "hips", "shoulder", "sleeve", "length", "height"},
		Image:        "/static/img/designs/img-20210818-120224-164.jpg",
		Images:       []string{"/static/img/designs/img-20210818-120224-164.jpg"},
		PriceFrom:    4000, ProductionDays: 8, Featured: false,
		FabricSlugs: []string{"ankara-lilac-bloom", "ankara-meadow-disc", "kitenge-amber-sunburst"},
	},
	{
		Slug: "upendo-fit-flare", Name: "Upendo Fit-and-Flare Dress", Category: "women",
		Style: "dress", Occasion: "wedding-guest",
		Description:  "A fit-and-flare dress that shapes at the waist and moves at the hem — a reliable choice for weddings, introductions and celebrations. Tailored to your measurements for a smooth line.",
		Measurements: []string{"bust", "underbust", "waist", "hips", "shoulder", "sleeve", "armhole", "length", "height"},
		Image:        "/static/img/designs/img-20210826-092001-735.jpg",
		Images:       []string{"/static/img/designs/img-20210826-092001-735.jpg"},
		PriceFrom:    5800, ProductionDays: 10, Featured: false,
		FabricSlugs: []string{"kitenge-ember-mandala", "kitenge-midnight-orb", "ankara-cobalt-triangle"},
	},
	{
		Slug: "jambo-everyday-dress", Name: "Jambo Everyday Dress", Category: "women",
		Style: "dress", Occasion: "casual",
		Description:  "An easy everyday dress in African print — comfortable, breathable, and cut to your own proportions. The kind of piece you reach for often because it fits without fuss.",
		Measurements: []string{"bust", "waist", "hips", "shoulder", "sleeve", "length", "height"},
		Image:        "/static/img/designs/img-20210901-130810-961.jpg",
		Images:       []string{"/static/img/designs/img-20210901-130810-961.jpg"},
		PriceFrom:    3800, ProductionDays: 7, Featured: false,
		FabricSlugs: []string{"ankara-meadow-disc", "ankara-earth-diamond", "kitenge-crimson-spiral"},
	},
	{
		Slug: "furaha-celebration-dress", Name: "Furaha Celebration Dress", Category: "women",
		Style: "dress", Occasion: "occasion",
		Description:  "A celebration-ready dress made to measure in rich print and careful finishing. Designed to photograph well and wear comfortably through a long day of events.",
		Measurements: []string{"bust", "underbust", "waist", "hips", "shoulder", "sleeve", "armhole", "length", "height"},
		Image:        "/static/img/designs/img-20230729-123010-530.jpg",
		Images:       []string{"/static/img/designs/img-20230729-123010-530.jpg"},
		PriceFrom:    6200, ProductionDays: 12, Featured: false,
		FabricSlugs: []string{"ankara-amber-fan", "kitenge-amber-sunburst", "kitenge-royal-bloom"},
	},
	{
		Slug: "taji-heritage-set", Name: "Taji Heritage Set", Category: "couples",
		Style: "set", Occasion: "wedding",
		Description:  "A heritage-inspired coordinated set for traditional ceremonies and weddings. Each garment is cut to its own measurements while the fabrics and finishing tie the pair together.",
		Measurements: []string{"bust", "waist", "hips", "shoulder", "sleeve", "length", "neck", "chest", "trouser-length", "height"},
		Image:        "/static/img/designs/img-20211130-115717-358.jpg",
		Images:       []string{"/static/img/designs/img-20211130-115717-358.jpg", "/static/img/designs/img-20211129-110253-390.jpg"},
		PriceFrom:    15000, ProductionDays: 18, Featured: false,
		FabricSlugs: []string{"kitenge-ember-mandala", "kitenge-crimson-dot", "ankara-indigo-orbit"},
	},
}
