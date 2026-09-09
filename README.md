# cnqso.com

This is my personal website. I built it using Go's net/http library because I wanted more freedom than I had with other frameworks.

I try to keep things simple, fast, and portable. Mostly I want iteration to be fast and development to be fun.

The goal is to have one place where I can host all my web dev projects, forever.

## Build and run

The server uses Go 1.25 or newer. Docker builds TypeScript and Spirals from their
lockfiles, runs the Go tests, and packages the server and templates. Authored
fonts, images, and audio are versioned; personal content stays outside Git.

```sh
git submodule update --init --recursive
./scripts/prepare-data.sh
docker compose up -d --build
```

The container listens on `127.0.0.1:1739`; put an HTTPS reverse proxy in front of
it. Writable bind mounts must be accessible to the container's UID/GID 1000.
On a fresh Linux host, prepare `webm/`, `go/db/`, and `go/files/` accordingly.

For local Go development, run `npm ci && npm run build` at the repository root,
then run `CNQSO_COMPILE_TYPESCRIPT=false go run .` from `go/`. Re-run the asset
build when TypeScript changes. Building Spirals locally additionally requires
`npm ci --legacy-peer-deps` and `PUBLIC_URL=/spirals npm run build` in its submodule,
after creating `go/static/react/spirals/`.

## Deployment

Production polls GitHub with a Linux/systemd timer. Install it once:

```sh
sudo ./scripts/install-autodeploy.sh
```

The timer checks roughly every minute after the previous run finishes. It
fast-forwards `main`, synchronizes pinned submodules on every run, and builds only
when the revision changes or the server needs recovery. Local tracked edits in
the main repository or any submodule stop deployment. Submodule revisions are
managed by the main repository; change their pins in Git rather than on the server.

An interrupted submodule update is retried on the next poll. A failed build
leaves the running container alone. A successful build recreates the container,
including when the image is unchanged but the old process is unhealthy. Recovery
of an existing deployment does not depend on a successful GitHub fetch.
There is no automatic rollback of an application that builds but fails at runtime.

```sh
systemctl list-timers cnqso-web-deploy.timer
journalctl -u cnqso-web-deploy.service -f
sudo systemctl start cnqso-web-deploy.service
```

## Digital library

The library uses ordinary folders and files, with no catalog database:

```text
go/files/
  .admin-password       # salted password hash; never served
  Fiction/              # private
  personal-book.epub     # private
  public/               # the only subtree exposed at /odir/
    papers/
```

- `/odir/` browses and downloads public files, including nested folders.
- `/admin/odir/` provides the full library after signing in with one administrator password.
- Uploads default to a private destination. Choose a **Public** destination to share a file.
- **New folder** creates a folder in the currently viewed location. Open `public/`
  first to create a public folder.
- Uploads accept any regular file up to 100 MiB. Existing names are never overwritten,
  and an interrupted upload never publishes a partial file.
- PDFs can open in the browser and support range requests. Other files download
  as attachments so uploaded HTML or scripts cannot run as part of the website.
- Private responses are not cached, and private library paths are redacted from analytics.

Set or change the password after building the image:

```sh
./scripts/set-library-password.sh
```

The command prompts without echoing the password and stores only a salted bcrypt
hash in the persistent library. It does not place the password in command arguments,
Git, or logs. Administration remains disabled until a password is set. Changing it
revokes existing sessions. Sessions expire after 12 hours and reset on server restart.
For native development, pipe a password from a hidden terminal prompt into
`go run ./cmd/library-password` from `go/`.

The forms also provide the upload endpoint: `POST /admin/odir/upload`, using the
signed-in session cookie and a multipart body containing `csrf`, `destination`,
then `file`. `destination` is relative to `go/files/`; only destinations beginning
with `public/` (or exactly `public`) are public. The request must supply this site's
Origin header or Referer. Folder creation uses `POST /admin/odir/mkdir` with
`csrf`, `parent`, and `name`. No separate API key or upload service is required.

### Traffic dashboard

`/dashboard` and its per-IP pages and JSON APIs require the same admin password
and session as the library. The library admin has a dashboard link. Login returns
you to the requested dashboard page; logout or password rotation revokes both.
The shared HttpOnly, SameSite=Strict cookie is scoped to `/`. This replaces the old
library-only cookie, so browsers must sign in again after this update.

Traffic summaries exclude health checks and admin/dashboard traffic, including
those records in existing history. Route totals group query-string variants.
Server error summaries count 5xx responses; the status breakdown still includes
4xx responses. Timing averages include zeroes, and new requests retain fractional
milliseconds. Old integer timings and original address strings are preserved as recorded;
reports use a separate normalized IP column, populated for historical rows too.
These are request/address statistics, not a count of human visitors.

New access records omit query strings and health checks. Startup also strips query
strings and fragments from historical access URLs and redacts private library and
dashboard paths. This cleanup preserves event counts and other fields and runs
again after rollback/re-upgrade. History API responses apply the same redaction.
Back up existing databases into a restricted location before upgrading; any old
backups and infrastructure logs may still contain the original URLs.
Client addresses are
normalized without ports. `X-Real-IP` is used only when the direct peer matches
`CNQSO_TRUSTED_PROXIES`; the proxy must overwrite this header. Forwarded chains are
not accepted. Native runs trust only loopback by default; Compose also trusts the
usual Docker bridge range (`172.16.0.0/12`). Override `CNQSO_TRUSTED_PROXIES` with
comma-separated CIDRs if your proxy or Docker network differs.

