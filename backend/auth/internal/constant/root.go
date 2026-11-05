package constant

const RoleUser = "user"

const (
	AuthIdTTL       = 500
	OtpTTL          = 300
	AccessTokenTTL  = 600
	RefreshTokenTTL = 15552000
	NonceTTL        = 604800
	// a login code can be requested once per this many seconds for an email
	NoPasswordEmailOTPCooldownTTL = 60
	// wrong guesses of a login code before it stops working
	NoPasswordEmailOTPMaxAttempts = 5
	// time to scan the QR code and enter its first code
	TOTPSetupTTL = 900
	// wrong guesses of an authenticator code per window, then it is locked until the window ends
	TOTPMaxAttempts = 5
	TOTPAttemptTTL  = 300
)

// what an emailed code or an authenticator setup is for, one made for signup can't replace a working authenticator
const (
	OTPPurposeSignup = "signup"
	OTPPurposeReset  = "reset"
)

const (
	AppleKeyUrl    = "https://appleid.apple.com/auth/keys"
	AppleIssuerUrl = "https://appleid.apple.com"
)
