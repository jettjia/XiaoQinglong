package knowledge_base

type SysKnowledgeBase struct {
	Ulid         string `json:"ulid"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
	DeletedAt    int64  `json:"deleted_at"`
	CreatedBy    string `json:"created_by"`
	UpdatedBy    string `json:"updated_by"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	RetrievalUrl string `json:"retrieval_url"`
	Token        string `json:"token"`
	Enabled      bool   `json:"enabled"`
}
