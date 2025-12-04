package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/totegamma/concrnt-playground"
)

const (
	defaultTimeout = 3 * time.Second
	maxFailCount   = 23 // max 10 minutes
)

type Client struct {
	client          *http.Client
	userAgent       string
	defaultResolver string
}

func New(defaultResolver string) *Client {
	httpClient := http.Client{
		Timeout: defaultTimeout,
	}
	c := &Client{
		client:          &httpClient,
		defaultResolver: defaultResolver,
	}
	httpClient.Transport = c
	return c
}

// NewClient is a backward-compatible constructor.
func NewClient(defaultResolver string) *Client {
	return New(defaultResolver)
}

func (c *Client) RoundTrip(req *http.Request) (*http.Response, error) {
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	return http.DefaultTransport.RoundTrip(req)
}

func (c *Client) resolverHost(resolver string) string {
	if resolver != "" {
		return resolver
	}
	return c.defaultResolver
}

func (c *Client) GetEntity(ctx context.Context, address string) (concrnt.Entity, error) {
	return c.GetEntityWithResolver(ctx, "", address)
}

func (c *Client) GetEntityWithResolver(ctx context.Context, resolver string, address string) (concrnt.Entity, error) {
	host := c.resolverHost(resolver)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+host+"/.well-known/concrnt/entity/"+address, nil)
	if err != nil {
		return concrnt.Entity{}, fmt.Errorf("failed to create entity request: %v", err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return concrnt.Entity{}, fmt.Errorf("failed to get entity: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return concrnt.Entity{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	var entity concrnt.Entity
	err = json.NewDecoder(resp.Body).Decode(&entity)
	if err != nil {
		return concrnt.Entity{}, fmt.Errorf("failed to decode entity: %v", err)
	}
	return entity, nil
}

func (c *Client) GetServer(ctx context.Context, domainOrCSID string) (concrnt.WellKnownConcrnt, error) {
	return c.GetServerWithResolver(ctx, "", domainOrCSID)
}

func (c *Client) GetServerWithResolver(ctx context.Context, resolver string, domainOrCSID string) (concrnt.WellKnownConcrnt, error) {
	switch {
	case concrnt.IsCSID(domainOrCSID):
		return c.fetchServerFromResolver(ctx, resolver, domainOrCSID)
	default:
		return c.fetchServerFromDomain(ctx, domainOrCSID)
	}
}

func (c *Client) fetchServerFromDomain(ctx context.Context, domain string) (concrnt.WellKnownConcrnt, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+domain+"/.well-known/concrnt", nil)
	if err != nil {
		return concrnt.WellKnownConcrnt{}, fmt.Errorf("failed to create request: %v", err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return concrnt.WellKnownConcrnt{}, fmt.Errorf("failed to get well-known concrnt: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return concrnt.WellKnownConcrnt{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var wkc concrnt.WellKnownConcrnt
	err = json.NewDecoder(resp.Body).Decode(&wkc)
	if err != nil {
		return concrnt.WellKnownConcrnt{}, fmt.Errorf("failed to decode well-known concrnt: %v", err)
	}
	return wkc, nil
}

func (c *Client) fetchServerFromResolver(ctx context.Context, resolver string, identifier string) (concrnt.WellKnownConcrnt, error) {
	host := c.resolverHost(resolver)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+host+"/.well-known/concrnt/server/"+identifier, nil)
	if err != nil {
		return concrnt.WellKnownConcrnt{}, fmt.Errorf("failed to create resolver request: %v", err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return concrnt.WellKnownConcrnt{}, fmt.Errorf("failed to resolve server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return concrnt.WellKnownConcrnt{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var wkc concrnt.WellKnownConcrnt
	if err := json.NewDecoder(resp.Body).Decode(&wkc); err != nil {
		return concrnt.WellKnownConcrnt{}, fmt.Errorf("failed to decode resolved server: %v", err)
	}
	return wkc, nil
}

func (c *Client) GetResource(ctx context.Context, uri string, accept string, result any) error {
	owner, key, err := concrnt.ParseCCURI(uri)
	if err != nil {
		return fmt.Errorf("failed to parse cc uri: %v", err)
	}

	info, err := c.resolveServerForOwner(ctx, owner)
	if err != nil {
		return fmt.Errorf("failed to get server info: %v", err)
	}

	endpoint, ok := info.Endpoints["net.concrnt.core.resource"]
	if !ok {
		return fmt.Errorf("resource endpoint not found")
	}

	endpoint = strings.ReplaceAll(endpoint, "{ccid}", owner)
	endpoint = strings.ReplaceAll(endpoint, "{key}", key)
	endpoint = strings.ReplaceAll(endpoint, "{uri}", url.QueryEscape(uri))
	endpoint = "https://" + info.Domain + endpoint

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get resource: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode resource: %v", err)
	}

	return nil
}

func (c *Client) resolveServerForOwner(ctx context.Context, owner string) (concrnt.WellKnownConcrnt, error) {
	switch {
	case concrnt.IsCCID(owner):
		entity, err := c.GetEntity(ctx, owner)
		if err != nil {
			return concrnt.WellKnownConcrnt{}, fmt.Errorf("failed to resolve entity: %v", err)
		}
		return c.fetchServerFromDomain(ctx, entity.Domain)
	case concrnt.IsCSID(owner):
		return c.fetchServerFromResolver(ctx, "", owner)
	default:
		return c.fetchServerFromDomain(ctx, owner)
	}
}
