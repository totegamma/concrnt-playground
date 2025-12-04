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

type client struct {
	client          *http.Client
	userAgent       string
	defaultResolver string
}

func NewClient(defaultResolver string) *client {
	httpClient := http.Client{
		Timeout: defaultTimeout,
	}
	c := &client{
		client:          &httpClient,
		defaultResolver: defaultResolver,
	}
	httpClient.Transport = c
	return c
}

func (c *client) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", c.userAgent)
	return http.DefaultTransport.RoundTrip(req)
}

func (c *client) resolveResolver(ctx context.Context, resolver string) (string, error) {

	if resolver == "" {
		return c.defaultResolver, nil
	}

	if concrnt.IsCCID(resolver) {
		entity, err := c.GetEntity(ctx, resolver)
		if err != nil {
			return "", fmt.Errorf("failed to get entity for ccid %s: %v", resolver, err)
		}
		return entity.Domain, nil
	}

	if concrnt.IsCSID(resolver) {
		wkc, err := c.GetServer(ctx, resolver)
		if err != nil {
			return "", fmt.Errorf("failed to get server for csid %s: %v", resolver, err)
		}
		return wkc.Domain, nil
	}

	return resolver, nil
}

func (c *client) GetEntity(ctx context.Context, address string) (concrnt.Entity, error) {

	url := "https://" + c.defaultResolver + "/.well-known/concrnt/entity/" + address
	resp, err := http.Get(url)
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

func (c *client) GetServer(ctx context.Context, domainOrCSID string) (concrnt.WellKnownConcrnt, error) {

	domain, err := c.resolveResolver(ctx, domainOrCSID)
	if err != nil {
		return concrnt.WellKnownConcrnt{}, fmt.Errorf("failed to resolve ccid to domain: %v", err)
	}

	resp, err := http.Get("https://" + domain + "/.well-known/concrnt")
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

func (c *client) GetResource(ctx context.Context, uri string, accept string, result any) error {

	owner, key, err := concrnt.ParseCCURI(uri)
	if err != nil {
		return fmt.Errorf("failed to parse cc uri: %v", err)
	}

	info, err := c.GetServer(ctx, owner)
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

	fmt.Printf("Resolved endpoint: %s\n", endpoint)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get resource: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return fmt.Errorf("failed to decode resource: %v", err)
	}

	return nil
}
