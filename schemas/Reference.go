package schemas

type Reference struct {
	Href        string `json:"href"`
	ContentType string `json:"contentType,omitempty"`
}
