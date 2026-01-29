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

type CCURI struct {
	Scheme string  `json:"schema"`
	Owner  string  `json:"owner"`
	Key    string  `json:"key"`
	CDID   string  `json:"cdid"`
	Hint   *string `json:"hint,omitempty"`
}

func ParseCCURI(escaped string) (*CCURI, error) {

	uriString, err := url.QueryUnescape(escaped)
	if err != nil {
		return nil, fmt.Errorf("invalid uri encoding")
	}
	uri, err := url.Parse(uriString)
	if err != nil {
		return nil, fmt.Errorf("invalid uri")
	}

	user := uri.User.String()
	path := uri.Path
	key := strings.TrimPrefix(path, "/")

	owner := uri.Host
	var hint *string = nil
	if user != "" {
		owner = user
		hint = &uri.Host
	}

	switch uri.Scheme {
	case "cckv":
		return &CCURI{
			Scheme: uri.Scheme,
			Owner:  owner,
			Key:    key,
			CDID:   "",
			Hint:   hint,
		}, nil
	case "ccfs":
		return &CCURI{
			Scheme: uri.Scheme,
			Owner:  owner,
			Key:    "",
			CDID:   key,
			Hint:   hint,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported uri scheme")
	}
}

func ComposeCCURI(scheme, owner, key string) string {
	u := &url.URL{
		Scheme: scheme,
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