SQLite uses WAL and indexes on timestamp and `(client_ip, timestamp)`. Startup
adds indexes and fills the normalized IP column without deleting history or
replacing original address values. Application queries
use a bounded connection pool; a separate logging connection and a 64-entry worker
queue keep database writes off the request path. Write failures and queue overflow
are reported to stderr. Analytics is best-effort during overload or abrupt process
failure; normal shutdown drains the queue. Dashboard queries have a three-second
deadline. Existing history is retained; there is no automatic deletion policy.

For a live database backup, use SQLite's backup command/API rather than copying
only `db.db`: recent committed data can reside in `db.db-wal`. Alternatively, stop
the server before copying the database directory.

### Visitor journeys

`/dashboard/journeys` lists browsers seen in the last 30 days. Each browser has a
chronological view of page and file requests, with a new visit after 30 minutes
of inactivity. Adjacent PDF/file requests within five minutes are collapsed;
raw mode preserves every recorded request. Detail pages use stable timestamp/ID
cursors for 200 requests per page, and browser lists show 50 entries per page.
A long visit can span pages. Downloads indicate requests, not confirmed reads,
and there is no time-on-page measurement. Browser labels can also represent bots.

Recognition uses a random 128-bit first-party `cnqso_visitor` cookie with a fixed
30-day expiry, HttpOnly, SameSite=Lax, and Secure outside localhost. It is issued
on page/file responses, not health checks, admin pages, redirects, or assets.
Clearing cookies starts a new label; it is never reconstructed from IPs or device
characteristics. GPC (`Sec-GPC: 1`) and DNT (`DNT: 1`) disable recognition and clear
an existing visitor cookie on public requests. Ordinary operational access logs
continue. The cookie never grants access to administration.

There is no device probing, third-party service, or new client-side script. Referrers retain only an external origin or a same-site path,
without queries, fragments, or admin paths. These are pseudonymous browser records,
not identified people. Journeys begin with this update; older logs remain available
in the IP dashboard. The 30-day view/cookie window is not automatic deletion of
stored history. A site's privacy notice should describe this first-party analytics.

### Experimental visitor fingerprints

Passive fingerprinting supplements cookie-based journeys. On public page/file
requests, the server hashes the browser/version (User-Agent), language preferences,
and low-entropy browser hints already supplied by the browser (Sec-CH-UA, platform,
and mobile). It does not include IP addresses, cookies, URLs, or account identity.
Language and hint values are not stored separately. Hashes use HMAC with a random,
process-local secret and are separated by hostname and UTC date. Signatures change
at UTC midnight and whenever the server restarts.

Open a visitor journey to see its recent signatures and up to 20 **possible**
matching browser labels from the last 24 hours. Shared browser settings can match
unrelated visitors; changed settings or privacy protections can split one visitor.
Matches never merge journeys or restore a deleted cookie. The cookie remains the
primary browser identifier. Bots can spoof these inputs, and fingerprint matches
must never be used as authentication or evidence of a person's identity.

`CNQSO_FINGERPRINTING=false` disables collection and the comparison panel. GPC and
Do Not Track disable both fingerprints and cookie recognition. Admin routes and
assets do not collect fingerprints. Existing fingerprint values remain in historical
logs; daily rotation bounds new comparisons, not historical data retention. Describe
this collection in the site's privacy notice before publishing it.

### Existing ODIR content

Before starting the new container, `scripts/prepare-data.sh` copies the old
`go/odir/papers/` into `go/files/public/papers/` once. It preserves the old directory
and any existing destination files. Existing `/odir/papers/...` URLs keep working.
The completion marker prevents later deployments from restoring files you
intentionally removed. Import old papers before the first migration, or copy them
explicitly afterward. The old font URL redirects to the versioned static font.

### Moving or restoring the site

Back up `go/files/` (including the hidden password hash), `go/db/` database files,
and `webm/`. Restore them with ownership accessible to UID 1000 before starting
Compose. The library's public/private state is determined entirely by folder
placement, so copying the directory preserves it. A public download cannot be
recalled after somebody has saved a copy.

The older Petrarchive scraper still writes new images into the container. Preserve
those separately before replacing it; migrating that archive storage is outside
this library change.

## Checks

```sh
(cd go && go test -race ./... && go vet ./...)
python3 -m unittest discover -s tests -v
shellcheck scripts/*.sh
npm ci && npm run build
docker build -f go/Dockerfile .
```

### TODO
* Get sokodle running on here
    * Port all nextjs endpoints to go
    * Switch to sqlite
    * Restructure in a not-vibe-coded and extensible way
    * Make like 14 levels
    * Post it on reddit
### Backburner
* Update all links
* Automate npm build steps for submodules
    * Get off npm??
* Bypass wordledata repo for reverseWordleSolver
* Convert spirals to ts
* Get sanity CMS running here
    * Maybe abandon it. Latency is an aesthetic issue
    * Remove all lame blog articles
* Get rid of my github.io site
