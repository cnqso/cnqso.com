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
- Private responses are not cached, and admin paths are redacted from public analytics.

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
