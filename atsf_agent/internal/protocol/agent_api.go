package protocol

type NodePayload struct {
	NodeID         string `json:"node_id"`
	Name           string `json:"name"`
	IP             string `json:"ip"`
	AgentVersion   string `json:"agent_version"`
	NginxVersion   string `json:"nginx_version"`
	CurrentVersion string `json:"current_version"`
	LastError      string `json:"last_error"`
}

type ApplyLogPayload struct {
	NodeID  string `json:"node_id"`
	Version string `json:"version"`
	Result  string `json:"result"`
	Message string `json:"message"`
}

type ActiveConfigResponse struct {
	Version        string `json:"version"`
	Checksum       string `json:"checksum"`
	RenderedConfig string `json:"rendered_config"`
	CreatedAt      string `json:"created_at"`
}
