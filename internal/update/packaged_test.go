package update

import (
	"runtime"
	"testing"
)

func TestDebVersion(t *testing.T) {
	for in, want := range map[string]string{"v1.2.3": "1.2.3", "1.2.3": "1.2.3", "v1.3.0-rc1": "1.3.0~rc1"} {
		if got := DebVersion(in); got != want {
			t.Errorf("DebVersion(%q) = %q, want %q", in, got, want)
		}
	}
	if got, want := GUI.debName("v1.2.3"), "lazytrackit-desktop_1.2.3_"+runtime.GOARCH+".deb"; got != want {
		t.Errorf("debName = %q, want %q", got, want)
	}
}
