package trackit

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

// projectTagDto mirrors ProjectTagVM. The spec types tagId as a string, but
// per the project's established wire-format quirks (color fields lying
// about their type too), it's decoded loosely rather than trusted outright.
type projectTagDto struct {
	TagID    looseString `json:"tagId"`
	TagName  string      `json:"tagName"`
	TagColor looseString `json:"tagColor"`
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
// tags plus every system tag (which applies across all projects). An empty
// projectID (no project selected/available yet) skips the project-specific
// lookup but still returns system tags.
func (c *Client) ListTags(ctx context.Context, projectID string) ([]provider.Tag, error) {
	projectTags, err := c.listProjectTags(ctx, projectID)
	if err != nil {
		return nil, err
	}
	systemTags, err := c.listSystemTags(ctx)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool, len(projectTags))
	out := make([]provider.Tag, 0, len(projectTags)+len(systemTags))
	for _, t := range projectTags {
		seen[t.ID] = true
		out = append(out, t)
	}
	for _, t := range systemTags {
		if seen[t.ID] {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

func (c *Client) listProjectTags(ctx context.Context, projectID string) ([]provider.Tag, error) {
	if projectID == "" {
		return nil, nil
	}
	path := fmt.Sprintf("/api/Tags/GetByProject/%s", url.PathEscape(projectID))
	var vm projectTagListVm
	if err := c.doJSON(ctx, "GET", path, nil, &vm); err != nil {
		// TrackIt answers a project with no tags defined with a 500 (not a
		// 404), worded as "... not found" — that's an empty result, not a
		// real failure, so don't surface it as one.
		var apiErr *APIError
		if errors.As(err, &apiErr) && strings.Contains(strings.ToLower(apiErr.Error()), "not found") {
			return nil, nil
		}
		return nil, err
	}
	out := make([]provider.Tag, 0, len(vm.ProjectTags))
	for _, t := range vm.ProjectTags {
		out = append(out, provider.Tag{
			ID:    string(t.TagID),
			Name:  t.TagName,
			Color: string(t.TagColor),
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
