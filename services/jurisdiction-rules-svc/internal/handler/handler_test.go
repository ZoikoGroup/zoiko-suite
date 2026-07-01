package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"zoiko.io/jurisdiction-rules-svc/internal/domain"
	"zoiko.io/jurisdiction-rules-svc/internal/handler"
	"zoiko.io/jurisdiction-rules-svc/internal/store"
)

// ── stub store ────────────────────────────────────────────────────────────────

// stubStore implements handler.JurisdictionStore for unit testing.
// No DB, no network — purely in-memory.
type stubStore struct {
	jurisdiction  *domain.Jurisdiction
	jurisdictions []*domain.Jurisdiction
	ancestors     []*domain.Jurisdiction
	err           error
}

func (s *stubStore) FindByID(_ context.Context, _ string) (*domain.Jurisdiction, error) {
	return s.jurisdiction, s.err
}

func (s *stubStore) List(_ context.Context, _ store.ListParams) ([]*domain.Jurisdiction, error) {
	return s.jurisdictions, s.err
}

func (s *stubStore) FindAncestors(_ context.Context, _ string) ([]*domain.Jurisdiction, error) {
	return s.ancestors, s.err
}

// ── helpers ───────────────────────────────────────────────────────────────────

// newTestRouter wires a Handler onto a chi router exactly as main.go does.
func newTestRouter(store handler.JurisdictionStore) http.Handler {
	r := chi.NewRouter()
	h := handler.New(store, zap.NewNop())
	handler.RegisterRoutes(r, h)
	return r
}

// executeRequest fires req against the given handler and returns the recorder.
func executeRequest(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

// ── tests ─────────────────────────────────────────────────────────────────────

// TestGetJurisdiction_200_ActiveExists verifies the happy path:
// an active, non-expired jurisdiction returns 200 with the full JSON body.
func TestGetJurisdiction_200_ActiveExists(t *testing.T) {
	now := time.Now().UTC()
	want := &domain.Jurisdiction{
		JurisdictionID:       "01J000000000000000000000AA",
		JurisdictionCode:     "GB",
		JurisdictionName:     "United Kingdom",
		JurisdictionType:     "COUNTRY",
		AuthorityType:        "FEDERAL",
		EffectiveFrom:        now.Add(-365 * 24 * time.Hour),
		ActiveFlag:           true,
		CreatedAt:            now.Add(-365 * 24 * time.Hour),
		CreatedByPrincipalID: "principal-test",
		SchemaVersion:        "1.0",
	}

	h := newTestRouter(&stubStore{jurisdiction: want})
	req := httptest.NewRequest(http.MethodGet, "/v1/jurisdictions/01J000000000000000000000AA", nil)
	req.Header.Set("X-Correlation-ID", "corr-001")

	rr := executeRequest(h, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — body: %s", rr.Code, rr.Body.String())
	}

	var got domain.Jurisdiction
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if got.JurisdictionID != want.JurisdictionID {
		t.Errorf("jurisdiction_id mismatch: got %q, want %q", got.JurisdictionID, want.JurisdictionID)
	}
	if got.JurisdictionCode != want.JurisdictionCode {
		t.Errorf("jurisdiction_code mismatch: got %q, want %q", got.JurisdictionCode, want.JurisdictionCode)
	}
	if rr.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", rr.Header().Get("Content-Type"))
	}
}

// TestGetJurisdiction_404_NotFound verifies that domain.ErrJurisdictionNotFound
// produces a 404 — not a 503.
// This covers: unknown ID, inactive (active_flag=false), and expired (effective_to in past).
// All three map to ErrJurisdictionNotFound in the store layer (SQL returns no rows).
func TestGetJurisdiction_404_NotFound(t *testing.T) {
	h := newTestRouter(&stubStore{err: domain.ErrJurisdictionNotFound})
	req := httptest.NewRequest(http.MethodGet, "/v1/jurisdictions/does-not-exist", nil)

	rr := executeRequest(h, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d — body: %s", rr.Code, rr.Body.String())
	}

	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode error body: %v", err)
	}
	if body["error"] != "jurisdiction_not_found" {
		t.Errorf("expected error=jurisdiction_not_found, got %q", body["error"])
	}
}

