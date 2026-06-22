package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// MastodonClient handles posting to Mastodon
type MastodonClient struct {
	instanceURL string
	accessToken string
	client      *http.Client
}

// MastodonStatus represents the response from creating a status
type MastodonStatus struct {
	ID        string `json:"id"`
	URI       string `json:"uri"`
	URL       string `json:"url"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

// NewMastodonClient creates a new Mastodon client from environment variables
// Expected env vars: MASTODON_INSTANCE_URL, MASTODON_ACCESS_TOKEN
func NewMastodonClient() (*MastodonClient, error) {
	instanceURL := os.Getenv("MASTODON_INSTANCE_URL")
	accessToken := os.Getenv("MASTODON_ACCESS_TOKEN")

	if instanceURL == "" {
		return nil, fmt.Errorf("MASTODON_INSTANCE_URL environment variable not set")
	}
	if accessToken == "" {
		return nil, fmt.Errorf("MASTODON_ACCESS_TOKEN environment variable not set")
	}

	return &MastodonClient{
		instanceURL: instanceURL,
		accessToken: accessToken,
		client:      &http.Client{},
	}, nil
}

// PostStatus posts a message to Mastodon
func (mc *MastodonClient) PostStatus(message string) (*MastodonStatus, error) {
	url := fmt.Sprintf("%s/api/v1/statuses", mc.instanceURL)

	// Create request body
	payload := map[string]string{
		"status": message,
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request body: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", mc.accessToken))

	// Send request
	resp, err := mc.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("mastodon API returned status %d", resp.StatusCode)
	}

	// Parse response
	var status MastodonStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &status, nil
}

func postToMastodon(ctx context.Context, message string) error {
	ctx, span := otel.Tracer("social").Start(ctx, "post-to-mastodon")
	defer span.End()

	span.SetAttributes(attribute.String("social_media.platform", "mastodon"))

	// Create client from environment variables
	client, err := NewMastodonClient()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.SetAttributes(attribute.String("exception.slug", "mastodon_init_failed"))
		return fmt.Errorf("Failed to post to Mastodon: %v\n", err)
	}

	// Post a status
	status, err := client.PostStatus(message)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.SetAttributes(attribute.String("exception.slug", "mastodon_post_failed"))
		return fmt.Errorf("Failed to post to Mastodon: %v", err)
	}

	fmt.Printf("Successfully posted! URL: %s\n", status.URL)

	fmt.Println("Posted to Mastodon")
	return nil
}
