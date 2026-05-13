package discovery_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/thadamski/job-discovery/internal/discovery"
)

func TestFetchGreenhouse(t *testing.T) {
	updatedAt, _ := time.Parse(time.RFC3339, "2024-03-15T10:00:00Z")

	fixture := map[string]interface{}{
		"jobs": []map[string]interface{}{
			{
				"id":           12345,
				"title":        "Software Engineer",
				"absolute_url": "https://boards.greenhouse.io/acme/jobs/12345",
				"content":      "<p>Great job</p>",
				"updated_at":   updatedAt.Format(time.RFC3339),
				"location": map[string]interface{}{
					"name": "San Francisco, CA",
				},
			},
			{
				"id":           67890,
				"title":        "Product Manager",
				"absolute_url": "https://boards.greenhouse.io/acme/jobs/67890",
				"content":      "<p>Exciting role</p>",
				"updated_at":   nil,
				"location":     nil,
			},
		},
	}

	fixtureJSON, err := json.Marshal(fixture)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "acme") {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixtureJSON)
	}))
	defer srv.Close()

	// Override the base URL by patching the function — but since FetchGreenhouse
	// uses a hardcoded URL we test it via an httptest stub that intercepts on slug.
	// We expose a testable variant via the internal package by calling FetchGreenhouseURL.
	listings, err := discovery.FetchGreenhouseURL(context.Background(), "acme", srv.URL+"/v1/boards")
	if err != nil {
		t.Fatalf("FetchGreenhouseURL: %v", err)
	}

	if len(listings) != 2 {
		t.Fatalf("got %d listings, want 2", len(listings))
	}

	cases := []struct {
		idx          int
		wantID       string
		wantTitle    string
		wantLoc      *string
		wantPostedAt bool
	}{
		{
			idx:          0,
			wantID:       "12345",
			wantTitle:    "Software Engineer",
			wantLoc:      strPtr("San Francisco, CA"),
			wantPostedAt: true,
		},
		{
			idx:          1,
			wantID:       "67890",
			wantTitle:    "Product Manager",
			wantLoc:      nil,
			wantPostedAt: false,
		},
	}

	for _, tc := range cases {
		l := listings[tc.idx]
		if l.ExternalID != tc.wantID {
			t.Errorf("[%d] ExternalID = %q, want %q", tc.idx, l.ExternalID, tc.wantID)
		}
		if l.Title != tc.wantTitle {
			t.Errorf("[%d] Title = %q, want %q", tc.idx, l.Title, tc.wantTitle)
		}
		if tc.wantLoc == nil && l.Location != nil {
			t.Errorf("[%d] Location = %q, want nil", tc.idx, *l.Location)
		}
		if tc.wantLoc != nil {
			if l.Location == nil {
				t.Errorf("[%d] Location = nil, want %q", tc.idx, *tc.wantLoc)
			} else if *l.Location != *tc.wantLoc {
				t.Errorf("[%d] Location = %q, want %q", tc.idx, *l.Location, *tc.wantLoc)
			}
		}
		if tc.wantPostedAt && l.PostedAt == nil {
			t.Errorf("[%d] PostedAt is nil, want non-nil", tc.idx)
		}
		if !tc.wantPostedAt && l.PostedAt != nil {
			t.Errorf("[%d] PostedAt = %v, want nil", tc.idx, l.PostedAt)
		}
		if len(l.RawPayload) == 0 {
			t.Errorf("[%d] RawPayload is empty", tc.idx)
		}
	}
}

func strPtr(s string) *string { return &s }
