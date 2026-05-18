//go:build integration

package store_test

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/thadamski/job-discovery/internal/store"
	"github.com/thadamski/job-discovery/internal/store/db"
)

// migrationsDir returns the absolute path to the migrations directory.
func migrationsDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "migrations")
}

func TestStoreIntegration(t *testing.T) {
	ctx := context.Background()

	pgContainer, err := postgres.Run(
		ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("terminate postgres container: %v", err)
		}
	})

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("get connection string: %v", err)
	}

	// Run migrations.
	m, err := migrate.New("file://"+migrationsDir(), dsn)
	if err != nil {
		t.Fatalf("create migrate: %v", err)
	}
	if err = m.Up(); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	st, pool, err := store.New(ctx, dsn)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	t.Run("CreateAndGetCompany", func(t *testing.T) {
		company, err := st.CreateCompany(ctx, db.CreateCompanyParams{
			Name:      "Test Corp",
			BoardKind: "greenhouse",
			BoardSlug: "test-corp",
			Priority:  2,
			Tags:      []string{"fintech"},
			Active:    true,
		})
		if err != nil {
			t.Fatalf("CreateCompany: %v", err)
		}

		if company.Name != "Test Corp" {
			t.Errorf("Name = %q, want %q", company.Name, "Test Corp")
		}
		if company.ID == (uuid.UUID{}) {
			t.Error("ID is zero UUID")
		}

		got, err := st.GetCompany(ctx, company.ID)
		if err != nil {
			t.Fatalf("GetCompany: %v", err)
		}
		if got.ID != company.ID {
			t.Errorf("GetCompany ID = %v, want %v", got.ID, company.ID)
		}
	})

	t.Run("InsertListingIfNew_NewVsDuplicate", func(t *testing.T) {
		// Create a company first.
		company, err := st.CreateCompany(ctx, db.CreateCompanyParams{
			Name:      "Listing Corp",
			BoardKind: "greenhouse",
			BoardSlug: "listing-corp",
			Priority:  1,
			Tags:      []string{},
			Active:    true,
		})
		if err != nil {
			t.Fatalf("CreateCompany: %v", err)
		}

		params := db.InsertListingIfNewParams{
			CompanyID:   company.ID,
			ExternalID:  "ext-001",
			Title:       "Software Engineer",
			URL:         "https://example.com/job/1",
			Description: "A great job",
			RawPayload:  []byte(`{"id": 1}`),
		}

		// First insert — should be new.
		listing, isNew, err := st.InsertListingIfNew(ctx, params)
		if err != nil {
			t.Fatalf("InsertListingIfNew (first): %v", err)
		}
		if !isNew {
			t.Error("first insert: isNew = false, want true")
		}
		if listing.ExternalID != "ext-001" {
			t.Errorf("ExternalID = %q, want %q", listing.ExternalID, "ext-001")
		}

		// Second insert — should be duplicate.
		_, isNew, err = st.InsertListingIfNew(ctx, params)
		if err != nil {
			t.Fatalf("InsertListingIfNew (second): %v", err)
		}
		if isNew {
			t.Error("second insert: isNew = true, want false")
		}
	})
}
