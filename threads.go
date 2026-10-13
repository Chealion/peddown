package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// threadsGraphURL is the Threads Graph API root. Token endpoints live directly
// under it; publishing endpoints are versioned under threadsBaseURL.
const threadsGraphURL = "https://graph.threads.net"

// threadsBaseURL is the versioned Threads Graph API root for publishing.
const threadsBaseURL = threadsGraphURL + "/v1.0"

// threadsHTTPClient is shared by all Threads calls so a hung request cannot block
// a run indefinitely.
var threadsHTTPClient = &http.Client{Timeout: 30 * time.Second}

// Meta only allows refreshing a long-lived token once it is at least 24 hours
// old, so there is no point trying more often than that.
const threadsTokenRefreshInterval = 24 * time.Hour

// threadsMaxTextLength is the documented character limit for a text-only post.
// Emojis count as their UTF-8 byte length, so this is checked in bytes.
const threadsMaxTextLength = 500

// ThreadsClient handles posting to Threads (Meta's graph.threads.net API).
type ThreadsClient struct {
	UserID      string
	AccessToken string
	client      *http.Client
}

// ThreadsContainer is the response from creating a media container (step 1).
type ThreadsContainer struct {
	ID string `json:"id"`
}

// ThreadsPost is the response from publishing a container (step 2).
type ThreadsPost struct {
	ID string `json:"id"`
}

// ThreadsAPIError handles error messages from the Threads Graph API.
type ThreadsAPIError struct {
	Err struct {
		Message      string `json:"message"`
		Type         string `json:"type"`
		Code         int    `json:"code"`
		ErrorSubcode int    `json:"error_subcode"`
		FbtraceID    string `json:"fbtrace_id"`
	} `json:"error"`
}

func (e *ThreadsAPIError) Error() string {
	return fmt.Sprintf("threads API error (code %d, subcode %d): %s",
		e.Err.Code, e.Err.ErrorSubcode, e.Err.Message)
}

// IsTokenInvalid reports whether the API rejected the access token. Meta uses
// code 190 (OAuthException) for an invalid, revoked or expired token; subcode
// 463 is specifically "session has expired".
func (e *ThreadsAPIError) IsTokenInvalid() bool {
	return e.Err.Code == 190
}

// ThreadsTokenResponse is the response from the token refresh endpoint.
type ThreadsTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// NewThreadsClient creates a new Threads client from the given access token and
// the THREADS_USER_ID environment variable.
func NewThreadsClient(accessToken string) (*ThreadsClient, error) {
	userID := os.Getenv("THREADS_USER_ID")

	if userID == "" {
		return nil, fmt.Errorf("THREADS_USER_ID environment variable not set")
	}
	if accessToken == "" {
		return nil, fmt.Errorf("Threads access token is empty")
	}

	return &ThreadsClient{
		UserID:      userID,
		AccessToken: accessToken,
		client:      threadsHTTPClient,
	}, nil
}

