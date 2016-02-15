package plugin

type Plugin struct {
}

func (p *Plugin) Relay(args RelayArgs) error {
	return SessionNotFoundError{}
}

type SessionNotFoundError struct {
}

func (e SessionNotFoundError) Error() string {
	return "sessions are not found."
}

const (
	RelaySend = 1 + iota
	RelayClose
)

type RelayArgs struct {
	Method  int
	Keys    []string
	Message []byte
}

type RelayResp struct {
	RegisteredKeys []string
	NotExistsKeys  []string
}

type ReceivedMessage struct {
	Keys    []string
	Message []byte
}
