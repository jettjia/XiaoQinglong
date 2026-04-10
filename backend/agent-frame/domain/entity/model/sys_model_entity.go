package model

type SysModel struct {
	Ulid          string `json:"ulid"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
	DeletedAt     int64  `json:"deleted_at"`
	CreatedBy     string `json:"created_by"`
	UpdatedBy     string `json:"updated_by"`
	Name          string `json:"name"`
	Provider      string `json:"provider"`
	BaseUrl       string `json:"base_url"`
	ApiKey        string `json:"api_key"`
	ModelType     string `json:"model_type"`
	Category      string `json:"category"`
	Status        string `json:"status"`
	Latency       string `json:"latency"`
	ContextWindow string `json:"context_window"`
	Usage         int    `json:"usage"`
}