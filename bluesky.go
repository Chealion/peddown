package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

type BskyClient struct {
	Handle     string
	Password   string
	AccessJwt  string
	UserDID    string
	HttpClient *http.Client
}

type BlueskyLoginRequest struct {
	Identifier string `json:"identifier"` // Handle or Email
	Password   string `json:"password"`   // App Password
}

type BlueskyLoginResponse struct {
	AccessJwt string `json:"accessJwt"`
	Did       string `json:"did"`
	Handle    string `json:"handle"`
}

// BlueskyPost is the actual content of the post (Record in Bluesky's API docs)
type BlueskyPost struct {
	Type      string    `json:"$type"` // Must be "app.bsky.feed.post"
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"createdAt"`
	Facet     []Facet   `json:"facets,omitempty"`
}

// Bluesky Facets - which contain features (mentions or links)
type FacetSpan struct {
	Start  int
	End    int
	Handle string
	URL    string
	Tag    string
}

type FacetIndex struct {
	ByteStart int `json:"byteStart"`
	ByteEnd   int `json:"byteEnd"`
}

type MentionFeature struct {
	Type string `json:"$type"`
	DID  string `json:"did"`
}

type LinkFeature struct {
	Type string `json:"$type"`
	URI  string `json:"uri"`
}

type HashtagFeature struct {
	Type string `json:"$type"`
	Tag  string `json:"tag"`
}

type ResolveHandleResponse struct {
	DID string `json:"did"`
}

type Facet struct {
	Index    FacetIndex `json:"index"`
	Features []any      `json:"features"`
}

// BlueskyCreateRecordRequest is the RPC wrapper to save the post to your repo
type BlueskyCreateRecordRequest struct {
	Repo       string      `json:"repo"`       // Your DID
	Collection string      `json:"collection"` // "app.bsky.feed.post"
	Record     BlueskyPost `json:"record"`
}

// BlueskyAPIError handles error messages from the AT Protocol
type BlueskyAPIError struct {
	ErrorName string `json:"error"`
	Message   string `json:"message"`
}

func NewBlueskyClient(handle, password string) *BskyClient {
	return &BskyClient{
		Handle:     handle,
		Password:   password,
		HttpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *BskyClient) Authenticate() error {
	endpoint := "https://bsky.social/xrpc/com.atproto.server.createSession"

	reqBody := BlueskyLoginRequest{
		Identifier: c.Handle,
		Password:   c.Password,
	}

	jsonData, _ := json.Marshal(reqBody)
	resp, err := c.HttpClient.Post(endpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login failed: status %d", resp.StatusCode)
	}

	var loginResp BlueskyLoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return err
	}

	c.AccessJwt = loginResp.AccessJwt
	c.UserDID = loginResp.Did
	return nil
}

func parseMentions(text string) []FacetSpan {
	var spans []FacetSpan
	// https://github.com/bluesky-social/indigo/blob/main/atproto/syntax/handle.go#L11
	mentionRegex := regexp.MustCompile(`([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?`)

	textBytes := []byte(text)
	matches := mentionRegex.FindAllSubmatchIndex(textBytes, -1)

	for _, match := range matches {
		//0 and 1 are the start and end indices of the entire handle including domain
		start := match[0]
		end := match[1]
		handleBytes := textBytes[start:end] // +1 to skip the @

		spans = append(spans, FacetSpan{
			Start:  start,
			End:    end,
			Handle: string(handleBytes),
		})
	}

	return spans
}

func parseHashtags(text string) []FacetSpan {
	var spans []FacetSpan
	hashtagRegex := regexp.MustCompile(`#[a-zA-Z0-9]*`)

	textBytes := []byte(text)
	matches := hashtagRegex.FindAllSubmatchIndex(textBytes, -1)

	for _, match := range matches {
		//0 and 1 are the start and end indices of the entire handle including domain
		start := match[0]
		end := match[1]
		hashtagBytes := textBytes[start:end] // +1 to skip the @

		spans = append(spans, FacetSpan{
			Start: start,
			End:   end,
			Tag:   string(hashtagBytes),
		})
	}

	return spans
}

func parseFacets(message string) ([]Facet, error) {
	var facets []Facet

	// Parse mentions and resolve to DIDs
	for _, m := range parseMentions(message) {
		// Resolve handle to DID
		resolveURL := fmt.Sprintf("https://bsky.social/xrpc/com.atproto.identity.resolveHandle?handle=%s",
			url.QueryEscape(m.Handle))

		resp, err := http.Get(resolveURL)
		if err != nil {
			return nil, fmt.Errorf("resolve handle: %w", err)
		}

		// If the handle can't be resolved, just skip it!
		if resp.StatusCode == 400 {
			resp.Body.Close()
			continue
		}

		var resolveResp ResolveHandleResponse
		if err := json.NewDecoder(resp.Body).Decode(&resolveResp); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decode resolve response: %w", err)
		}
		resp.Body.Close()

		facets = append(facets, Facet{
			Index: FacetIndex{
				// Go back one to include the @ symbol
				ByteStart: m.Start - 1,
				ByteEnd:   m.End,
			},
			Features: []any{
				MentionFeature{
					Type: "app.bsky.richtext.facet#mention",
					DID:  resolveResp.DID,
				},
			},
		})
	}

	// Parse hashtags and link to https://bsky.app/hashtag/<hashtag>
	for _, m := range parseHashtags(message) {
		facets = append(facets, Facet{
			Index: FacetIndex{
				// Go back one to include the @ symbol
				ByteStart: m.Start - 1,
				ByteEnd:   m.End,
			},
			Features: []any{
				HashtagFeature{
					Type: "app.bsky.richtext.facet#tag",
					Tag:  m.Tag,
				},
			},
		})
	}

	return facets, nil
}

func (c *BskyClient) CreateBlueskyPost(message string) error {
	endpoint := "https://bsky.social/xrpc/com.atproto.repo.createRecord"

	// Parse out facets to include in the post
	facets, err := parseFacets(message)

	post := BlueskyPost{
		Type:      "app.bsky.feed.post",
		Text:      message,
		Facet:     facets,
		CreatedAt: time.Now().UTC(),
	}

	reqBody := BlueskyCreateRecordRequest{
		Repo:       c.UserDID, // The post belongs to your DID
		Collection: "app.bsky.feed.post",
		Record:     post,
	}

	jsonData, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.AccessJwt)

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var apiErr BlueskyAPIError
		json.NewDecoder(resp.Body).Decode(&apiErr)
		return fmt.Errorf("post failed (%s): %s", apiErr.ErrorName, apiErr.Message)
	}

	return nil
}

func postToBluesky(ctx context.Context, message string) error {
	ctx, span := otel.Tracer("social").Start(ctx, "post-to-bluesky")
	defer span.End()

	span.SetAttributes(attribute.String("social_media.platform", "bluesky"))

	handle := os.Getenv("BSKY_HANDLE")
	password := os.Getenv("BSKY_PASSWORD")

	if handle == "" || password == "" {
		err := fmt.Errorf("Error: BSKY_HANDLE and BSKY_PASSWORD environment variables are required - did not post to Bluesky")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.SetAttributes(attribute.String("exception.slug", "bluesky_env_missing"))
		return err
	}

	client := NewBlueskyClient(handle, password)

	if err := client.Authenticate(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.SetAttributes(attribute.String("exception.slug", "bluesky_auth_failed"))
		return fmt.Errorf("Bluesky authentication failed: %v", err)
	}

	if err := client.CreateBlueskyPost(message); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.SetAttributes(attribute.String("exception.slug", "bluesky_post_failed"))
		return fmt.Errorf("Failed to post: %v", err)
	}

	fmt.Println("Posted to Bluesky")
	return nil
}