// CreateContainer creates a text media container (step 1 of 2).
// It returns the container ID to be passed to PublishContainer.
func (c *ThreadsClient) CreateContainer(ctx context.Context, text string) (string, error) {
	endpoint := fmt.Sprintf("%s/%s/threads", threadsBaseURL, c.UserID)

	form := url.Values{}
	form.Set("media_type", "TEXT")
	form.Set("text", text)
	form.Set("access_token", c.AccessToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("create container request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("create container: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", decodeThreadsError(resp)
	}

	var container ThreadsContainer
	if err := json.NewDecoder(resp.Body).Decode(&container); err != nil {
		return "", fmt.Errorf("decode container response: %w", err)
	}

	if container.ID == "" {
		return "", fmt.Errorf("create container: empty container ID in response")
	}

	return container.ID, nil
}

// PublishContainer publishes a previously created container (step 2 of 2).
// It returns the published Threads media ID.
func (c *ThreadsClient) PublishContainer(ctx context.Context, creationID string) (string, error) {
	endpoint := fmt.Sprintf("%s/%s/threads_publish", threadsBaseURL, c.UserID)

	form := url.Values{}
	form.Set("creation_id", creationID)
	form.Set("access_token", c.AccessToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("publish container request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("publish container: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", decodeThreadsError(resp)
	}

	var post ThreadsPost
	if err := json.NewDecoder(resp.Body).Decode(&post); err != nil {
		return "", fmt.Errorf("decode publish response: %w", err)
	}

	if post.ID == "" {
		return "", fmt.Errorf("publish container: empty media ID in response")
	}

	return post.ID, nil
}

// decodeThreadsError converts a non-200 Threads API response into an error. When
// the body carries a structured error it is returned as *ThreadsAPIError so
// callers can inspect the code (for example to detect an expired token).
func decodeThreadsError(resp *http.Response) error {
	var apiErr ThreadsAPIError
	if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil || apiErr.Err.Message == "" {
		return fmt.Errorf("threads API returned status %d", resp.StatusCode)
	}

	return &apiErr
}

// isThreadsTokenInvalid reports whether err is a Threads API error rejecting the
// access token (expired, revoked or otherwise invalid).
func isThreadsTokenInvalid(err error) bool {
	var apiErr *ThreadsAPIError
	return errors.As(err, &apiErr) && apiErr.IsTokenInvalid()
}

// RefreshLongLivedToken refreshes a long-lived Threads access token, extending
// it by 60 days. The token must be at least 24 hours old and not expired. No app
// secret is required for a refresh.
func RefreshLongLivedToken(ctx context.Context, accessToken string) (string, int, error) {
	query := url.Values{}
	query.Set("grant_type", "th_refresh_token")
	query.Set("access_token", accessToken)

	return requestThreadsToken(ctx, threadsGraphURL+"/refresh_access_token?"+query.Encode())
}

// requestThreadsToken performs a GET against a Threads token endpoint and
// returns the resulting access token and its lifetime in seconds.
func requestThreadsToken(ctx context.Context, endpoint string) (string, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		// The endpoint embeds credentials in its query string, so keep the error
		// free of the URL.
		return "", 0, fmt.Errorf("build threads token request failed")
	}

	resp, err := threadsHTTPClient.Do(req)
	if err != nil {
		// The token is part of the query string, and url.Error includes the
		// full URL, so unwrap it to avoid leaking the token into logs.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			return "", 0, fmt.Errorf("threads token request failed: %s: %w", urlErr.Op, urlErr.Err)
		}
		return "", 0, fmt.Errorf("threads token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", 0, decodeThreadsError(resp)
	}

	var tokenResp ThreadsTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", 0, fmt.Errorf("decode threads token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return "", 0, fmt.Errorf("threads token response did not include an access token")
	}

	return tokenResp.AccessToken, tokenResp.ExpiresIn, nil
}

// readThreadsToken reads the access token from the file at path. The file's
// modification time is returned so callers can decide whether it is old enough
// to refresh (it is updated whenever a refreshed token is written back).
func readThreadsToken(path string) (string, time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("stat threads token file %s: %w", path, err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("read threads token file %s: %w", path, err)
	}

	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", time.Time{}, fmt.Errorf("threads token file %s is empty", path)
	}

	return token, info.ModTime(), nil
}

// writeThreadsToken writes the access token to path atomically and with 0600
// permissions, so a refresh never leaves a partially written or world-readable
// token behind.
func writeThreadsToken(path, token string) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".threads-token-*")
	if err != nil {
		return fmt.Errorf("create temp token file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod temp token file: %w", err)
	}
	if _, err := tmp.WriteString(token); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp token file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp token file: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace token file %s: %w", path, err)
	}

	return nil
}

// resolveThreadsAccessToken returns the access token to use. The token lives in
// the file named by THREADS_TOKEN_FILE and is refreshed in place once it is old
// enough for Meta to allow a refresh.
func resolveThreadsAccessToken(ctx context.Context) (string, error) {
	path := os.Getenv("THREADS_TOKEN_FILE")
	if path == "" {
		return "", fmt.Errorf("THREADS_TOKEN_FILE environment variable not set")
	}

	token, refreshedAt, err := readThreadsToken(path)
	if err != nil {
		return "", fmt.Errorf("%w (populate it with a long-lived Threads token)", err)
	}

	if time.Since(refreshedAt) < threadsTokenRefreshInterval {
		return token, nil
	}

	newToken, _, err := RefreshLongLivedToken(ctx, token)
	if err != nil {
		if isThreadsTokenInvalid(err) {
			log.Printf("error: Threads token in %s is expired or invalid and cannot be refreshed; replace it with a new long-lived token", path)
		} else {
			// Non-fatal: the existing token may still be valid, so keep using
			// it rather than dropping a post over a failed refresh.
			log.Printf("warning: could not refresh Threads token, using existing token: %v", err)
		}
		return token, nil
	}

	if err := writeThreadsToken(path, newToken); err != nil {
		return "", err
	}

	log.Printf("Refreshed Threads access token in %s", path)
	return newToken, nil
}

