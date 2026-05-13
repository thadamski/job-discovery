package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/thadamski/job-discovery/internal/discovery"
	"github.com/thadamski/job-discovery/internal/events"
	"github.com/thadamski/job-discovery/internal/obs"
	"github.com/thadamski/job-discovery/internal/store"
	"github.com/thadamski/job-discovery/internal/store/db"
)

// contextKey is the package-local type for context keys.
type contextKey int

const (
	requestIDKey contextKey = iota
)

// WithRequestID stores a request ID in the context.
func WithRequestID(ctx context.Context, rid string) context.Context {
	return context.WithValue(ctx, requestIDKey, rid)
}

// RequestIDFromCtx retrieves the request ID stored by WithRequestID.
func RequestIDFromCtx(ctx context.Context) string {
	rid, _ := ctx.Value(requestIDKey).(string)
	return rid
}

// AppHandler implements StrictServerInterface.
type AppHandler struct {
	store store.Store
	pub   *events.Publisher
	obs   *obs.Obs
}

// NewAppHandler constructs an AppHandler.
func NewAppHandler(st store.Store, pub *events.Publisher, o *obs.Obs) *AppHandler {
	return &AppHandler{store: st, pub: pub, obs: o}
}

// problem builds an RFC 7807 Problem detail.
func problem(status int, title, detail, requestID string) Problem {
	t := "about:blank"
	p := Problem{
		Type:   t,
		Title:  title,
		Status: status,
	}
	if detail != "" {
		p.Detail = &detail
	}
	if requestID != "" {
		p.RequestId = &requestID
	}
	return p
}

func internalError(ctx context.Context, detail string) Problem {
	return problem(500, "Internal Server Error", detail, RequestIDFromCtx(ctx))
}

func notFound(ctx context.Context, detail string) Problem {
	return problem(404, "Not Found", detail, RequestIDFromCtx(ctx))
}

func conflict(ctx context.Context, detail string) Problem {
	return problem(409, "Conflict", detail, RequestIDFromCtx(ctx))
}

func unprocessable(ctx context.Context, detail string) Problem {
	return problem(422, "Unprocessable Entity", detail, RequestIDFromCtx(ctx))
}

// dbCompanyToAPI converts a db.Company to the API Company type.
func dbCompanyToAPI(c db.Company) Company {
	id := c.ID
	at := c.CreatedAt
	bk := BoardKind(c.BoardKind)
	return Company{
		Id:        &id,
		Name:      c.Name,
		BoardKind: bk,
		BoardSlug: c.BoardSlug,
		Priority:  int(c.Priority),
		Tags:      c.Tags,
		Notes:     c.Notes,
		Active:    c.Active,
		CreatedAt: &at,
	}
}

// dbListingToAPI converts a db.Listing to the API Listing type.
func dbListingToAPI(l db.Listing) Listing {
	id := l.ID
	at := l.FetchedAt

	out := Listing{
		Id:          &id,
		CompanyId:   l.CompanyID,
		ExternalId:  l.ExternalID,
		Title:       l.Title,
		Location:    l.Location,
		Url:         l.URL,
		Description: l.Description,
		PostedAt:    l.PostedAt,
		FetchedAt:   &at,
		Status:      ListingStatus(l.Status),
	}

	if len(l.RawPayload) > 0 {
		var raw map[string]interface{}
		if err := json.Unmarshal(l.RawPayload, &raw); err == nil {
			out.RawPayload = &raw
		}
	}

	return out
}

// ListCompanies handles GET /api/v1/job-discovery/companies.
func (h *AppHandler) ListCompanies(ctx context.Context, request ListCompaniesRequestObject) (ListCompaniesResponseObject, error) {
	var bk *string
	if request.Params.BoardKind != nil {
		s := string(*request.Params.BoardKind)
		bk = &s
	}

	companies, err := h.store.ListCompanies(ctx, db.ListCompaniesParams{
		Active:    request.Params.Active,
		BoardKind: bk,
	})
	if err != nil {
		h.obs.Logger.ErrorContext(ctx, "listing companies", "error", err, "request_id", RequestIDFromCtx(ctx))
		return ListCompanies500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse(internalError(ctx, "failed to list companies")),
		}, nil
	}

	result := make(ListCompanies200JSONResponse, 0, len(companies))
	for _, c := range companies {
		result = append(result, dbCompanyToAPI(c))
	}

	return result, nil
}

