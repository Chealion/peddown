package main

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

func postToThreads(ctx context.Context, message string) error {
	_, span := otel.Tracer("social").Start(ctx, "post-to-threads")
	defer span.End()

	span.SetAttributes(attribute.String("social_media.platform", "threads"))

	return nil
}
