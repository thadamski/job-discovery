// Package events publishes domain events to NATS JetStream.
package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	// StreamName is the NATS JetStream stream that owns all job-hunt events.
	StreamName = "JOBHUNT"
	// SubjectListingDiscovered is published when a new listing is inserted for the first time.
	SubjectListingDiscovered = "jobhunt.listing.discovered"
)

// ListingDiscoveredEvent is the payload published on SubjectListingDiscovered.
type ListingDiscoveredEvent struct {
	ListingID   uuid.UUID `json:"listing_id"`
	CompanyName string    `json:"company_name"`
	Title       string    `json:"title"`
	URL         string    `json:"url"`
}

// Publisher publishes domain events to NATS JetStream.
type Publisher struct {
	js jetstream.JetStream
	nc *nats.Conn
}

// NewPublisher connects to NATS, ensures the JOBHUNT stream exists, and returns a Publisher.
func NewPublisher(ctx context.Context, natsURL string) (*Publisher, func(), error) {
	nc, err := nats.Connect(natsURL)
	if err != nil {
		return nil, nil, fmt.Errorf("connecting to NATS at %s: %w", natsURL, err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, nil, fmt.Errorf("creating JetStream context: %w", err)
	}

	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     StreamName,
		Subjects: []string{"jobhunt.>"},
	})
	if err != nil {
		nc.Close()
		return nil, nil, fmt.Errorf("ensuring NATS stream %s: %w", StreamName, err)
	}

	shutdown := func() { nc.Close() }

	return &Publisher{js: js, nc: nc}, shutdown, nil
}

// PublishListingDiscovered serialises and publishes a ListingDiscoveredEvent.
func (p *Publisher) PublishListingDiscovered(ctx context.Context, evt ListingDiscoveredEvent) error {
	payload, err := json.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshalling ListingDiscoveredEvent: %w", err)
	}

	if _, err = p.js.Publish(ctx, SubjectListingDiscovered, payload); err != nil {
		return fmt.Errorf("publishing %s: %w", SubjectListingDiscovered, err)
	}

	return nil
}
