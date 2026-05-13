package discovery

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/thadamski/job-discovery/internal/events"
	"github.com/thadamski/job-discovery/internal/obs"
	"github.com/thadamski/job-discovery/internal/store"
	"github.com/thadamski/job-discovery/internal/store/db"
)

// RefreshOpts controls which companies are refreshed.
type RefreshOpts struct {
	CompanyID *uuid.UUID
	BoardKind *string
}

// CompanyResult holds the per-company outcome of a Refresh call.
type CompanyResult struct {
	Company      db.Company
	ListingsSeen int
	ListingsNew  int
	Error        error
}

// Refresh fetches listings for all (or a subset of) active companies, inserts
// any new ones, and publishes a ListingDiscovered event for each new insertion.
func Refresh(ctx context.Context, st store.Store, pub *events.Publisher, o *obs.Obs, opts RefreshOpts) ([]CompanyResult, error) {
	var companies []db.Company
	var err error

	if opts.CompanyID != nil {
		company, err := st.GetCompany(ctx, *opts.CompanyID)
		if err != nil {
			return nil, fmt.Errorf("fetching company %s: %w", opts.CompanyID, err)
		}
		companies = []db.Company{company}
	} else {
		companies, err = st.GetActiveCompanies(ctx, opts.BoardKind)
		if err != nil {
			return nil, fmt.Errorf("fetching active companies: %w", err)
		}
	}

	results := make([]CompanyResult, 0, len(companies))

	for _, company := range companies {
		result := refreshCompany(ctx, st, pub, o, company)
		results = append(results, result)
	}

	return results, nil
}

func refreshCompany(ctx context.Context, st store.Store, pub *events.Publisher, o *obs.Obs, company db.Company) CompanyResult {
	log := o.Logger.With(
		slog.String("company", company.Name),
		slog.String("board_kind", company.BoardKind),
		slog.String("board_slug", company.BoardSlug),
	)

	if company.BoardKind != "greenhouse" {
		log.InfoContext(ctx, "skipping unsupported board kind")
		return CompanyResult{
			Company: company,
			Error:   fmt.Errorf("unsupported board_kind: %s", company.BoardKind),
		}
	}

	listings, err := FetchGreenhouse(ctx, company.BoardSlug)
	if err != nil {
		log.ErrorContext(ctx, "fetching Greenhouse listings", slog.String("error", err.Error()))
		return CompanyResult{Company: company, Error: err}
	}

	seen := len(listings)
	newCount := 0

	for _, fl := range listings {
		inserted, isNew, insertErr := st.InsertListingIfNew(ctx, db.InsertListingIfNewParams{
			CompanyID:   company.ID,
			ExternalID:  fl.ExternalID,
			Title:       fl.Title,
			Location:    fl.Location,
			Url:         fl.URL,
			Description: fl.Description,
			RawPayload:  fl.RawPayload,
			PostedAt:    fl.PostedAt,
		})
		if insertErr != nil {
			log.ErrorContext(ctx, "inserting listing", slog.String("external_id", fl.ExternalID), slog.String("error", insertErr.Error()))
			continue
		}

		if isNew {
			newCount++
			o.ListingsDiscoveredTotal.WithLabelValues("new").Inc()

			evt := events.ListingDiscoveredEvent{
				ListingID:   inserted.ID,
				CompanyName: company.Name,
				Title:       inserted.Title,
				URL:         inserted.Url,
			}

			if pubErr := pub.PublishListingDiscovered(ctx, evt); pubErr != nil {
				log.ErrorContext(
					ctx, "publishing listing.discovered event",
					slog.String("listing_id", inserted.ID.String()),
					slog.String("error", pubErr.Error()),
				)
			} else {
				o.NATSPublishedTotal.WithLabelValues(events.SubjectListingDiscovered).Inc()
			}
		} else {
			o.ListingsDiscoveredTotal.WithLabelValues("duplicate").Inc()
		}
	}

	return CompanyResult{
		Company:      company,
		ListingsSeen: seen,
		ListingsNew:  newCount,
	}
}
