package channel

type SysChannel struct {
	Ulid        string         `json:"ulid"`
	CreatedAt   int64          `json:"created_at"`
	UpdatedAt   int64          `json:"updated_at"`
	DeletedAt   int64          `json:"deleted_at"`
	CreatedBy   string         `json:"created_by"`
	UpdatedBy   string         `json:"updated_by"`
	Name        string         `json:"name"`
	Code        string         `json:"code"`
	Description string         `json:"description"`
	Icon        string         `json:"icon"`
	Enabled     bool           `json:"enabled"`
	Sort        int            `json:"sort"`
	Config      map[string]any `json:"config"` // 渠道特定配置
}