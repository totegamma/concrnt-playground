package domain

// Entity represents the core user/server identity without persistence concerns.
type Entity struct {
	ID                   string  `json:"ccid"`
	Alias                *string `json:"alias,omitempty"`
	Domain               string  `json:"domain"`
	Tag                  string  `json:"tag,omitempty"`
	AffiliationDocument  string  `json:"affiliationDocument"`
	AffiliationSignature string  `json:"affiliationSignature"`
}

// EntityMeta is auxiliary metadata associated with an Entity.
type EntityMeta struct {
	ID      string  `json:"ccid"`
	Inviter *string `json:"inviter,omitempty"`
	Info    string  `json:"info"`
}
