package application

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/concrnt/chunkline"
	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/client"
	"github.com/totegamma/concrnt-playground/internal/infrastructure/repository"
)

type ChunklineApplication struct {
	repo     *repository.ChunklineRepository
	resolver *chunkline.Client
}

func NewChunklineApplication(
	repo *repository.ChunklineRepository,
	client *client.Client,
) *ChunklineApplication {

	resolver := &resolver{
		client: client,
	}
	clc := chunkline.NewClient(resolver)

	return &ChunklineApplication{
		repo:     repo,
		resolver: clc,
	}
}

func (app *ChunklineApplication) GetChunklineManifest(ctx context.Context, uri string) (*chunkline.Manifest, error) {
	return app.repo.GetChunklineManifest(ctx, uri)
}

func (app *ChunklineApplication) LookupLocalItrs(ctx context.Context, uris []string, chunkID int64) (map[string]int64, error) {
	return app.repo.LookupLocalItrs(ctx, uris, chunkID)
}

func (app *ChunklineApplication) LoadLocalBody(ctx context.Context, uri string, chunkID int64) ([]chunkline.BodyItem, error) {
	return app.repo.LoadLocalBody(ctx, uri, chunkID)
}

func (app *ChunklineApplication) GetRecent(ctx context.Context, uris []string, until time.Time) ([]chunkline.BodyItem, error) {

	items, err := app.resolver.QueryDescending(ctx, uris, until, 100)
	if err != nil {
		return nil, fmt.Errorf("failed to query descending: %v", err)
	}

	return items, nil

}

type resolver struct {
	client *client.Client
}

func (r *resolver) ResolveTimelines(ctx context.Context, timelines []string) (map[string]chunkline.Manifest, error) {

	result := make(map[string]chunkline.Manifest)
	for _, tl := range timelines {
		var manifest chunkline.Manifest
		err := r.client.GetResource(ctx, tl, "application/chunkline+json", client.Options{}, &manifest)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve timeline %s: %v", tl, err)
		}
		result[tl] = manifest
	}
	return result, nil

}

func (r *resolver) GetRemovedItems(ctx context.Context, timelines []string) (map[string][]string, error) {
	result := make(map[string][]string)
	for _, tl := range timelines {
		result[tl] = []string{}
	}
	return result, nil
}

func (r *resolver) LookupChunkItrs(ctx context.Context, timelines []string, until time.Time) (map[string]string, error) {
	fmt.Println("Looking up chunk iterators...")

	manifests, err := r.ResolveTimelines(ctx, timelines)
	if err != nil {
		return nil, err
	}

	results := make(map[string]string)
	for _, tl := range timelines {

		manifest := manifests[tl]

		if manifest.Descending.Iterator == "" {
			return nil, fmt.Errorf("timeline %s does not support descending iteration", tl)
		}

		owner, _, err := concrnt.ParseCCURI(tl)
		if err != nil {
			return nil, fmt.Errorf("failed to parse timeline URI %s: %v", tl, err)
		}

		result, err := r.client.HttpRequestText(
			ctx,
			"GET",
			owner,
			strings.ReplaceAll(manifest.Descending.Iterator, "{chunk}", fmt.Sprintf("%d", manifest.Time2Chunk(until))),
		)

		results[tl] = result
	}
	return results, nil
}

func (r *resolver) LoadChunkBodies(ctx context.Context, query map[string]string) (map[string]chunkline.BodyChunk, error) {
	fmt.Println("Loading chunk bodies...")

	uris := []string{}
	for itr := range query {
		uris = append(uris, itr)
	}

	manifests, err := r.ResolveTimelines(ctx, uris)
	if err != nil {
		return nil, err
	}

	result := make(map[string]chunkline.BodyChunk)
	for tl, itr := range query {

		manifest := manifests[tl]

		owner, _, err := concrnt.ParseCCURI(tl)
		if err != nil {
			return nil, fmt.Errorf("failed to parse timeline URI %s: %v", tl, err)
		}

		var items []chunkline.BodyItem
		err = r.client.HttpRequest(
			ctx,
			"GET",
			owner,
			strings.ReplaceAll(manifest.Descending.Body, "{chunk}", itr),
			&items,
		)

		chunkID, err := strconv.ParseInt(itr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid chunk ID %s: %v", itr, err)
		}

		result[tl] = chunkline.BodyChunk{
			URI:     tl,
			ChunkID: chunkID,
			Items:   items,
		}

	}
	return result, nil

}
