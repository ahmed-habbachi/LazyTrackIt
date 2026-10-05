package weekbooking

import (
	"context"

	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

// ListTags always returns an empty list: the week-booking API has no notion
// of tags on an entry, just project_id, hours, and a free-text note.
func (c *Client) ListTags(ctx context.Context, projectID string) ([]provider.Tag, error) {
	return nil, nil
}
