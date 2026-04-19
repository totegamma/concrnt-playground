package schemas

type Reference struct {
	Href   string `json:"href"`
	Schema string `json:"schema,omitempty"`
}
