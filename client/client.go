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

	"github.com/patrickmn/go-cache"
	"github.com/totegamma/concrnt-playground"
)

const (
	defaultTimeout = 3 * time.Second
	maxFailCount   = 23 // max 10 minutes
)

type Client struct {
	client          *http.Client
	cache           *cache.Cache
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
		cache:           cache.New(10*time.Minute, 15*time.Minute),
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
	fmt.Println("Resolving resolver:", resolver)

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
		wkc, err := c.GetServer(ctx, resolver, "")
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
	fmt.Printf("Making request to URL: %s\n", url)
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

func (c *Client) GetEntity(ctx context.Context, address string, hint string) (concrnt.Entity, error) {
	fmt.Printf("Getting entity for address: %s with hint: %s\n", address, hint)

	cacheKey := "entity:" + address
	x, found := c.cache.Get(cacheKey)
	if found {
		fmt.Println("Cache hit for entity:", address)
		return x.(concrnt.Entity), nil
	}

	opts := Options{Resolver: c.defaultResolver}
	if hint != "" {
		opts.Resolver = hint
	}

	var entity concrnt.Entity
	err := c.GetResource(ctx, "cc://"+address, "application/json", opts, &entity)
	if err != nil {
		return concrnt.Entity{}, fmt.Errorf("failed to get entity: %v", err)
	}

	c.cache.Set(cacheKey, entity, cache.DefaultExpiration)

	return entity, nil
}

func (c *Client) GetServer(ctx context.Context, domainOrCSID, hint string) (concrnt.WellKnownConcrnt, error) {
	fmt.Printf("Getting server for domain or CSID: %s\n", domainOrCSID)

	cacheKey := "server:" + domainOrCSID

	x, found := c.cache.Get(cacheKey)
	if found {
		fmt.Println("Cache hit for well-known concrnt:", domainOrCSID)
		return x.(concrnt.WellKnownConcrnt), nil
	}

	if concrnt.IsCSID(domainOrCSID) {
		var wkc concrnt.WellKnownConcrnt
		err := c.GetResource(ctx, "cc://"+domainOrCSID, "application/json", Options{Resolver: c.defaultResolver}, &wkc)
		if err != nil {
			return concrnt.WellKnownConcrnt{}, fmt.Errorf("failed to get well-known concrnt: %v", err)
		}
		c.cache.Set(cacheKey, wkc, cache.DefaultExpiration)
		return wkc, nil
	} else {

		domain := domainOrCSID
		if hint != "" {
			domain = hint
		}

		url := "https://" + domain + "/.well-known/concrnt"
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return concrnt.WellKnownConcrnt{}, fmt.Errorf("failed to create request: %v", err)
		}
		resp, err := c.client.Do(req)
		if err != nil {
			return concrnt.WellKnownConcrnt{}, fmt.Errorf("failed to perform request: %v", err)
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
		c.cache.Set(cacheKey, wkc, cache.DefaultExpiration)
		return wkc, nil
	}
}

func (c *Client) GetResource(ctx context.Context, uri string, accept string, opts Options, result any) error {
	fmt.Printf("Getting resource for URI: %s\n", uri)

	owner, key, err := concrnt.ParseCCURI(uri)
	if err != nil {
		return fmt.Errorf("failed to parse cc uri: %v", err)
	}

	fmt.Printf("Parsed URI - Owner: %s, Key: %s\n", owner, key)

	var info concrnt.WellKnownConcrnt
	if opts.Resolver != "" {
		info, err = c.GetServer(ctx, opts.Resolver, "")
		if err != nil {
			return fmt.Errorf("failed to get server for resolver %s: %v", opts.Resolver, err)
		}
	} else {
		domain, err := c.resolveResolver(ctx, owner)
		if err != nil {
			return fmt.Errorf("failed to resolve default resolver: %v", err)
		}
		info, err = c.GetServer(ctx, domain, "")
		if err != nil {
			return fmt.Errorf("failed to get server for default resolver %s: %v", domain, err)
		}
	}

	endpoint, ok := info.Endpoints["net.concrnt.core.resource"]
	if !ok {
		return fmt.Errorf("resource endpoint not found")
	}

	template := endpoint.Template

	template = strings.ReplaceAll(template, "{ccid}", owner)
	template = strings.ReplaceAll(template, "{key}", key)
	template = strings.ReplaceAll(template, "{uri}", url.QueryEscape(uri))
	template = "https://" + info.Domain + template

	fmt.Printf("Resolved endpoint: %s\n", template)

	req, err := http.NewRequest("GET", template, nil)
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
