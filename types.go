package concrnt

import (
	"time"
)

const (
	SchemaDelete string = "https://schema.concrnt.net/delete.json"
)

type Policy struct {
	URL      string  `json:"url"`
	Params   *string `json:"params,omitempty"`
	Defaults *string `json:"defaults,omitempty"`
}

type Document[T any] struct {
	// CIP-1
	Key   *string `json:"key,omitempty"`
	Value T       `json:"value"`

	Author string `json:"author"`

	ContentType *string `json:"contentType,omitempty"`
	Schema      *string `json:"schema,omitempty"`

	CreateAt time.Time `json:"createAt"`

	// CIP-5
	MemberOf *[]string `json:"memberOf,omitempty"`

	// CIP-6
	Owner          *string `json:"owner,omitempty"`
	Associate      *string `json:"associate,omitempty"`
	AssociationKey *string `json:"associationKey,omitempty"`

	// CIP-8
	Policies *[]Policy `json:"policies,omitempty"`
}

type SchemaDeleteType string

type Proof struct {
	Type      string `json:"type"`
	Signature string `json:"signature"`
}

type SignedDocument struct {
	Document string `json:"document"`
	Proof    Proof  `json:"proof"`
}
