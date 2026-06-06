package service

import (
	"log/slog"
	"sync"
	"time"
)

type WSMessage struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

type WSClient struct {
	id   string
	send chan WSMessage
	done chan struct{}
	once sync.Once
}

func (client *WSClient) ID() string {
	if client == nil {
		return ""
	}
	return client.id
}

func (client *WSClient) Messages() <-chan WSMessage {
	if client == nil {
		return nil
	}
	return client.send
}

func (client *WSClient) Done() <-chan struct{} {
	if client == nil {
		return nil
	}
	return client.done
}

func (client *WSClient) Send(message WSMessage) bool {
	if client == nil {
		return false
	}
	select {
	case <-client.done:
		return false
	case client.send <- message:
		return true
	default:
		return false
	}
}

func (client *WSClient) Close() {
	if client == nil {
		return
	}
	client.once.Do(func() {
		close(client.done)
	})
}

type WSHub struct {
	name    string
	mu      sync.RWMutex
	clients map[string]*WSClient
	done    chan struct{}
}

func NewWSHub(name string) *WSHub {
	h := &WSHub{
		name:    name,
		clients: make(map[string]*WSClient),
		done:    make(chan struct{}),
	}
	go h.startPingLoop()
	return h
}

func (h *WSHub) Close() {
	close(h.done)
}

func (h *WSHub) startPingLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-h.done:
			return
		case <-ticker.C:
			h.mu.RLock()
			if len(h.clients) == 0 {
				h.mu.RUnlock()
				continue
			}
			clients := make([]*WSClient, 0, len(h.clients))
			for _, client := range h.clients {
				clients = append(clients, client)
			}
			h.mu.RUnlock()

			for _, client := range clients {
				if !client.Send(WSMessage{
					Type: "ping",
				}) {
					slog.Warn("ws client send ping failed, queue full, disconnecting", "hub", h.name, "id", client.id)
					h.Disconnect(client.id)
				}
			}
		}
	}
}

func ShutdownWSHubs() {
	DefaultAgentWSHub.Close()
	DefaultFlaredWSHub.Close()
	DefaultRelayWSHub.Close()
}

func (h *WSHub) Register(id string) *WSClient {
	client := &WSClient{
		id:   id,
		send: make(chan WSMessage, 16),
		done: make(chan struct{}),
	}
	h.mu.Lock()
	if existing := h.clients[id]; existing != nil {
		slog.Debug("ws replacing existing connection", "hub", h.name, "id", id)
		existing.Close()
	}
	h.clients[id] = client
	count := len(h.clients)
	h.mu.Unlock()
	slog.Debug("ws connection registered", "hub", h.name, "id", id, "client_count", count)
	return client
}

func (h *WSHub) Unregister(client *WSClient) {
	if client == nil {
		return
	}
	h.mu.Lock()
	if current := h.clients[client.id]; current == client {
		delete(h.clients, client.id)
	}
	count := len(h.clients)
	h.mu.Unlock()
	client.Close()
	slog.Debug("ws connection unregistered", "hub", h.name, "id", client.id, "client_count", count)
}

func (h *WSHub) Disconnect(id string) {
	h.mu.Lock()
	client := h.clients[id]
	if client != nil {
		delete(h.clients, id)
	}
	count := len(h.clients)
	h.mu.Unlock()

	if client != nil {
		client.Close()
		slog.Debug("ws connection forcefully disconnected", "hub", h.name, "id", id, "client_count", count)
	}
}

func (h *WSHub) IsConnected(id string) bool {
	h.mu.RLock()
	client := h.clients[id]
	h.mu.RUnlock()
	if client == nil {
		return false
	}
	select {
	case <-client.done:
		return false
	default:
		return true
	}
}

func (h *WSHub) SendMessage(id string, message WSMessage) bool {
	h.mu.RLock()
	client := h.clients[id]
	h.mu.RUnlock()
	if client == nil {
		return false
	}
	ok := client.Send(message)
	if !ok {
		slog.Debug("ws send queued message failed", "hub", h.name, "id", id, "type", message.Type)
	}
	return ok
}

type WSBroadcastResult struct {
	ClientCount  int      `json:"client_count"`
	SuccessCount int      `json:"success_count"`
	FailedIDs    []string `json:"failed_ids"`
}

func (h *WSHub) Broadcast(message WSMessage) WSBroadcastResult {
	h.mu.RLock()
	clients := make([]*WSClient, 0, len(h.clients))
	for _, client := range h.clients {
		clients = append(clients, client)
	}
	h.mu.RUnlock()

	var result WSBroadcastResult
	result.ClientCount = len(clients)
	for _, client := range clients {
		if client.Send(message) {
			result.SuccessCount++
			continue
		}
		result.FailedIDs = append(result.FailedIDs, client.ID())
	}
	return result
}
