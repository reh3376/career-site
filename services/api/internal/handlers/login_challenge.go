package handlers

import (
	"sync"
	"time"
)

// challengeAfterFailures is how many failed sign-ins from one address
// are allowed before a Turnstile token is required.
//
// Two, so an ordinary mistyped password never meets a challenge and a
// third wrong guess does.
const challengeAfterFailures = 2

// failureWindow is how long a failure counts toward that total.
const failureWindow = 15 * time.Minute

// failureCounter tracks recent failed sign-ins per client address.
//
// **Why this exists, and why Login is not simply always challenged.**
// Turnstile went on to sign-in always-on on 2026-10-13, and the deploy's
// own health check failed on the next rollout: live-check.sh signs in
// with the admin password over ssh to exercise the admin RPCs, and a
// server has no browser to solve a challenge with. The same was true of
// exercise-admin.sh and auth-enforce-checks.sh. Those three are how
// production is verified, so losing them to protect sign-in was a bad
// trade made without noticing.
//
// Challenging after repeated failures keeps the control where it earns
// its keep, which is the shape the proto comment described before any of
// it was built. It costs browsers nothing: the widget is on the page
// either way and has already solved by the time anyone presses the
// button, so a member sees no difference.
//
// **Keyed on the address alone, not on (address, email).** The rate
// limiter next to this one keys on both, so one person mistyping their
// own password cannot lock out everyone behind the same NAT. This
// counter is looking for a different shape: many addresses tried from
// one source, which is what credential stuffing is, and keying it on
// the email would hand an attacker a fresh allowance per guess.
//
// In-process, like the limiter beside it. A second api container would
// have its own count; see docs/backlog.md on both.
type failureCounter struct {
	mu   sync.Mutex
	seen map[string]failureEntry
}

type failureEntry struct {
	n  int
	at time.Time
}

func newFailureCounter() *failureCounter {
	return &failureCounter{seen: map[string]failureEntry{}}
}

// count returns recent failures for the key without recording one.
func (f *failureCounter) count(key string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.seen[key]
	if !ok || time.Since(e.at) > failureWindow {
		delete(f.seen, key)
		return 0
	}
	return e.n
}

// record adds a failure and restarts its window.
func (f *failureCounter) record(key string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e := f.seen[key]
	if time.Since(e.at) > failureWindow {
		e.n = 0
	}
	e.n++
	e.at = time.Now()
	f.seen[key] = e

	// Cheap sweep, since this map is only ever touched on a failure and
	// a successful sign-in clears its own key. Without it a long-running
	// process accumulates one entry per attacking address forever.
	if len(f.seen) > 1024 {
		for k, v := range f.seen {
			if time.Since(v.at) > failureWindow {
				delete(f.seen, k)
			}
		}
	}
}

// clear forgets a key, called when a sign-in succeeds.
func (f *failureCounter) clear(key string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.seen, key)
}
