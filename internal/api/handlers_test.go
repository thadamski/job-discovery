package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/thadamski/job-discovery/internal/api"
	"github.com/thadamski/job-discovery/internal/events"
	"github.com/thadamski/job-discovery/internal/obs"
	"github.com/thadamski/job-discovery/internal/store"
	"github.com/thadamski/job-discovery/internal/store/db"
)

// mockStore is a test double for store.Store.
type mockStore struct {
	listCompanies       func(ctx context.Context, params db.ListCompaniesParams) ([]db.Company, error)
	getCompany          func(ctx context.Context, id uuid.UUID) (db.Company, error)
	createCompany       func(ctx context.Context, params db.CreateCompanyParams) (db.Company, error)
	getActiveCompanies  func(ctx context.Context, boardKind *string) ([]db.Company, error)
	insertListingIfNew  func(ctx context.Context, params db.InsertListingIfNewParams) (db.Listing, bool, error)
	getListing          func(ctx context.Context, id uuid.UUID) (db.Listing, error)
	listListings        func(ctx context.Context, params db.ListListingsParams) ([]db.Listing, error)
	updateListingStatus func(ctx context.Context, params db.UpdateListingStatusParams) (db.Listing, error)
}

func (m *mockStore) ListCompanies(ctx context.Context, params db.ListCompaniesParams) ([]db.Company, error) {
	return m.listCompanies(ctx, params)
}

func (m *mockStore) GetCompany(ctx context.Context, id uuid.UUID) (db.Company, error) {
	return m.getCompany(ctx, id)
}

func (m *mockStore) CreateCompany(ctx context.Context, params db.CreateCompanyParams) (db.Company, error) {
	return m.createCompany(ctx, params)
}

func (m *mockStore) GetActiveCompanies(ctx context.Context, boardKind *string) ([]db.Company, error) {
	return m.getActiveCompanies(ctx, boardKind)
}

func (m *mockStore) InsertListingIfNew(ctx context.Context, params db.InsertListingIfNewParams) (db.Listing, bool, error) {
	return m.insertListingIfNew(ctx, params)
}

func (m *mockStore) GetListing(ctx context.Context, id uuid.UUID) (db.Listing, error) {
	return m.getListing(ctx, id)
}

func (m *mockStore) ListListings(ctx context.Context, params db.ListListingsParams) ([]db.Listing, error) {
	return m.listListings(ctx, params)
}

func (m *mockStore) UpdateListingStatus(ctx context.Context, params db.UpdateListingStatusParams) (db.Listing, error) {
	return m.updateListingStatus(ctx, params)
}

var _ store.Store = (*mockStore)(nil)

// buildRouter wires the given store into a chi router and returns it.
func buildRouter(t *testing.T, st store.Store) http.Handler {
	t.Helper()

	o, shutdown, err := obs.New(context.Background(), "error")
	if err != nil {
		t.Fatalf("obs.New: %v", err)
	}
	t.Cleanup(shutdown)

	// Publisher is nil — no NATS in unit tests.
	appHandler := api.NewAppHandler(st, (*events.Publisher)(nil), o)
	strict := api.NewStrictHandler(appHandler, nil)

	r := chi.NewRouter()
	r.Mount("/", api.Handler(strict))
	return r
}

func TestListCompanies(t *testing.T) {
	companyID := uuid.New()
	now := time.Now().UTC()

	cases := []struct {
		name       string
		store      store.Store
		wantStatus int
		wantLen    int
	}{
		{
			name: "returns companies",
			store: &mockStore{
				listCompanies: func(_ context.Context, _ db.ListCompaniesParams) ([]db.Company, error) {
					return []db.Company{
						{
							ID:        companyID,
							Name:      "Acme",
							BoardKind: "greenhouse",
							BoardSlug: "acme",
							Priority:  2,
							Tags:      []string{},
							Active:    true,
							CreatedAt: now,
						},
					}, nil
				},
			},
			wantStatus: http.StatusOK,
			wantLen:    1,
		},
		{
			name: "returns empty list",
			store: &mockStore{
				listCompanies: func(_ context.Context, _ db.ListCompaniesParams) ([]db.Company, error) {
					return []db.Company{}, nil
				},
			},
			wantStatus: http.StatusOK,
			wantLen:    0,
		},
		{
			name: "500 on store error",
			store: &mockStore{
				listCompanies: func(_ context.Context, _ db.ListCompaniesParams) ([]db.Company, error) {
					return nil, errors.New("db down")
				},
			},
			wantStatus: http.StatusInternalServerError,
			wantLen:    -1, // not checked
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := buildRouter(t, tc.store)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/job-discovery/companies", nil)
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			if rr.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tc.wantStatus)
			}

			if tc.wantLen >= 0 {
				var body []interface{}
				if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
					t.Fatalf("decode body: %v", err)
				}
				if len(body) != tc.wantLen {
					t.Errorf("len(body) = %d, want %d", len(body), tc.wantLen)
				}
			}
		})
	}
}

