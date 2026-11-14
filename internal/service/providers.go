// Package service holds LazyTrackIt's front-end-agnostic application logic:
// building providers from the config, the per-provider session state
// (authenticated member, projects, remembered defaults), and the rules for
// turning user input into a valid time entry. Both the terminal UI and the
// desktop GUI sit on top of it, so neither duplicates business logic.
package service

import (
	"fmt"
	"sort"

	"github.com/ahmed-habbachi/lazytrackit/internal/config"
	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
	"github.com/ahmed-habbachi/lazytrackit/internal/provider/toggl"
	"github.com/ahmed-habbachi/lazytrackit/internal/provider/trackit"
	"github.com/ahmed-habbachi/lazytrackit/internal/provider/weekbooking"
)

// BuildProviders instantiates every provider declared in the config, keyed
// by its name, plus their names in a stable (sorted) display order.
func BuildProviders(cfg *config.Config) (map[string]provider.Provider, []string, error) {
	providers := make(map[string]provider.Provider, len(cfg.Providers))
	names := make([]string, 0, len(cfg.Providers))
	for name, pcfg := range cfg.Providers {
		prov, err := newProvider(name, pcfg)
		if err != nil {
			return nil, nil, err
		}
		if pcfg.Attendance {
			if _, ok := prov.(provider.Attendance); !ok {
				return nil, nil, fmt.Errorf("provider %q has attendance enabled, but type %q doesn't support check-in/check-out", name, pcfg.Type)
			}
		}
		providers[name] = prov
		names = append(names, name)
	}
	sort.Strings(names)
	return providers, names, nil
}

func newProvider(name string, pcfg config.ProviderConfig) (provider.Provider, error) {
	switch pcfg.Type {
	case "trackit", "":
		return trackit.New(name, pcfg)
	case "toggl":
		return toggl.New(name, pcfg)
	case "weekbooking":
		return weekbooking.New(name, pcfg)
	default:
		return nil, fmt.Errorf("unknown provider type %q for provider %q", pcfg.Type, name)
	}
}
