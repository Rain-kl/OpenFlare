package service

const (
	RelayWSConnectedLastSeenValue = "__OPENFLARE_WS_CONNECTED__"
)

var DefaultRelayWSHub = NewWSHub("relay")

func RegisterRelayWSClient(nodeID string) *WSClient {
	return DefaultRelayWSHub.Register(nodeID)
}

func UnregisterRelayWSClient(client *WSClient) {
	DefaultRelayWSHub.Unregister(client)
}

func IsRelayWSConnected(nodeID string) bool {
	return DefaultRelayWSHub.IsConnected(nodeID)
}

func SendRelayWSPing(nodeID string) bool {
	return DefaultRelayWSHub.SendMessage(nodeID, WSMessage{
		Type: "ping",
	})
}

func SendRelayWSPong(nodeID string) bool {
	return DefaultRelayWSHub.SendMessage(nodeID, WSMessage{
		Type: "pong",
	})
}

func SendRelayWSConfig(nodeID string, config *RelayConfig) bool {
	if config == nil {
		return false
	}
	return DefaultRelayWSHub.SendMessage(nodeID, WSMessage{
		Type:    "relay_config",
		Payload: config,
	})
}
