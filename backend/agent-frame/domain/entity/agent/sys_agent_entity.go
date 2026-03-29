package agent

type SysAgent struct {
	Ulid        string `json:"ulid"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
	DeletedAt   int64  `json:"deleted_at"`
	CreatedBy   string `json:"created_by"`
	UpdatedBy   string `json:"updated_by"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Model       string `json:"model"`
	Config      string `json:"config"`
	IsSystem    bool   `json:"is_system"`
	Enabled     bool   `json:"enabled"`
}
