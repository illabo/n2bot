package tg

// Config for Telegram client package.
type Config struct {
	// Token is the Telegram Bot API token string
	// in format "Bot ID:Bot password",
	// ID shouldn't have preceding "bot" as used in API calls,
	// just put the number.
	Token string
	// ConnTimeout is the time in seconds to keep connection alive.
	// Telegram documentation reccomends to set this value resonably high
	// to prevent connectivity problems as DoS protection may be active.
	// However default timeout of 0 is ok for testing purpose.
	// ConnTimeout defaults to 0.
	ConnTimeout uint
}
