# LazyTrackIt

A terminal UI for time tracking, built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).
Backends are pluggable behind `internal/provider.Provider`; the first (and
currently only) implementation targets the **Case.TrackIt** REST API.

## Setup

1. Build and run once:

   ```
   go run .
   ```

   On first run there's no config yet, so LazyTrackIt writes a starter file
   to `$XDG_CONFIG_HOME/lazytrackit/config.yaml` (or `~/.config/lazytrackit/config.yaml`)
   and exits.

2. Edit that file:

   ```yaml
   active_provider: trackit
   providers:
     trackit:
       type: trackit
       base_url: https://trackit.case-tunisia.com
       auth:
         issuer: https://auth2.case-tunisia.com/realms/case
         client_id: <your Keycloak client id>
         client_secret: ""   # only if the client is confidential
         scopes: [openid, profile, offline_access]
   ```

   The Keycloak client referenced by `client_id` must have the **OAuth 2.0
   Device Authorization Grant** enabled (Keycloak: client's *Capability
   config* → *OAuth 2.0 Device Authorization Grant* on).

3. Run `go run .` again. On first login you'll see a URL and a short code;
   approve it in your browser (LazyTrackIt tries to open it for you). Tokens
   are cached under `~/.local/state/lazytrackit/tokens/`, so you won't need
   to log in again until the refresh token expires.

## Using it

- `n` — new time entry
- `e` / `enter` — edit the selected entry
- `x` — delete the selected entry (asks to confirm)
- `r` — refresh
- `[` / `]` — previous / next week
- `t` — jump back to the current week
- `q` / `ctrl+c` — quit

In the entry form: `tab`/`shift+tab` (or `↑`/`↓`) move between fields,
`←`/`→` cycle the selected project, `enter` saves, `esc` cancels.

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
