package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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

	fmt.Println("Initialize Client with default resolver:", defaultResolver)
	c := &Client{
		client:          &httpClient,
		defaultResolver: defaultResolver,
	}
	httpClient.Transport = c
	return c
}

type Options struct {
	Resolver string
}

func (c *Client) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", c.userAgent)
	return http.DefaultTransport.RoundTrip(req)
}

func (c *Client) resolveResolver(ctx context.Context, resolver string) (string, error) {

	if resolver == "" {
		return c.defaultResolver, nil
	}

	if concrnt.IsCCID(resolver) {
		entity, err := c.GetEntity(ctx, resolver, "")
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

func (c *Client) HttpRequest(ctx context.Context, method, resolver, path string, response any) error {

	if resolver == "" || resolver == c.defaultResolver {
		resolver = c.defaultResolver
		fmt.Println("defaultResolver:", c.defaultResolver)
		fmt.Println("Using default resolver:", resolver)
	} else {
		domain, err := c.resolveResolver(ctx, resolver)
		if err != nil {
			return fmt.Errorf("failed to resolve resolver: %v", err)
		}
		resolver = domain
		fmt.Println("Resolved resolver to domain:", resolver)
	}

	if resolver == "" {
		return fmt.Errorf("resolver cannot be empty")
	}

	url := "https://" + resolver + path
	fmt.Printf("Making request to URL: %s\n", url)
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to perform request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return fmt.Errorf("failed to decode response: %v", err)
	}

	return nil

}

func (c *Client) HttpRequestText(ctx context.Context, method, resolver, path string) (string, error) {

	domain, err := c.resolveResolver(ctx, resolver)
	if err != nil {
		return "", fmt.Errorf("failed to resolve resolver: %v", err)
	}

	url := "https://" + domain + path
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to perform request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	return string(bytes), nil

}

func (c *Client) GetEntity(ctx context.Context, address string, resolver string) (concrnt.Entity, error) {

	if resolver == "" {
		resolver = c.defaultResolver
	}

	var entity concrnt.Entity
	err := c.HttpRequest(ctx, "GET", resolver, "/resource"+address, &entity)
	if err != nil {
		return concrnt.Entity{}, fmt.Errorf("failed to get entity: %v", err)
	}

	return entity, nil
}

func (c *Client) GetServer(ctx context.Context, domainOrCSID string) (concrnt.WellKnownConcrnt, error) {

	var wkc concrnt.WellKnownConcrnt
	err := c.HttpRequest(ctx, "GET", domainOrCSID, "/.well-known/concrnt", &wkc)
	if err != nil {
		return concrnt.WellKnownConcrnt{}, fmt.Errorf("failed to get well-known concrnt: %v", err)
	}

	return wkc, nil
}

func (c *Client) GetResource(ctx context.Context, uri string, accept string, result any) error {

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
