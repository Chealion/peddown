package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/dghubble/oauth1"
	"github.com/g8rswimmer/go-twitter/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// Empty since it's not used
type authorize struct{}

func (a authorize) Add(req *http.Request) {}

func postToX(ctx context.Context, message string) error {
	ctx, span := otel.Tracer("social").Start(ctx, "post-to-x")
	defer span.End()

	span.SetAttributes(attribute.String("social_media.platform", "x"))

	xAPIKey := os.Getenv("X_API_KEY")
	xAPISecret := os.Getenv("X_API_SECRET")
	xAccessToken := os.Getenv("X_ACCESS_TOKEN")
	xAccessSecret := os.Getenv("X_ACCESS_TOKEN_SECRET")

	if xAPIKey == "" || xAPISecret == "" || xAccessToken == "" || xAccessSecret == "" {
		err := fmt.Errorf("Error: X environment variables are required - did not post to X")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.SetAttributes(attribute.String("exception.slug", "x_env_missing"))
		return err
	}

	config := oauth1.NewConfig(xAPIKey, xAPISecret)
	xHTTPClient := config.Client(oauth1.NoContext, &oauth1.Token{
		Token:       xAccessToken,
		TokenSecret: xAccessSecret,
	})

	xClient := &twitter.Client{
		Authorizer: authorize{},
		Client:     xHTTPClient,
		Host:       "https://api.twitter.com",
	}

	req := twitter.CreateTweetRequest{
		Text: message,
	}

	tweetResponse, err := xClient.CreateTweet(ctx, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.SetAttributes(attribute.String("exception.slug", "x_post_failed"))
		return fmt.Errorf("Failed to post to X: %v", err)
	}

	enc, err := json.MarshalIndent(tweetResponse, "", "    ")
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.SetAttributes(attribute.String("exception.slug", "x_marshal_failed"))
		return fmt.Errorf("Failed to post to X: %v", err)
	}
	fmt.Println(string(enc))

	fmt.Println("Posted to X")
	return nil
}
