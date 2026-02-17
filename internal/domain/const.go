package domain

const (
	RequesterCtxKey = "cc-requester"
	ReferrerCtxKey  = "cc-referrer"
)

const (
	RequesterHeader = "cc-requester"
	ReferrerHeader  = "cc-referrer"
)

type CommitMode int

const (
	CommitModeUnknown CommitMode = iota
	CommitModeExecute
	CommitModeDryRun
	CommitModeLocalOnlyExec
)

type PolicyEvalResult int

const (
	PolicyEvalResultDefault PolicyEvalResult = iota
	PolicyEvalResultNever
	PolicyEvalResultDeny
	PolicyEvalResultAllow
	PolicyEvalResultAlways
	PolicyEvalResultError
)

const (
	Unknown = iota
	LocalUser
	RemoteUser
	RemoteServer
)

func RequesterTypeString(t int) string {
	switch t {
	case LocalUser:
		return "LocalUser"
	case RemoteUser:
		return "RemoteUser"
	case RemoteServer:
		return "RemoteServer"
	case Unknown:
		return "Unknown"
	default:
		return "Error"
	}
}

type Service struct {
	Name         string `yaml:"name"`
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	Path         string `yaml:"path"`
	PreservePath bool   `yaml:"preservePath"`
	InjectCors   bool   `yaml:"injectCors"`
	Gone         bool   `yaml:"gone"`
}
