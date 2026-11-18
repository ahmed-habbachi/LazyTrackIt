# LazyTrackIt

Log your work time and check in or out, without opening a browser.

LazyTrackIt comes as two apps that share the same settings and logins:

- **The desktop app** (`lazytrackit-gui`): an icon in the system tray and a
  window showing your week. Best for most people.
- **The terminal app** (`lazytrackit`): the same thing as a keyboard-driven
  terminal UI.

Both work with **Case.TrackIt** (sign in through your browser), **Toggl
Track** and **Zeiterfassung / week booking** (API token). You can use the
two apps side by side.

Downloads are on the [Releases page](../../releases).

**Contents**

- [Desktop app](#desktop-app)
  - [Install it](#install-the-desktop-app)
  - [First start](#first-start)
  - [Using it](#using-the-desktop-app)
  - [Updates](#desktop-app-updates)
  - [Troubleshooting](#desktop-app-troubleshooting)
- [Terminal app](#terminal-app)
- [Configuration](#configuration)
- [Verifying a download](#verifying-a-download)
- [Building from source](#building-from-source)
- [Developer notes](#developer-notes)

---

## Desktop app

### Install the desktop app

Pick the file for your system from the latest release.

#### Windows

1. Download `lazytrackit-setup_<version>_windows_amd64.exe` and run it.
2. The installer isn't code-signed, so Windows SmartScreen may warn about
   an unknown publisher: click **More info** → **Run anyway**.
3. Follow the steps. No admin rights are needed: it installs for your user
   only, in `%LOCALAPPDATA%\Programs\LazyTrackIt\`. It adds a Start Menu
   shortcut (and a desktop one if you tick it) and offers to start the app
   at the end.

To uninstall, use **Settings → Apps → Installed apps**. Running a newer
installer upgrades in place.

The app needs the WebView2 runtime, which comes with Windows 11 and current
Windows 10.

#### Ubuntu / Debian

For Ubuntu 24.04 or newer (and Debian 13 or newer):

1. Download `lazytrackit-desktop_<version>_amd64.deb`.
2. Install it from a terminal in your download folder:

   ```sh
   sudo apt install ./lazytrackit-desktop_*.deb
   ```

   Keep the `./`: it tells apt to install the file rather than look for a
   package by that name. apt installs whatever the app needs along with it
   (WebKitGTK, GTK).
3. Start **LazyTrackIt** from your app menu.

To uninstall: `sudo apt remove lazytrackit-desktop`.

#### Other Linux distributions

1. Download `lazytrackit-gui_<version>_linux_amd64_desktop.tar.gz`.
2. Install the libraries it needs, GTK 3 and WebKitGTK 4.1:

   ```sh
   sudo apt install libgtk-3-0 libwebkit2gtk-4.1-0   # Debian/Ubuntu
   sudo dnf install gtk3 webkit2gtk4.1               # Fedora
   sudo pacman -S gtk3 webkit2gtk-4.1                # Arch
   ```

3. Extract it into `~/.local/bin` and start it:

   ```sh
   mkdir -p ~/.local/bin
   tar -xzf lazytrackit-gui_*_linux_amd64_desktop.tar.gz -C ~/.local/bin
   ~/.local/bin/lazytrackit-gui
   ```

On its first start it adds **LazyTrackIt** to your app menu, so from then
on you can start it from there.

Keep it in a folder you own, such as `~/.local/bin`, not `/usr/local/bin`:
the app updates itself by replacing its own file, which it can't do in a
folder only root can write to.

### First start

1. Start the app.
2. **If your organization's build already has your TrackIt server set up**,
   the app goes straight to sign-in: a window shows a short code and your
   browser opens. Approve the sign-in there, and your week appears.
3. **Otherwise** a welcome message appears. Click **Open settings**, then
   either edit the **trackit** provider (pencil icon) with your server's
   details, or click **Add a provider** to set up Toggl Track or week booking
   instead. See [Configuration](#configuration) for what each field means.
   As soon as you save, the app connects and signs you in.

You stay signed in between starts, until your server's login expires.

### Using the desktop app

- **Your week**: the window shows this week's entries, day by day, with the
  total you've logged against what's expected. Use the arrows at the top to
  change week and **This week** to come back.
- **Log time**: click **+** on a day (or **New entry**), pick the project,
  set the times and a description, and save. Click an entry to edit or
  delete it. Locked entries (🔒) can't be changed.
- **Check in / check out**: when attendance is turned on for your provider
  (see [Check-in / check-out](#check-in--check-out)), the sidebar shows
  whether you're checked in, with a **Check in** / **Check out** button.
  Checking in asks where you work today (Office, Home…).
- **Tray icon**: left-click shows or hides the window. Its menu has check in
  / check out, **Restart to update** when an update is ready, and **Quit**.
  Closing the window only hides it; the app keeps running in the tray until
  you choose **Quit**.
- **Settings** (bottom of the sidebar): switch between providers, add, edit
  or remove them, turn on **Start at login**, and see the app's and the
  server's versions.
- **Start at login**: turn it on in Settings. The app then starts in the
  tray, without opening its window, when you log in.
- **Check-in reminder**: with attendance on, the app reminds you with a
  notification on weekdays if you haven't checked in by 09:30, then every
  30 minutes until noon, and 2 minutes after you start or wake your
  computer (between 07:00 and 17:00). Once it has seen you checked in that
  day, it stays quiet. Click the notification (or its **Check in** button)
  to check in. To change the time or turn it off, see
  [Other settings](#other-settings).
- **Only one copy runs**: starting the app again just brings its window to
  the front.

Keyboard shortcuts:

| Key | Action |
|---|---|
| `n` | New entry |
| `←` / `→` | Previous / next week |
| `t` | This week |
| `r` | Refresh |
| `c` | Check in / out |
| `Esc` | Close a dialog |
| `Ctrl+Enter` | Save the entry form |

### Desktop app updates

- **Windows installer and the Linux `.tar.gz`**: the app checks for a new
  release when it starts and about once a day after that, and installs it
  in the background. When it's ready, the window shows **Restart now** and
  the tray menu shows **Restart to update**.
- **Ubuntu / Debian package**: the app can't update a package itself.
  When a new version is out, the window says so and shows a **Download**
  button. Install the downloaded file the same way as the first time
  (`sudo apt install ./lazytrackit-desktop_*.deb`); the app then offers
  **Restart now**.

To turn update checks off, see [Other settings](#other-settings).

### Desktop app troubleshooting

**`error while loading shared libraries: libwebkit2gtk-4.1.so.0`** (Linux):
WebKitGTK isn't installed. On Ubuntu or Debian, install the `.deb` package
instead of the `.tar.gz` (it installs it for you), or install the
libraries listed under [Other Linux distributions](#other-linux-distributions).

**The app doesn't start on an older Linux** (for example Ubuntu 22.04): the
release is built on Ubuntu 24.04 and may need newer system libraries than
your distribution has. [Build it yourself](#desktop-app-1) instead.

**No tray icon on GNOME** (Ubuntu's default desktop shows it; plain GNOME
doesn't): install the
[AppIndicator extension](https://extensions.gnome.org/extension/615/appindicator-support/),
or use the window and the app menu instead.

**The app menu entry disappeared** (`.tar.gz` and Windows zip installs): the
app adds it only once, so deleting it sticks. To get it back, delete the
file `launcher-created` from `~/.local/state/lazytrackit` (Linux) or
`%LOCALAPPDATA%\lazytrackit` (Windows) and start the app again.

**Something about updates looks wrong**: every update check is logged to
`update.log` in the config folder (see [Configuration](#configuration)).

---

## Terminal app

The terminal app does the same things as the desktop app, in a terminal.

### Install the terminal app

#### Linux

1. Download `lazytrackit_<version>_linux_<arch>.tar.gz` (`amd64` for most
   PCs, `arm64` for ARM machines).
2. Extract it into `~/.local/bin`:

   ```sh
   mkdir -p ~/.local/bin
   tar -xzf lazytrackit_*_linux_*.tar.gz -C ~/.local/bin
   ```

3. Run `lazytrackit`. If your shell can't find it, `~/.local/bin` isn't on
   your `PATH`: run `~/.local/bin/lazytrackit` or add that folder to your
   `PATH`.

As with the desktop app, keep it in a folder you own so it can update
itself.

#### Windows

1. Download `lazytrackit_<version>_windows_amd64.zip`, right-click it →
   **Extract All**.
2. Move `lazytrackit.exe` to `%LOCALAPPDATA%\Programs\lazytrackit\` (a
   folder you own, so it can update itself).
3. Run it from **Windows Terminal** or PowerShell. The old Command Prompt
   window doesn't draw its icons and borders correctly. SmartScreen may
   warn about an unknown publisher: click **More info** → **Run anyway**.

### First run

1. Run `lazytrackit`.
2. If you don't have a config yet, it writes a starter one, prints where,
   and exits. If your organization's build has your TrackIt server set up,
   just run it again. Otherwise, edit that file first (see
   [Configuration](#configuration)), or run it again and add a provider
   with `p` → **+ Add a provider…**.
3. For TrackIt, it shows a URL and a short code on first sign-in and opens
   your browser. Approve the sign-in there.

If you already use the desktop app, there's nothing to set up: both apps
share the same config and logins.

### Using the terminal app

| Key | Action |
|---|---|
| `n` | New entry |
| `e` / `Enter` | Edit the selected entry |
| `x` | Delete the selected entry (asks first) |
| `r` | Refresh |
| `[` / `]` | Previous / next week |
| `t` | Back to this week |
| `c` | Check in / out (when attendance is on) |
| `p` | Providers: switch, add (`a`), edit (`e`), remove (`d`) |
| `q` / `Ctrl+C` | Quit |

In the entry form: `Tab` / `Shift+Tab` (or `↑` / `↓`) move between fields,
`←` / `→` change the project, `Enter` saves, `Esc` cancels.

In the check-in form, pick a work location with `↑` / `↓` (it starts on the
one you used last), add an optional note, and press `Enter`. The check-out
form takes an optional note; leave it empty to keep the one from check-in.

### Terminal app updates

On start, the terminal app looks for a new release and, if there is one,
installs it in the background. You keep working; the new version runs the
next time you start it, and a line at the bottom of the list says when it's
ready. If it can't update (offline, or installed in a folder you can't
write to), it keeps working with the version you have; the folder problem
is the only one it tells you about.

When you double-click `lazytrackit.exe` on Windows instead of running it
from a terminal, messages it prints before exiting (such as where it wrote
the starter config) wait for you to press Enter, so they don't vanish.

---

## Configuration

Both apps read one file, `config.yaml`:

| System | Path |
|---|---|
| Linux | `~/.config/lazytrackit/config.yaml` (or `$XDG_CONFIG_HOME/lazytrackit/config.yaml`) |
| Windows | `%APPDATA%\lazytrackit\config.yaml` |

The apps create it on first start. You rarely need to edit it by hand:
providers can be added, edited and removed from the desktop app's
**Settings** or the terminal app's `p` menu. Either way, the apps only
change the lines of the provider involved, and keep your comments and
layout.

Logins and per-machine state (which provider you used last, your last
project and tags) are stored separately, under `~/.local/state/lazytrackit/`
on Linux and `%LOCALAPPDATA%\lazytrackit\` on Windows.

### Case.TrackIt

```yaml
active_provider: trackit
providers:
  trackit:
    type: trackit
    base_url: https://trackit.yourcompany.com
    auth:
      issuer: https://auth.yourcompany.com/realms/yourrealm
      client_id: lazytrackit
      client_secret: ""   # only if the client is confidential
      scopes: [openid, profile, offline_access]
    attendance: true      # check-in / check-out, TrackIt 4.0.0+
```

- `base_url`: your TrackIt API address, with no trailing `/`.
- `issuer`: the Keycloak (or other OIDC) realm used to sign in.
- `client_id`: a Keycloak client with the **OAuth 2.0 Device Authorization
  Grant** turned on (Keycloak: the client's *Capability config*).

Sign-in happens in your browser. You stay signed in until the server's
refresh token expires.

### Toggl Track

```yaml
providers:
  toggl:
    type: toggl
    auth:
      api_token: <your Toggl API token>
```

Get the token from [track.toggl.com/profile](https://track.toggl.com/profile).
`base_url` is optional (default `https://api.track.toggl.com`). There's no
sign-in step: the token is the login, until you regenerate it.

### Zeiterfassung / week booking

```yaml
providers:
  weekbooking:
    type: weekbooking
    base_url: https://zeiterfassung.example.com
    auth:
      api_token: <your week-booking API token>
```

Mint an API token on the server (`POST /api/v1/me/api-tokens`, or whatever
page your deployment offers). This backend books hours per day rather than
start and end times, has no tags, and can close a week: entries in a closed
week show as locked.

### Several providers

You can configure as many providers as you like, for example two TrackIt
accounts, or a test and a production server. Switch between them in the
desktop app's Settings or with `p` in the terminal app. The app remembers
which one you used last on this machine. That choice wins over
`active_provider`, which only sets the default for a fresh install.

A new TrackIt provider starts with the sign-in settings of one you already
have, since another account or environment usually shares them. Each
provider keeps its own login and its own last-used project and tags, so
switching back and forth doesn't sign you out.

A provider's name can't be changed, since its login and remembered project
are stored under it. Changing a TrackIt provider's issuer or client ID signs
you out of it. Removing a provider also deletes its saved login. You can't
remove your last provider, and removing the active one switches to another.

### Check-in / check-out

TrackIt 4.0.0 and later also record attendance: checking in and out of a
work location such as Office or Home. Turn it on per provider with
`attendance: true` (or the **Check-in / check-out** switch in the provider
form). Toggl and week booking don't support it.

### Other settings

These go at the top level of `config.yaml`:

```yaml
# Time of the desktop app's check-in reminder ("HH:MM"), or "off".
check_in_reminder: "08:45"

# Don't check for new releases.
check_for_updates: false
```

---

## Verifying a download

Each release file has a matching `.sha256` file. To check a download:

```sh
sha256sum -c lazytrackit-desktop_<version>_amd64.deb.sha256     # Linux
CertUtil -hashfile lazytrackit-setup_<version>_windows_amd64.exe SHA256   # Windows
```

On Windows, compare the printed hash with the one in the `.sha256` file.

---

## Building from source

Releases are built by `.github/workflows/release.yml` when a `vX.Y.Z` tag is
pushed. To build your own:

### Terminal app

```sh
go build -o lazytrackit .
GOOS=windows GOARCH=amd64 go build -o lazytrackit.exe .   # from Linux
```

It's pure Go, so it cross-compiles with no extra tools. Builds without a
version (plain `go build` / `go run .`) never update themselves.

### Desktop app

```sh
go build -tags gtk3 -o lazytrackit-gui ./cmd/lazytrackit-gui
```

The desktop app is built with [Wails v3](https://v3.wails.io). Its
frontend is plain HTML/CSS/JS in `internal/gui/frontend`, embedded into the
binary; there's no Node or npm step.

- **Linux** needs cgo and the development packages (Debian/Ubuntu:
  `sudo apt install build-essential libgtk-3-dev libwebkit2gtk-4.1-dev`;
  Arch: `sudo pacman -S base-devel gtk3 webkit2gtk-4.1`). Keep `-tags gtk3`:
  without it Wails builds against GTK 4 / WebKitGTK 6.0 and fails with
  `Package 'webkitgtk-6.0' not found`. The tag does nothing on Windows.
- **Windows** needs no C compiler, so it cross-compiles. Add
  `-H windowsgui` so no console window opens:
  `GOOS=windows go build -ldflags "-H windowsgui" -o lazytrackit-gui.exe ./cmd/lazytrackit-gui`.
  The exe's icon, manifest and version details come from
  `cmd/lazytrackit-gui/rsrc_windows_amd64.syso`. To change them, edit
  `packaging/windows/` and run `packaging/windows/gen-syso.sh` (needs
  `llvm-windres` or `windres`).
- **Windows installer**: `packaging/windows/lazytrackit.iss` is an
  [Inno Setup 6](https://jrsoftware.org/isinfo.php) script:
  `iscc /DAppVersion=v1.2.3 /DNumVersion=1.2.3 /DBinDir=<folder with lazytrackit-gui.exe> packaging\windows\lazytrackit.iss`.
- **Debian package**: on the Ubuntu/Debian version you're targeting, with
  `dpkg-dev` installed, run
  `packaging/linux/build-deb.sh v1.2.3 ./lazytrackit-gui dist`. It works
  out the package's dependencies from the binary.

### Presetting the starter config

Builds can come with a TrackIt server already filled in, so a fresh install
works without any setup (see `internal/config/preset.go`). The release
workflow does this from the repository variables `PRESET_TRACKIT_BASE_URL`,
`PRESET_TRACKIT_ISSUER`, `PRESET_TRACKIT_CLIENT_ID` and
`PRESET_TRACKIT_ATTENDANCE`. By hand:

```sh
pkg=github.com/ahmed-habbachi/lazytrackit/internal/config
go build -ldflags "-X $pkg.PresetBaseURL=https://trackit.example.com -X $pkg.PresetIssuer=https://auth.example.com/realms/example -X $pkg.PresetAttendance=true" .
```

This only affects new installs. It holds no secrets (TrackIt sign-in uses
a public client), but the values can be read from the binary.

---

## Developer notes

### Running the checks

```sh
go build -tags gtk3 ./... && go vet -tags gtk3 ./...
go test -tags gtk3 ./...
go run .
```

The `gtk3` tag is only needed on Linux, because `./...` includes the
desktop app.

### TrackIt wire-format notes

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

### Developing a provider

Both apps go through `internal/service`, which only ever talks to the
`provider.Provider` interface in `internal/provider/provider.go`. A backend
is a Go package implementing it, plus its registration in
`internal/service`. The three built-in providers are good templates for the
two auth styles:

| Provider | Auth | Notable differences |
|---|---|---|
| `internal/provider/trackit` | OIDC device-code flow (`internal/auth`), cached + refreshed tokens | project-scoped + system tags, locked entries, attendance |
| `internal/provider/toggl` | static API token (HTTP Basic) | workspace-scoped, no locking, tags not tied to projects |
| `internal/provider/weekbooking` | static API token (Bearer) | no start/end times (hours per ISO weekday), no tags, closed weeks |

#### 1. Package layout

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

#### 2. Constructor and config

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
- `New` must not do network I/O; the apps call the provider later from
  background goroutines.
- `name` is the config key (e.g. `trackit2`), not the type. Use it as the
  key for anything persisted per account (`auth.SaveTokens(name, …)`), since
  several accounts of the same type can be configured side by side.

Register the type in `newProvider` in `internal/service/providers.go`:

```go
case "mybackend":
	return mybackend.New(name, pcfg)
```

To make it addable from the apps' provider forms, add it to
`ProviderTypes` in `internal/service/providerconfig.go` (and map its fields
in `ProviderDraft`). Also add a commented example to `exampleConfig` in
`internal/config/config.go` and a section under
[Configuration](#configuration) in this README.

#### 3. The `Provider` contract

Every method gets a `context.Context` with a timeout (20s, 15min for
`Login`); pass it to every HTTP request. Methods are called from goroutines,
so guard any cached state (tokens, the current member) with a mutex.

| Method | What the apps expect |
|---|---|
| `Name()` | The config key passed to `New`. Used to key per-provider state (last-used project/tags). |
| `IsAuthenticated()` | Whether credentials are already available **without a network call**: a cached token file, or a configured API key. If false, the app calls `Login` first. |
| `Login(ctx, onPrompt)` | Obtain and persist credentials. Call `onPrompt(provider.LoginPrompt{Message, URL, Code, Expires})` only if the user has to do something out-of-band (the app shows it and opens `URL` in a browser), then block until done or `ctx` expires. Static-token providers never call `onPrompt`; they just validate the token (e.g. by calling `Me`). |
| `Me(ctx)` | The authenticated user. `ID` is what the app later puts in `TimeEntry.MemberID`; `UserID` is any second backend identifier you need. `FullName`/`Email`/`Username` are display-only (`DisplayName()` falls back in that order) and may be empty. Cache the result if other calls need it. |
| `ListProjects(ctx)` | Projects the user can book on. The apps offer every project returned (they don't filter on `Active`), so leave out ones that can't take time. `ClientName`/`Color` may be empty. |
| `ListTags(ctx, projectID)` | Tags offered in the entry form for that project. `projectID` may be `""` (no project yet). Set `IsSystem` for tags valid on every project. Backends without tags return an empty slice, not an error. |
| `ListTimeEntries(ctx, memberID, from, to)` | The user's entries whose date is within `[from, to]`, both inclusive local-midnight dates (the apps ask for Monday–Sunday of the shown week). Return them in display order. |
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
  the apps; set it for anything the backend would refuse to change (locked
  periods, closed weeks).
- `TagIDs` are the IDs from `ListTags`.
- `MemberID` is usually redundant: if the API scopes everything to the token's
  user (e.g. `/me/...` routes), ignore the parameter.

Errors are shown to the user verbatim, so include the method and path and
the backend's own message, as `APIError` does in the existing providers.

#### 4. Optional: check-in / check-out

To support attendance, also implement `provider.Attendance`:

| Method | What the apps expect |
|---|---|
| `AttendanceEnabled()` | Return `cfg.Attendance` from `New`. The feature is only shown when this is true. |
| `ListWorkLocations(ctx)` | Active locations, in display order. |
| `LastWorkLocation(ctx)` | ID of the most recently used location, or `""`. Used to preselect it. |
| `AttendanceStatus(ctx)` | The open session, or `nil, nil` when not checked in. |
| `CheckIn(ctx, workLocationID, note)` | Open a session and return it. |
| `CheckOut(ctx, session, note)` | Close it and return it with `CheckOut` set. Empty `note` means keep the existing one. |

Add `var _ provider.Attendance = (*Client)(nil)` so a signature drift fails
the build. `internal/service` rejects `attendance: true` on any provider
that doesn't implement the interface.

#### 5. Trying it out

Add a `providers.<name>` entry with `type: <your type>` to your
`config.yaml` and switch to it (`p` in the terminal app, Settings in the
desktop app). Pure helpers (date/ID conversion, DTO mapping) are the
easiest things to unit-test, since they need no server.
