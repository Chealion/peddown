package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/planar"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

// GeoLookup holds parsed polygons for ward and community lookups
type GeoLookup struct {
	wards       map[string]orb.MultiPolygon
	communities map[string]orb.MultiPolygon
}

// NewGeoLookup creates a new GeoLookup instance
func NewGeoLookup() *GeoLookup {
	return &GeoLookup{
		wards:       make(map[string]orb.MultiPolygon),
		communities: make(map[string]orb.MultiPolygon),
	}
}

// LoadWards loads ward polygons from database
func (gl *GeoLookup) LoadWards(ctx context.Context, db *IncidentDB) error {
	ctx, span := otel.Tracer("geo").Start(ctx, "geo.load-wards")
	defer span.End()

	wards, err := db.GetWards(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("get wards from db: %w", err)
	}

	for _, ward := range wards {
		mp, err := parseMultiPolygon(ward.Multipolygon)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("parse ward %s multipolygon: %w", ward.WardNum, err)
		}
		gl.wards[ward.WardNum] = mp
	}

	return nil
}

// LoadCommunities loads community polygons from database
func (gl *GeoLookup) LoadCommunities(ctx context.Context, db *IncidentDB) error {
	ctx, span := otel.Tracer("geo").Start(ctx, "geo.load-communities")
	defer span.End()

	communities, err := db.GetCommunities(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("get communities from db: %w", err)
	}

	for _, community := range communities {
		mp, err := parseMultiPolygon(community.Multipolygon)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return fmt.Errorf("parse community %s multipolygon: %w", community.CommCode, err)
		}
		gl.communities[community.CommCode] = mp
	}

	return nil
}

// FindWard returns the ward number containing the given point, or empty string if not found
func (gl *GeoLookup) FindWard(lat, lon float64) string {
	point := orb.Point{lon, lat} // orb uses [lon, lat] order

	for wardNum, mp := range gl.wards {
		if multiPolygonContains(mp, point) {
			return wardNum
		}
	}

	return ""
}

// FindCommunity returns the community code containing the given point, or empty string if not found
func (gl *GeoLookup) FindCommunity(lat, lon float64) string {
	point := orb.Point{lon, lat} // orb uses [lon, lat] order

	for commCode, mp := range gl.communities {
		if multiPolygonContains(mp, point) {
			return commCode
		}
	}

	return ""
}

// parseMultiPolygon parses a GeoJSON multipolygon into an orb.MultiPolygon
func parseMultiPolygon(geoJSON json.RawMessage) (orb.MultiPolygon, error) {
	// The Socrata API returns GeoJSON geometry objects
	var geometry geojson.Geometry
	if err := json.Unmarshal(geoJSON, &geometry); err != nil {
		return nil, fmt.Errorf("unmarshal geojson: %w", err)
	}

	geo := geometry.Geometry()

	switch g := geo.(type) {
	case orb.MultiPolygon:
		return g, nil
	case orb.Polygon:
		// Convert single polygon to multipolygon
		return orb.MultiPolygon{g}, nil
	default:
		return nil, fmt.Errorf("unexpected geometry type: %T", geo)
	}
}

// multiPolygonContains checks if a point is inside any polygon of the multipolygon
func multiPolygonContains(mp orb.MultiPolygon, point orb.Point) bool {
	for _, polygon := range mp {
		if planar.PolygonContains(polygon, point) {
			return true
		}
	}
	return false
}
