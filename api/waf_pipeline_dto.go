package api

import "HomeWaf/wafpipeline"

// PipelinePortRequest identifies an independent listener policy.
type PipelinePortRequest struct {
	Port uint16 `json:"port" example:"5000"`
}

type PipelineStateRequest struct {
	Port   uint16 `json:"port" example:"5000"`
	ID     string `json:"id"`
	Action string `json:"action" enums:"release,permanent,whitelist"`
}

type PipelineExplainRequest struct {
	Config   wafpipeline.PortConfig `json:"config"`
	ClientIP string                 `json:"client_ip" example:"203.0.113.10"`
	Method   string                 `json:"method" example:"GET"`
	Path     string                 `json:"path" example:"/"`
	Headers  map[string]string      `json:"headers"`
	Body     string                 `json:"body"`
}

type PipelineTemplateIDRequest struct {
	ID string `json:"id"`
}

type PipelineTemplatePreviewRequest struct {
	Config   wafpipeline.PortConfig `json:"config"`
	ID       string                 `json:"template_id" example:"family-nas"`
	Sections []string               `json:"sections"`
}

type PipelineCopyRequest struct {
	Source     uint16 `json:"source" example:"5000"`
	Target     uint16 `json:"target" example:"5001"`
	IncludeIPs bool   `json:"include_ips"`
}

type PipelineUpstreamRequest struct {
	Upstream string `json:"upstream" example:"https://nas.home:5001"`
	DialIP   string `json:"dial_ip,omitempty"`
}

type PipelineGateUserRequest struct {
	Port     uint16 `json:"port" example:"5000"`
	Username string `json:"username"`
}

type PipelineGateSessionRequest struct {
	Port uint16 `json:"port" example:"5000"`
	ID   string `json:"id"`
}

type PipelineGateTokenRequest struct {
	Port     uint16 `json:"port" example:"5000"`
	Revoke   bool   `json:"revoke"`
	Revision int64  `json:"revision" example:"1"`
}
