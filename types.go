package concrnt

import (
	"time"
)

type WellKnownConcrnt struct {
	Version   string            `json:"version"`
	Domain    string            `json:"domain"`
	CSID      string            `json:"csid"`
	Layer     string            `json:"layer"`
	Endpoints map[string]string `json:"endpoints"`
}

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

type Entity struct {
	CCID                 string `json:"ccid"`
	Domain               string `json:"domain"`
	AffiliationDocument  string `json:"affiliationDocument"`
	AffiliationSignature string `json:"affiliationSignature"`
}
