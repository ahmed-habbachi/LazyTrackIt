package trackit

import (
	"context"
	"fmt"
	"strconv"

	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

// memberDto mirrors the subset of MemberDto we use. As of 4.0.0 the server's
// mapper deliberately leaves userName unset, so Username usually ends up
// empty; it's only DisplayName's last-resort fallback after FullName/Email.
type memberDto struct {
	ID       int32  `json:"id"`
	FullName string `json:"fullName"`
	UserID   string `json:"userId"`
	UserName string `json:"userName"`
	Email    string `json:"email"`
}

// memberDetailsVm is the response of GET /api/Members/me (TrackIt >= 4.0.0),
// which looks the member up by the bearer token's own userId server-side.
// Unlike /api/Members/ByUsername/{username}, it can't return someone else's
// record, and it replaces the old /api/UserSettings/me + /api/Profile/{id}
// pair (UserSettings was removed in 4.0.0) with a single call.
type memberDetailsVm struct {
	Member *memberDto `json:"member"`
}

// Me resolves the authenticated user's TrackIt member record.
func (c *Client) Me(ctx context.Context) (provider.Member, error) {
	var vm memberDetailsVm
	if err := c.doJSON(ctx, "GET", "/api/Members/me", nil, &vm); err != nil {
		return provider.Member{}, err
	}
	if vm.Member == nil {
		return provider.Member{}, fmt.Errorf("GET /api/Members/me: response has no member")
	}

	m := provider.Member{
		ID:       strconv.Itoa(int(vm.Member.ID)),
		UserID:   vm.Member.UserID,
		FullName: vm.Member.FullName,
		Username: vm.Member.UserName,
		Email:    vm.Member.Email,
	}
	c.mu.Lock()
	c.me = &m
	c.mu.Unlock()
	return m, nil
}
