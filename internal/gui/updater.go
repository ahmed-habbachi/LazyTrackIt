package gui

import (
	"context"
	"fmt"
	"time"

	"github.com/ahmed-habbachi/lazytrackit/internal/update"
)

// The desktop app usually lives in the tray for days, so it doesn't only
// check for updates when it starts: it checks again about once a day, and
// sooner after a check that couldn't reach GitHub (e.g. it started before
// the network was up).
const (
	updateInterval = 24 * time.Hour
	updateRetry    = time.Hour
	updateTick     = 10 * time.Minute
	updateTimeout  = 2 * time.Minute
)

// updateCheck downloads and installs a newer release if there is one.
type updateCheck func(ctx context.Context) update.CheckResult

// runUpdater checks right away and then whenever the next check is due,
// until an update is installed (one is enough: the next start runs it) or
// self-update turns out to be impossible. Due times use the wall clock, so
// time spent asleep counts towards the day. wait blocks until it's time to
// look again and returns false to stop.
func runUpdater(ctx context.Context, svc *Service, check updateCheck, wait func() bool, now func() time.Time) {
	var next time.Time // zero: due now
	for {
		if !now().Before(next) {
			cctx, cancel := context.WithTimeout(ctx, updateTimeout)
			res := check(cctx)
			cancel()
			switch {
			case res.Installed:
				svc.setUpdateReady(res.NewVersion)
				return
			case res.Available != "":
				// Installed by the package manager: the user installs the
				// newer package, and a later check sees it on disk
				// (res.Installed) and offers the restart.
				svc.setUpdateAvailable(res.Available, res.DownloadURL)
				next = now().Add(updateRetry)
			case res.Unwritable != "":
				svc.setUpdateNotice(fmt.Sprintf("Installed at %s, which you can't write to, so auto-update can't work. Move LazyTrackIt to a folder you own to enable it.", res.Unwritable))
				return
			case res.Failed:
				next = now().Add(updateRetry)
			default:
				next = now().Add(updateInterval)
			}
		}
		if !wait() {
			return
		}
	}
}

// wallClock is time.Now without its monotonic reading, which stops while
// the machine sleeps.
func wallClock() time.Time { return time.Now().Round(0) }
