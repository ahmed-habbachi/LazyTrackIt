package trackit

import (
	"context"
	"fmt"
	"net/url"

	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

// projectTagDto mirrors ProjectTagVM. The spec types tagId as a string, but
// per the project's established wire-format quirks (color fields lying
// about their type too), it's decoded loosely rather than trusted outright.
type projectTagDto struct {
	TagID       looseString `json:"tagId"`
	TagName     string      `json:"tagName"`
	TagColor    looseString `json:"tagColor"`
	IsSystemTag bool        `json:"isSystemTag"`
}

type projectTagListVm struct {
	ProjectTags []projectTagDto `json:"projectTags"`
	Count       int32           `json:"count"`
}

// tagDto mirrors TagDto, as returned by the catalogue-wide GET /api/Tags.
// Unlike ProjectTagVM's tagId, the id here is genuinely an int32 on the
// wire, matching TagDetailsVM/TimeEntryTags elsewhere.
type tagDto struct {
	ID       int32       `json:"id"`
	Name     string      `json:"name"`
	Color    looseString `json:"color"`
	IsSystem bool        `json:"isSystem"`
}

type tagListVm struct {
	Tags  []tagDto `json:"tags"`
	Count int32    `json:"count"`
}

// ListTags returns the tags usable on the given project: the project's own
// tags plus every system tag (which applies across all projects). Since
// TrackIt 4.0.0, GET /api/Tags/GetByProject/{id} returns both itself. An
// empty projectID (no project selected/available yet) falls back to just the
// system tags from the catalogue-wide GET /api/Tags.
func (c *Client) ListTags(ctx context.Context, projectID string) ([]provider.Tag, error) {
	if projectID == "" {
		return c.listSystemTags(ctx)
	}
	path := fmt.Sprintf("/api/Tags/GetByProject/%s", url.PathEscape(projectID))
	var vm projectTagListVm
	if err := c.doJSON(ctx, "GET", path, nil, &vm); err != nil {
		return nil, err
	}
	out := make([]provider.Tag, 0, len(vm.ProjectTags))
	for _, t := range vm.ProjectTags {
		out = append(out, provider.Tag{
			ID:       string(t.TagID),
			Name:     t.TagName,
			Color:    string(t.TagColor),
			IsSystem: t.IsSystemTag,
		})
	}
	return out, nil
}

func (c *Client) listSystemTags(ctx context.Context) ([]provider.Tag, error) {
	var vm tagListVm
	if err := c.doJSON(ctx, "GET", "/api/Tags", nil, &vm); err != nil {
		return nil, err
	}
	out := make([]provider.Tag, 0, len(vm.Tags))
	for _, t := range vm.Tags {
		if !t.IsSystem {
			continue
		}
		out = append(out, provider.Tag{
			ID:       idToString(t.ID),
			Name:     t.Name,
			Color:    string(t.Color),
			IsSystem: true,
		})
	}
	return out, nil
}
