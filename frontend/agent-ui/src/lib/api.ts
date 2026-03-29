const API_BASE = '/api/xiaoqinglong/agent-frame/v1';

export interface ApiResponse<T = any> {
  code: number;
  message: string;
  data: T;
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