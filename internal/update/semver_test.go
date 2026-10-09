package update

import "testing"

func TestIsNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"v0.0.2", "v0.0.1", true},
		{"v0.1.0", "v0.0.9", true},
		{"v1.0.0", "v0.9.9", true},
		{"v0.0.1", "v0.0.1", false},
		{"v0.0.1", "v0.0.2", false},
		{"v0.0.2", "dev", false},
		{"not-a-version", "v0.0.1", false},
		{"v0.0.2", "not-a-version", false},
	}
	for _, c := range cases {
		if got := isNewer(c.latest, c.current); got != c.want {
			t.Errorf("isNewer(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}

func TestFindAsset(t *testing.T) {
	assets := []asset{
		{Name: "lazytrackit_v0.0.2_linux_amd64.tar.gz"},
		{Name: "lazytrackit_v0.0.2_linux_amd64.tar.gz.sha256"},
		{Name: "lazytrackit_v0.0.2_windows_amd64.zip"},
	}

	a, ok := findAsset(assets, "_linux_amd64.tar.gz")
	if !ok || a.Name != "lazytrackit_v0.0.2_linux_amd64.tar.gz" {
		t.Errorf("findAsset linux_amd64.tar.gz = %+v, %v", a, ok)
	}

	if _, ok := findAsset(assets, "_darwin_amd64.tar.gz"); ok {
		t.Error("findAsset should not match a platform with no published asset")
	}
}
