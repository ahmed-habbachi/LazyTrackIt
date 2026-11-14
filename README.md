# LazyTrackIt

A terminal UI for time tracking, built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).
Backends are pluggable behind `internal/provider.Provider`. Three are built
in: **Case.TrackIt** (OIDC device-code login), **Toggl Track** (static API
token), and a customer **Zeiterfassung / week-booking** API (also a static
API token).

## Downloading a release

Tagged versions (`vX.Y.Z`) are built automatically for Linux (amd64, arm64)
and Windows (amd64) — grab the archive for your platform from the
[Releases page](../../releases).

### Linux

```
tar -xzf lazytrackit_<version>_linux_<arch>.tar.gz
```

`tar` preserves the executable bit, so no `chmod +x` should be needed. Put
the extracted `lazytrackit` binary at `~/.local/bin/lazytrackit` — per-user,
no root needed, and works out of the box if `~/.local/bin` is already on
your `PATH` (most distros add it by default for login shells):

```
mkdir -p ~/.local/bin && mv lazytrackit ~/.local/bin/
```

Then run it with `lazytrackit` (if it's on your `PATH`) or the full path.

Don't install it under a root-owned directory such as `/usr/local/bin`
instead: auto-update (see below) runs as your regular user and can never
write there, so you'd be stuck manually re-downloading every release. If
LazyTrackIt ever does end up somewhere unwritable, it detects this and tells
you so, rather than silently never updating.

### Windows

Right-click the `.zip` → **Extract All** (or any archive tool — it's a
standard zip, no extra software required). Put the extracted
`lazytrackit.exe` under:

- `%LOCALAPPDATA%\Programs\lazytrackit\` — per-user, no admin rights
  needed. This is the conventional spot for user-installed apps that don't
  come with their own installer.

Then run it from **Windows Terminal** or PowerShell (see
[Running on Windows](#running-on-windows) below for why, and for what
happens if you double-click it instead). Since the `.exe` isn't
code-signed, Windows SmartScreen may flag it as from an "unknown
publisher" — click **More info** → **Run anyway**.

### Verifying a download (optional)

Each archive has a matching `.sha256` file:

```
sha256sum -c lazytrackit_<version>_linux_<arch>.tar.gz.sha256   # Linux
CertUtil -hashfile lazytrackit_<version>_windows_amd64.zip SHA256  # Windows (PowerShell/cmd)
```

They'll still need their own `config.yaml` (see Setup below) — the binary
itself contains no account-specific configuration.

### Auto-updates

Every tagged build knows its own version, and on startup LazyTrackIt checks
GitHub for a newer release. If one's available for your OS/arch, it's
downloaded, checksum-verified, and swapped in for the installed binary
automatically — no action needed, and your current session is never
interrupted. The new version takes effect the next time you start
LazyTrackIt, and the app lets you know when that's ready with a line at the
bottom of the entry list.

If the check or install fails for any reason (offline, rate-limited, no
build for your platform, etc.) it just stays quiet and you keep using the
version you have — nothing is ever surfaced as an error. The one exception
is the install directory itself not being writable (e.g. LazyTrackIt lives
under `/usr/local/bin`): that's a standing problem, not a transient one, so
it's reported with a persistent line at the bottom of the entry list instead
of staying silent. Every check's outcome — including a quiet no-op and why —
is also appended to `update.log` in the config directory, for when you want
to double check what actually happened.

Binaries built with plain `go build`/`go run` (rather than a tagged release)
don't have a version baked in, so they skip the check entirely. To disable
it outright, add to `config.yaml`:

```yaml
check_for_updates: false
```

## Setup

1. Build and run once:

   ```
   go run .
   ```

   On first run there's no config yet, so LazyTrackIt writes a starter file
   and exits, **printing the exact path it wrote to** so you know what to
   edit next. That path is:

   - `$XDG_CONFIG_HOME/lazytrackit/config.yaml` if `XDG_CONFIG_HOME` is set,
   - `%APPDATA%\lazytrackit\config.yaml` on Windows (typically
     `C:\Users\<you>\AppData\Roaming\lazytrackit\config.yaml`),
   - `~/.config/lazytrackit/config.yaml` on Linux/macOS otherwise.

2. Edit that file:

   ```yaml
   active_provider: trackit
   providers:
     trackit:
       type: trackit
       base_url: https://trackit.yourcompany.com
       auth:
         issuer: https://auth.yourcompany.com/realms/yourrealm
         client_id: <your Keycloak client id>
         client_secret: ""   # only if the client is confidential
         scopes: [openid, profile, offline_access]
   ```

   The Keycloak client referenced by `client_id` must have the **OAuth 2.0
   Device Authorization Grant** enabled (Keycloak: client's *Capability
   config* → *OAuth 2.0 Device Authorization Grant* on).

3. Run `go run .` again. On first login you'll see a URL and a short code;
   approve it in your browser (LazyTrackIt tries to open it for you). Tokens
   are cached under `~/.local/state/lazytrackit/tokens/` (on Windows:
   `%LOCALAPPDATA%\lazytrackit\tokens\`), so you won't need to log in again
   until the refresh token expires.

### Using Toggl Track instead

Toggl has no device-code flow to run — grab your API token from
[track.toggl.com/profile](https://track.toggl.com/profile) and configure it
directly:

```yaml
active_provider: toggl
providers:
  toggl:
    type: toggl
    auth:
      api_token: <your Toggl API token>
```

`base_url` can be omitted (defaults to `https://api.track.toggl.com`). There's
no login step or token cache: the token in `config.yaml` is the credential,
and it doesn't expire until you regenerate it in Toggl.

### Using a Zeiterfassung / week-booking deployment instead

This one also has no device-code flow (its OIDC login is session-cookie
based, meant for the browser, not a CLI): mint an API token the same way a
CLI script would and configure it directly:

```yaml
active_provider: weekbooking
providers:
  weekbooking:
    type: weekbooking
    base_url: https://zeiterfassung.example.com
    auth:
      api_token: <your week-booking API token>
```

Like Toggl, there's no login step or token cache here either. Note the
backend's own model: entries are booked as decimal hours against an ISO
(year, week, weekday) rather than a start/end time, there's no tag concept,
and a week can be "closed" — entries in a closed week come back from
LazyTrackIt as locked (read-only) and the API rejects writes against them
with a `week_closed` error.

## Desktop app (tray)

`lazytrackit-gui` is a desktop version of the same app: an icon in the
system tray plus a window with your week, built with
[Wails v3](https://v3.wails.io). It reads the same `config.yaml`, uses the
same cached logins and remembered project/tags, and talks to the same
providers. You can use it and the terminal UI side by side.

- **Tray**: left-click shows or hides the window. The menu also shows
  your check-in state, lets you check in or out, and has **Start at login**
  and **Quit**. Closing the window only hides it; the app keeps running
  until you choose **Quit**.
- **Start at login**: tick it in the tray menu or the window's sidebar.
  This adds a login item (Windows: `HKCU\…\Run`; Linux:
  `~/.config/autostart/lazytrackit.desktop`) that starts the app with
  `--hidden`, so it boots straight into the tray without opening a window.
- **Only one copy runs**: launching it again just brings the existing
  window to the front.
- **Keyboard**: `n` new entry, `←`/`→` change week, `t` this week, `r`
  refresh, `c` check in/out, `Esc` close. In the entry form, Ctrl+Enter
  saves.
- **Check-in reminder**: with `attendance: true` on the provider, the app
  shows a desktop notification on weekdays when you haven't checked in:
  at 09:30 and every 30 minutes after until noon, and 2 minutes after the
  app starts or your computer wakes from sleep (between 07:00 and 17:00).
  It always checks your live status first, and once it has seen you
  checked in that day it stays quiet. Its **Check in** button (or a click
  on the notification) opens the check-in dialog. Change the time with
  `check_in_reminder: "08:45"` in `config.yaml`, or turn it off with
  `check_in_reminder: "off"`.
- **Updates**: these work like the TUI's auto-update, but each app only ever
  installs its own build. GUI archives are named
  `lazytrackit-gui_<version>_<os>_<arch>_desktop.<ext>`.

Install it the same way as the TUI: put `lazytrackit-gui` /
`lazytrackit-gui.exe` next to it, in `~/.local/bin` or
`%LOCALAPPDATA%\Programs\lazytrackit\`.

**Linux requirements**: the app renders with WebKitGTK, so you need
`libgtk-3-0` and `libwebkit2gtk-4.1-0` (preinstalled on most desktop
distros). Unlike the TUI, the Linux GUI is linked dynamically. It needs a
glibc at least as new as the release build machine's, so on an older distro,
build it yourself (see below). On GNOME, the tray icon only shows with the
[AppIndicator extension](https://extensions.gnome.org/extension/615/appindicator-support/).
Without it, use the window and your app launcher instead.

**Windows requirements**: the WebView2 runtime, which is built into
Windows 11 and current Windows 10.

### Building it

```
go build -tags gtk3 -o lazytrackit-gui ./cmd/lazytrackit-gui
```

- **Windows**: no C compiler is needed, so it cross-compiles like the TUI.
  Add `-ldflags "-H windowsgui"` so no console window opens with it:
  `GOOS=windows go build -ldflags "-H windowsgui" -o lazytrackit-gui.exe ./cmd/lazytrackit-gui`.
- **Linux**: this needs cgo and the dev packages (Debian/Ubuntu:
  `sudo apt install build-essential libgtk-3-dev libwebkit2gtk-4.1-dev`;
  Arch: `sudo pacman -S base-devel gtk3 webkit2gtk-4.1`). Keep the
  `-tags gtk3`: without it Wails v3 builds against GTK 4 and WebKitGTK 6.0
  instead, and the build fails with `Package 'webkitgtk-6.0' not found`.
  The tag has no effect on Windows.
- **Frontend**: plain HTML/CSS/JS in `internal/gui/frontend`, embedded into
  the binary. There's no Node or npm step.

## Building a Windows executable

Tagged releases are built automatically (see
[Downloading a release](#downloading-a-release) above) — this section is
only for building your own one-off binary, e.g. from an untagged commit.

You don't need Windows to build the Windows binary — Go cross-compiles from
Linux/macOS out of the box:

```
GOOS=windows GOARCH=amd64 go build -o lazytrackit.exe .
```

This produces a single `lazytrackit.exe` with no external dependencies to
install. To send it to friends for testing:

1. Zip it up (recommended, since some mail/chat tools block raw `.exe`
   attachments):

   ```
   zip lazytrackit-windows.zip lazytrackit.exe
   ```

2. Send `lazytrackit-windows.zip` via whatever channel is convenient (email,
   Slack, Discord, a shared drive link, etc.).
3. Tell your friend to unzip it and double-click `lazytrackit.exe`, or run
   it from Windows Terminal / PowerShell (see below for why that's
   preferred over the legacy Command Prompt).
4. Since the `.exe` isn't code-signed, Windows SmartScreen may show an
   "unknown publisher" warning on first launch — click **More info** → **Run
   anyway** to proceed.

They'll still need their own `config.yaml` (see Setup above) — the binary
itself contains no account-specific configuration.

## Running on Windows

- **Config file**: `%APPDATA%\lazytrackit\config.yaml` — this is what you
  edit with your `base_url` and Keycloak `client_id` (step 2 above). The app
  prints this exact path the first time it runs, before you've created it.
- **Cached tokens / remembered last-used project & tags**: both live under
  `%LOCALAPPDATA%\lazytrackit\`, separate from the config file since they're
  machine-local and shouldn't roam between PCs the way hand-edited config
  should.
- **Terminal**: run it from **Windows Terminal** or PowerShell rather than
  the legacy Command Prompt console host — the UI uses emoji and box-drawing
  characters that the old console doesn't render correctly.
- If you double-click `lazytrackit.exe` directly (instead of running it from
  an already-open terminal) on a run that only prints a message and exits —
  e.g. the first-run config path, or an error — it'll wait for you to press
  Enter before the window closes, so the message doesn't just flash and
  disappear.

## Using it

- `n` — new time entry
- `e` / `enter` — edit the selected entry
- `x` — delete the selected entry (asks to confirm)
- `r` — refresh
- `[` / `]` — previous / next week
- `t` — jump back to the current week
- `c` — check in / check out (only when attendance is enabled, see below)
- `p` — switch provider (only shown once more than one is configured)
- `q` / `ctrl+c` — quit

In the entry form: `tab`/`shift+tab` (or `↑`/`↓`) move between fields,
`←`/`→` cycle the selected project, `enter` saves, `esc` cancels.

## Check-in / check-out

TrackIt 4.0.0+ also tracks attendance (checking in and out of a work
location such as Office or Home). It's off by default, since not every
provider type supports it; turn it on per provider in `config.yaml`:

```yaml
providers:
  trackit:
    type: trackit
    attendance: true
    # ...
```

The main list then shows whether you're checked in (and where, since when),
and `c` opens a check-in form — pick a work location with `↑`/`↓` (it
defaults to the one you used last), add an optional note, `enter` to
confirm — or, while checked in, a check-out form with an optional note
(left empty, the note from check-in is kept). Enabling `attendance` on a
provider type that doesn't support it (Toggl, week booking) is rejected at
startup.

## Using multiple providers

You can configure more than one provider under `providers:` in
`config.yaml` — e.g. two different TrackIt accounts, or a staging and
production environment (see the commented-out example in the starter
config). Press `p` from the main list to switch between them.

Whichever provider you last switched to is remembered locally (alongside
your cached tokens, so it doesn't roam between machines) and becomes the
default on the next run, taking precedence over `active_provider` in
`config.yaml`. Each provider keeps its own cached login and its own
remembered last-used project/tags, so switching back and forth doesn't
require logging in again unless a provider's session has actually expired.

## TrackIt wire-format notes

The OpenAPI spec documents field *shapes* but not always their semantics, so
these were confirmed against a live account (TrackIt 4.0.0):

- `timeFrom` / `timeTo` / `timeActual` are **seconds after midnight** on the
  entry's `date` (e.g. `32400` = 09:00), not minutes.
- `date`, `checkInTime` and `checkOutTime` are .NET `DateTime`s, sometimes
  serialized without a timezone; they're parsed as local time.
- `color` fields were JSON numbers before 4.0.0 and are strings since;
  `looseString` (`internal/provider/trackit/json.go`) accepts either.
- `GET /api/Members/me` resolves the caller from the bearer token (use it,
  not `/api/Members/ByUsername/{username}`). It leaves `userName` empty.
- `GET /api/Tags/GetByProject/{id}` returns the project's tags **and** every
  system tag (`isSystemTag`).
- `GET /api/Attendance/me/status` answers "not checked in" with an empty
  body, and `POST /api/Attendance/{id}/checkout` only keeps the check-in note
  when `note` is omitted/null — an empty string wipes it.
- Validation failures come back as ASP.NET `ValidationProblemDetails`;
  `APIError` surfaces the per-field `errors` rather than the generic title.

## Developing a provider

The UI (`internal/ui`) only ever talks to the `provider.Provider` interface
in `internal/provider/provider.go`. A backend is a Go package implementing
it, plus a line in `main.go` and a config entry. The three built-in
providers are good templates for the two auth styles:

| Provider | Auth | Notable differences |
|---|---|---|
| `internal/provider/trackit` | OIDC device-code flow (`internal/auth`), cached + refreshed tokens | project-scoped + system tags, locked entries, attendance |
| `internal/provider/toggl` | static API token (HTTP Basic) | workspace-scoped, no locking, tags not tied to projects |
| `internal/provider/weekbooking` | static API token (Bearer) | no start/end times (hours per ISO weekday), no tags, closed weeks |

### 1. Package layout

Create `internal/provider/<name>/` with, by convention:

- `client.go` — `AuthConfig`, `Client`, `New`, `Name`, `IsAuthenticated`,
  `Login`, and a `doJSON(ctx, method, path, body, out)` helper that adds
  auth headers, encodes/decodes JSON and turns non-2xx responses into an
  error.
- `apierror.go` — an `APIError` type that pulls a readable message out of the
  backend's error body (it's shown verbatim on the error screen and in
  forms, so make it useful).
- one file per resource (`members.go`/`me.go`, `projects.go`, `tags.go`,
  `timeentries.go`) holding the wire DTOs (unexported, `json`-tagged
  structs) and the conversion to/from the `provider` types.

Keep wire types private to the package; only `provider.*` types cross the
boundary.

### 2. Constructor and config

```go
type AuthConfig struct {
	APIToken string `yaml:"api_token"`
}

func New(name string, cfg config.ProviderConfig) (*Client, error) {
	var authCfg AuthConfig
	if err := cfg.Auth.Decode(&authCfg); err != nil {
		return nil, fmt.Errorf("decoding auth config for provider %q: %w", name, err)
	}
	return &Client{name: name, baseURL: strings.TrimRight(cfg.BaseURL, "/"), cfg: authCfg, http: &http.Client{Timeout: 30 * time.Second}}, nil
}
```

- `config.ProviderConfig` gives you `Type`, `BaseURL`, `Attendance` and a raw
  `Auth` YAML node. The `auth:` block's shape is yours to define: decode it
  into your own `AuthConfig`.
- `New` must not do network I/O; the UI calls the provider later from
  background goroutines.
- `name` is the config key (e.g. `trackit2`), not the type. Use it as the
  key for anything persisted per account (`auth.SaveTokens(name, …)`), since
  several accounts of the same type can be configured side by side.

Register the type in `buildProviders` in `main.go`:

```go
case "mybackend":
	prov, err := mybackend.New(name, pcfg)
	if err != nil {
		return nil, nil, err
	}
	providers[name] = prov
```

and add a commented example to `exampleConfig` in
`internal/config/config.go` and a "Using … instead" section to this README.

### 3. The `Provider` contract

Every method gets a `context.Context` with a timeout (20s, 15min for
`Login`); pass it to every HTTP request. Methods are called from goroutines,
so guard any cached state (tokens, the current member) with a mutex.

| Method | What the UI expects |
|---|---|
| `Name()` | The config key passed to `New`. Used to key per-provider state (last-used project/tags). |
| `IsAuthenticated()` | Whether credentials are already available **without a network call**: a cached token file, or a configured API key. If false, the UI calls `Login` first. |
| `Login(ctx, onPrompt)` | Obtain and persist credentials. Call `onPrompt(provider.LoginPrompt{Message, URL, Code, Expires})` only if the user has to do something out-of-band (the UI shows it and opens `URL` in a browser), then block until done or `ctx` expires. Static-token providers never call `onPrompt`; they just validate the token (e.g. by calling `Me`). |
| `Me(ctx)` | The authenticated user. `ID` is what the UI later puts in `TimeEntry.MemberID`; `UserID` is any second backend identifier you need. `FullName`/`Email`/`Username` are display-only (`DisplayName()` falls back in that order) and may be empty. Cache the result if other calls need it. |
| `ListProjects(ctx)` | Projects the user can book on. The UI offers every project returned (it doesn't filter on `Active`), so leave out ones that can't take time. `ClientName`/`Color` may be empty. |
| `ListTags(ctx, projectID)` | Tags offered in the entry form for that project. `projectID` may be `""` (no project yet). Set `IsSystem` for tags valid on every project. Backends without tags return an empty slice, not an error. |
| `ListTimeEntries(ctx, memberID, from, to)` | The user's entries whose date is within `[from, to]`, both inclusive local-midnight dates (the UI asks for Monday–Sunday of the shown week). Return them in display order. |
| `CreateTimeEntry(ctx, e)` | Create `e` (`ID` is empty) and return it as stored. |
| `UpdateTimeEntry(ctx, e)` | Replace the entry with `e.ID` by `e` (every field is set, not a patch) and return it as stored. |
| `DeleteTimeEntry(ctx, e)` | Delete it. You get the whole entry in case the API needs more than the ID. |

`provider.TimeEntry` field notes:

- IDs are strings everywhere; convert to/from integer wire IDs at the
  boundary (`strconv`) and return an error for an unparsable one.
- `Start`/`End` are local `time.Time`s on the same day; `Duration()` is
  `End-Start`. If the backend only stores a duration, anchor `Start` at the
  day's midnight and set `End = Start + duration` (see `weekbooking`).
  If it has running timers, report them with `End = Start` rather than a zero
  `End`.
- `ProjectName` is shown in the list as-is; fill it from the response.
- `Locked` entries are shown with a 🔒 and can't be edited or deleted from
  the UI; set it for anything the backend would refuse to change (locked
  periods, closed weeks).
- `TagIDs` are the IDs from `ListTags`.
- `MemberID` is usually redundant: if the API scopes everything to the token's
  user (e.g. `/me/...` routes), ignore the parameter.

Errors are shown to the user verbatim (error screen, with `r` to retry), so
include the method and path and the backend's own message, as `APIError`
does in the existing providers.

### 4. Optional: check-in / check-out

To support attendance, also implement `provider.Attendance`:

| Method | What the UI expects |
|---|---|
| `AttendanceEnabled()` | Return `cfg.Attendance` from `New`. The feature is only shown when this is true. |
| `ListWorkLocations(ctx)` | Active locations, in display order. |
| `LastWorkLocation(ctx)` | ID of the most recently used location, or `""`. Used to preselect it. |
| `AttendanceStatus(ctx)` | The open session, or `nil, nil` when not checked in. |
| `CheckIn(ctx, workLocationID, note)` | Open a session and return it. |
| `CheckOut(ctx, session, note)` | Close it and return it with `CheckOut` set. Empty `note` means keep the existing one. |

Add `var _ provider.Attendance = (*Client)(nil)` so a signature drift fails
the build. `main.go` rejects `attendance: true` on any provider that doesn't
implement the interface.

### 5. Trying it out

```sh
go build -tags gtk3 ./... && go vet -tags gtk3 ./...
go test -tags gtk3 ./...
go run .
```

The `gtk3` tag is only needed on Linux, because `./...` includes the desktop
app (see [Building it](#building-it)).

Add a `providers.<name>` entry with `type: <your type>` to your
`config.yaml` and press `p` in the app to switch to it. Pure helpers
(date/ID conversion, DTO mapping) are the easiest things to unit-test,
since they need no server.
