package service

import (
	"context"
	"errors"
	"slices"
	"sync"

	"github.com/ahmed-habbachi/lazytrackit/internal/provider"
)

// loginFlight is one in-progress login, shared by every caller that asks to
// authenticate the same provider generation while it runs, so a GUI that
// calls Authenticate from several places never starts parallel device-code
// flows.
type loginFlight struct {
	done chan struct{}
	err  error

	mu     sync.Mutex
	prompt *provider.LoginPrompt
	subs   []func(provider.LoginPrompt)
}

// subscribe registers onPrompt for the flight's prompt, replaying it
// immediately if it has already been shown.
func (f *loginFlight) subscribe(onPrompt func(provider.LoginPrompt)) {
	if onPrompt == nil {
		return
	}
	f.mu.Lock()
	f.subs = append(f.subs, onPrompt)
	p := f.prompt
	f.mu.Unlock()
	if p != nil {
		onPrompt(*p)
	}
}

// publish records p and forwards it to every subscriber.
func (f *loginFlight) publish(p provider.LoginPrompt) {
	f.mu.Lock()
	f.prompt = &p
	subs := slices.Clone(f.subs)
	f.mu.Unlock()
	for _, sub := range subs {
		sub(p)
	}
}

// Authenticate logs in to the provider if it isn't authenticated already.
// onPrompt is called if the user has to do something out-of-band (see
// provider.Provider.Login). Concurrent calls for the same provider share a
// single login. Switching provider cancels it, and it returns ErrStale.
func (sc Scope) Authenticate(ctx context.Context, onPrompt func(provider.LoginPrompt)) error {
	if sc.prov.IsAuthenticated() {
		return nil
	}

	flight, leader, err := sc.joinLogin()
	if err != nil {
		return err
	}
	flight.subscribe(onPrompt)
	if leader {
		sc.runLogin(ctx, flight)
	}

	select {
	case <-flight.done:
		return sc.finish(flight.err)
	case <-ctx.Done():
		return ctx.Err()
	}
}

// joinLogin returns the generation's in-flight login, creating it (and
// reporting leader=true) if there's none.
func (sc Scope) joinLogin() (flight *loginFlight, leader bool, err error) {
	s := sc.s
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.gen != sc.gen {
		return nil, false, ErrStale
	}
	if s.login != nil {
		return s.login, false, nil
	}
	s.login = &loginFlight{done: make(chan struct{})}
	return s.login, true, nil
}

// runLogin performs the login in the background so the leader, like every
// other caller, can stop waiting when its own ctx ends without aborting the
// login others share. It is cancelled only by LoginTimeout or a provider
// switch.
func (sc Scope) runLogin(parent context.Context, flight *loginFlight) {
	ctx, done, err := sc.begin(context.WithoutCancel(parent), LoginTimeout)
	if err != nil {
		flight.err = err
		sc.endLogin(flight)
		return
	}
	go func() {
		defer done()
		flight.err = sc.prov.Login(ctx, flight.publish)
		sc.endLogin(flight)
	}()
}

// endLogin marks flight finished and clears it from the Session, so a later
// call (e.g. after a failure) starts a fresh login.
func (sc Scope) endLogin(flight *loginFlight) {
	s := sc.s
	s.mu.Lock()
	if s.login == flight {
		s.login = nil
	}
	s.mu.Unlock()
	close(flight.done)
}

// Connect gets the provider ready for use: it logs in if there are no saved
// credentials, then loads the profile. If the saved credentials turn out to
// have expired for good (provider.ErrReauthRequired, e.g. the server ended
// the login session), it logs in once more and retries, so an expired login
// leads to a fresh login prompt instead of an error that retrying can't fix.
// It returns the Scope to use from then on, which includes the profile.
func (sc Scope) Connect(ctx context.Context, onPrompt func(provider.LoginPrompt)) (Scope, error) {
	if err := sc.Authenticate(ctx, onPrompt); err != nil {
		return sc, err
	}
	loaded, err := sc.LoadProfile(ctx)
	if !errors.Is(err, provider.ErrReauthRequired) {
		return loaded, err
	}
	if err := sc.Authenticate(ctx, onPrompt); err != nil {
		return sc, err
	}
	return sc.LoadProfile(ctx)
}
