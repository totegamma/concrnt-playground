package domain

const (
	RequesterCtxKey = "cc-requester"
)

const (
	RequesterHeader = "cc-requester"
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