// refreshThreadsToken forces a refresh of the token in THREADS_TOKEN_FILE and
// writes the result back. Used by the `threads-refresh` command.
func refreshThreadsToken(ctx context.Context) (int, error) {
	path := os.Getenv("THREADS_TOKEN_FILE")
	if path == "" {
		return 0, fmt.Errorf("THREADS_TOKEN_FILE environment variable not set")
	}

	token, _, err := readThreadsToken(path)
	if err != nil {
		return 0, err
	}

	newToken, expiresIn, err := RefreshLongLivedToken(ctx, token)
	if err != nil {
		return 0, err
	}

	if err := writeThreadsToken(path, newToken); err != nil {
		return 0, err
	}

	return expiresIn, nil
}

// threadsPublishAttempts and threadsPublishBackoff guard against the documented
// container race: publishing can fail transiently while Threads finishes
// processing the container. Text containers are usually ready immediately, so a
// couple of short retries is enough without blocking the caller.
const (
	threadsPublishAttempts = 3
	threadsPublishBackoff  = 3 * time.Second
)

// threadsErrorSlug maps an API error to an exception slug and, when the token is
// rejected, logs a message pointing at the token file. It returns an empty slug
// for errors that are not token related.
func threadsErrorSlug(err error) string {
	var apiErr *ThreadsAPIError
	if !errors.As(err, &apiErr) {
		return ""
	}

	if apiErr.IsTokenInvalid() {
		log.Printf("error: Threads rejected the access token (code %d, subcode %d); re-populate %s with a fresh long-lived token",
			apiErr.Err.Code, apiErr.Err.ErrorSubcode, os.Getenv("THREADS_TOKEN_FILE"))
		return "threads_token_expired"
	}

	return ""
}

func postToThreads(ctx context.Context, message string) error {
	ctx, span := otel.Tracer("social").Start(ctx, "post-to-threads")
	defer span.End()

	span.SetAttributes(attribute.String("social_media.platform", "threads"))

	accessToken, err := resolveThreadsAccessToken(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.SetAttributes(attribute.String("exception.slug", "threads_token_file_missing"))
		return fmt.Errorf("Failed to post to Threads: %v", err)
	}

	client, err := NewThreadsClient(accessToken)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.SetAttributes(attribute.String("exception.slug", "threads_env_missing"))
		return fmt.Errorf("Failed to post to Threads: %v", err)
	}

	if len(message) > threadsMaxTextLength {
		err := fmt.Errorf("message is %d characters, Threads allows %d", len(message), threadsMaxTextLength)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.SetAttributes(attribute.String("exception.slug", "threads_message_too_long"))
		return fmt.Errorf("Failed to post to Threads: %v", err)
	}

	containerID, err := client.CreateContainer(ctx, message)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		slug := threadsErrorSlug(err)
		if slug == "" {
			slug = "threads_container_failed"
		}
		span.SetAttributes(attribute.String("exception.slug", slug))
		return fmt.Errorf("Failed to create Threads container: %v", err)
	}

	span.SetAttributes(attribute.String("threads.container_id", containerID))

	var postID string
	for attempt := 1; attempt <= threadsPublishAttempts; attempt++ {
		postID, err = client.PublishContainer(ctx, containerID)
		if err == nil {
			break
		}

		if attempt < threadsPublishAttempts {
			select {
			case <-ctx.Done():
				err = ctx.Err()
				attempt = threadsPublishAttempts
			case <-time.After(threadsPublishBackoff):
			}
		}
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		slug := threadsErrorSlug(err)
		if slug == "" {
			slug = "threads_publish_failed"
		}
		span.SetAttributes(attribute.String("exception.slug", slug))
		return fmt.Errorf("Failed to publish Threads post: %v", err)
	}

	span.SetAttributes(attribute.String("threads.post_id", postID))
	fmt.Printf("Successfully posted to Threads! Post ID: %s\n", postID)
	return nil
}
