package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/gen/career/v1/careerv1connect"
	"github.com/reh3376/career-site/services/api/internal/config"
	"github.com/reh3376/career-site/services/api/internal/db"
	"github.com/reh3376/career-site/services/api/internal/sidecar"
)

// newTestServer builds a server with nil dependencies. Callers that need live
// dependencies should stand them up separately; readyz treats nil as
// "not ready" so every negative path is well-defined.
func newTestServer(t *testing.T, sc *sidecar.Client, pool *db.Pool) http.Handler {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(config.Config{Addr: ":0"}, log, Deps{Sidecar: sc, DB: pool}).routes()
}

func TestHealthz(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)
	newTestServer(t, nil, nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body healthPayload
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "ok" {
		t.Fatalf("status = %q, want %q", body.Status, "ok")
	}
}

func TestReadyzUnreadyWhenSidecarMissing(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/readyz", nil)
	newTestServer(t, nil, nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	var body healthPayload
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "unready" {
		t.Fatalf("status = %q, want %q", body.Status, "unready")
	}
	if body.Checks["sidecar"] {
		t.Fatalf("checks.sidecar = true, want false")
	}
}

func TestSystemGetVersion(t *testing.T) {
	ts := httptest.NewServer(newTestServer(t, nil, nil))
	defer ts.Close()

	client := careerv1connect.NewSystemServiceClient(ts.Client(), ts.URL+"/api")
	resp, err := client.GetVersion(t.Context(), connect.NewRequest(&v1.GetVersionRequest{}))
	if err != nil {
		t.Fatalf("GetVersion: %v", err)
	}
	if resp.Msg.Version == "" {
		t.Fatalf("version is empty")
	}
	if !strings.HasPrefix(resp.Msg.GoVersion, "go") {
		t.Fatalf("go_version = %q, want a go... string", resp.Msg.GoVersion)
	}
}
