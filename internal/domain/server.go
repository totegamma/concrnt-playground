package domain

import "github.com/totegamma/concrnt-playground"

// Server represents a remote Concrnt node descriptor.
type Server struct {
	Domain  string `json:"domain"`
	CSID    string `json:"csid"`
	Layer   string `json:"layer"`
	Version string `json:"version"`
	// WellKnown keeps the original well-known response for reuse.
	WellKnown concrnt.WellKnownConcrnt `json:"wellKnown"`
}
