package service

const (
	FlaredWSConnectedLastSeenValue = "__OPENFLARE_FLARED_WS_CONNECTED__"

	FlaredWSMessageTypeActiveConfig = "active_config"
	FlaredWSMessageTypeForceSync    = "force_sync"
	FlaredWSMessageTypePong         = "pong"
)

var DefaultFlaredWSHub = NewWSHub("flared")

func RegisterFlaredWSClient(nodeID string) *WSClient {
	return DefaultFlaredWSHub.Register(nodeID)
}

func UnregisterFlaredWSClient(client *WSClient) {
	DefaultFlaredWSHub.Unregister(client)
}

func DisconnectFlaredWSClient(nodeID string) {
	DefaultFlaredWSHub.Disconnect(nodeID)
}

func IsFlaredWSConnected(nodeID string) bool {
	return DefaultFlaredWSHub.IsConnected(nodeID)
}

func SendFlaredWSPong(nodeID string) bool {
	return DefaultFlaredWSHub.SendMessage(nodeID, WSMessage{
		Type: FlaredWSMessageTypePong,
	})
}

func SendFlaredWSActiveConfig(nodeID string, activeConfig *ActiveConfigMeta) bool {
	if activeConfig == nil {
		return false
	}
	return DefaultFlaredWSHub.SendMessage(nodeID, WSMessage{
		Type:    FlaredWSMessageTypeActiveConfig,
		Payload: activeConfig,
	})
}

func BroadcastFlaredWSActiveConfig(activeConfig *ActiveConfigMeta) WSBroadcastResult {
	if activeConfig == nil {
		return WSBroadcastResult{}
	}
	result := DefaultFlaredWSHub.Broadcast(WSMessage{
		Type:    FlaredWSMessageTypeActiveConfig,
		Payload: activeConfig,
	})
	return result
}
