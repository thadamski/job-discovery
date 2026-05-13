// Package discovery fetches job listings from external job boards.
package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const greenhouseBaseURL = "https://boards-api.greenhouse.io/v1/boards"

// FetchedListing holds a normalised job listing fetched from an external board.
type FetchedListing struct {
	ExternalID  string
	Title       string
	Location    *string
	URL         string
	Description string
	RawPayload  []byte
	PostedAt    *time.Time
}

// greenhouseResponse is the top-level JSON structure returned by the Greenhouse jobs API.
type greenhouseResponse struct {
	Jobs []greenhouseJob `json:"jobs"`
}

type greenhouseJob struct {
	ID          int64               `json:"id"`
	Title       string              `json:"title"`
	Location    *greenhouseLocation `json:"location"`
	AbsoluteURL string              `json:"absolute_url"`
	Content     string              `json:"content"`
	UpdatedAt   *time.Time          `json:"updated_at"`
}

type greenhouseLocation struct {
	Name string `json:"name"`
}

// FetchGreenhouse fetches all jobs for a Greenhouse board slug.
// It uses a 30-second HTTP timeout and requires no authentication.
func FetchGreenhouse(ctx context.Context, boardSlug string) ([]FetchedListing, error) {
	return FetchGreenhouseURL(ctx, boardSlug, greenhouseBaseURL)
}

// FetchGreenhouseURL is the testable variant of FetchGreenhouse that accepts a custom base URL.
func FetchGreenhouseURL(ctx context.Context, boardSlug, baseURL string) ([]FetchedListing, error) {
	url := fmt.Sprintf("%s/%s/jobs?content=true", baseURL, boardSlug)

	client := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building Greenhouse request for %s: %w", boardSlug, err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching Greenhouse jobs for %s: %w", boardSlug, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading Greenhouse response body for %s: %w", boardSlug, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Greenhouse API returned %d for board %s", resp.StatusCode, boardSlug)
	}

	var ghResp greenhouseResponse
	if err = json.Unmarshal(body, &ghResp); err != nil {
		return nil, fmt.Errorf("unmarshalling Greenhouse response for %s: %w", boardSlug, err)
	}

	listings := make([]FetchedListing, 0, len(ghResp.Jobs))
	for _, job := range ghResp.Jobs {
		raw, err := json.Marshal(job)
		if err != nil {
			return nil, fmt.Errorf("marshalling raw job payload for job %d: %w", job.ID, err)
		}

		fl := FetchedListing{
			ExternalID:  fmt.Sprintf("%d", job.ID),
			Title:       job.Title,
			URL:         job.AbsoluteURL,
			Description: job.Content,
			RawPayload:  raw,
			PostedAt:    job.UpdatedAt,
		}

		if job.Location != nil {
			loc := job.Location.Name
			fl.Location = &loc
		}

		listings = append(listings, fl)
	}

	return listings, nil
}
