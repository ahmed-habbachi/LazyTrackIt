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
the extracted `lazytrackit` binary wherever you keep user-installed
binaries:

- `~/.local/bin/lazytrackit` — per-user, no root needed; works out of the
  box if `~/.local/bin` is already on your `PATH` (most distros add it by
  default for login shells):

  ```
  mkdir -p ~/.local/bin && mv lazytrackit ~/.local/bin/
  ```

- `/usr/local/bin/lazytrackit` — system-wide, needs `sudo mv`:

  ```
  sudo mv lazytrackit /usr/local/bin/
  ```

Then run it with `lazytrackit` (if it's on your `PATH`) or the full path.

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
write permission on the install directory, no build for your platform,
etc.) it just stays quiet and you keep using the version you have — nothing
is ever surfaced as an error.

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
- `p` — switch provider (only shown once more than one is configured)
- `q` / `ctrl+c` — quit

In the entry form: `tab`/`shift+tab` (or `↑`/`↓`) move between fields,
`←`/`→` cycle the selected project, `enter` saves, `esc` cancels.

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

## Wire-format notes

The OpenAPI spec documents field *shapes* but not their exact semantics, so
a couple of things were confirmed against a live account rather than just
the spec:

- `timeFrom` / `timeTo` / `timeActual` are **seconds after midnight** (e.g.
  `32400` = 09:00), not minutes as the field names might suggest. LazyTrackIt
  converts to/from minutes-after-midnight at the API boundary in
  `internal/provider/trackit/timeentries.go` (`secondsToMinutes` /
  `minutesToSeconds`) so the rest of the app just deals in minutes.
- `color` fields come back from this deployment as JSON numbers despite the
  spec declaring them strings; `looseString` in
  `internal/provider/trackit/json.go` accepts either.

If something still looks off after creating/editing an entry, these two
files are the places to check first.

## Adding another backend

Implement `provider.Provider` (see `internal/provider/provider.go`) in a new
package, register it in `main.go`'s provider-type switch, and add a
`providers.<name>` entry with a matching `type` to the config.
