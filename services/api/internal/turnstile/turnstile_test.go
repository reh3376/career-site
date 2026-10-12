package turnstile

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New("0xTESTSECRET").WithURL(srv.URL)
}

func TestVerifyPassesOnSuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"success":true}`)
	})
	v, err := c.Verify(context.Background(), "tok", "203.0.113.7")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !v.OK || v.FailedOpen {
		t.Fatalf("got %+v, want OK without FailedOpen", v)
	}
}

func TestVerifyRejectsOnFailure(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"success":false,"error-codes":["invalid-input-response"]}`)
	})
	v, err := c.Verify(context.Background(), "tok", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.OK {
		t.Fatal("a token Cloudflare rejected was admitted")
	}
	if len(v.ErrorCodes) != 1 || v.ErrorCodes[0] != "invalid-input-response" {
		t.Fatalf("error codes not carried through: %+v", v.ErrorCodes)
	}
	if v.InvalidSecret() {
		t.Fatal("a visitor-side rejection was reported as our secret being wrong")
	}
}

// Our secret being wrong and the visitor failing the challenge look
// identical to the user and must not look identical in the logs: one is
// a rollout to fix, the other is the control working.
func TestInvalidSecretIsDistinguishable(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"success":false,"error-codes":["invalid-input-secret"]}`)
	})
	v, _ := c.Verify(context.Background(), "tok", "")
	if !v.InvalidSecret() {
		t.Fatalf("invalid-input-secret not recognised: %+v", v)
	}
}

// An empty token short-circuits: no round trip, and the handler sees the
// same shape of rejection it would get from Cloudflare.
func TestEmptyTokenIsRejectedWithoutCallingOut(t *testing.T) {
	called := false
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		called = true
		_, _ = io.WriteString(w, `{"success":true}`)
	})
	v, err := c.Verify(context.Background(), "   ", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.OK {
		t.Fatal("an empty token was admitted")
	}
	if called {
		t.Fatal("an empty token still cost a siteverify round trip")
	}
}

func TestSendsSecretTokenAndRemoteIP(t *testing.T) {
	var got struct{ secret, response, remoteip, ctype string }
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		got.secret = r.PostForm.Get("secret")
		got.response = r.PostForm.Get("response")
		got.remoteip = r.PostForm.Get("remoteip")
		got.ctype = r.Header.Get("Content-Type")
		_, _ = io.WriteString(w, `{"success":true}`)
	})
	if _, err := c.Verify(context.Background(), "tok-123", "203.0.113.7"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.secret != "0xTESTSECRET" || got.response != "tok-123" || got.remoteip != "203.0.113.7" {
		t.Fatalf("wrong form fields: %+v", got)
	}
	if got.ctype != "application/x-www-form-urlencoded" {
		t.Fatalf("wrong content type %q; Cloudflare rejects anything else", got.ctype)
	}
}

func TestTransportFailuresAreErrorsNotPasses(t *testing.T) {
	cases := []struct {
		name string
		h    http.HandlerFunc
	}{
		{"http 500", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}},
		{"http 502 with a proxy error page", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = io.WriteString(w, "<html>bad gateway</html>")
		}},
		{"200 with a body that is not json", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "<html>captive portal</html>")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, tc.h)
			v, err := c.Verify(context.Background(), "tok", "")
			if err == nil {
				t.Fatal("a broken response produced no error; FailOpen would have nothing to log")
			}
			if v.OK {
				t.Fatal("the bare client admitted a request it never got an answer for; " +
					"that decision belongs to FailOpen, not here")
			}
		})
	}
}

// The reason the timeout is short: FailOpen admits the request anyway, so
// a long timeout only makes the form slower for the same outcome.
func TestClientHasABoundedTimeout(t *testing.T) {
	c := New("0xTESTSECRET")
	if c.http.Timeout <= 0 || c.http.Timeout > 10*time.Second {
		t.Fatalf("timeout is %v; a form submission waits on this", c.http.Timeout)
	}
}

// --- FailOpen ---

type stubVerifier struct {
	v   Verdict
	err error
}

func (s stubVerifier) Verify(context.Context, string, string) (Verdict, error) {
	return s.v, s.err
}

func TestFailOpenAdmitsWhenSiteverifyIsUnreachable(t *testing.T) {
	boom := errors.New("dial tcp: no route to host")
	f := FailOpen{Inner: stubVerifier{v: Verdict{OK: false}, err: boom}}
	v, err := f.Verify(context.Background(), "tok", "")
	if !v.OK {
		t.Fatal("an unreachable Cloudflare blocked the request; the decision was to fail open")
	}
	if !v.FailedOpen {
		t.Fatal("FailedOpen is false, so the caller has no way to log that the check did not run")
	}
	if !errors.Is(err, boom) {
		t.Fatalf("the transport error was swallowed: %v", err)
	}
}

// Fail open covers "we could not ask", not "the answer was no". A token
// Cloudflare actually rejected stays rejected, or the control is theatre.
func TestFailOpenDoesNotRescueARealRejection(t *testing.T) {
	f := FailOpen{Inner: stubVerifier{v: Verdict{OK: false, ErrorCodes: []string{"invalid-input-response"}}}}
	v, err := f.Verify(context.Background(), "tok", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.OK {
		t.Fatal("FailOpen admitted a token Cloudflare rejected")
	}
}

func TestFailOpenPassesSuccessThrough(t *testing.T) {
	f := FailOpen{Inner: stubVerifier{v: Verdict{OK: true}}}
	v, err := f.Verify(context.Background(), "tok", "")
	if err != nil || !v.OK || v.FailedOpen {
		t.Fatalf("got %+v err=%v, want a clean pass", v, err)
	}
}

// A cancelled request is the visitor leaving, not Cloudflare failing.
// Admitting it would be inventing a pass for a request nobody is waiting
// on. This matches auth.FailOpen's handling of the same case.
func TestFailOpenDoesNotAdmitACancelledRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f := FailOpen{Inner: stubVerifier{v: Verdict{OK: false}, err: context.Canceled}}
	v, _ := f.Verify(ctx, "tok", "")
	if v.OK {
		t.Fatal("a cancelled request was admitted")
	}
}

func TestNoopAlwaysPasses(t *testing.T) {
	v, err := Noop{}.Verify(context.Background(), "", "")
	if err != nil || !v.OK {
		t.Fatalf("Noop must pass so dev works without a Cloudflare account: %+v %v", v, err)
	}
}
