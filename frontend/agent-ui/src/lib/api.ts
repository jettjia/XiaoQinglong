const API_BASE = '/api/xiaoqinglong/agent-frame/v1';

export interface ApiResponse<T = any> {
  code: number;
  message: string;
  data: T;
}

export interface Model {
  ulid: string;
  created_at: number;
  updated_at: number;
  created_by: string;
  updated_by: string;
  name: string;
  provider: string;
  baseUrl: string;
  api_key: string;
  modelType: 'llm' | 'embedding';
  category: string;
  status: string;
  latency: string;
  contextWindow: string;
  usage: number;
}

export const configApi = {
  async getAppConfig(): Promise<string> {
    const res = await fetch(`${API_BASE}/config/app`);
    const json = await res.json();
    // Handle both wrapped {code, data} and direct {content} formats
    if (json.data && json.data.content) {
      return json.data.content;
    }
    if (json.content) {
      return json.content;
    }
    throw new Error('Invalid response format');
  },

  async saveAppConfig(content: string): Promise<void> {
    const res = await fetch(`${API_BASE}/config/app`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ content }),
    });
    if (!res.ok) {
      throw new Error('Failed to save app config');
    }
  },

  async getSkillsConfig(): Promise<string> {
    const res = await fetch(`${API_BASE}/config/skills`);
    const json = await res.json();
    // Handle both wrapped {code, data} and direct {content} formats
    if (json.data && json.data.content) {
      return json.data.content;
    }
    if (json.content) {
      return json.content;
    }
    throw new Error('Invalid response format');
  },

  async saveSkillsConfig(content: string): Promise<void> {
    const res = await fetch(`${API_BASE}/config/skills`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ content }),
    });
    if (!res.ok) {
      throw new Error('Failed to save skills config');
    }
  },
};

export const modelApi = {
  async create(data: {
    name: string;
    provider: string;
    baseUrl: string;
    apiKey: string;
    modelType: 'llm' | 'embedding';
    category: string;
  }): Promise<{ ulid: string }> {
    const res = await fetch(`${API_BASE}/model`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to create model');
    }
    return json.data;
  },

  async update(ulid: string, data: {
    name?: string;
    provider?: string;
    baseUrl?: string;
    apiKey?: string;
    modelType?: 'llm' | 'embedding';
    category?: string;
    status?: string;
    latency?: string;
  }): Promise<void> {
    const res = await fetch(`${API_BASE}/model/${ulid}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (!res.ok) {
      const json = await res.json();
      throw new Error(json.message || 'Failed to update model');
    }
  },

  async delete(ulid: string): Promise<void> {
    const res = await fetch(`${API_BASE}/model/${ulid}`, {
      method: 'DELETE',
    });
    if (!res.ok) {
      const json = await res.json();
      throw new Error(json.message || 'Failed to delete model');
    }
  },

  async findById(ulid: string): Promise<Model> {
    const res = await fetch(`${API_BASE}/model/${ulid}`);
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to find model');
    }
    // API returns object directly, not wrapped in {data: ...}
    return json.data || json;
  },

  async findAll(modelType?: 'llm' | 'embedding'): Promise<Model[]> {
    const res = await fetch(`${API_BASE}/model/all`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ modelType }),
    });
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to find models');
    }
    // API returns array directly, not wrapped in {data: [...]}
    return Array.isArray(json) ? json : (json.data || []);
  },

  async findPage(params: {
    query?: any[];
    page_data?: { page: number; page_size: number };
    sort_data?: { field: string; order: 'asc' | 'desc' };
  }): Promise<{ entries: Model[]; page_data: { page: number; page_size: number; total: number } }> {
    const res = await fetch(`${API_BASE}/model/page`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(params),
    });
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to find models');
    }
    // API returns object directly, not wrapped in {data: ...}
    return json.data || json;
  },
};