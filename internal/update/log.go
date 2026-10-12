package update

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ahmed-habbachi/lazytrackit/internal/config"
)

// logf appends one timestamped line to update.log in the config directory.
// The update check is otherwise entirely silent (see CheckAndApply), so this
// is the only record of why a check did or didn't result in an install.
// Logging is itself best-effort: a failure to write has nothing useful to
// report, so it's simply dropped.
func logf(format string, args ...any) {
	dir, err := config.Dir()
	if err != nil {
		return
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(dir, "update.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s "+format+"\n", append([]any{time.Now().Format(time.RFC3339)}, args...)...)
}
