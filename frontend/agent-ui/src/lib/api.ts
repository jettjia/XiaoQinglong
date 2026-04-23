// For production Wails build, use direct localhost URL since proxy doesn't apply
// For dev, Vite proxy handles /api routing
// Detect Wails environment (http://wails.localhost) and use correct API endpoint
const isWails = window.location.hostname === 'wails.localhost';
const API_BASE = isWails
  ? 'http://wails.localhost:9292/api/xiaoqinglong/agent-frame/v1'
  : (import.meta.env.VITE_AGENT_FRAME_API_URL || 'http://localhost:9292/api/xiaoqinglong/agent-frame/v1');

import type { Agent } from '../types';

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

export interface KnowledgeBase {
  ulid: string;
  created_at: number;
  updated_at: number;
  created_by: string;
  updated_by: string;
  name: string;
  description: string;
  retrievalUrl: string;
  token: string;
  enabled: boolean;
}

export interface RecallResult {
  title: string;
  content: string;
  score: number;
}

export const knowledgeBaseApi = {
  async create(data: {
    name: string;
    description?: string;
    retrievalUrl: string;
    token?: string;
    enabled?: boolean;
  }): Promise<{ ulid: string }> {
    const res = await fetch(`${API_BASE}/knowledge_base`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to create knowledge base');
    }
    return json.data;
  },

  async update(ulid: string, data: {
    name?: string;
    description?: string;
    retrievalUrl?: string;
    token?: string;
    enabled?: boolean;
  }): Promise<void> {
    const res = await fetch(`${API_BASE}/knowledge_base/${ulid}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (!res.ok) {
      const json = await res.json();
      throw new Error(json.message || 'Failed to update knowledge base');
    }
  },

  async delete(ulid: string): Promise<void> {
    const res = await fetch(`${API_BASE}/knowledge_base/${ulid}`, {
      method: 'DELETE',
    });
    if (!res.ok) {
      const json = await res.json();
      throw new Error(json.message || 'Failed to delete knowledge base');
    }
  },

  async findById(ulid: string): Promise<KnowledgeBase> {
    const res = await fetch(`${API_BASE}/knowledge_base/${ulid}`);
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to find knowledge base');
    }
    return json.data || json;
  },

  async findAll(): Promise<KnowledgeBase[]> {
    const res = await fetch(`${API_BASE}/knowledge_base/all`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({}),
    });
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to find knowledge bases');
    }
    return Array.isArray(json) ? json : (json.data || []);
  },

  async recallTest(ulid: string, query: string, topK: number = 5): Promise<RecallResult[]> {
    const res = await fetch(`${API_BASE}/knowledge_base/${ulid}/recall`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ query, top_k: topK }),
    });
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to recall');
    }
    return json.data || json;
  },
};

// Command API - 魔法盒命令执行
export interface CommandResult {
  success: boolean;
  action: string;
  result?: any;
  navigate_to?: string;
  message?: string;
  prefilled?: Record<string, any>;
  show_guidance?: boolean;
}

export const commandApi = {
  async execute(command: string): Promise<CommandResult> {
    const res = await fetch(`${API_BASE}/command/execute`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ command }),
    });
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to execute command');
    }
    return json.data || json;
  },
};

export interface Skill {
  ulid: string;
  created_at: number;
  updated_at: number;
  created_by: string;
  updated_by: string;
  name: string;
  description: string;
  skill_type: 'mcp' | 'tool' | 'a2a' | 'skill';
  version: string;
  path: string;
  enabled: boolean;
  config: string;
  is_system: boolean;
  risk_level?: 'low' | 'medium' | 'high';
}

export interface CheckSkillNameResult {
  exists: boolean;
  message: string;
}

export const skillApi = {
  async create(data: {
    name: string;
    description?: string;
    skillType: 'mcp' | 'tool' | 'a2a' | 'skill';
    version?: string;
    path: string;
    enabled?: boolean;
    config?: string;
  }): Promise<{ ulid: string }> {
    const res = await fetch(`${API_BASE}/skill`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to create skill');
    }
    return json.data;
  },

  async update(ulid: string, data: {
    name?: string;
    description?: string;
    skillType?: 'mcp' | 'tool' | 'a2a';
    version?: string;
    path?: string;
    enabled?: boolean;
    config?: string;
  }): Promise<void> {
    const res = await fetch(`${API_BASE}/skill/${ulid}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (!res.ok) {
      const json = await res.json();
      throw new Error(json.message || 'Failed to update skill');
    }
  },

  async delete(ulid: string): Promise<void> {
    const res = await fetch(`${API_BASE}/skill/${ulid}`, {
      method: 'DELETE',
    });
    if (!res.ok) {
      const json = await res.json();
      throw new Error(json.message || 'Failed to delete skill');
    }
  },

  async findById(ulid: string): Promise<Skill> {
    const res = await fetch(`${API_BASE}/skill/${ulid}`);
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to find skill');
    }
    return json.data || json;
  },

  async findAll(params?: { skill_type?: string; name?: string }): Promise<Skill[]> {
    const res = await fetch(`${API_BASE}/skill/all`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(params || {}),
    });
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to find skills');
    }
    return Array.isArray(json) ? json : (json.data || []);
  },

  async findPage(params: {
    query?: any[];
    page_data?: { page: number; page_size: number };
    sort_data?: { field: string; order: 'asc' | 'desc' };
  }): Promise<{ entries: Skill[]; page_data: { page: number; page_size: number; total: number } }> {
    const res = await fetch(`${API_BASE}/skill/page`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(params),
    });
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to find skills');
    }
    return json.data || json;
  },

  async checkName(name: string): Promise<CheckSkillNameResult> {
    const res = await fetch(`${API_BASE}/skill/check-name`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name }),
    });
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to check skill name');
    }
    return json.data || json;
  },

  async upload(file: File): Promise<Skill> {
    const formData = new FormData();
    formData.append('file', file);

    const res = await fetch(`${API_BASE}/skill/upload`, {
      method: 'POST',
      body: formData,
    });
    const json = await res.json();
    if (!res.ok) {
      const errMsg = json.cause ? `${json.message}\n${json.cause}` : (json.message || 'Failed to upload skill');
      throw new Error(errMsg);
    }
    return json.data || json;
  },
};

