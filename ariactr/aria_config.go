package ariactr

// Config for aria2 client package.
type Config struct {
	// Aria2RPCURL is the URL to send RPC calls to.
	// Defaults to "http://localhost:6800/jsonrpc" when empty.
	Aria2RPCURL string
	// PollingInterval is the time in seconds to check active tasks status.
	// Status determined by batch of aria2.tellStatus calls for every active task.
	// PollingInterval can't be 0 and defaults to 10 seconds when 0.
	PollingInterval uint
}