// CreateCompany handles POST /api/v1/job-discovery/companies.
func (h *AppHandler) CreateCompany(ctx context.Context, request CreateCompanyRequestObject) (CreateCompanyResponseObject, error) {
	body := request.Body
	if body == nil {
		return CreateCompany400ApplicationProblemPlusJSONResponse{
			BadRequestApplicationProblemPlusJSONResponse(problem(400, "Bad Request", "missing request body", RequestIDFromCtx(ctx))),
		}, nil
	}

	priority := int16(2)
	if body.Priority != nil {
		priority = int16(*body.Priority) //nolint:gosec // priority is bounded 1-5 by OpenAPI schema
	}

	active := true
	if body.Active != nil {
		active = *body.Active
	}

	var tags []string
	if body.Tags != nil {
		tags = *body.Tags
	}

	company, err := h.store.CreateCompany(ctx, db.CreateCompanyParams{
		Name:      body.Name,
		BoardKind: string(body.BoardKind),
		BoardSlug: body.BoardSlug,
		Priority:  priority,
		Tags:      tags,
		Notes:     body.Notes,
		Active:    active,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return CreateCompany409ApplicationProblemPlusJSONResponse{
				ConflictApplicationProblemPlusJSONResponse(conflict(ctx, "company with this board_kind and board_slug already exists")),
			}, nil
		}
		h.obs.Logger.ErrorContext(ctx, "creating company", "error", err, "request_id", RequestIDFromCtx(ctx))
		return CreateCompany500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse(internalError(ctx, "failed to create company")),
		}, nil
	}

	apiCompany := dbCompanyToAPI(company)
	loc := fmt.Sprintf("/api/v1/job-discovery/companies/%s", company.ID)
	return CreateCompany201JSONResponse{
		Body:    apiCompany,
		Headers: CreateCompany201ResponseHeaders{Location: &loc},
	}, nil
}

// ListListings handles GET /api/v1/job-discovery/listings.
func (h *AppHandler) ListListings(ctx context.Context, request ListListingsRequestObject) (ListListingsResponseObject, error) {
	params := request.Params

	limit := int32(100)
	if params.Limit != nil && *params.Limit > 0 {
		limit = int32(*params.Limit) //nolint:gosec // limit is bounded 1-500 by OpenAPI schema
	}

	offset := int32(0)
	if params.Offset != nil && *params.Offset > 0 {
		offset = int32(*params.Offset) //nolint:gosec // offset is non-negative by OpenAPI schema
	}

	var statusFilter *string
	if params.Status != nil {
		s := string(*params.Status)
		statusFilter = &s
	}

	var companyIDFilter *uuid.UUID
	var boardSlugFilter *string
	if params.Company != nil {
		c := *params.Company
		id, err := uuid.Parse(c)
		if err == nil {
			companyIDFilter = &id
		} else {
			boardSlugFilter = &c
		}
	}

	listings, err := h.store.ListListings(ctx, db.ListListingsParams{
		Status:    statusFilter,
		CompanyID: companyIDFilter,
		BoardSlug: boardSlugFilter,
		Lim:       limit,
		Off:       offset,
	})
	if err != nil {
		h.obs.Logger.ErrorContext(ctx, "listing listings", "error", err, "request_id", RequestIDFromCtx(ctx))
		return ListListings500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse(internalError(ctx, "failed to list listings")),
		}, nil
	}

	result := make(ListListings200JSONResponse, 0, len(listings))
	for _, l := range listings {
		result = append(result, dbListingToAPI(l))
	}

	return result, nil
}

// GetListing handles GET /api/v1/job-discovery/listings/{id}.
func (h *AppHandler) GetListing(ctx context.Context, request GetListingRequestObject) (GetListingResponseObject, error) {
	listing, err := h.store.GetListing(ctx, request.Id)
	if err != nil {
		if isNotFound(err) {
			return GetListing404ApplicationProblemPlusJSONResponse{
				NotFoundApplicationProblemPlusJSONResponse(notFound(ctx, fmt.Sprintf("listing %s not found", request.Id))),
			}, nil
		}
		h.obs.Logger.ErrorContext(ctx, "getting listing", "error", err, "request_id", RequestIDFromCtx(ctx))
		return GetListing500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse(internalError(ctx, "failed to get listing")),
		}, nil
	}

	return GetListing200JSONResponse(dbListingToAPI(listing)), nil
}