export const agentApi = {
  async findAll(): Promise<Agent[]> {
    const res = await fetch(`${API_BASE}/agent/all`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({}),
    });
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to find agents');
    }
    return json.data || json;
  },

  async findById(ulid: string): Promise<Agent> {
    const res = await fetch(`${API_BASE}/agent/${ulid}`, {
      method: 'GET',
    });
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to find agent');
    }
    return json.data || json;
  },

  async create(agent: Partial<Agent>): Promise<{ ulid: string }> {
    const res = await fetch(`${API_BASE}/agent`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(agent),
    });
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to create agent');
    }
    return json.data || json;
  },

  async update(ulid: string, agent: Partial<Agent>): Promise<void> {
    const res = await fetch(`${API_BASE}/agent/${ulid}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(agent),
    });
    if (res.status === 204) {
      return; // No Content
    }
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to update agent');
    }
  },

  async delete(ulid: string): Promise<void> {
    const res = await fetch(`${API_BASE}/agent/${ulid}`, {
      method: 'DELETE',
    });
    if (!res.ok) {
      const json = await res.json();
      throw new Error(json.message || 'Failed to delete agent');
    }
  },

  async updateEnabled(ulid: string, enabled: boolean): Promise<void> {
    const res = await fetch(`${API_BASE}/agent/${ulid}/enabled`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ulid, enabled }),
    });
    if (!res.ok) {
      const json = await res.json();
      throw new Error(json.message || 'Failed to update agent enabled');
    }
  },

  async upload(config: {
    name: string;
    description: string;
    icon: string;
    model: string;
    config: string;
    enabled: boolean;
  }): Promise<{ ulid: string }> {
    const res = await fetch(`${API_BASE}/agent/upload`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(config),
    });
    const json = await res.json();
    if (!res.ok) {
      const errMsg = json.cause ? `${json.message}\n${json.cause}` : (json.message || 'Failed to upload agent');
      throw new Error(errMsg);
    }
    return json.data || json;
  },
};

export interface Channel {
  ulid: string;
  created_at: number;
  updated_at: number;
  created_by: string;
  updated_by: string;
  name: string;
  code: string;
  description: string;
  icon: string;
  enabled: boolean;
  sort: number;
}

export const channelApi = {
  async findAll(): Promise<Channel[]> {
    const res = await fetch(`${API_BASE}/channel/all`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({}),
    });
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to find channels');
    }
    return Array.isArray(json) ? json : (json.data || []);
  },
};

// ====== Chat APIs ======

export interface ChatSession {
  ulid: string;
  user_id: string;
  agent_id: string;
  title: string;
  channel: string;
  model: string;
  status: string;
  created_at: number;
  updated_at: number;
  created_by: string;
  updated_by: string;
}

export interface ChatMessage {
  ulid: string;
  session_id: string;
  role: string;
  content: string;
  model: string;
  tokens: number;
  latency_ms: number;
  trace: string;
  status: string;
  error_msg: string;
  metadata: string;
  created_at: number;
  updated_at: number;
}

export interface ChatApproval {
  ulid: string;
  message_id: string;
  session_id: string;
  tool_name: string;
  tool_type: string;
  risk_level: string;
  parameters: string;
  status: string;
  interrupt_id: string;
  approved_by: string;
  approved_at: number;
  reason: string;
  created_at: number;
  updated_at: number;
}

export const chatApi = {
  // Chat Session APIs
  async createSession(data: {
    user_id: string;
    agent_id: string;
    title?: string;
    channel?: string;
    model?: string;
    status?: string;
  }): Promise<{ ulid: string }> {
    const res = await fetch(`${API_BASE}/chat/session`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to create session');
    }
    return json.data || json;
  },

  async getSession(ulid: string): Promise<ChatSession> {
    const res = await fetch(`${API_BASE}/chat/session/${ulid}`);
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to get session');
    }
    return json.data || json;
  },

  async updateSession(data: {
    ulid: string;
    title?: string;
    status?: string;
  }): Promise<void> {
    const res = await fetch(`${API_BASE}/chat/session`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (!res.ok) {
      const json = await res.json();
      throw new Error(json.message || 'Failed to update session');
    }
  },

  async deleteSession(ulid: string): Promise<void> {
    const res = await fetch(`${API_BASE}/chat/session/${ulid}`, {
      method: 'DELETE',
    });
    if (!res.ok) {
      const json = await res.json();
      throw new Error(json.message || 'Failed to delete session');
    }
  },

  async getSessionsByUserId(userId: string, status?: string): Promise<ChatSession[]> {
    const res = await fetch(`${API_BASE}/chat/session/byUserId`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ user_id: userId, status }),
    });
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to get sessions');
    }
    return Array.isArray(json) ? json : (json.data || []);
  },

  // Chat Message APIs
  async createMessage(data: {
    session_id: string;
    role: string;
    content: string;
    model?: string;
    input_tokens?: number;
    output_tokens?: number;
    total_tokens?: number;
    latency_ms?: number;
    trace?: string;
    status?: string;
    error_msg?: string;
    metadata?: string;
    files?: string; // JSON array of file info
    a2ui?: any; // A2UI rendering data
  }): Promise<{ ulid: string }> {
    // 如果有 a2ui 数据，存储在 metadata 中
    const payload = { ...data };
    if (data.a2ui) {
      const existingMeta = data.metadata ? JSON.parse(data.metadata) : {};
      existingMeta.a2ui = data.a2ui;
      payload.metadata = JSON.stringify(existingMeta);
    }
    delete payload.a2ui;
    const res = await fetch(`${API_BASE}/chat/message`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to create message');
    }
    return json.data || json;
  },

  async updateMessage(data: {
    ulid: string;
    content?: string;
    tokens?: number;
    status?: string;
    error_msg?: string;
  }): Promise<void> {
    const res = await fetch(`${API_BASE}/chat/message`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (!res.ok) {
      const json = await res.json();
      throw new Error(json.message || 'Failed to update message');
    }
  },

  async getMessage(ulid: string): Promise<ChatMessage> {
    const res = await fetch(`${API_BASE}/chat/message/${ulid}`);
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to get message');
    }
    return json.data || json;
  },

  async getMessagesBySessionId(sessionId: string): Promise<ChatMessage[]> {
    const res = await fetch(`${API_BASE}/chat/message/bySessionId`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ session_id: sessionId }),
    });
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to get messages');
    }
    return Array.isArray(json) ? json : (json.data || []);
  },

  // Chat Approval APIs
  async createApproval(data: {
    message_id: string;
    session_id: string;
    tool_name: string;
    tool_type?: string;
    risk_level?: string;
    parameters?: string;
    status?: string;
    interrupt_id?: string;
  }): Promise<{ ulid: string }> {
    const res = await fetch(`${API_BASE}/chat/approval`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to create approval');
    }
    return json.data || json;
  },

  async approveApproval(ulid: string, approvedBy: string, reason?: string): Promise<void> {
    const res = await fetch(`${API_BASE}/chat/approval/approve`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ulid, approved_by: approvedBy, reason }),
    });
    if (!res.ok) {
      const json = await res.json();
      throw new Error(json.message || 'Failed to approve');
    }
  },

  async rejectApproval(ulid: string, approvedBy: string, reason?: string): Promise<void> {
    const res = await fetch(`${API_BASE}/chat/approval/reject`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ulid, approved_by: approvedBy, reason }),
    });
    if (!res.ok) {
      const json = await res.json();
      throw new Error(json.message || 'Failed to reject');
    }
  },

  async getApproval(ulid: string): Promise<ChatApproval> {
    const res = await fetch(`${API_BASE}/chat/approval/${ulid}`);
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to get approval');
    }
    return json.data || json;
  },

  async getApprovalByMessageId(messageId: string): Promise<ChatApproval> {
    const res = await fetch(`${API_BASE}/chat/approval/byMessageId`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message_id: messageId }),
    });
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to get approval');
    }
    return json.data || json;
  },

  async getPendingApprovals(): Promise<ChatApproval[]> {
    const res = await fetch(`${API_BASE}/chat/approval/pending`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({}),
    });
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to get pending approvals');
    }
    return Array.isArray(json) ? json : (json.data || []);
  },

  async getApprovalsByUserId(userId: string): Promise<ChatApproval[]> {
    const res = await fetch(`${API_BASE}/chat/approval/byUserId`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ user_id: userId }),
    });
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to get approvals');
    }
    return Array.isArray(json) ? json : (json.data || []);
  },

  // Runner API - for agent execution
  async runAgent(data: {
    agent_id: string;
    user_id: string;
    session_id?: string;
    input: string;
    files?: any[];
    is_test?: boolean;
  }): Promise<any> {
    const res = await fetch(`${API_BASE}/runner/run`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (!res.ok) {
      const json = await res.json();
      throw new Error(json.message || 'Failed to run agent');
    }
    return res.json();
  },

  // Runner API - streaming version that returns raw Response for SSE
  async runAgentStream(data: {
    agent_id: string;
    user_id: string;
    session_id?: string;
    input: string;
    files?: any[];
    is_test?: boolean;
  }): Promise<Response> {
    const res = await fetch(`${API_BASE}/runner/run`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (!res.ok) {
      throw new Error(`Failed to run agent: ${res.status}`);
    }
    return res;
  },

  // Resume agent execution after approval
  async resumeAgent(data: {
    interrupt_id: string;
    approved: boolean;
    approved_by?: string;
    reason?: string;
  }): Promise<any> {
    const res = await fetch(`${API_BASE}/runner/resume`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (!res.ok) {
      const json = await res.json();
      throw new Error(json.message || 'Failed to resume agent');
    }
    return res.json();
  },

  // Stop agent execution
  async stopAgent(checkpoint_id: string, session_id?: string): Promise<{ stopped: boolean }> {
    console.log('[API] stopAgent called, checkpoint_id:', checkpoint_id, 'session_id:', session_id);
    const res = await fetch(`${API_BASE}/runner/stop`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ checkpoint_id, session_id }),
    });
    if (!res.ok) {
      const json = await res.json();
      throw new Error(json.message || 'Failed to stop agent');
    }
    return res.json();
  },

  // Upload files for agent execution
  async uploadFiles(sessionId: string, files: File[]): Promise<{
    files: Array<{
      name: string;
      size: number;
      type: string;
      virtual_path: string;
    }>;
    count: number;
  }> {
    const formData = new FormData();
    formData.append('session_id', sessionId);
    files.forEach(file => {
      formData.append('files', file);
    });

    const res = await fetch(`${API_BASE}/runner/upload`, {
      method: 'POST',
      body: formData,
    });
    if (!res.ok) {
      const json = await res.json();
      throw new Error(json.message || 'Failed to upload files');
    }
    return res.json();
  },

  // Job execution APIs
  async getJobExecutions(agentId: string, limit: number = 50): Promise<any> {
    const res = await fetch(`${API_BASE}/job/execution/byAgentId?agent_id=${agentId}&limit=${limit}`);
    if (!res.ok) {
      const json = await res.json();
      throw new Error(json.message || 'Failed to get job executions');
    }
    return res.json();
  },

  async getJobExecutionDetail(ulid: string): Promise<any> {
    const res = await fetch(`${API_BASE}/job/execution/${ulid}`);
    if (!res.ok) {
      const json = await res.json();
      throw new Error(json.message || 'Failed to get job execution detail');
    }
    return res.json();
  },
};

// ====== Dashboard APIs ======

export interface DashboardOverview {
  active_agents: number;
  periodic_agents: number;
  tasks_completed: number;
  total_tokens: number;
  active_knowledge_sources: number;
}

export interface TokenUsageItem {
  agent_id: string;
  agent_name: string;
  total_tokens: number;
}

export interface AgentUsageItem {
  agent_id: string;
  agent_name: string;
  session_count: number;
  message_count: number;
}

export interface ChannelActivityItem {
  channel_id: string;
  channel_name: string;
  status: 'active' | 'inactive';
  message_count: number;
}

export const dashboardApi = {
  // Dashboard 统计概览
  async getOverview(): Promise<DashboardOverview | null> {
    try {
      const res = await fetch(`${API_BASE}/dashboard/overview`);
      if (!res.ok) {
        const json = await res.json().catch(() => ({}));
        throw new Error(json.message || `Request failed: ${res.status}`);
      }
      return await res.json();
    } catch (e) {
      console.error('getOverview failed:', e);
      return null;
    }
  },

  // Token 使用排行
  async getTokenUsageRanking(limit: number = 10): Promise<TokenUsageItem[]> {
    try {
      const res = await fetch(`${API_BASE}/dashboard/token-ranking?limit=${limit}`);
      if (!res.ok) {
        const json = await res.json().catch(() => ({}));
        throw new Error(json.message || `Request failed: ${res.status}`);
      }
      const data = await res.json();
      // Handle both array and object response
      if (Array.isArray(data)) return data;
      if (data.rankings && Array.isArray(data.rankings)) return data.rankings;
      return [];
    } catch (e) {
      console.error('getTokenUsageRanking failed:', e);
      return [];
    }
  },

  // 智能体使用排行
  async getAgentUsageRanking(limit: number = 10): Promise<AgentUsageItem[]> {
    try {
      const res = await fetch(`${API_BASE}/dashboard/agent-ranking?limit=${limit}`);
      if (!res.ok) {
        const json = await res.json().catch(() => ({}));
        throw new Error(json.message || `Request failed: ${res.status}`);
      }
      const data = await res.json();
      if (Array.isArray(data)) return data;
      if (data.rankings && Array.isArray(data.rankings)) return data.rankings;
      return [];
    } catch (e) {
      console.error('getAgentUsageRanking failed:', e);
      return [];
    }
  },

  // 渠道活动统计
  async getChannelActivity(): Promise<ChannelActivityItem[]> {
    try {
      const res = await fetch(`${API_BASE}/dashboard/channel-activity`);
      if (!res.ok) {
        const json = await res.json().catch(() => ({}));
        throw new Error(json.message || `Request failed: ${res.status}`);
      }
      const data = await res.json();
      // Handle both array and object response
      if (Array.isArray(data)) return data;
      if (data.channels && Array.isArray(data.channels)) return data.channels;
      return [];
    } catch (e) {
      console.error('getChannelActivity failed:', e);
      return [];
    }
  },

  // 最近会话
  async getRecentSessions(limit: number = 10): Promise<ChatSession[]> {
    try {
      const res = await fetch(`${API_BASE}/dashboard/recent-sessions?limit=${limit}`);
      if (!res.ok) {
        const json = await res.json().catch(() => ({}));
        throw new Error(json.message || `Request failed: ${res.status}`);
      }
      const data = await res.json();
      if (Array.isArray(data)) return data;
      if (data.sessions && Array.isArray(data.sessions)) return data.sessions;
      return [];
    } catch (e) {
      console.error('getRecentSessions failed:', e);
      return [];
    }
  },
};

// ====== Plugin APIs ======

export interface Plugin {
  id: string;
  name: string;
  icon: string;
  description: string;
  auth_type: 'device' | 'oauth2';
  version: string;
  author: string;
  status: 'available' | 'installed' | 'authorized';
  instance_id?: string;
}

export interface PluginInstance {
  ulid: string;
  plugin_id: string;
  status: 'active' | 'revoked' | 'expired';
  user_info?: PluginUserInfo;
  authorized_at: number;
  expires_at?: number;
}

export interface PluginUserInfo {
  open_id: string;
  name: string;
  avatar: string;
  email: string;
}

export interface StartAuthResponse {
  auth_type: string;
  auth_url?: string;
  state: string;
  device_code?: string;
  user_code?: string;
  verification_url?: string;
  expires_in?: number;
  interval?: number;
}

export interface PollAuthResponse {
  status: 'pending' | 'authorized' | 'expired' | 'denied';
  instance_id?: string;
  user_info?: PluginUserInfo;
}

export const pluginApi = {
  // 获取插件列表
  async getPlugins(): Promise<{ plugins: Plugin[] }> {
    const res = await fetch(`${API_BASE}/plugin/list`);
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to get plugins');
    }
    return json.data || json;
  },

  // 获取用户插件实例
  async getUserInstances(): Promise<{ instances: PluginInstance[] }> {
    const res = await fetch(`${API_BASE}/plugin/instances`);
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to get user instances');
    }
    return json.data || json;
  },

  // 获取实例详情
  async getInstanceById(ulid: string): Promise<PluginInstance> {
    const res = await fetch(`${API_BASE}/plugin/instance/${ulid}`);
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to get instance');
    }
    return json.data || json;
  },

  // 删除插件实例
  async deleteInstance(ulid: string): Promise<void> {
    const res = await fetch(`${API_BASE}/plugin/instance/${ulid}`, {
      method: 'DELETE',
    });
    if (!res.ok) {
      const json = await res.json();
      throw new Error(json.message || 'Failed to delete instance');
    }
  },

  // 刷新令牌
  async refreshToken(ulid: string): Promise<{ status: 'active' | 'expired' }> {
    const res = await fetch(`${API_BASE}/plugin/instance/${ulid}/refresh`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ulid }),
    });
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to refresh token');
    }
    return json.data || json;
  },

  // 开始授权
  async startAuth(pluginId: string): Promise<StartAuthResponse> {
    const res = await fetch(`${API_BASE}/plugin/auth/start`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ plugin_id: pluginId }),
    });
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to start auth');
    }
    return json.data || json;
  },

  // 轮询授权状态
  async pollAuth(state: string): Promise<PollAuthResponse> {
    const res = await fetch(`${API_BASE}/plugin/auth/poll`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ state }),
    });
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to poll auth');
    }
    return json.data || json;
  },

  // 获取RSA公钥
  async getPublicKey(): Promise<{ public_key: string }> {
    const res = await fetch(`${API_BASE}/plugin/public-key`);
    const json = await res.json();
    if (!res.ok) {
      throw new Error(json.message || 'Failed to get public key');
    }
    return json.data || json;
  },
};