func TestCreateCompany(t *testing.T) {
	companyID := uuid.New()
	now := time.Now().UTC()

	successStore := &mockStore{
		createCompany: func(_ context.Context, params db.CreateCompanyParams) (db.Company, error) {
			return db.Company{
				ID:        companyID,
				Name:      params.Name,
				BoardKind: params.BoardKind,
				BoardSlug: params.BoardSlug,
				Priority:  params.Priority,
				Tags:      params.Tags,
				Active:    params.Active,
				CreatedAt: now,
			}, nil
		},
	}

	conflictStore := &mockStore{
		createCompany: func(_ context.Context, _ db.CreateCompanyParams) (db.Company, error) {
			return db.Company{}, errors.New("ERROR: duplicate key value violates unique constraint (SQLSTATE 23505)")
		},
	}

	cases := []struct {
		name       string
		store      store.Store
		body       interface{}
		wantStatus int
	}{
		{
			name:  "creates company",
			store: successStore,
			body: map[string]interface{}{
				"name":       "Acme",
				"board_kind": "greenhouse",
				"board_slug": "acme",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name:  "409 on duplicate",
			store: conflictStore,
			body: map[string]interface{}{
				"name":       "Acme",
				"board_kind": "greenhouse",
				"board_slug": "acme",
			},
			wantStatus: http.StatusConflict,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := buildRouter(t, tc.store)
			bodyJSON, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/job-discovery/companies", bytes.NewReader(bodyJSON))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			if rr.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d (body: %s)", rr.Code, tc.wantStatus, rr.Body.String())
			}
		})
	}
}

func TestGetListing(t *testing.T) {
	listingID := uuid.New()
	companyID := uuid.New()
	now := time.Now().UTC()

	listing := db.Listing{
		ID:          listingID,
		CompanyID:   companyID,
		ExternalID:  "12345",
		Title:       "Software Engineer",
		URL:         "https://example.com/job",
		Description: "Great job",
		RawPayload:  []byte(`{}`),
		FetchedAt:   now,
		Status:      "new",
	}

	cases := []struct {
		name       string
		store      store.Store
		id         string
		wantStatus int
	}{
		{
			name: "200 found",
			store: &mockStore{
				getListing: func(_ context.Context, _ uuid.UUID) (db.Listing, error) {
					return listing, nil
				},
			},
			id:         listingID.String(),
			wantStatus: http.StatusOK,
		},
		{
			name: "404 not found",
			store: &mockStore{
				getListing: func(_ context.Context, _ uuid.UUID) (db.Listing, error) {
					return db.Listing{}, errors.New("no rows in result set")
				},
			},
			id:         listingID.String(),
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := buildRouter(t, tc.store)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/job-discovery/listings/"+tc.id, nil)
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			if rr.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d (body: %s)", rr.Code, tc.wantStatus, rr.Body.String())
			}
		})
	}
}

func TestUpdateListing(t *testing.T) {
	listingID := uuid.New()
	companyID := uuid.New()
	now := time.Now().UTC()

	listing := db.Listing{
		ID:          listingID,
		CompanyID:   companyID,
		ExternalID:  "12345",
		Title:       "Software Engineer",
		URL:         "https://example.com/job",
		Description: "Great job",
		RawPayload:  []byte(`{}`),
		FetchedAt:   now,
		Status:      "shortlisted",
	}

	cases := []struct {
		name       string
		store      store.Store
		body       interface{}
		wantStatus int
	}{
		{
			name: "200 updated",
			store: &mockStore{
				updateListingStatus: func(_ context.Context, _ db.UpdateListingStatusParams) (db.Listing, error) {
					return listing, nil
				},
			},
			body:       map[string]interface{}{"status": "shortlisted"},
			wantStatus: http.StatusOK,
		},
		{
			name: "422 invalid status",
			store: &mockStore{
				updateListingStatus: func(_ context.Context, _ db.UpdateListingStatusParams) (db.Listing, error) {
					return db.Listing{}, nil
				},
			},
			body:       map[string]interface{}{"status": "invalid_status_value"},
			wantStatus: http.StatusUnprocessableEntity,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := buildRouter(t, tc.store)
			bodyJSON, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPatch, "/api/v1/job-discovery/listings/"+listingID.String(), bytes.NewReader(bodyJSON))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			r.ServeHTTP(rr, req)

			if rr.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d (body: %s)", rr.Code, tc.wantStatus, rr.Body.String())
			}
		})
	}
}
