package policy

type Conclusion int

const (
	UNSET Conclusion = iota
	OK
	NG
	ALLOW
	DENY
)

func ParseConclusion(s string) Conclusion {
	switch s {
	case "allow":
		return ALLOW
	case "deny":
		return DENY
	case "ok":
		return OK
	case "ng":
		return NG
	default:
		return UNSET
	}
}

func (c Conclusion) Or(other Conclusion) Conclusion {
	if c == UNSET {
		return other
	}
	if other == UNSET {
		return c
	}
	if (c == DENY && other == ALLOW) || (c == ALLOW && other == DENY) {
		return UNSET
	}
	if c == DENY || other == DENY {
		return DENY
	}
	if c == ALLOW || other == ALLOW {
		return ALLOW
	}
	if (c == OK && other == NG) || (c == NG && other == OK) {
		return UNSET
	}
	if c == OK || other == OK {
		return OK
	}
	if c == NG || other == NG {
		return NG
	}
	return UNSET
}

type RequestContext struct {
	Requester       any            `json:"requester"`
	RequesterDomain any            `json:"requester_domain"`
	Parent          any            `json:"parent"`
	This            any            `json:"this"`
	Params          map[string]any `json:"params"`
}

type PolicyDocument struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Versions    map[string]Policy `json:"versions"`
}

type Policy struct {
	Statements map[string][]Stmt `json:"statements"`
	Defaults   map[string]bool   `json:"defaults"`
}

type Stmt struct {
	Emit      string `json:"emit"`
	Condition Expr   `json:"condition"`
}

type Expr struct {
	Operator string `json:"op"`
	Args     []Expr `json:"args"`
	Const    any    `json:"const,omitempty"`
}

type EvalResult struct {
	Operator string       `json:"op"`
	Args     []EvalResult `json:"args"`
	Result   any          `json:"result"`
	Error    string       `json:"error"`
}
