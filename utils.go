package concrnt

import (
	"fmt"
	"net/url"
	"strings"
)

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
