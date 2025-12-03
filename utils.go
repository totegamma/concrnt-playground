package concrnt

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

func JsonPrint(tag string, v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Printf("%s: error marshaling: %v\n", tag, err)
		return
	}
	fmt.Printf("%s: %s\n", tag, string(b))
}

func ParseCCURI(escaped string) (string, string, error) {
	uriString, err := url.QueryUnescape(escaped)
	if err != nil {
		return "", "", fmt.Errorf("invalid uri encoding")
	}
	uri, err := url.Parse(uriString)
	if err != nil {
		return "", "", fmt.Errorf("invalid uri")
	}

	if uri.Scheme != "cc" {
		return "", "", fmt.Errorf("unsupported uri scheme")
	}

	owner := uri.Host
	path := uri.Path

	key := strings.TrimPrefix(path, "/")

	return owner, key, nil
}

func ComposeCCURI(owner, key string) string {
	return fmt.Sprintf("cc://%s/%s", owner, key)
}
