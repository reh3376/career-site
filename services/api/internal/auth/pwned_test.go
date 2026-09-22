package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// "password" has SHA-1 5BAA61E4C9B93F3F0682250B6CF8331B7EE68FD8; HIBP
// answers a range query for prefix 5BAA6 with 35-character suffixes.
func TestHIBPMatchesSHA1Suffix(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Any prefix gets the same page; only the 5BAA6 page contains the
		// "password" suffix, so the clean passphrase below must not match.
		if strings.HasSuffix(r.URL.Path, "/5BAA6") {
			_, _ = w.Write([]byte("0018A45C4D1DEF81644B54AB7F969B88D65:1\r\n1E4C9B93F3F0682250B6CF8331B7EE68FD8:3861493\r\n00D4F6E8FA6EECAD2A3AA415EEC418D38EC:2\r\n"))
			return
		}
		_, _ = w.Write([]byte("0018A45C4D1DEF81644B54AB7F969B88D65:1\r\n"))
	}))
	defer srv.Close()
	c := &HIBPChecker{Client: srv.Client(), Endpoint: srv.URL + "/range/"}

	breached, err := c.IsBreached(context.Background(), "password")
	if err != nil {
		t.Fatal(err)
	}
	if !breached {
		t.Fatal("expected \"password\" to be reported as breached")
	}
	clean, err := c.IsBreached(context.Background(), "a-very-specific-passphrase-9f3a")
	if err != nil {
		t.Fatal(err)
	}
	if clean {
		t.Fatal("expected an unlisted password to be reported clean")
	}
}
