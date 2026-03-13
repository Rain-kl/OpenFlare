package protocol

type APIResponse[T any] struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type HeartbeatAPIResponse struct {
	Success       bool              `json:"success"`
	Message       string            `json:"message"`
	Data          any               `json:"data"`
	AgentSettings *AgentSettings    `json:"agent_settings,omitempty"`
	ActiveConfig  *ActiveConfigMeta `json:"active_config,omitempty"`
}

type HeartbeatResult struct {
	AgentSettings *AgentSettings
	ActiveConfig  *ActiveConfigMeta
}

type AgentSettings struct {
	HeartbeatInterval   int    `json:"heartbeat_interval"`
	AutoUpdate          bool   `json:"auto_update"`
	UpdateRepo          string `json:"update_repo"`
	UpdateNow           bool   `json:"update_now"`
	UpdateChannel       string `json:"update_channel"`
	UpdateTag           string `json:"update_tag"`
	RestartOpenrestyNow bool   `json:"restart_openresty_now"`
}

const (
	OpenrestyStatusHealthy   = "healthy"
	OpenrestyStatusUnhealthy = "unhealthy"
	OpenrestyStatusUnknown   = "unknown"
)

type NodePayload struct {
	NodeID           string `json:"node_id"`
	Name             string `json:"name"`
	IP               string `json:"ip"`
	AgentVersion     string `json:"agent_version"`
	NginxVersion     string `json:"nginx_version"`
	CurrentVersion   string `json:"current_version"`
	LastError        string `json:"last_error"`
	OpenrestyStatus  string `json:"openresty_status"`
	OpenrestyMessage string `json:"openresty_message"`
}

type RegisterNodeResponse struct {
	NodeID     string `json:"node_id"`
	AgentToken string `json:"agent_token"`
	Name       string `json:"name"`
}

type ApplyLogPayload struct {
	NodeID              string `json:"node_id"`
	Version             string `json:"version"`
	Result              string `json:"result"`
	Message             string `json:"message"`
	Checksum            string `json:"checksum"`
	MainConfigChecksum  string `json:"main_config_checksum"`
	RouteConfigChecksum string `json:"route_config_checksum"`
	SupportFileCount    int    `json:"support_file_count"`
}

type ActiveConfigResponse struct {
	Version        string        `json:"version"`
	Checksum       string        `json:"checksum"`
	MainConfig     string        `json:"main_config"`
	RouteConfig    string        `json:"route_config"`
	RenderedConfig string        `json:"rendered_config"`
	SupportFiles   []SupportFile `json:"support_files"`
	CreatedAt      string        `json:"created_at"`
}

type ActiveConfigMeta struct {
	Version  string `json:"version"`
	Checksum string `json:"checksum"`
}

type SupportFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}
