package concrnt

import (
	"time"
)

type ConcrntEndpoint struct {
	Template string    `json:"template"`
	Method   string    `json:"method"`
	Query    *[]string `json:"query,omitempty"`
}

type WellKnownConcrnt struct {
	Version   string                     `json:"version"`
	Domain    string                     `json:"domain"`
	CSID      string                     `json:"csid"`
	Layer     string                     `json:"layer"`
	Endpoints map[string]ConcrntEndpoint `json:"endpoints"`
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

	Schema string `json:"schema,omitempty"`

	CreatedAt time.Time `json:"createdAt"`

	// CIP-5
	MemberOf *[]string `json:"memberOf,omitempty"`

	// CIP-6
	Owner              *string `json:"owner,omitempty"`
	Associate          *string `json:"associate,omitempty"`
	AssociationVariant *string `json:"associationVariant,omitempty"`

	// CIP-8
	Policies *[]Policy `json:"policies,omitempty"`
}

type SchemaDeleteType string

type Proof struct {
	Type      string  `json:"type"`
	Signature *string `json:"signature,omitempty"`
	Href      *string `json:"href,omitempty"`
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

type RegisterRequest[T any] struct {
	AffiliationDocument  string  `json:"affiliationDocument"`
	AffiliationSignature string  `json:"affiliationSignature"`
	Meta                 T       `json:"meta,omitempty"`
	InviteToken          *string `json:"inviteToken,omitempty"`
}
