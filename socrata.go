package main

import (
	"context"
	"fmt"
	"os"

	requests "github.com/carlmjohnson/requests"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// Socrata Query Structs
type SocrataPage struct {
	PageNumber int
	PageSize   int
}

type SocrataQuery struct {
	Query string
	Page  SocrataPage
}

// Socrata Connection Structs
type SocrataClient struct {
	domain   string
	appToken string
}

func NewSocrataClient() (*SocrataClient, error) {
	domain := "data.calgary.ca"
	appToken := os.Getenv("SOCRATA_APP_TOKEN")

	return &SocrataClient{
		domain:   domain,
		appToken: appToken,
	}, nil
}

// FetchCommunities retrieves community boundaries from ab7m-fwn6 dataset
func (sc *SocrataClient) FetchCommunities(ctx context.Context) ([]Community, error) {
	ctx, span := otel.Tracer("socrata").Start(ctx, "socrata.fetch-communities")
	defer span.End()

	query := SocrataQuery{
		Query: "SELECT comm_code, name, multipolygon",
		Page: SocrataPage{
			PageNumber: 1,
			PageSize:   500,
		},
	}

	endpoint := fmt.Sprintf("https://%s/api/v3/views/%s/query.json", sc.domain, "ab7m-fwn6")

	var communities []Community
	err := requests.
		URL(endpoint).
		BodyJSON(&query).
		Header("X-App-Token", sc.appToken).
		ToJSON(&communities).
		Fetch(ctx)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.SetAttributes(attribute.String("exception.slug", "socrata_fetch_communities_failed"))
		return nil, fmt.Errorf("fetch communities: %w", err)
	}

	return communities, nil
}

// FetchWards retrieves ward boundaries from tz8z-hyaz dataset
func (sc *SocrataClient) FetchWards(ctx context.Context) ([]Ward, error) {
	ctx, span := otel.Tracer("socrata").Start(ctx, "socrata.fetch-wards")
	defer span.End()

	query := SocrataQuery{
		Query: "SELECT ward_num, multipolygon",
		Page: SocrataPage{
			PageNumber: 1,
			PageSize:   50,
		},
	}

	endpoint := fmt.Sprintf("https://%s/api/v3/views/%s/query.json", sc.domain, "tz8z-hyaz")

	var wards []Ward
	err := requests.
		URL(endpoint).
		BodyJSON(&query).
		Header("X-App-Token", sc.appToken).
		ToJSON(&wards).
		Fetch(ctx)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.SetAttributes(attribute.String("exception.slug", "socrata_fetch_wards_failed"))
		return nil, fmt.Errorf("fetch wards: %w", err)
	}

	return wards, nil
}

func (sc *SocrataClient) FetchIncidents(ctx context.Context, limit int) ([]Incident, error) {
	ctx, span := otel.Tracer("socrata").Start(ctx, "socrata.fetch-incidents")
	defer span.End()

	span.SetAttributes(attribute.Int("socrata.limit", limit))

	query := SocrataQuery{
		Query: "SELECT * WHERE description LIKE '%pedestrian%' ORDER BY start_dt desc",
		Page: SocrataPage{
			PageNumber: 1,
			PageSize:   limit,
		},
	}

	endpoint := fmt.Sprintf("https://%s/api/v3/views/%s/query.json", sc.domain, "35ra-9556")

	var incidents []Incident
	err := requests.
		URL(endpoint).
		BodyJSON(&query).
		Header("X-App-Token", sc.appToken).
		ToJSON(&incidents).
		Fetch(ctx)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.SetAttributes(attribute.String("exception.slug", "socrata_fetch_incidents_failed"))
		return nil, fmt.Errorf("fetch incidents: %w", err)
	}

	return incidents, nil
}