// TestGetJurisdiction_503_StoreUnavailable is the critical test that was missing
// before this PR was opened. It proves that a database failure returns 503 and
// NOT 404. This distinction is what enforces the fail-closed contract:
// tenant-entity-registry-svc must reject an assignment when it gets 503,
// not silently accept it as "not found".
func TestGetJurisdiction_503_StoreUnavailable(t *testing.T) {
	h := newTestRouter(&stubStore{err: domain.ErrStoreUnavailable})
	req := httptest.NewRequest(http.MethodGet, "/v1/jurisdictions/any-id", nil)
	req.Header.Set("X-Correlation-ID", "corr-503-test")

	rr := executeRequest(h, req)

	// This MUST be 503, not 404. If it were 404, a DB outage would look like
	// "jurisdiction not found" and the assignment could slip through depending
	// on caller behaviour. We test the status code explicitly.
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 on store unavailability, got %d — body: %s", rr.Code, rr.Body.String())
	}

	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode error body: %v", err)
	}
	if body["error"] != "store_unavailable" {
		t.Errorf("expected error=store_unavailable, got %q", body["error"])
	}
}

// TestGetJurisdiction_503_IsDistinctFrom_404 explicitly asserts the status codes
// differ — making the pass/fail criterion machine-readable in CI and impossible
// to accidentally swap.
func TestGetJurisdiction_503_IsDistinctFrom_404(t *testing.T) {
	notFoundStore := &stubStore{err: domain.ErrJurisdictionNotFound}
	unavailableStore := &stubStore{err: domain.ErrStoreUnavailable}

	req404 := httptest.NewRequest(http.MethodGet, "/v1/jurisdictions/x", nil)
	req503 := httptest.NewRequest(http.MethodGet, "/v1/jurisdictions/x", nil)

	rr404 := executeRequest(newTestRouter(notFoundStore), req404)
	rr503 := executeRequest(newTestRouter(unavailableStore), req503)

	if rr404.Code != http.StatusNotFound {
		t.Errorf("not-found error: expected 404, got %d", rr404.Code)
	}
	if rr503.Code != http.StatusServiceUnavailable {
		t.Errorf("unavailable error: expected 503, got %d", rr503.Code)
	}
	if rr404.Code == rr503.Code {
		t.Errorf("FAIL: 404 and 503 paths returned the same status %d — fail-closed contract is broken", rr404.Code)
	}
}

// TestGetJurisdiction_CorrelationID verifies that X-Correlation-ID is echoed
// back in the response headers on both success and error paths.
func TestGetJurisdiction_CorrelationID(t *testing.T) {
	tests := []struct {
		name    string
		store   handler.JurisdictionStore
		corrID  string
	}{
		{
			name:   "echo on 200",
			store:  &stubStore{jurisdiction: &domain.Jurisdiction{JurisdictionID: "x", SchemaVersion: "1.0"}},
			corrID: "trace-abc",
		},
		{
			name:   "echo on 404",
			store:  &stubStore{err: domain.ErrJurisdictionNotFound},
			corrID: "trace-def",
		},
		{
			name:   "echo on 503",
			store:  &stubStore{err: domain.ErrStoreUnavailable},
			corrID: "trace-ghi",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/jurisdictions/any", nil)
			req.Header.Set("X-Correlation-ID", tc.corrID)
			rr := executeRequest(newTestRouter(tc.store), req)

			got := rr.Header().Get("X-Correlation-ID")
			if got != tc.corrID {
				t.Errorf("correlation ID not echoed: got %q, want %q", got, tc.corrID)
			}
		})
	}
}

// ── ListJurisdictions tests ───────────────────────────────────────────────────

// TestListJurisdictions_200_ReturnsArray verifies that a successful store.List
// returns 200 with a JSON array body.
func TestListJurisdictions_200_ReturnsArray(t *testing.T) {
	items := []*domain.Jurisdiction{
		{JurisdictionID: "aaa", JurisdictionCode: "DE", JurisdictionType: "COUNTRY", SchemaVersion: "1.0"},
		{JurisdictionID: "bbb", JurisdictionCode: "FR", JurisdictionType: "COUNTRY", SchemaVersion: "1.0"},
	}
	h := newTestRouter(&stubStore{jurisdictions: items})
	req := httptest.NewRequest(http.MethodGet, "/v1/jurisdictions?type=COUNTRY&active=true", nil)

	rr := executeRequest(h, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — body: %s", rr.Code, rr.Body.String())
	}
	var got []domain.Jurisdiction
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode list body: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 jurisdictions, got %d", len(got))
	}
}

