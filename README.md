# Gojias Collections

A browser-based web app for **Gojias Collections**, a Kisumu business that designs and sews
kitenge- and Ankara-inspired attire. Built with Go, `html/template` and SQLite — no mobile
frameworks, no client-side framework, no build step for the frontend.

> **All three build stages are in.** The public site has the catalogue (Home, Fabrics,
> Designs and detail pages), the five-step order/measurements wizard, confirmation with a
> PDF summary and order tracking. The staff dashboard at `/admin` covers login, dashboard
> stats, fabric and design CRUD with image uploads, order management with status updates
> and a printable measurement sheet, and business settings. Only the editorial pages
> (How to Measure, About, Contact) still show a "coming next" placeholder.

## Requirements

- Go 1.22 or newer (developed on Go 1.26)
- No CGO, no network access at runtime. SQLite is embedded via `modernc.org/sqlite`.

## Run it locally

```bash
go run .
# open http://localhost:8080
```

Flags:

```bash
go run . -addr :9000 -db ./data/gojias.db
```

On first start the server creates `gojias.db`, applies the schema and seeds eight fabrics and
seven designs. Restarting never duplicates the sample rows (seeding only runs when the fabrics
table is empty).

### Admin account

The first admin account is created on first start. Set it explicitly, or a temporary password
is generated and printed once in the server log:

```bash
ADMIN_EMAIL=you@example.com ADMIN_PASSWORD='choose-a-long-password' go run .
```

Sign in at `/admin`. Change the password under **Settings** (bcrypt-hashed; never stored in
plain text). Sessions live in memory, so restarting the server signs staff out.

Delete `gojias.db` (and its `-wal`/`-shm` siblings) to start over from the seed data.

## Project structure

```
main.go                  HTTP server, routes, template helpers, site settings
order_handlers.go        Order wizard, confirmation, PDF summary, tracking, uploads, rate limit
admin.go                 Sessions, login, dashboard, orders, settings, printable sheet
admin_catalog.go         Fabric and design CRUD handlers
admin_upload.go          Shared image validation/storage for uploads
internal/store/store.go  Schema, migrations, seed trigger and catalogue queries
internal/store/orders.go Order table, references, statuses and tracking lookups
internal/store/admin.go  Admin users, settings and catalogue mutations
internal/store/seed.go   Sample fabrics and designs (with replacement notes)
internal/measure/        Measurement fields, tips, ranges, unit conversion and validation
internal/pdf/            Tiny dependency-free PDF writer for the order summary
internal/notify/         SMTP notification for new orders and status updates
templates/base.html      Document shell: meta, fonts, header/footer slots
templates/partials/      Header, footer, shared cards, pager, empty state
templates/pages/         One file per page; each defines "content"
templates/admin/         Dashboard shell, pages and the printable sheet
static/css/style.css     The whole theme
static/css/admin.css     Dashboard layout, tables, forms and print rules
static/js/main.js        Progressive enhancement (nav, gallery, filter auto-submit)
static/js/order.js       Order wizard: steps, validation, unit conversion, estimate
static/img/              Generated SVG placeholder art
uploads/                 Uploaded photos; served at /uploads/
```

Templates and `static/` are embedded in the binary with `//go:embed`. The `uploads/` folder is
read from disk so new images appear without a rebuild.

## Replacing the placeholder images

Every fabric and design currently points at generated SVGs under
`static/img/fabrics/<slug>-1..3.svg` and `static/img/designs/<slug>-1..3.svg`.

To use real photography:

1. The normal route is the dashboard: add or edit a fabric/design and upload photos.
2. To change the seed data itself, edit the `Image` / `Images` values in
   `internal/store/seed.go`, delete `gojias.db` and restart.

## Data model (stage 1)

| Table            | Purpose                                                        |
| ---------------- | -------------------------------------------------------------- |
| `fabrics`        | Name, type, colours, care, width, price per yard/metre, stock   |
| `fabric_images`  | Ordered gallery images per fabric                               |
| `designs`        | Category, style, occasion, starting price, production days      |
| `design_images`  | Ordered gallery images per design                               |
| `design_fabrics` | Which fabrics we recommend for which design                     |
| `orders`         | One row per order: design/fabric snapshots, measurements JSON, customer details, estimate |
| `order_events`   | Status history that drives the tracking timeline                   |
| `admins`         | Dashboard accounts with bcrypt password hashes                      |
| `settings`       | Business details edited in the dashboard (key/value)                |

Prices are stored as Kenyan shillings (KES), whole numbers in the seed data. Prices are per
`unit` (`yard` or `metre`) for fabrics, and "from" prices for designs.

## What works in the catalogue

- Home page: hero, how-it-works strip, featured designs and fabrics, why-choose-us,
  testimonials, CTA band.
- Fabrics: search, filter by type/colour/max price/in stock, sorting, pagination,
  empty state, detail page with gallery, specs, care notes, recommended designs,
  similar fabrics and both CTAs (order link carries `?fabric=<slug>` for stage 2).
- Designs: category tabs, search, style/price filters, sorting, pagination, detail page with
  gallery, production time, required measurements and recommended fabrics
  (`?design=<slug>` carried for stage 2).