// UpdateListing handles PATCH /api/v1/job-discovery/listings/{id}.
func (h *AppHandler) UpdateListing(ctx context.Context, request UpdateListingRequestObject) (UpdateListingResponseObject, error) {
	body := request.Body
	if body == nil || body.Status == nil {
		return UpdateListing400ApplicationProblemPlusJSONResponse{
			BadRequestApplicationProblemPlusJSONResponse(problem(400, "Bad Request", "status field is required", RequestIDFromCtx(ctx))),
		}, nil
	}

	if !body.Status.Valid() {
		return UpdateListing422ApplicationProblemPlusJSONResponse{
			UnprocessableEntityApplicationProblemPlusJSONResponse(unprocessable(ctx, fmt.Sprintf("invalid status value: %s", *body.Status))),
		}, nil
	}

	listing, err := h.store.UpdateListingStatus(ctx, db.UpdateListingStatusParams{
		ID:     request.Id,
		Status: string(*body.Status),
	})
	if err != nil {
		if isNotFound(err) {
			return UpdateListing404ApplicationProblemPlusJSONResponse{
				NotFoundApplicationProblemPlusJSONResponse(notFound(ctx, fmt.Sprintf("listing %s not found", request.Id))),
			}, nil
		}
		h.obs.Logger.ErrorContext(ctx, "updating listing", "error", err, "request_id", RequestIDFromCtx(ctx))
		return UpdateListing500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse(internalError(ctx, "failed to update listing")),
		}, nil
	}

	return UpdateListing200JSONResponse(dbListingToAPI(listing)), nil
}

// TriggerRefresh handles POST /api/v1/job-discovery/refresh.
func (h *AppHandler) TriggerRefresh(ctx context.Context, request TriggerRefreshRequestObject) (TriggerRefreshResponseObject, error) {
	startedAt := time.Now().UTC()

	opts := discovery.RefreshOpts{}
	if request.Body != nil {
		if request.Body.CompanyId != nil {
			opts.CompanyID = request.Body.CompanyId
		}
		if request.Body.BoardKind != nil {
			bk := string(*request.Body.BoardKind)
			opts.BoardKind = &bk
		}
	}

	results, err := discovery.Refresh(ctx, h.store, h.pub, h.obs, opts)
	if err != nil {
		h.obs.Logger.ErrorContext(ctx, "triggering refresh", "error", err, "request_id", RequestIDFromCtx(ctx))
		return TriggerRefresh500ApplicationProblemPlusJSONResponse{
			InternalErrorApplicationProblemPlusJSONResponse(internalError(ctx, "refresh failed: "+err.Error())),
		}, nil
	}

	finishedAt := time.Now().UTC()

	outcomes := make([]CompanyRefreshOutcome, 0, len(results))
	var errCount int
	var totalSeen, totalNew int

	for _, r := range results {
		bk := BoardKind(r.Company.BoardKind)
		o := CompanyRefreshOutcome{
			CompanyId:    r.Company.ID,
			Name:         r.Company.Name,
			BoardKind:    bk,
			BoardSlug:    r.Company.BoardSlug,
			ListingsSeen: r.ListingsSeen,
			ListingsNew:  r.ListingsNew,
		}
		if r.Error != nil {
			errMsg := r.Error.Error()
			o.Error = &errMsg
			errCount++
		}
		totalSeen += r.ListingsSeen
		totalNew += r.ListingsNew
		outcomes = append(outcomes, o)
	}

	return TriggerRefresh200JSONResponse(RefreshResult{
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
		Companies:  outcomes,
		Skipped:    []SkippedCompany{},
		Totals: struct {
			CompaniesFetched int `json:"companies_fetched"`
			Errors           int `json:"errors"`
			ListingsNew      int `json:"listings_new"`
			ListingsSeen     int `json:"listings_seen"`
		}{
			CompaniesFetched: len(results),
			Errors:           errCount,
			ListingsSeen:     totalSeen,
			ListingsNew:      totalNew,
		},
	}), nil
}

// isNotFound returns true if the error indicates a "not found" condition from pgx.
func isNotFound(err error) bool {
	return err != nil && err.Error() == "no rows in result set"
}

// isUniqueViolation returns true if the error is a PostgreSQL unique constraint violation.
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "23505")
}
