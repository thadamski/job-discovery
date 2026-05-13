// Package store provides the data-access layer backed by PostgreSQL.
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/thadamski/job-discovery/internal/store/db"
)

// Store is the repository interface used across the service.
type Store interface {
	// Companies
	ListCompanies(ctx context.Context, params db.ListCompaniesParams) ([]db.Company, error)
	GetCompany(ctx context.Context, id uuid.UUID) (db.Company, error)
	CreateCompany(ctx context.Context, params db.CreateCompanyParams) (db.Company, error)
	GetActiveCompanies(ctx context.Context, boardKind *string) ([]db.Company, error)

	// Listings
	// InsertListingIfNew inserts a new listing and returns (listing, true, nil).
	// If (company_id, external_id) already exists it returns (zero, false, nil).
	InsertListingIfNew(ctx context.Context, params db.InsertListingIfNewParams) (db.Listing, bool, error)
	GetListing(ctx context.Context, id uuid.UUID) (db.Listing, error)
	ListListings(ctx context.Context, params db.ListListingsParams) ([]db.Listing, error)
	UpdateListingStatus(ctx context.Context, params db.UpdateListingStatusParams) (db.Listing, error)
}

type pgStore struct {
	q *db.Queries
}

// New opens a pgx connection pool and returns a Store and the pool (for healthz checks).
func New(ctx context.Context, dsn string) (Store, *pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("opening pgx pool: %w", err)
	}

	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("pinging postgres: %w", err)
	}

	return &pgStore{q: db.New(pool)}, pool, nil
}

func (s *pgStore) ListCompanies(ctx context.Context, params db.ListCompaniesParams) ([]db.Company, error) {
	return s.q.ListCompanies(ctx, params)
}

func (s *pgStore) GetCompany(ctx context.Context, id uuid.UUID) (db.Company, error) {
	return s.q.GetCompany(ctx, id)
}

func (s *pgStore) CreateCompany(ctx context.Context, params db.CreateCompanyParams) (db.Company, error) {
	return s.q.CreateCompany(ctx, params)
}

func (s *pgStore) GetActiveCompanies(ctx context.Context, boardKind *string) ([]db.Company, error) {
	return s.q.GetActiveCompanies(ctx, boardKind)
}

func (s *pgStore) InsertListingIfNew(ctx context.Context, params db.InsertListingIfNewParams) (db.Listing, bool, error) {
	listing, err := s.q.InsertListingIfNew(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// ON CONFLICT DO NOTHING returned no rows → duplicate.
			return db.Listing{}, false, nil
		}
		return db.Listing{}, false, err
	}
	return listing, true, nil
}

func (s *pgStore) GetListing(ctx context.Context, id uuid.UUID) (db.Listing, error) {
	return s.q.GetListing(ctx, id)
}

func (s *pgStore) ListListings(ctx context.Context, params db.ListListingsParams) ([]db.Listing, error) {
	return s.q.ListListings(ctx, params)
}

func (s *pgStore) UpdateListingStatus(ctx context.Context, params db.UpdateListingStatusParams) (db.Listing, error) {
	return s.q.UpdateListingStatus(ctx, params)
}