- Responsive layout, sticky header with mobile menu, floating WhatsApp button, SEO titles,
  meta descriptions and image `alt` text.

## Order flow (stage 2)

`GET /order` renders one form with five steps: **Design → Fabric → Measurements →
Details → Review**. JavaScript moves between steps, converts units and pre-validates; the
server re-validates every field on `POST /order`, so a tampered or skipped client check
cannot produce an invalid order. The wizard itself does need JavaScript (the measurement
fields and step controls are JS-driven) — without it the page points customers at WhatsApp.

- **Design**: pick from the catalogue or "My own idea" with a description and an optional
  inspiration photo (JPG/PNG/WebP, max 5 MB, checked by content type, not just extension).
- **Fabric**: pick from stock (out-of-stock bolts are disabled) or "I'll bring my own
  fabric". The yardage comes from a per-style estimate and is editable.
- **Measurements**: fields change with the design's category (women, men, children,
  couples/family, bridal), a cm/inches toggle converts values as you switch, every field
  has a "?" tip, and validation messages are specific. Measurements can be remembered in
  `localStorage` for next time — no account needed.
- **Details**: name, phone, optional email, pickup or delivery, address, preferred date.
- **Review**: summary with live estimate and production time, terms checkbox, submit.

On submit the server recomputes the estimate (design "from" price + fabric price ×
yardage), allocates the next reference for the year (`GC-2026-0001`), stores the order with
a random `public_token`, writes a `new` status event and redirects to
`/orders/<ref>?t=<token>`. That page shows the summary, a PDF download
(`/orders/<ref>/summary.pdf?t=<token>`) and a WhatsApp button. Order summaries are only
reachable with the token, so references cannot be guessed to read someone's measurements.

Tracking (`GET /track`, `POST /track`) requires the reference **and** the phone number used
on the order. Phone numbers are normalised to digits, so `0712 345 678` also matches
`+254712345678`. The timeline shows New → Confirmed → Cutting → Sewing → Ready → Delivered
with the date each stage was reached.

### Notifications

The shop notification is always written to the log. To send real email, set:

```bash
SMTP_HOST=smtp.example.com SMTP_PORT=587 SMTP_USER=... SMTP_PASS=... \
MAIL_FROM=orders@gojiascollections.co.ke MAIL_TO=owner@gojiascollections.co.ke go run .
```

If SMTP is not configured the order still saves and the confirmation page still works.

### Abuse protection

`POST /order` is limited to 6 submissions per IP per 10 minutes (in memory), request
bodies are capped, and uploaded files are limited to 5 MB and verified by MIME type.

## Admin dashboard (stage 3)

`/admin` is protected by a bcrypt-checked password and a server-side session in an
HttpOnly, SameSite=Lax cookie (12-hour expiry). Every dashboard POST also carries a
per-session CSRF token; a mismatch returns 403. Login attempts are rate limited per IP.

- **Dashboard** — new / in-progress / ready / delivered-this-month cards, recent orders,
  and out-of-stock fabric alerts.
- **Fabrics & designs** — searchable tables, add/edit forms, multiple image uploads (JPG,
  PNG or WebP, 5 MB each, at most 6 at a time), image removal, featured and stock toggles,
  and delete with a confirmation prompt. Deleting a record also removes its uploaded files.
  Slugs are generated from the name and de-duplicated; designs store required measurements
  (checkboxes) and recommended fabrics (multi-select).
- **Orders** — search by reference/name/phone, filter by status and date, open an order to
  see the client, garment, full measurements and inspiration photo, move it through
  New → Confirmed → Cutting → Sewing → Ready → Delivered (optionally emailing the customer),
  keep private workshop notes, and record a deposit. The printable tailor's sheet opens in
  a new tab and prints cleanly (Ctrl/Cmd+P); it includes measurements, client notes and
  workshop notes.
- **Settings** — business name, WhatsApp number, phone, email, address and hours are saved
  to the database and applied across the public site immediately (footer, WhatsApp links).
  Password changes require the current password and at least 8 characters.

## Deploying

The app is a single binary plus an optional `uploads/` directory.

```bash
GOOS=linux GOARCH=amd64 go build -o gojias .
./gojias -addr :8080 -db /var/lib/gojias/gojias.db
```

- Put it behind nginx/Caddy as a reverse proxy for TLS, and set `-addr 127.0.0.1:8080` so the
  app is not exposed directly.
- Persist the SQLite file and the `uploads/` folder (a mounted volume on a VPS, or a container
  volume on Fly.io/Render/Railway). SQLite is fine for this catalogue size; back up by copying
  the `.db` file while the server is stopped.
- Systemd unit sketch:

  ```ini
  [Service]
  WorkingDirectory=/opt/gojias
  ExecStart=/opt/gojias/gojias -addr 127.0.0.1:8080 -db /var/lib/gojias/gojias.db
  Restart=always
  User=gojias
  ```

## Next stages

1. Content pages: How to Measure guide, About and Contact (form, map, hours).
2. JSON API (`GET /api/fabrics`, `GET /api/designs`, `POST /api/orders`, tracking).
3. Optional extras from the brief: M-Pesa payments, customer accounts with saved
   measurements, wishlist/reviews, SMS, discount codes.
