package gateway

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/concrnt/chunkline"
	"github.com/patrickmn/go-cache"

	"github.com/totegamma/concrnt-playground"
	"github.com/totegamma/concrnt-playground/client"
	"github.com/totegamma/concrnt-playground/internal/usecase"
)

type ChunklineGateway struct {
	client   *client.Client
	cache    *cache.Cache
	resolver *chunkline.Client
}

func NewChunklineGateway(cl *client.Client) *ChunklineGateway {
	r := &resolver{
		client: cl,
		cache:  cache.New(10*time.Minute, 15*time.Minute),
	}
	return &ChunklineGateway{
		client:   cl,
		cache:    r.cache,
		resolver: chunkline.NewClient(r),
	}
}

func (g *ChunklineGateway) QueryDescending(ctx context.Context, uris []string, until time.Time, limit int) ([]chunkline.BodyItem, error) {
	return g.resolver.QueryDescending(ctx, uris, until, limit)
}

// resolver implements chunkline resolver callbacks.
type resolver struct {
	client *client.Client
	cache  *cache.Cache
}

func (r *resolver) ResolveTimelines(ctx context.Context, timelines []string) (map[string]chunkline.Manifest, error) {

	result := make(map[string]chunkline.Manifest)
	remaining := []string{}

	for _, tl := range timelines {
		if cached, found := r.cache.Get(tl); found {
			result[tl] = cached.(chunkline.Manifest)
		} else {
			remaining = append(remaining, tl)
		}
	}

	for _, tl := range remaining {
		var manifest chunkline.Manifest
		err := r.client.GetResource(ctx, tl, "application/chunkline+json", client.Options{}, &manifest)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve timeline %s: %v", tl, err)
		}
		result[tl] = manifest
		r.cache.Set(tl, manifest, cache.DefaultExpiration)
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
		if err != nil {
			return nil, err
		}

		results[tl] = result
	}
	return results, nil
}

func (r *resolver) LoadChunkBodies(ctx context.Context, query map[string]string) (map[string]chunkline.BodyChunk, error) {

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
		if err != nil {
			return nil, err
		}

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

var _ usecase.ChunklineGateway = (*ChunklineGateway)(nil)
