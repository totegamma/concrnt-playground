package domain

const (
	RequesterTypeCtxKey      = "cc-requesterType"
	RequesterIdCtxKey        = "cc-requesterId"
	RequesterTagCtxKey       = "cc-requesterTag"
	RequesterServerCtxKey    = "cc-requesterServer"
	RequesterServerTagsKey   = "cc-requesterServerTags"
	RequesterKeychainKey     = "cc-requesterKeychain"
	RequesterPassportKey     = "cc-requesterPassport"
	RequesterIsRegisteredKey = "cc-requesterIsRegistered"
	CaptchaVerifiedKey       = "cc-captchaVerified"
)

const (
	RequesterTypeHeader         = "cc-requester-type"
	RequesterIdHeader           = "cc-requester-ccid"
	RequesterTagHeader          = "cc-requester-tag"
	RequesterServerHeader       = "cc-requester-domain"
	RequesterServerTagsHeader   = "cc-requester-domain-tags"
	RequesterKeychainHeader     = "cc-requester-keychain"
	RequesterPassportHeader     = "passport"
	RequesterIsRegisteredHeader = "cc-requester-is-registered"
	CaptchaVerifiedHeader       = "cc-captcha-verified"
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