// TestListJurisdictions_200_EmptyArrayNotNull ensures that when the store
// returns nil (no rows), the handler still returns [] and NOT null.
// Callers that iterate the JSON array must not get a null-pointer surprise.
func TestListJurisdictions_200_EmptyArrayNotNull(t *testing.T) {
	h := newTestRouter(&stubStore{jurisdictions: nil})
	req := httptest.NewRequest(http.MethodGet, "/v1/jurisdictions", nil)

	rr := executeRequest(h, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body := rr.Body.String()
	// Must start with "[" not "null".
	if len(body) == 0 || body[0] != '[' {
		t.Errorf("expected JSON array, got: %s", body)
	}
}

// TestListJurisdictions_503_StoreUnavailable verifies that a store error
// on List returns 503.
func TestListJurisdictions_503_StoreUnavailable(t *testing.T) {
	h := newTestRouter(&stubStore{err: domain.ErrStoreUnavailable})
	req := httptest.NewRequest(http.MethodGet, "/v1/jurisdictions", nil)

	rr := executeRequest(h, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d — body: %s", rr.Code, rr.Body.String())
	}
	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode error body: %v", err)
	}
	if body["error"] != "store_unavailable" {
		t.Errorf("expected error=store_unavailable, got %q", body["error"])
	}
}

// ── GetAncestors tests ────────────────────────────────────────────────────────

// TestGetAncestors_200_Chain verifies that a non-empty ancestor list returns
// 200 with an ordered JSON array.
func TestGetAncestors_200_Chain(t *testing.T) {
	chain := []*domain.Jurisdiction{
		{JurisdictionID: "parent-id", JurisdictionCode: "EU", JurisdictionType: "SUPRANATIONAL", SchemaVersion: "1.0"},
	}
	h := newTestRouter(&stubStore{ancestors: chain})
	req := httptest.NewRequest(http.MethodGet, "/v1/jurisdictions/child-id/ancestors", nil)

	rr := executeRequest(h, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — body: %s", rr.Code, rr.Body.String())
	}
	var got []domain.Jurisdiction
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode ancestors body: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 ancestor, got %d", len(got))
	}
	if got[0].JurisdictionID != "parent-id" {
		t.Errorf("expected ancestor ID parent-id, got %q", got[0].JurisdictionID)
	}
}

// TestGetAncestors_200_RootReturnsEmptyArray verifies that a root jurisdiction
// (no parent) returns 200 with an empty array — NOT 404.
func TestGetAncestors_200_RootReturnsEmptyArray(t *testing.T) {
	h := newTestRouter(&stubStore{ancestors: nil}) // nil = root, no ancestors
	req := httptest.NewRequest(http.MethodGet, "/v1/jurisdictions/root-id/ancestors", nil)

	rr := executeRequest(h, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for root jurisdiction, got %d", rr.Code)
	}
	body := rr.Body.String()
	if len(body) == 0 || body[0] != '[' {
		t.Errorf("expected empty JSON array [], got: %s", body)
	}
}

// TestGetAncestors_404_JurisdictionNotFound verifies that an unknown
// jurisdiction_id returns 404 from the ancestors endpoint.
func TestGetAncestors_404_JurisdictionNotFound(t *testing.T) {
	h := newTestRouter(&stubStore{err: domain.ErrJurisdictionNotFound})
	req := httptest.NewRequest(http.MethodGet, "/v1/jurisdictions/unknown-id/ancestors", nil)

	rr := executeRequest(h, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d — body: %s", rr.Code, rr.Body.String())
	}
	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode error body: %v", err)
	}
	if body["error"] != "jurisdiction_not_found" {
		t.Errorf("expected error=jurisdiction_not_found, got %q", body["error"])
	}
}

// TestGetAncestors_503_StoreUnavailable verifies that a database failure
// during ancestor traversal returns 503 — not 404.
func TestGetAncestors_503_StoreUnavailable(t *testing.T) {
	h := newTestRouter(&stubStore{err: domain.ErrStoreUnavailable})
	req := httptest.NewRequest(http.MethodGet, "/v1/jurisdictions/any-id/ancestors", nil)

	rr := executeRequest(h, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d — body: %s", rr.Code, rr.Body.String())
	}
	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode error body: %v", err)
	}
	if body["error"] != "store_unavailable" {
		t.Errorf("expected error=store_unavailable, got %q", body["error"])
	}
}

