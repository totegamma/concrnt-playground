package schemas

const (
	DeleteURL       string = "https://schema.concrnt.net/delete.json"
	AffiliationURL  string = "https://schema.concrnt.net/affiliation.json"
	EnactSubkeyURL  string = "https://schema.concrnt.net/subkey-enact.json"
	RevokeSubkeyURL string = "https://schema.concrnt.net/subkey-revoke.json"
	ItemURL         string = "https://schema.concrnt.net/item.json"
)

type Item struct {
	Href string `json:"href,omitempty"`
}
