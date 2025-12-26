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
	u := &url.URL{
		Scheme: "cc",
		Host:   owner,
		Path:   key,
	}
	return u.String()
}

func hasChar(s string, c byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return true
		}
	}
	return false
}

func IsCCID(keyID string) bool {
	return len(keyID) == 42 && keyID[:3] == "con" && !hasChar(keyID, '.')
}

func IsCSID(keyID string) bool {
	return len(keyID) == 42 && keyID[:3] == "ccs" && !hasChar(keyID, '.')
}

func IsCKID(keyID string) bool {
	return len(keyID) == 42 && keyID[:3] == "cck" && !hasChar(keyID, '.')
}
