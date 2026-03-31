import React from 'react';
import ReactMarkdown from 'react-markdown';
import {
  Plus,
  Workflow,
  Zap,
  Database,
  Play,
  Save,
  ChevronRight,
  Settings2,
  Search,
  Box,
  MessageSquare,
  Wrench,
  Users,
  Globe,
  Trash2,
  Send,
  Bot,
  User,
  Sparkles,
  Terminal,
  Brain,
  ShieldAlert,
  ShieldCheck,
  Filter,
  Timer,
  Code,
  Layout,
  X,
  Loader2
} from 'lucide-react';
import { motion, AnimatePresence } from 'motion/react';
import { cn } from '../lib/utils';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { modelApi, skillApi, knowledgeBaseApi, agentApi, channelApi, chatApi } from '../lib/api';
import { Message, Variable } from '../types';

export function AgentOrchestrator() {
  const { t } = useTranslation();

  // Backend Data State
  const [backendModels, setBackendModels] = React.useState<any[]>([]);
  const [backendKBs, setBackendKBs] = React.useState<any[]>([]);
  const [backendSkills, setBackendSkills] = React.useState<any[]>([]);
  const [backendChannels, setBackendChannels] = React.useState<any[]>([]);
  const [loading, setLoading] = React.useState(true);

  // Load data from backend
  React.useEffect(() => {
    const loadData = async () => {
      try {
        setLoading(true);
        const [models, kbs, skills, channels] = await Promise.all([
          modelApi.findAll(),
          knowledgeBaseApi.findAll(),
          skillApi.findAll(),
          channelApi.findAll(),
        ]);
        setBackendModels(models || []);
        setBackendKBs(kbs || []);
        // 映射 risk_level 到 riskLevel
        const mappedSkills = (skills || []).map((s: any) => ({
          ...s,
          riskLevel: s.risk_level || 'low',
        }));
        setBackendSkills(mappedSkills);
        setBackendChannels(channels || []);
      } catch (err) {
        console.error('Failed to load data:', err);
      } finally {
        setLoading(false);
      }
    };
    loadData();
  }, []);

  // Agent Configuration State
  const [agentConfig, setAgentConfig] = React.useState({
    name: 'New Agent',
    description: 'A custom orchestrated agent',
    systemPrompt: '',
    models: {
      default: '',
      rewrite: '',
      skill: '',
      summarize: '',
    },
    temperature: 0.7,
    maxTokens: 2048,
    topK: 3,
    rerank: false,
    selectedKBs: [] as string[],
    selectedSkills: [] as string[],
    requireApproval: false,
    approvalThreshold: 'high' as 'low' | 'medium' | 'high',
    channels: ['web'] as string[],
    isPeriodic: false,
    cronRule: '',
    memoryLimit: 10,
    longTermMemory: false,
    variables: [
      { name: 'query', type: 'string', required: true },
      { name: 'user_id', type: 'string', required: false }
    ] as Variable[],
    retryCount: 3,
    retryInterval: 5,
    timeout: 60,
    endpoint: 'http://localhost:18080/run',
    maxIterations: 10,
    stream: true,
    sandbox: {
      enabled: false,
      mode: 'docker' as 'docker' | 'local',
      image: 'sandbox-code-interpreter:v1.0.3',
      workdir: '/workspace',
      timeoutMs: 120000,
      env: {} as Record<string, string>,
    },
    responseSchema: {
      type: 'text',
      version: '1.0',
      strict: true,
      schema: '{}',
    },
  });

  // Set default models when loaded
  React.useEffect(() => {
    if (backendModels.length > 0) {
      setAgentConfig(prev => ({
        ...prev,
        models: {
          default: backendModels[0].ulid || backendModels[0].id,
          rewrite: backendModels[0].ulid || backendModels[0].id,
          skill: backendModels[0].ulid || backendModels[0].id,
          summarize: backendModels[0].ulid || backendModels[0].id,
        }
      }));
    }
  }, [backendModels]);

  // Deploy Modal State
  const [isDeployModalOpen, setIsDeployModalOpen] = React.useState(false);
  const [deployForm, setDeployForm] = React.useState({
    name: '',
    description: '',
    icon: 'Bot',
  });

  const AGENT_ICONS = ['Bot', 'User', 'Sparkles', 'Brain', 'Zap', 'Workflow', 'MessageSquare', 'Globe', 'Terminal', 'Code'];

  const handleDeployAsAgent = () => {
    setDeployForm({
      name: agentConfig.name,
      description: agentConfig.description,
      icon: 'Bot',
    });
    setIsDeployModalOpen(true);
  };

  const handleConfirmDeploy = async () => {
    try {
      setLoading(true);

      // 生成可运行的完整配置版本 (config_json)
      const generateFullConfig = () => {
        const models: any = {};
        const modelTypes = ['default', 'rewrite', 'skill', 'summarize'] as const;
        modelTypes.forEach(type => {
          const modelId = agentConfig.models[type];
          const model = backendModels.find(m => m.ulid === modelId || m.id === modelId);
          if (model) {
            models[type] = {
              provider: model.provider,
              name: model.name,
              api_key: model.api_key || '',
              api_base: model.baseUrl || '',
            };
          }
        });

        // 转换 skills (工具)
        const tools = backendSkills
          .filter(s => agentConfig.selectedSkills.includes(s.ulid || s.id))
          .filter(s => s.skill_type === 'tool' || s.skill_type === 'mcp')
          .map(s => ({
            type: 'http',
            name: s.name,
            description: s.description || '',
            endpoint: s.endpoint || s.path || '',
            method: 'GET',
            headers: {},
            risk_level: s.riskLevel || 'low',
          }));

        // 转换 skills (mcp)
        const mcps = backendSkills
          .filter(s => agentConfig.selectedSkills.includes(s.ulid || s.id))
          .filter(s => s.skill_type === 'mcp')
          .map(s => ({
            name: s.name,
            transport: 'http',
            endpoint: s.mcpUrl || s.path || '',
            headers: {},
            risk_level: s.riskLevel || 'low',
          }));

        // 转换 skills (a2a)
        const a2as = backendSkills
          .filter(s => agentConfig.selectedSkills.includes(s.ulid || s.id))
          .filter(s => s.skill_type === 'a2a')
          .map(s => ({
            name: s.name,
            endpoint: s.endpoint || s.path || '',
            headers: {},
            risk_level: s.riskLevel || 'low',
          }));

        // 转换 skills (skill类型)
        const skillsConfig = backendSkills
          .filter(s => agentConfig.selectedSkills.includes(s.ulid || s.id))
          .filter(s => s.skill_type === 'skill')
          .map(s => ({
            id: s.name,
            name: s.name,
            description: s.description || '',
            instruction: s.description || '',
            scope: 'both',
            trigger: 'manual',
            entry_script: '',
            file_path: s.path || '',
            risk_level: s.riskLevel || 'medium',
          }));

        // 转换 knowledge
        const knowledge = backendKBs
          .filter(kb => agentConfig.selectedKBs.includes(kb.ulid || kb.id))
          .map(kb => ({
            id: kb.name,
            name: kb.name,
            content: kb.description || '',
            score: 0.9,
            metadata: {},
          }));

        return {
          endpoint: agentConfig.endpoint,
          models,
          system_prompt: agentConfig.systemPrompt,
          user_message: '',
          tools,
          mcps,
          a2a: a2as,
          skills: skillsConfig,
          sandbox: agentConfig.sandbox.enabled ? {
            enabled: true,
            mode: agentConfig.sandbox.mode,
            image: agentConfig.sandbox.image,
            workdir: agentConfig.sandbox.workdir,
            timeout_ms: agentConfig.sandbox.timeoutMs,
            network: 'bridge',
            env: agentConfig.sandbox.env || {},
            limits: {
              cpu: '0.5',
              memory: '512m',
            },
          } : { enabled: false },
          options: {
            temperature: agentConfig.temperature,
            max_tokens: agentConfig.maxTokens,
            max_iterations: agentConfig.maxIterations,
            max_tool_calls: 20,
            max_a2a_calls: 5,
            stream: agentConfig.stream,
            retry: {
              max_attempts: agentConfig.retryCount,
              initial_delay_ms: agentConfig.retryInterval * 1000,
              max_delay_ms: 10000,
              backoff_multiplier: 2.0,
              retryable_errors: ['timeout', 'rate_limit', 'server_error'],
            },
            routing: {
              default_model: 'default',
              rewrite_prompt: '请优化以下用户Query，使其更加清晰、准确，便于理解和执行。只返回优化后的Query，不要其他内容。',
              summarize_prompt: '请总结以下内容，提取关键信息，保持简洁。只返回总结内容，不要其他内容。',
            },
            approval_policy: {
              enabled: agentConfig.requireApproval,
              risk_threshold: agentConfig.approvalThreshold,
              auto_approve: [],
            },
          },
          context: {
            session_id: '',
            user_id: '',
            channel_id: agentConfig.channels[0] || 'web',
            skills_dir: 'skills',
            variables: {},
          },
          knowledge,
          sub_agents: subAgents,
        };
      };

      // 构建 Agent 配置
      const agentData = {
        name: deployForm.name,
        description: deployForm.description,
        icon: deployForm.icon,
        model: agentConfig.models.default,
        enabled: true,
        channels: JSON.stringify(agentConfig.channels),
        is_periodic: agentConfig.isPeriodic,
        cron_rule: agentConfig.cronRule,
        // config: ID版本，用于数据库关联
        config: JSON.stringify({
          systemPrompt: agentConfig.systemPrompt,
          models: agentConfig.models,
          temperature: agentConfig.temperature,
          maxTokens: agentConfig.maxTokens,
          topK: agentConfig.topK,
          rerank: agentConfig.rerank,
          selectedSkills: agentConfig.selectedSkills,
          selectedKBs: agentConfig.selectedKBs,
          requireApproval: agentConfig.requireApproval,
          approvalThreshold: agentConfig.approvalThreshold,
          channels: agentConfig.channels,
          isPeriodic: agentConfig.isPeriodic,
          cronRule: agentConfig.cronRule,
          memoryLimit: agentConfig.memoryLimit,
          longTermMemory: agentConfig.longTermMemory,
          variables: agentConfig.variables,
          retryCount: agentConfig.retryCount,
          retryInterval: agentConfig.retryInterval,
          timeout: agentConfig.timeout,
          endpoint: agentConfig.endpoint,
          maxIterations: agentConfig.maxIterations,
          stream: agentConfig.stream,
          sandbox: agentConfig.sandbox,
          responseSchema: agentConfig.responseSchema,
          subAgents: subAgents,
        }),
        // config_json: 可直接运行的完整配置版本
        config_json: JSON.stringify(generateFullConfig(), null, 2),
        is_system: false,
      };
      console.log('Deploy agent:', agentData);
      const result = await agentApi.create(agentData as any);
      console.log('Agent created:', result);
      // 保存部署的 agent ID 用于测试
      if (result && result.ulid) {
        setDeployedAgentId(result.ulid);
      }
      toast.success(`Agent "${deployForm.name}" created successfully!`);
      setIsDeployModalOpen(false);
    } catch (err: any) {
      console.error('Failed to create agent:', err);
      toast.error(err.message || 'Failed to create agent');
    } finally {
      setLoading(false);
    }
  };

  const handleSaveDraft = () => {
    try {
      // 保存草稿到 localStorage
      const draftData = {
        ...agentConfig,
        savedAt: Date.now(),
      };
      localStorage.setItem('orchestrator_draft', JSON.stringify(draftData));
      toast.success('Draft saved successfully!');
    } catch (err: any) {
      console.error('Failed to save draft:', err);
      toast.error(err.message || 'Failed to save draft');
    }
  };

  // 加载草稿
  React.useEffect(() => {
    const savedDraft = localStorage.getItem('orchestrator_draft');
    if (savedDraft) {
      try {
        const draft = JSON.parse(savedDraft);
        // 恢复草稿，但保留 name/description 不覆盖（用户可能改了）
        setAgentConfig(prev => ({
          ...draft,
          name: prev.name,
          description: prev.description,
        }));
      } catch (e) {
        console.error('Failed to load draft:', e);
      }
    }
  }, []);

  // Test Chat State
  const [testMessages, setTestMessages] = React.useState<Message[]>([]);
  const [testInput, setTestInput] = React.useState('');
  const [isTesting, setIsTesting] = React.useState(false);
  const [isTestPanelCollapsed, setIsTestPanelCollapsed] = React.useState(false);
  const testScrollRef = React.useRef<HTMLDivElement>(null);
  const testAbortControllerRef = React.useRef<AbortController | null>(null);
  const [deployedAgentId, setDeployedAgentId] = React.useState<string | null>(null);
  const [testCheckpointId, setTestCheckpointId] = React.useState<string | null>(null);

  // Skill Category State
  const [skillCategory, setSkillCategory] = React.useState<'all' | 'built-in' | 'mcp' | 'tool' | 'a2a' | 'skill'>('all');

  // Sub-Agents State
  const [subAgents, setSubAgents] = React.useState<any[]>([]);
  const [isSubAgentModalOpen, setIsSubAgentModalOpen] = React.useState(false);
  const [editingSubAgent, setEditingSubAgent] = React.useState<any>(null);

  const filteredSkills = backendSkills.filter(skill =>
    skillCategory === 'all' || skill.skill_type === skillCategory
  );

  const handleToggleKB = (kbId: string) => {
    setAgentConfig(prev => ({
      ...prev,
      selectedKBs: prev.selectedKBs.includes(kbId)
        ? prev.selectedKBs.filter(id => id !== kbId)
        : [...prev.selectedKBs, kbId]
    }));
  };

  const handleToggleSkill = (skillId: string) => {
    setAgentConfig(prev => ({
      ...prev,
      selectedSkills: prev.selectedSkills.includes(skillId)
        ? prev.selectedSkills.filter(id => id !== skillId)
        : [...prev.selectedSkills, skillId]
    }));
  };

  const handleToggleChannel = (channel: string) => {
    setAgentConfig(prev => ({
      ...prev,
      channels: prev.channels.includes(channel)
        ? prev.channels.filter(c => c !== channel)
        : [...prev.channels, channel]
    }));
  };

  // Sub-Agent handlers
  const handleAddSubAgent = () => {
    setEditingSubAgent({
      id: `agent_${Date.now()}`,
      name: '',
      description: '',
      prompt: '',
      max_iterations: 5,
      timeout_ms: 120000,
    });
    setIsSubAgentModalOpen(true);
  };

  const handleEditSubAgent = (agent: any) => {
    setEditingSubAgent({ ...agent });
    setIsSubAgentModalOpen(true);
  };

  const handleDeleteSubAgent = (agentId: string) => {
    setSubAgents(prev => prev.filter(a => a.id !== agentId));
  };

  const handleSaveSubAgent = () => {
    if (!editingSubAgent) return;

    const exists = subAgents.find(a => a.id === editingSubAgent.id);
    if (exists) {
      setSubAgents(prev => prev.map(a => a.id === editingSubAgent.id ? editingSubAgent : a));
    } else {
      setSubAgents(prev => [...prev, editingSubAgent]);
    }
    setIsSubAgentModalOpen(false);
    setEditingSubAgent(null);
  };

  const handleApprove = async (messageId: string) => {
    // 找到待审批消息
    const msg = testMessages.find(m => m.id === messageId);
    if (!msg || !msg.interruptId) {
      console.error('No interrupt_id found for approval');
      return;
    }

    if (!testCheckpointId) {
      console.error('No checkpoint_id found for resume');
      return;
    }

    try {
      // 调用 resume API，使用正确的 checkpoint_id 格式
      const response = await chatApi.resumeAgent({
        checkpoint_id: testCheckpointId,
        approvals: [{
          interrupt_id: msg.interruptId,
          approved: true
        }]
      });

      // 更新消息状态
      setTestMessages(prev => prev.map(m => {
        if (m.id === messageId) {
          return {
            ...m,
            status: 'completed',
            content: response.content || response.output || "工具已成功执行。",
            thinking: response.thinking,
            trace: response.trace
          };
        }
        return m;
      }));
    } catch (err: any) {
      console.error('Failed to approve:', err);
      setTestMessages(prev => prev.map(m => {
        if (m.id === messageId) {
          return {
            ...m,
            status: 'failed',
            content: m.content + `\n\n❌ **批准失败**: ${err.message}`
          };
        }
        return m;
      }));
    }
  };

  const handleReject = async (messageId: string) => {
    // 找到待审批消息
    const msg = testMessages.find(m => m.id === messageId);
    if (!msg || !msg.interruptId) {
      console.error('No interrupt_id found for rejection');
      return;
    }

    if (!testCheckpointId) {
      console.error('No checkpoint_id found for resume');
      return;
    }

    try {
      // 调用 resume API，使用正确的 checkpoint_id 格式
      const response = await chatApi.resumeAgent({
        checkpoint_id: testCheckpointId,
        approvals: [{
          interrupt_id: msg.interruptId,
          approved: false
        }]
      });

      // 更新消息状态
      setTestMessages(prev => prev.map(m => {
        if (m.id === messageId) {
          return {
            ...m,
            status: 'failed',
            content: response.content || "操作已被用户拒绝。",
            trace: response.trace
          };
        }
        return m;
      }));
    } catch (err: any) {
      console.error('Failed to reject:', err);
      setTestMessages(prev => prev.map(m => {
        if (m.id === messageId) {
          return {
            ...m,
            status: 'failed',
            content: m.content + `\n\n❌ **拒绝失败**: ${err.message}`
          };
        }
        return m;
      }));
    }
  };

  const handleSendMessage = async () => {
    if (!testInput.trim()) return;

    // 检查是否已部署 agent
    if (!deployedAgentId) {
      toast('请先部署 Agent 再进行测试');
      return;
    }

    const userMsg: Message = {
      id: Date.now().toString(),
      role: 'user',
      content: testInput,
      timestamp: new Date()
    };

    setTestMessages(prev => [...prev, userMsg]);
    setTestInput('');
    setIsTesting(true);
    testAbortControllerRef.current = new AbortController();

    try {
      // 调用真实的 runner API (流式版本)
      const response = await chatApi.runAgentStream({
        agent_id: deployedAgentId,
        user_id: 'test-user',
        session_id: '',  // 测试模式不需要 session
        input: testInput,
        is_test: true
      });

      // 检查是否流式响应
      const contentType = response.headers.get('content-type') || '';
      const isStreaming = contentType.includes('text/event-stream');

      if (isStreaming) {
        // 流式响应处理
        const reader = response.body?.getReader();
        if (!reader) {
          throw new Error('No reader available');
        }

        const assistantMsgId = (Date.now() + 1).toString();
        let accumulatedContent = '';
        let streamTokenUsage: { prompt_tokens?: number; completion_tokens?: number; total_tokens?: number } = {};

        // 先创建一条空消息用于流式更新
        const assistantMessage: Message = {
          id: assistantMsgId,
          role: 'assistant',
          content: '',
          timestamp: new Date(),
          status: 'streaming'
        };
        setTestMessages(prev => [...prev, assistantMessage]);

        const decoder = new TextDecoder();
        let buffer = '';
        let currentEventType = '';

        try {
          while (true) {
            const { done, value } = await reader.read();
            if (done) break;

            buffer += decoder.decode(value, { stream: true });
            const lines = buffer.split('\n');
            buffer = lines.pop() || '';

            for (const line of lines) {
              const trimmedLine = line.trim();
              if (!trimmedLine) continue;

              if (trimmedLine.startsWith('event: ')) {
                currentEventType = trimmedLine.slice(7).trim();
              } else if (trimmedLine.startsWith('data: ')) {
                // Standard SSE format with "data: " prefix
                const dataStr = trimmedLine.slice(6).trim();
                try {
                  const data = JSON.parse(dataStr);

                  if (data.text) {
                    accumulatedContent += data.text;
                    setTestMessages(prev => prev.map(m =>
                      m.id === assistantMsgId
                        ? { ...m, content: accumulatedContent }
                        : m
                    ));
                  }

                  if (currentEventType === 'done') {
                    streamTokenUsage = {
                      prompt_tokens: data.prompt_tokens,
                      completion_tokens: data.completion_tokens,
                      total_tokens: data.total_tokens
                    };
                    setTestMessages(prev => prev.map(m =>
                      m.id === assistantMsgId
                        ? { ...m, content: data.content || accumulatedContent, status: 'completed' }
                        : m
                    ));

                    // Store checkpoint_id for resume
                    if (data.checkpoint_id) {
                      setTestCheckpointId(data.checkpoint_id);
                    }
                  }

                  if (currentEventType === 'error') {
                    console.error('Stream error:', data.error);
                  }
                } catch (e) {
                  // 忽略解析错误
                }
              } else if (trimmedLine.startsWith('{')) {
                // Runner sends JSON directly after event line without "data: " prefix
                try {
                  const data = JSON.parse(trimmedLine);

                  if (data.text) {
                    accumulatedContent += data.text;
                    setTestMessages(prev => prev.map(m =>
                      m.id === assistantMsgId
                        ? { ...m, content: accumulatedContent }
                        : m
                    ));
                  }

                  if (currentEventType === 'done') {
                    streamTokenUsage = {
                      prompt_tokens: data.prompt_tokens,
                      completion_tokens: data.completion_tokens,
                      total_tokens: data.total_tokens
                    };
                    setTestMessages(prev => prev.map(m =>
                      m.id === assistantMsgId
                        ? { ...m, content: data.content || accumulatedContent, status: 'completed' }
                        : m
                    ));

                    // Store checkpoint_id for resume
                    if (data.checkpoint_id) {
                      setTestCheckpointId(data.checkpoint_id);
                    }
                  }

                  if (currentEventType === 'error') {
                    console.error('Stream error:', data.error);
                  }
                } catch (e) {
                  // 忽略解析错误
                }
              }
            }
          }
        } finally {
          reader.releaseLock();
        }

        // 记录 token 消耗（测试面板不需要持久化）
        if (streamTokenUsage.total_tokens) {
          console.log(`Token usage - prompt: ${streamTokenUsage.prompt_tokens}, completion: ${streamTokenUsage.completion_tokens}, total: ${streamTokenUsage.total_tokens}`);
        }

        setIsTesting(false);
        testAbortControllerRef.current = null;
        return;
      }

      // 非流式响应处理
      const data = await response.json();

      const aiMsg: Message = {
        id: (Date.now() + 1).toString(),
        role: 'assistant',
        content: data.content || data.output || "I'm sorry, I couldn't generate a response.",
        timestamp: new Date(),
        thinking: data.thinking,
        trace: data.trace,
        status: data.status
      };
      setTestMessages(prev => [...prev, aiMsg]);

      // Store checkpoint_id for resume
      if (data.checkpoint_id) {
        setTestCheckpointId(data.checkpoint_id);
      }

      // 处理 pending approvals
      if (data.pending_approvals && data.pending_approvals.length > 0) {
        for (const approval of data.pending_approvals) {
          const pendingApproval: Message = {
            id: (Date.now() + 2).toString(),
            role: 'assistant',
            content: `我需要使用 **${approval.tool_name}** 工具来处理您的请求。由于这是一个 **${approval.risk_level}** 风险的操作，需要您的审批。`,
            timestamp: new Date(),
            status: 'pending_approval',
            interruptId: approval.interrupt_id,
            trace: [
              { id: '1', type: 'thought' as const, label: '思考', content: `用户请求：${testInput}。需要调用 ${approval.tool_name}。`, status: 'success' as const, timestamp: new Date() },
              { id: '2', type: 'tool' as const, label: approval.tool_name, content: '等待人工审批...', status: 'pending' as const, timestamp: new Date() }
            ]
          };
          setTestMessages(prev => [...prev, pendingApproval]);
        }
      }
    } catch (err: any) {
      console.error('Failed to run agent:', err);
      const errorMsg: Message = {
        id: (Date.now() + 1).toString(),
        role: 'assistant',
        content: `错误: ${err.message || 'Failed to get response'}`,
        timestamp: new Date(),
        status: 'failed'
      };
      setTestMessages(prev => [...prev, errorMsg]);
    } finally {
      setIsTesting(false);
      testAbortControllerRef.current = null;
    }
  };

  const stopTestGeneration = () => {
    if (testAbortControllerRef.current) {
      testAbortControllerRef.current.abort();
    }
  };

  const getSkillIcon = (type: string) => {
    switch (type) {
      case 'mcp': return <Box size={14} />;
      case 'tool': return <Wrench size={14} />;
      case 'a2a': return <Users size={14} />;
      default: return <Zap size={14} />;
    }
  };

  return (
    <div className="h-full flex flex-col bg-slate-50 overflow-hidden">
      {/* Header */}
      <header className="h-16 border-b border-slate-200 bg-white flex items-center justify-between px-6 shrink-0 z-20">
        <div className="flex items-center gap-4">
          <div className="w-10 h-10 rounded-xl bg-brand-500/10 flex items-center justify-center text-brand-500">
            <Workflow size={20} />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <input
                value={agentConfig.name}
                onChange={(e) => setAgentConfig(prev => ({ ...prev, name: e.target.value }))}
                className="text-sm font-bold text-slate-900 bg-transparent border-none focus:ring-0 p-0 w-40"
              />
              <span className="text-[10px] font-bold uppercase tracking-wider bg-slate-100 text-slate-500 px-1.5 py-0.5 rounded">{t('orchestrator.draft')}</span>
            </div>
            <p className="text-[10px] text-slate-400 font-medium uppercase tracking-wider">{t('orchestrator.orchestratorMode')}</p>
          </div>
        </div>
        <div className="flex items-center gap-3">
          <button
            onClick={handleSaveDraft}
            className="flex items-center gap-2 px-4 py-2 bg-white border border-slate-200 rounded-lg text-sm font-bold text-slate-600 hover:bg-slate-50 transition-all"
          >
            <Save size={16} />
            {t('orchestrator.saveDraft')}
          </button>
          <button
            onClick={handleDeployAsAgent}
            className="flex items-center gap-2 px-4 py-2 bg-brand-500 text-white rounded-lg text-sm font-bold hover:bg-brand-600 transition-all shadow-lg shadow-brand-500/20"
          >
            <Sparkles size={16} />
            {t('orchestrator.deployAsAgent')}
          </button>
        </div>
      </header>

      <div className="flex-1 flex overflow-hidden">
        {/* Left Panel: Configuration */}
        <div className="flex-1 overflow-y-auto p-6 space-y-8 scrollbar-hide">
          {/* Visual Pipeline Preview */}
          <section className="space-y-4">
            <div className="flex items-center gap-2 text-slate-900">
              <Workflow size={18} className="text-brand-500" />
              <h2 className="font-bold">{t('orchestrator.visualPipeline')}</h2>
            </div>
            <div className="bg-slate-900 rounded-2xl p-8 flex items-center justify-between relative overflow-hidden">
              <div className="absolute inset-0 opacity-10 bg-[radial-gradient(circle_at_center,_var(--tw-gradient-stops))] from-brand-500 via-transparent to-transparent" />

              {[
                { id: 'trigger', label: t('orchestrator.pipelineTrigger'), icon: Zap, active: true },
                { id: 'retrieval', label: t('orchestrator.knowledgeBases'), icon: Database, active: agentConfig.selectedKBs.length > 0 },
                { id: 'reasoning', label: t('orchestrator.pipelineReasoning'), icon: Brain, active: true },
                { id: 'tools', label: t('orchestrator.pipelineTools'), icon: Wrench, active: agentConfig.selectedSkills.length > 0 },
                { id: 'approval', label: t('orchestrator.pipelineApproval'), icon: ShieldAlert, active: agentConfig.requireApproval },
                { id: 'response', label: t('orchestrator.pipelineResponse'), icon: Sparkles, active: true },
              ].map((step, idx, arr) => (
                <React.Fragment key={step.id}>
                  <div className={cn(
                    "flex flex-col items-center gap-3 relative z-10 transition-all duration-500",
                    step.active ? "opacity-100 scale-110" : "opacity-30 grayscale"
                  )}>
                    <div className={cn(
                      "w-12 h-12 rounded-xl flex items-center justify-center transition-all shadow-lg",
                      step.active ? "bg-brand-500 text-white shadow-brand-500/20" : "bg-slate-800 text-slate-500"
                    )}>
                      <step.icon size={24} />
                    </div>
                    <span className="text-[10px] font-bold uppercase tracking-widest text-slate-400">{step.label}</span>
                  </div>
                  {idx < arr.length - 1 && (
                    <div className="flex-1 h-px bg-slate-800 relative mx-4">
                      <motion.div
                        initial={{ left: '-100%' }}
                        animate={{ left: '100%' }}
                        transition={{ repeat: Infinity, duration: 2, ease: "linear" }}
                        className={cn(
                          "absolute top-0 h-full w-1/2 bg-gradient-to-r from-transparent via-brand-500 to-transparent",
                          !arr[idx + 1].active && "hidden"
                        )}
                      />
                    </div>
                  )}
                </React.Fragment>
              ))}
            </div>
          </section>

          {/* Model Selection */}
          <section className="space-y-4">
            <div className="flex items-center gap-2 text-slate-900">
              <Brain size={18} className="text-brand-500" />
              <h2 className="font-bold">{t('orchestrator.modelConfig')}</h2>
            </div>
            <div className="grid grid-cols-4 gap-4">
              <div className="space-y-2">
                <label className="text-[10px] font-bold text-slate-400 uppercase tracking-widest">Default</label>
                <select
                  value={agentConfig.models.default}
                  onChange={(e) => setAgentConfig(prev => ({ ...prev, models: { ...prev.models, default: e.target.value } }))}
                  className="w-full bg-white border border-slate-200 rounded-xl px-4 py-2.5 text-sm focus:ring-2 focus:ring-brand-500/20 outline-none"
                >
                  {backendModels.map(m => <option key={m.ulid || m.id} value={m.ulid || m.id}>{m.name}</option>)}
                </select>
              </div>
              <div className="space-y-2">
                <label className="text-[10px] font-bold text-slate-400 uppercase tracking-widest">Rewrite</label>
                <select
                  value={agentConfig.models.rewrite}
                  onChange={(e) => setAgentConfig(prev => ({ ...prev, models: { ...prev.models, rewrite: e.target.value } }))}
                  className="w-full bg-white border border-slate-200 rounded-xl px-4 py-2.5 text-sm focus:ring-2 focus:ring-brand-500/20 outline-none"
                >
                  {backendModels.map(m => <option key={m.ulid || m.id} value={m.ulid || m.id}>{m.name}</option>)}
                </select>
              </div>
              <div className="space-y-2">
                <label className="text-[10px] font-bold text-slate-400 uppercase tracking-widest">Skill</label>
                <select
                  value={agentConfig.models.skill}
                  onChange={(e) => setAgentConfig(prev => ({ ...prev, models: { ...prev.models, skill: e.target.value } }))}
                  className="w-full bg-white border border-slate-200 rounded-xl px-4 py-2.5 text-sm focus:ring-2 focus:ring-brand-500/20 outline-none"
                >
                  {backendModels.map(m => <option key={m.ulid || m.id} value={m.ulid || m.id}>{m.name}</option>)}
                </select>
              </div>
              <div className="space-y-2">
                <label className="text-[10px] font-bold text-slate-400 uppercase tracking-widest">Summarize</label>
                <select
                  value={agentConfig.models.summarize}
                  onChange={(e) => setAgentConfig(prev => ({ ...prev, models: { ...prev.models, summarize: e.target.value } }))}
                  className="w-full bg-white border border-slate-200 rounded-xl px-4 py-2.5 text-sm focus:ring-2 focus:ring-brand-500/20 outline-none"
                >
                  {backendModels.map(m => <option key={m.ulid || m.id} value={m.ulid || m.id}>{m.name}</option>)}
                </select>
              </div>
            </div>

            {/* Advanced Model Params */}
            <div className="p-4 bg-white border border-slate-100 rounded-xl space-y-4">
              <div className="flex items-center gap-2 text-[10px] font-bold text-slate-400 uppercase tracking-widest">
                <Settings2 size={12} />
                {t('orchestrator.advancedParams')}
              </div>
              <div className="grid grid-cols-2 gap-6">
                <div className="space-y-2">
                  <div className="flex justify-between">
                    <label className="text-xs font-medium text-slate-600">{t('orchestrator.temperature')}</label>
                    <span className="text-xs font-bold text-brand-500">{agentConfig.temperature}</span>
                  </div>
                  <input
                    type="range"
                    min="0"
                    max="1"
                    step="0.1"
                    value={agentConfig.temperature}
                    onChange={(e) => setAgentConfig(prev => ({ ...prev, temperature: parseFloat(e.target.value) }))}
                    className="w-full accent-brand-500"
                  />
                </div>
                <div className="space-y-2">
                  <div className="flex justify-between">
                    <label className="text-xs font-medium text-slate-600">{t('orchestrator.maxTokens')}</label>
                    <span className="text-xs font-bold text-brand-500">{agentConfig.maxTokens}</span>
                  </div>
                  <input
                    type="range"
                    min="256"
                    max="8192"
                    step="256"
                    value={agentConfig.maxTokens}
                    onChange={(e) => setAgentConfig(prev => ({ ...prev, maxTokens: parseInt(e.target.value) }))}
                    className="w-full accent-brand-500"
                  />
                </div>
              </div>
            </div>
          </section>

          {/* Knowledge Bases */}
          <section className="space-y-4">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2 text-slate-900">
                <Database size={18} className="text-brand-500" />
                <h2 className="font-bold">{t('orchestrator.knowledgeBases')}</h2>
              </div>
              <div className="flex items-center gap-2">
                <button
                  onClick={() => setAgentConfig(prev => ({ ...prev, selectedKBs: backendKBs.map(kb => kb.ulid || kb.id) }))}
                  className="text-[10px] font-bold text-brand-500 hover:text-brand-600 uppercase tracking-wider"
                >
                  {t('orchestrator.selectAll')}
                </button>
                <span className="text-slate-300">|</span>
                <button
                  onClick={() => setAgentConfig(prev => ({ ...prev, selectedKBs: [] }))}
                  className="text-[10px] font-bold text-slate-400 hover:text-slate-600 uppercase tracking-wider"
                >
                  {t('orchestrator.clearAll')}
                </button>
              </div>
            </div>

            {/* KB Search */}
            <div className="relative">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" size={14} />
              <input
                type="text"
                placeholder={t('orchestrator.searchKBs')}
                className="w-full pl-9 pr-4 py-2 bg-white border border-slate-200 rounded-xl text-xs focus:ring-2 focus:ring-brand-500/20 outline-none"
              />
            </div>

            <div className="grid grid-cols-2 gap-3">
              {backendKBs.map(kb => (
                <button
                  key={kb.ulid || kb.id}
                  onClick={() => handleToggleKB(kb.ulid || kb.id)}
                  className={cn(
                    "flex items-center gap-3 p-3 rounded-xl border-2 transition-all text-left group",
                    agentConfig.selectedKBs.includes(kb.ulid || kb.id)
                      ? "bg-brand-50 border-brand-500"
                      : "bg-white border-slate-100 hover:border-slate-200"
                  )}
                >
                  <div className={cn(
                    "p-2 rounded-lg transition-colors",
                    agentConfig.selectedKBs.includes(kb.ulid || kb.id) ? "bg-brand-500 text-white" : "bg-slate-100 text-slate-500 group-hover:bg-slate-200"
                  )}>
                    <Database size={16} />
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-bold text-slate-900 truncate">{kb.name}</p>
                    <p className="text-[10px] text-slate-400 line-clamp-1">{kb.description}</p>
                  </div>
                  {agentConfig.selectedKBs.includes(kb.ulid || kb.id) && (
                    <div className="w-4 h-4 rounded-full bg-brand-500 flex items-center justify-center text-white">
                      <Plus size={10} className="rotate-45" />
                    </div>
                  )}
                </button>
              ))}
            </div>

            {/* Retrieval Settings */}
            {agentConfig.selectedKBs.length > 0 && (
              <div className="p-4 bg-white border border-slate-100 rounded-xl space-y-4 animate-in fade-in slide-in-from-top-2">
                <div className="flex items-center gap-2 text-[10px] font-bold text-slate-400 uppercase tracking-widest">
                  <Filter size={12} />
                  {t('orchestrator.retrievalSettings')}
                </div>
                <div className="grid grid-cols-2 gap-6">
                  <div className="space-y-2">
                    <div className="flex justify-between">
                      <label className="text-xs font-medium text-slate-600">{t('orchestrator.topK')}</label>
                      <span className="text-xs font-bold text-brand-500">{agentConfig.topK}</span>
                    </div>
                    <input
                      type="range"
                      min="1"
                      max="10"
                      value={agentConfig.topK}
                      onChange={(e) => setAgentConfig(prev => ({ ...prev, topK: parseInt(e.target.value) }))}
                      className="w-full accent-brand-500"
                    />
                  </div>
                  <div className="flex items-center justify-between">
                    <div>
                      <h4 className="text-xs font-medium text-slate-900">{t('orchestrator.rerank')}</h4>
                      <p className="text-[10px] text-slate-500">{t('orchestrator.rerankDesc')}</p>
                    </div>
                    <button
                      onClick={() => setAgentConfig(prev => ({ ...prev, rerank: !prev.rerank }))}
                      className={cn(
                        "w-10 h-5 rounded-full transition-all relative",
                        agentConfig.rerank ? "bg-brand-500" : "bg-slate-200"
                      )}
                    >
                      <div className={cn(
                        "absolute top-0.5 w-4 h-4 bg-white rounded-full transition-all",
                        agentConfig.rerank ? "left-5.5" : "left-0.5"
                      )} />
                    </button>
                  </div>
                </div>
              </div>
            )}
          </section>

          {/* Skills & Tools */}
          <section className="space-y-4">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2 text-slate-900">
                <Zap size={18} className="text-brand-500" />
                <h2 className="font-bold">{t('orchestrator.skillsAndCapabilities')}</h2>
              </div>

              {/* Skill Category Tabs */}
              <div className="flex p-1 bg-slate-100 rounded-lg">
                {(['all', 'skill', 'mcp', 'tool', 'a2a'] as const).map((cat) => (
                  <button
                    key={cat}
                    onClick={() => setSkillCategory(cat)}
                    className={cn(
                      "px-3 py-1 text-[9px] font-bold uppercase tracking-wider rounded-md transition-all",
                      skillCategory === cat ? "bg-white text-slate-900 shadow-sm" : "text-slate-500 hover:text-slate-700"
                    )}
                  >
                    {cat === 'skill' ? t('skills.skillsTab') : (cat === 'all' ? 'All' : t(`skills.${cat}Tab`))}
                  </button>
                ))}
              </div>
            </div>

            <div className="grid grid-cols-3 gap-3">
              {filteredSkills.map(skill => (
                <button
                  key={skill.ulid || skill.id}
                  onClick={() => handleToggleSkill(skill.ulid || skill.id)}
                  className={cn(
                    "flex flex-col gap-2 p-3 rounded-xl border-2 transition-all text-left",
                    agentConfig.selectedSkills.includes(skill.ulid || skill.id)
                      ? "bg-brand-50 border-brand-500"
                      : "bg-white border-slate-100 hover:border-slate-200"
                  )}
                >
                  <div className="flex items-center justify-between">
                    <div className={cn(
                      "p-1.5 rounded-lg",
                      agentConfig.selectedSkills.includes(skill.ulid || skill.id) ? "bg-brand-500 text-white" : "bg-slate-100 text-slate-500"
                    )}>
                      {getSkillIcon(skill.type)}
                    </div>
                    <div className="flex items-center gap-1.5">
                      {skill.riskLevel && (
                        <span className={cn(
                          "text-[8px] font-bold uppercase tracking-wider px-1.5 py-0.5 rounded-full",
                          skill.riskLevel === 'high' ? "bg-red-100 text-red-600" :
                            skill.riskLevel === 'medium' ? "bg-amber-100 text-amber-600" :
                              "bg-emerald-100 text-emerald-600"
                        )}>
                          {skill.riskLevel}
                        </span>
                      )}
                      <span className="text-[9px] font-bold uppercase tracking-wider text-slate-400">{skill.type}</span>
                    </div>
                  </div>
                  <div>
                    <p className="text-xs font-bold text-slate-900 line-clamp-1">{skill.name}</p>
                    <p className="text-[10px] text-slate-500 line-clamp-1 mt-0.5">{skill.description}</p>
                  </div>
                </button>
              ))}
            </div>
          </section>

          {/* Sub-Agents Section */}
          <section className="space-y-4">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2 text-slate-900">
                <Users size={18} className="text-brand-500" />
                <h2 className="font-bold">{t('orchestrator.subAgents')}</h2>
              </div>
              <button
                onClick={handleAddSubAgent}
                className="flex items-center gap-1.5 px-3 py-1.5 bg-brand-500 text-white rounded-lg text-xs font-bold hover:bg-brand-600 transition-all"
              >
                <Plus size={14} />
                {t('orchestrator.addSubAgent')}
              </button>
            </div>

            <div className="bg-white border border-slate-200 rounded-2xl overflow-hidden">
              {subAgents.length === 0 ? (
                <div className="py-12 text-center">
                  <div className="w-12 h-12 rounded-2xl bg-slate-50 flex items-center justify-center text-slate-300 mx-auto mb-4">
                    <Users size={24} />
                  </div>
                  <p className="text-sm font-medium text-slate-500">{t('orchestrator.noSubAgents')}</p>
                  <p className="text-xs text-slate-400 mt-1">{t('orchestrator.noSubAgentsDesc')}</p>
                </div>
              ) : (
                <div className="divide-y divide-slate-100">
                  {subAgents.map((agent) => (
                    <div key={agent.id} className="p-4 hover:bg-slate-50 transition-colors">
                      <div className="flex items-start justify-between">
                        <div className="flex-1">
                          <div className="flex items-center gap-2">
                            <span className="text-sm font-bold text-slate-900">{agent.name || t('orchestrator.unnamedAgent')}</span>
                            <span className="text-[9px] font-mono bg-slate-100 text-slate-500 px-1.5 py-0.5 rounded">{agent.id}</span>
                          </div>
                          <p className="text-xs text-slate-500 mt-1 line-clamp-1">{agent.description || t('orchestrator.noDescription')}</p>
                          <div className="flex items-center gap-4 mt-2">
                            <span className="text-[10px] text-slate-400">
                              <span className="font-medium">{t('orchestrator.maxIterations')}:</span> {agent.max_iterations}
                            </span>
                            <span className="text-[10px] text-slate-400">
                              <span className="font-medium">{t('orchestrator.timeoutMs')}:</span> {agent.timeout_ms}ms
                            </span>
                          </div>
                        </div>
                        <div className="flex items-center gap-2 ml-4">
                          <button
                            onClick={() => handleEditSubAgent(agent)}
                            className="p-1.5 text-slate-400 hover:text-brand-500 hover:bg-brand-50 rounded-lg transition-all"
                          >
                            <Settings2 size={14} />
                          </button>
                          <button
                            onClick={() => handleDeleteSubAgent(agent.id)}
                            className="p-1.5 text-slate-400 hover:text-red-500 hover:bg-red-50 rounded-lg transition-all"
                          >
                            <Trash2 size={14} />
                          </button>
                        </div>
                      </div>
                      {agent.prompt && (
                        <div className="mt-3 p-2 bg-slate-50 rounded-lg">
                          <p className="text-[10px] text-slate-500 line-clamp-2 font-mono">{agent.prompt}</p>
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </div>

            {subAgents.length > 0 && (
              <div className="flex items-center gap-6 p-4 bg-slate-50 rounded-xl">
                <div className="flex items-center gap-2">
                  <span className="text-xs font-medium text-slate-500">{t('orchestrator.total')}:</span>
                  <span className="text-xs font-bold text-brand-500">{subAgents.length} {t('orchestrator.agentCount')}</span>
                </div>
                <div className="flex items-center gap-2">
                  <span className="text-xs font-medium text-slate-500">{t('orchestrator.maxConcurrent')}:</span>
                  <span className="text-xs font-bold text-brand-500">3</span>
                </div>
              </div>
            )}
          </section>

          {/* System Prompt */}
          <section className="space-y-4">
            <div className="flex items-center gap-2 text-slate-900">
              <Terminal size={18} className="text-brand-500" />
              <h2 className="font-bold">{t('orchestrator.systemPrompt')}</h2>
            </div>
            <div className="space-y-4">
              {/* Variable Management */}
              <div className="bg-white border border-slate-200 rounded-2xl p-4 space-y-4">
                <div className="flex items-center justify-between">
                  <div>
                    <h3 className="text-xs font-bold text-slate-900">{t('orchestrator.variables')}</h3>
                    <p className="text-[10px] text-slate-500">{t('orchestrator.variableDesc')}</p>
                  </div>
                  <button
                    onClick={() => setAgentConfig(prev => ({
                      ...prev,
                      variables: [...prev.variables, { name: '', type: 'string', required: false }]
                    }))}
                    className="p-1.5 bg-slate-100 text-slate-600 rounded-lg hover:bg-slate-200 transition-all"
                  >
                    <Plus size={14} />
                  </button>
                </div>
                <div className="space-y-2">
                  {agentConfig.variables.map((variable, idx) => (
                    <div key={idx} className="flex items-center gap-2 animate-in fade-in slide-in-from-left-2">
                      <input
                        value={variable.name}
                        onChange={(e) => {
                          const newVars = [...agentConfig.variables];
                          newVars[idx].name = e.target.value;
                          setAgentConfig(prev => ({ ...prev, variables: newVars }));
                        }}
                        placeholder={t('orchestrator.variableName')}
                        className="flex-1 bg-slate-50 border border-slate-100 rounded-lg px-3 py-1.5 text-xs focus:ring-2 focus:ring-brand-500/20 outline-none font-mono"
                      />
                      <select
                        value={variable.type}
                        onChange={(e) => {
                          const newVars = [...agentConfig.variables];
                          newVars[idx].type = e.target.value;
                          setAgentConfig(prev => ({ ...prev, variables: newVars }));
                        }}
                        className="bg-slate-50 border border-slate-100 rounded-lg px-2 py-1.5 text-xs focus:ring-2 focus:ring-brand-500/20 outline-none"
                      >
                        <option value="string">String</option>
                        <option value="number">Number</option>
                        <option value="boolean">Boolean</option>
                      </select>
                      <label className="flex items-center gap-1 cursor-pointer">
                        <input
                          type="checkbox"
                          checked={variable.required}
                          onChange={(e) => {
                            const newVars = [...agentConfig.variables];
                            newVars[idx].required = e.target.checked;
                            setAgentConfig(prev => ({ ...prev, variables: newVars }));
                          }}
                          className="rounded text-brand-500 focus:ring-brand-500 w-3 h-3"
                        />
                        <span className="text-[10px] font-bold text-slate-400 uppercase">{t('orchestrator.variableRequired')}</span>
                      </label>
                      <button
                        onClick={() => {
                          const newVars = agentConfig.variables.filter((_, i) => i !== idx);
                          setAgentConfig(prev => ({ ...prev, variables: newVars }));
                        }}
                        className="p-1.5 text-slate-400 hover:text-red-500 transition-colors"
                      >
                        <Trash2 size={14} />
                      </button>
                    </div>
                  ))}
                </div>
              </div>

              <textarea
                value={agentConfig.systemPrompt}
                onChange={(e) => setAgentConfig(prev => ({ ...prev, systemPrompt: e.target.value }))}
                placeholder={t('orchestrator.placeholderPrompt')}
                className="w-full h-48 bg-white border border-slate-200 rounded-2xl p-4 text-sm focus:ring-2 focus:ring-brand-500/20 outline-none resize-none font-mono"
              />
            </div>
          </section>

          {/* Memory Settings */}
          <section className="space-y-4">
            <div className="flex items-center gap-2 text-slate-900">
              <Database size={18} className="text-brand-500" />
              <h2 className="font-bold">{t('orchestrator.memorySettings')}</h2>
            </div>
            <div className="bg-white border border-slate-200 rounded-2xl p-6 space-y-6">
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="text-sm font-bold text-slate-900">{t('orchestrator.shortTermMemory')}</h3>
                  <p className="text-xs text-slate-500">{t('orchestrator.shortTermMemoryDesc')}</p>
                </div>
                <div className="flex items-center gap-3">
                  <input
                    type="range"
                    min="1"
                    max="50"
                    value={agentConfig.memoryLimit}
                    onChange={(e) => setAgentConfig(prev => ({ ...prev, memoryLimit: parseInt(e.target.value) }))}
                    className="w-32 accent-brand-500"
                  />
                  <span className="text-sm font-bold text-brand-500 w-8">{agentConfig.memoryLimit}</span>
                </div>
              </div>

              <div className="flex items-center justify-between pt-6 border-t border-slate-100">
                <div>
                  <h3 className="text-sm font-bold text-slate-900">{t('orchestrator.longTermMemory')}</h3>
                  <p className="text-xs text-slate-500">{t('orchestrator.longTermMemoryDesc')}</p>
                </div>
                <button
                  onClick={() => setAgentConfig(prev => ({ ...prev, longTermMemory: !prev.longTermMemory }))}
                  className={cn(
                    "w-12 h-6 rounded-full transition-all relative",
                    agentConfig.longTermMemory ? "bg-brand-500" : "bg-slate-200"
                  )}
                >
                  <div className={cn(
                    "absolute top-1 w-4 h-4 bg-white rounded-full transition-all",
                    agentConfig.longTermMemory ? "left-7" : "left-1"
                  )} />
                </button>
              </div>
            </div>
          </section>

          {/* Runner Endpoint */}
          <section className="space-y-4">
            <div className="flex items-center gap-2 text-slate-900">
              <Globe size={18} className="text-brand-500" />
              <h2 className="font-bold">Runner Endpoint</h2>
            </div>
            <div className="bg-white border border-slate-200 rounded-2xl p-6">
              <input
                type="text"
                value={agentConfig.endpoint}
                onChange={(e) => setAgentConfig(prev => ({ ...prev, endpoint: e.target.value }))}
                placeholder="http://localhost:18080/run"
                className="w-full px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/20"
              />
            </div>
          </section>

          {/* Execution Settings */}
          <section className="space-y-4">
            <div className="flex items-center gap-2 text-slate-900">
              <Timer size={18} className="text-brand-500" />
              <h2 className="font-bold">{t('orchestrator.executionSettings')}</h2>
            </div>
            <div className="bg-white border border-slate-200 rounded-2xl p-6 space-y-6">
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="text-sm font-bold text-slate-900">{t('orchestrator.retryCount')}</h3>
                </div>
                <div className="flex items-center gap-3">
                  <input
                    type="range"
                    min="0"
                    max="10"
                    value={agentConfig.retryCount}
                    onChange={(e) => setAgentConfig(prev => ({ ...prev, retryCount: parseInt(e.target.value) }))}
                    className="w-32 accent-brand-500"
                  />
                  <span className="text-sm font-bold text-brand-500 w-8">{agentConfig.retryCount}</span>
                </div>
              </div>

              <div className="flex items-center justify-between pt-6 border-t border-slate-100">
                <div>
                  <h3 className="text-sm font-bold text-slate-900">{t('orchestrator.retryInterval')}</h3>
                </div>
                <div className="flex items-center gap-3">
                  <input
                    type="range"
                    min="1"
                    max="60"
                    value={agentConfig.retryInterval}
                    onChange={(e) => setAgentConfig(prev => ({ ...prev, retryInterval: parseInt(e.target.value) }))}
                    className="w-32 accent-brand-500"
                  />
                  <span className="text-sm font-bold text-brand-500 w-8">{agentConfig.retryInterval}</span>
                </div>
              </div>

              <div className="flex items-center justify-between pt-6 border-t border-slate-100">
                <div>
                  <h3 className="text-sm font-bold text-slate-900">{t('orchestrator.timeout')}</h3>
                </div>
                <div className="flex items-center gap-3">
                  <input
                    type="range"
                    min="10"
                    max="300"
                    step="10"
                    value={agentConfig.timeout}
                    onChange={(e) => setAgentConfig(prev => ({ ...prev, timeout: parseInt(e.target.value) }))}
                    className="w-32 accent-brand-500"
                  />
                  <span className="text-sm font-bold text-brand-500 w-12">{agentConfig.timeout}</span>
                </div>
              </div>

              <div className="flex items-center justify-between pt-6 border-t border-slate-100">
                <div>
                  <h3 className="text-sm font-bold text-slate-900">{t('orchestrator.maxIterations')}</h3>
                </div>
                <div className="flex items-center gap-3">
                  <input
                    type="range"
                    min="1"
                    max="20"
                    value={agentConfig.maxIterations}
                    onChange={(e) => setAgentConfig(prev => ({ ...prev, maxIterations: parseInt(e.target.value) }))}
                    className="w-32 accent-brand-500"
                  />
                  <span className="text-sm font-bold text-brand-500 w-8">{agentConfig.maxIterations}</span>
                </div>
              </div>

              <div className="flex items-center justify-between pt-6 border-t border-slate-100">
                <div>
                  <h3 className="text-sm font-bold text-slate-900">{t('orchestrator.streamResponse')}</h3>
                </div>
                <button
                  onClick={() => setAgentConfig(prev => ({ ...prev, stream: !prev.stream }))}
                  className={cn(
                    "w-12 h-6 rounded-full transition-all relative",
                    agentConfig.stream ? "bg-brand-500" : "bg-slate-200"
                  )}
                >
                  <div className={cn(
                    "absolute top-1 w-4 h-4 rounded-full bg-white transition-all",
                    agentConfig.stream ? "left-7" : "left-1"
                  )} />
                </button>
              </div>
            </div>
          </section>

          {/* Sandbox Configuration */}
          <section className="space-y-4">
            <div className="flex items-center gap-2 text-slate-900">
              <Code size={18} className="text-brand-500" />
              <h2 className="font-bold">{t('orchestrator.sandbox')}</h2>
            </div>
            <div className="bg-white border border-slate-200 rounded-2xl p-6 space-y-6">
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="text-sm font-bold text-slate-900">{t('orchestrator.sandboxEnabled')}</h3>
                </div>
                <button
                  onClick={() => setAgentConfig(prev => ({ ...prev, sandbox: { ...prev.sandbox, enabled: !prev.sandbox.enabled } }))}
                  className={cn(
                    "w-12 h-6 rounded-full transition-all relative",
                    agentConfig.sandbox.enabled ? "bg-brand-500" : "bg-slate-200"
                  )}
                >
                  <div className={cn(
                    "absolute top-1 w-4 h-4 rounded-full bg-white transition-all",
                    agentConfig.sandbox.enabled ? "left-7" : "left-1"
                  )} />
                </button>
              </div>

              {agentConfig.sandbox.enabled && (
                <motion.div
                  initial={{ opacity: 0, height: 0 }}
                  animate={{ opacity: 1, height: 'auto' }}
                  className="space-y-4 pt-4 border-t border-slate-100"
                >
                  <div className="grid grid-cols-2 gap-4">
                    <div className="space-y-2">
                      <label className="text-[10px] font-bold uppercase tracking-wider text-slate-400">{t('orchestrator.sandboxMode')}</label>
                      <select
                        value={agentConfig.sandbox.mode}
                        onChange={(e) => setAgentConfig(prev => ({ ...prev, sandbox: { ...prev.sandbox, mode: e.target.value as 'docker' | 'local' } }))}
                        className="w-full px-4 py-2 bg-slate-50 border border-slate-200 rounded-lg text-sm focus:outline-none"
                      >
                        <option value="docker">Docker</option>
                        <option value="local">Local</option>
                      </select>
                    </div>
                    <div className="space-y-2">
                      <label className="text-[10px] font-bold uppercase tracking-wider text-slate-400">{t('orchestrator.sandboxTimeout')}</label>
                      <input
                        type="number"
                        value={agentConfig.sandbox.timeoutMs}
                        onChange={(e) => setAgentConfig(prev => ({ ...prev, sandbox: { ...prev.sandbox, timeoutMs: parseInt(e.target.value) } }))}
                        className="w-full px-4 py-2 bg-slate-50 border border-slate-200 rounded-lg text-sm focus:outline-none"
                      />
                    </div>
                  </div>
                  <div className="space-y-2">
                    <label className="text-[10px] font-bold uppercase tracking-wider text-slate-400">{t('orchestrator.sandboxImage')}</label>
                    <input
                      type="text"
                      value={agentConfig.sandbox.image}
                      onChange={(e) => setAgentConfig(prev => ({ ...prev, sandbox: { ...prev.sandbox, image: e.target.value } }))}
                      className="w-full px-4 py-2 bg-slate-50 border border-slate-200 rounded-lg text-sm focus:outline-none"
                    />
                  </div>
                  <div className="space-y-2">
                    <label className="text-[10px] font-bold uppercase tracking-wider text-slate-400">{t('orchestrator.sandboxWorkdir')}</label>
                    <input
                      type="text"
                      value={agentConfig.sandbox.workdir}
                      onChange={(e) => setAgentConfig(prev => ({ ...prev, sandbox: { ...prev.sandbox, workdir: e.target.value } }))}
                      className="w-full px-4 py-2 bg-slate-50 border border-slate-200 rounded-lg text-sm focus:outline-none"
                    />
                  </div>
                </motion.div>
              )}
            </div>
          </section>

          {/* Response Schema */}
          <section className="space-y-4">
            <div className="flex items-center gap-2 text-slate-900">
              <Layout size={18} className="text-brand-500" />
              <h2 className="font-bold">{t('orchestrator.responseSchema')}</h2>
            </div>
            <div className="bg-white border border-slate-200 rounded-2xl p-6 space-y-6">
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-2">
                  <label className="text-[10px] font-bold uppercase tracking-wider text-slate-400">{t('orchestrator.schemaType')}</label>
                  <select
                    value={agentConfig.responseSchema.type}
                    onChange={(e) => setAgentConfig(prev => ({ ...prev, responseSchema: { ...prev.responseSchema, type: e.target.value as any } }))}
                    className="w-full px-4 py-2 bg-slate-50 border border-slate-200 rounded-lg text-sm focus:outline-none"
                  >
                    <option value="text">{t('orchestrator.responseTypes.text')}</option>
                    <option value="markdown">{t('orchestrator.responseTypes.markdown')}</option>
                    <option value="a2ui">{t('orchestrator.responseTypes.a2ui')}</option>
                    <option value="audio">{t('orchestrator.responseTypes.audio')}</option>
                    <option value="image">{t('orchestrator.responseTypes.image')}</option>
                    <option value="video">{t('orchestrator.responseTypes.video')}</option>
                    <option value="mixed">{t('orchestrator.responseTypes.mixed')}</option>
                  </select>
                </div>
                <div className="space-y-2">
                  <label className="text-[10px] font-bold uppercase tracking-wider text-slate-400">{t('orchestrator.schemaVersion')}</label>
                  <input
                    type="text"
                    value={agentConfig.responseSchema.version}
                    onChange={(e) => setAgentConfig(prev => ({ ...prev, responseSchema: { ...prev.responseSchema, version: e.target.value } }))}
                    className="w-full px-4 py-2 bg-slate-50 border border-slate-200 rounded-lg text-sm focus:outline-none"
                  />
                </div>
              </div>
              <div className="space-y-2">
                <label className="text-[10px] font-bold uppercase tracking-wider text-slate-400">{t('orchestrator.schemaDefinition')}</label>
                <textarea
                  value={agentConfig.responseSchema.schema}
                  onChange={(e) => setAgentConfig(prev => ({ ...prev, responseSchema: { ...prev.responseSchema, schema: e.target.value } }))}
                  rows={6}
                  className="w-full px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl text-xs font-mono focus:outline-none focus:ring-2 focus:ring-brand-500/20"
                />
              </div>
            </div>
          </section>

          {/* Human Approval Node */}
          <section className="space-y-4">
            <div className="flex items-center gap-2 text-slate-900">
              <ShieldAlert size={18} className="text-brand-500" />
              <h2 className="font-bold">{t('orchestrator.humanApproval')}</h2>
            </div>
            <div className="bg-white border border-slate-200 rounded-2xl p-6">
              <div className="flex items-center justify-between mb-6">
                <div>
                  <h3 className="text-sm font-bold text-slate-900">{t('orchestrator.requireApproval')}</h3>
                  <p className="text-xs text-slate-500">{t('orchestrator.humanApprovalDesc')}</p>
                </div>
                <button
                  onClick={() => setAgentConfig(prev => ({ ...prev, requireApproval: !prev.requireApproval }))}
                  className={cn(
                    "w-12 h-6 rounded-full transition-all relative",
                    agentConfig.requireApproval ? "bg-brand-500" : "bg-slate-200"
                  )}
                >
                  <div className={cn(
                    "absolute top-1 w-4 h-4 bg-white rounded-full transition-all",
                    agentConfig.requireApproval ? "left-7" : "left-1"
                  )} />
                </button>
              </div>

              {agentConfig.requireApproval && (
                <div className="space-y-6 animate-in fade-in slide-in-from-top-2 pt-6 border-t border-slate-100">
                  <div className="space-y-3">
                    <label className="text-[10px] font-bold text-slate-400 uppercase tracking-widest">
                      {t('orchestrator.approvalThreshold')}
                    </label>
                    <div className="flex p-1 bg-slate-100 rounded-xl w-fit">
                      {(['low', 'medium', 'high'] as const).map((level) => (
                        <button
                          key={level}
                          onClick={() => setAgentConfig(prev => ({ ...prev, approvalThreshold: level }))}
                          className={cn(
                            "px-4 py-1.5 text-xs font-bold rounded-lg transition-all",
                            agentConfig.approvalThreshold === level
                              ? "bg-white text-slate-900 shadow-sm"
                              : "text-slate-500 hover:text-slate-700"
                          )}
                        >
                          {t(`orchestrator.riskLevels.${level}`)}
                        </button>
                      ))}
                    </div>
                    <p className="text-[10px] text-slate-400 italic">
                      {t('orchestrator.thresholdDesc', { level: t(`orchestrator.riskLevels.${agentConfig.approvalThreshold}`) })}
                    </p>
                  </div>

                  <div className="space-y-3">
                    <p className="text-[10px] font-bold text-slate-400 uppercase tracking-widest">
                      {t('orchestrator.affectedTools')}
                    </p>
                    <div className="grid grid-cols-2 gap-2">
                      {backendSkills
                        .filter(s => agentConfig.selectedSkills.includes(s.ulid || s.id))
                        .map(skill => {
                          const isIntercepted =
                            (agentConfig.approvalThreshold === 'low') ||
                            (agentConfig.approvalThreshold === 'medium' && (skill.riskLevel === 'medium' || skill.riskLevel === 'high')) ||
                            (agentConfig.approvalThreshold === 'high' && skill.riskLevel === 'high');

                          return (
                            <div
                              key={skill.ulid || skill.id}
                              className={cn(
                                "flex items-center justify-between p-3 rounded-xl border transition-all",
                                isIntercepted
                                  ? "bg-amber-50 border-amber-200 text-amber-900"
                                  : "bg-slate-50 border-slate-100 text-slate-400 opacity-60"
                              )}
                            >
                              <div className="flex flex-col">
                                <span className="text-xs font-bold">{skill.name}</span>
                                <span className="text-[8px] uppercase tracking-tighter opacity-70">Risk: {skill.riskLevel || 'low'}</span>
                              </div>
                              {isIntercepted ? (
                                <ShieldAlert size={14} className="text-amber-500" />
                              ) : (
                                <ShieldCheck size={14} className="text-slate-300" />
                              )}
                            </div>
                          );
                        })}
                    </div>
                  </div>
                </div>
              )}
            </div>
          </section>

          {/* Channels & Periodic Task */}
          <section className="grid grid-cols-2 gap-6">
            <div className="space-y-4">
              <div className="flex items-center gap-2 text-slate-900">
                <Globe size={18} className="text-brand-500" />
                <h2 className="font-bold">{t('orchestrator.channels')}</h2>
              </div>
              <div className="bg-white border border-slate-200 rounded-2xl p-4 space-y-2">
                {backendChannels.length > 0 ? backendChannels.filter(ch => ch.enabled).map(channel => (
                  <label key={channel.ulid || channel.code} className="flex items-center justify-between p-2 hover:bg-slate-50 rounded-lg cursor-pointer transition-colors">
                    <span className="text-sm font-medium text-slate-700">{channel.name}</span>
                    <input
                      type="checkbox"
                      checked={agentConfig.channels.includes(channel.code)}
                      onChange={() => handleToggleChannel(channel.code)}
                      className="rounded text-brand-500 focus:ring-brand-500"
                    />
                  </label>
                )) : ['api', 'web', 'feishu', 'dingtalk'].map(channel => (
                  <label key={channel} className="flex items-center justify-between p-2 hover:bg-slate-50 rounded-lg cursor-pointer transition-colors">
                    <span className="text-sm font-medium text-slate-700">{t(`orchestrator.${channel}Channel`)}</span>
                    <input
                      type="checkbox"
                      checked={agentConfig.channels.includes(channel)}
                      onChange={() => handleToggleChannel(channel)}
                      className="rounded text-brand-500 focus:ring-brand-500"
                    />
                  </label>
                ))}
              </div>
            </div>

            <div className="space-y-4">
              <div className="flex items-center gap-2 text-slate-900">
                <Play size={18} className="text-brand-500" />
                <h2 className="font-bold">{t('orchestrator.periodicTask')}</h2>
              </div>
              <div className="bg-white border border-slate-200 rounded-2xl p-6">
                <div className="flex items-center justify-between mb-4">
                  <div>
                    <h3 className="text-sm font-bold text-slate-900">{t('orchestrator.periodicTask')}</h3>
                    <p className="text-xs text-slate-500">{t('orchestrator.periodicTaskDesc')}</p>
                  </div>
                  <button
                    onClick={() => setAgentConfig(prev => ({ ...prev, isPeriodic: !prev.isPeriodic }))}
                    className={cn(
                      "w-12 h-6 rounded-full transition-all relative",
                      agentConfig.isPeriodic ? "bg-brand-500" : "bg-slate-200"
                    )}
                  >
                    <div className={cn(
                      "absolute top-1 w-4 h-4 bg-white rounded-full transition-all",
                      agentConfig.isPeriodic ? "left-7" : "left-1"
                    )} />
                  </button>
                </div>

                {agentConfig.isPeriodic && (
                  <div className="space-y-2 animate-in fade-in slide-in-from-top-2">
                    <label className="text-[10px] font-bold text-slate-400 uppercase tracking-widest">{t('orchestrator.cronRule')}</label>
                    <input
                      type="text"
                      value={agentConfig.cronRule}
                      onChange={(e) => setAgentConfig(prev => ({ ...prev, cronRule: e.target.value }))}
                      placeholder={t('orchestrator.placeholderCron')}
                      className="w-full bg-slate-50 border border-slate-200 rounded-xl px-4 py-2 text-sm focus:ring-2 focus:ring-brand-500/20 outline-none font-mono"
                    />
                  </div>
                )}
              </div>
            </div>
          </section>
        </div>

        {/* Right Panel: Test Chat */}
        <div className={cn(
          "border-l border-slate-200 bg-white flex flex-col transition-all duration-300 relative",
          isTestPanelCollapsed ? "w-12" : "w-[400px]"
        )}>
          {/* Collapse/Expand Toggle Button */}
          <button
            onClick={() => setIsTestPanelCollapsed(!isTestPanelCollapsed)}
            className={cn(
              "absolute -left-3 top-20 w-6 h-6 bg-white border border-slate-200 rounded-full flex items-center justify-center text-slate-400 hover:text-slate-600 shadow-sm z-30 transition-transform",
              isTestPanelCollapsed ? "rotate-180" : ""
            )}
          >
            <ChevronRight size={14} />
          </button>

          {isTestPanelCollapsed ? (
            <div className="flex flex-col items-center py-6 gap-8 h-full overflow-hidden">
              <div className="[writing-mode:vertical-rl] rotate-180 text-[10px] font-bold text-slate-400 uppercase tracking-widest whitespace-nowrap">
                {t('orchestrator.testPlayground')}
              </div>
              <div className="flex flex-col gap-4 mt-auto mb-6">
                <button onClick={() => setIsTestPanelCollapsed(false)} className="p-2 text-brand-500 hover:bg-brand-50 rounded-lg transition-colors">
                  <MessageSquare size={18} />
                </button>
              </div>
            </div>
          ) : (
            <>
              <div className="p-4 border-b border-slate-100 flex items-center justify-between shrink-0">
                <div className="flex items-center gap-2">
                  <MessageSquare size={18} className="text-brand-500" />
                  <h3 className="font-bold text-slate-900">{t('orchestrator.testPlayground')}</h3>
                  {deployedAgentId && (
                    <span className="text-[10px] px-2 py-0.5 bg-green-100 text-green-600 rounded-full">
                      已部署
                    </span>
                  )}
                  {!deployedAgentId && (
                    <span className="text-[10px] px-2 py-0.5 bg-amber-100 text-amber-600 rounded-full">
                      请先部署
                    </span>
                  )}
                </div>
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => setTestMessages([])}
                    className="text-[10px] font-bold text-slate-400 hover:text-slate-600 uppercase tracking-wider"
                  >
                    {t('orchestrator.clear')}
                  </button>
                </div>
              </div>

              <div className="flex-1 overflow-y-auto p-4 space-y-4 scrollbar-hide">
                {testMessages.length === 0 ? (
                  <div className="h-full flex flex-col items-center justify-center text-center p-6">
                    <div className="w-12 h-12 rounded-2xl bg-slate-50 flex items-center justify-center text-slate-300 mb-4">
                      <Bot size={24} />
                    </div>
                    {!deployedAgentId ? (
                      <>
                        <p className="text-sm font-medium text-slate-500">{t('orchestrator.deployFirst')}</p>
                        <p className="text-xs text-slate-400 mt-1">{t('orchestrator.deployFirstDesc')}</p>
                      </>
                    ) : (
                      <p className="text-sm font-medium text-slate-500">{t('orchestrator.startTesting') || 'Start chatting'}</p>
                    )}
                  </div>
                ) : (
                  testMessages.map((msg) => (
                    <div
                      key={msg.id}
                      className={cn(
                        "flex gap-3",
                        msg.role === 'user' ? "flex-row-reverse" : ""
                      )}
                    >
                      <div className={cn(
                        "w-8 h-8 rounded-lg flex items-center justify-center shrink-0",
                        msg.role === 'user' ? "bg-slate-900 text-white" : "bg-brand-500 text-white"
                      )}>
                        {msg.role === 'user' ? <User size={14} /> : <Bot size={14} />}
                      </div>
                      <div className={cn(
                        "p-3 rounded-2xl text-xs leading-relaxed shadow-sm max-w-[80%]",
                        msg.role === 'user' ? "bg-slate-900 text-white rounded-tr-none" : "bg-slate-50 text-slate-800 rounded-tl-none"
                      )}>
                        {msg.trace && (
                          <div className="mb-3 space-y-2 border-b border-slate-200 pb-3">
                            {msg.trace.map((t, i) => (
                              <div key={i} className="flex items-start gap-2 text-[10px] text-slate-500 italic">
                                {t.type === 'thought' && <Brain size={10} className="mt-0.5 shrink-0" />}
                                {t.type === 'tool' && <Wrench size={10} className="mt-0.5 shrink-0" />}
                                {t.type === 'retrieval' && <Database size={10} className="mt-0.5 shrink-0" />}
                                <span>{t.content}</span>
                              </div>
                            ))}
                          </div>
                        )}
                        {agentConfig.responseSchema.type === 'markdown' || agentConfig.responseSchema.type === 'mixed' ? (
                          <div className="markdown-body prose prose-slate prose-xs max-w-none">
                            <ReactMarkdown>{msg.content}</ReactMarkdown>
                          </div>
                        ) : (
                          msg.content
                        )}

                        {msg.status === 'pending_approval' && (
                          <div className="mt-4 p-3 bg-white/50 rounded-xl border border-amber-200 space-y-3">
                            <div className="flex items-center gap-2 text-amber-700 font-bold text-[10px] uppercase tracking-wider">
                              <ShieldAlert size={12} />
                              需要人工干预
                            </div>
                            <div className="flex gap-2">
                              <button
                                onClick={() => handleApprove(msg.id)}
                                className="flex-1 py-1.5 bg-brand-500 text-white rounded-lg text-[10px] font-bold hover:bg-brand-600 transition-colors"
                              >
                                批准
                              </button>
                              <button
                                onClick={() => handleReject(msg.id)}
                                className="flex-1 py-1.5 bg-slate-200 text-slate-700 rounded-lg text-[10px] font-bold hover:bg-slate-300 transition-colors"
                              >
                                拒绝
                              </button>
                            </div>
                          </div>
                        )}
                      </div>
                    </div>
                  ))
                )}
                {isTesting && (
                  <div className="flex gap-3">
                    <div className="w-8 h-8 rounded-lg bg-brand-500 text-white flex items-center justify-center shrink-0">
                      <Bot size={14} />
                    </div>
                    <div className="p-3 bg-slate-50 rounded-2xl rounded-tl-none flex gap-1">
                      <div className="w-1 h-1 bg-slate-400 rounded-full animate-bounce" />
                      <div className="w-1 h-1 bg-slate-400 rounded-full animate-bounce [animation-delay:0.2s]" />
                      <div className="w-1 h-1 bg-slate-400 rounded-full animate-bounce [animation-delay:0.4s]" />
                    </div>
                  </div>
                )}
                <div ref={testScrollRef} />
              </div>

              {/* Chat Input */}
              <div className="p-4 border-t border-slate-100 shrink-0">
                <div className="relative">
                  <textarea
                    value={testInput}
                    onChange={(e) => setTestInput(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' && !e.shiftKey) {
                        e.preventDefault();
                        handleSendMessage();
                      }
                    }}
                    placeholder={t('orchestrator.typeTestMessage')}
                    className="w-full bg-slate-50 border border-slate-200 rounded-xl pl-4 pr-12 py-3 text-xs focus:outline-none focus:ring-2 focus:ring-brand-500/20 resize-none"
                    rows={1}
                  />
                  <button
                    onClick={handleSendMessage}
                    disabled={isTesting || !testInput.trim()}
                    className={cn(
                      "absolute right-2 top-1/2 -translate-y-1/2 w-8 h-8 rounded-lg flex items-center justify-center transition-all",
                      testInput.trim() ? "bg-brand-500 text-white shadow-lg shadow-brand-500/20" : "bg-slate-200 text-slate-400"
                    )}
                  >
                    <Send size={14} />
                  </button>
                </div>
              </div>
            </>
          )}
        </div>
      </div>

      {/* Sub-Agent Edit Modal */}
      {isSubAgentModalOpen && editingSubAgent && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white rounded-2xl w-[560px] shadow-2xl max-h-[90vh] overflow-y-auto">
            {/* Modal Header */}
            <div className="p-6 border-b border-slate-100 flex items-center justify-between sticky top-0 bg-white">
              <h2 className="text-lg font-bold text-slate-900">
                {subAgents.find(a => a.id === editingSubAgent.id) ? t('orchestrator.editSubAgent') : t('orchestrator.addSubAgent')}
              </h2>
              <button
                onClick={() => setIsSubAgentModalOpen(false)}
                className="text-slate-400 hover:text-slate-600 transition-all"
              >
                <X size={20} />
              </button>
            </div>

            {/* Modal Content */}
            <div className="p-6 space-y-6">
              {/* ID */}
              <div className="space-y-2">
                <label className="text-xs font-bold text-slate-400 uppercase tracking-wider">{t('orchestrator.subAgentId')}</label>
                <input
                  type="text"
                  value={editingSubAgent.id}
                  onChange={(e) => setEditingSubAgent(prev => ({ ...prev, id: e.target.value }))}
                  className="w-full px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl text-sm focus:ring-2 focus:ring-brand-500/20 outline-none font-mono"
                  placeholder="e.g., researcher"
                />
                <p className="text-[10px] text-slate-400">{t('orchestrator.subAgentIdDesc')}</p>
              </div>

              {/* Name */}
              <div className="space-y-2">
                <label className="text-xs font-bold text-slate-400 uppercase tracking-wider">{t('orchestrator.subAgentName')}</label>
                <input
                  type="text"
                  value={editingSubAgent.name}
                  onChange={(e) => setEditingSubAgent(prev => ({ ...prev, name: e.target.value }))}
                  className="w-full px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl text-sm focus:ring-2 focus:ring-brand-500/20 outline-none"
                  placeholder="e.g., Researcher Agent"
                />
              </div>

              {/* Description */}
              <div className="space-y-2">
                <label className="text-xs font-bold text-slate-400 uppercase tracking-wider">{t('orchestrator.subAgentDesc')}</label>
                <input
                  type="text"
                  value={editingSubAgent.description}
                  onChange={(e) => setEditingSubAgent(prev => ({ ...prev, description: e.target.value }))}
                  className="w-full px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl text-sm focus:ring-2 focus:ring-brand-500/20 outline-none"
                  placeholder="Brief description of what this agent does"
                />
              </div>

              {/* Prompt */}
              <div className="space-y-2">
                <label className="text-xs font-bold text-slate-400 uppercase tracking-wider">{t('orchestrator.subAgentPrompt')}</label>
                <textarea
                  value={editingSubAgent.prompt}
                  onChange={(e) => setEditingSubAgent(prev => ({ ...prev, prompt: e.target.value }))}
                  className="w-full px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl text-sm focus:ring-2 focus:ring-brand-500/20 outline-none resize-none font-mono"
                  rows={5}
                  placeholder="You are a research assistant specialized in..."
                />
                <p className="text-[10px] text-slate-400">{t('orchestrator.subAgentPromptDesc')}</p>
              </div>

              {/* Advanced Settings */}
              <div className="p-4 bg-slate-50 rounded-xl space-y-4">
                <div className="flex items-center gap-2 text-[10px] font-bold text-slate-400 uppercase tracking-widest">
                  <Settings2 size={12} />
                  {t('orchestrator.advancedSettings')}
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <label className="text-xs font-medium text-slate-600">{t('orchestrator.maxIterations')}</label>
                    <input
                      type="number"
                      value={editingSubAgent.max_iterations}
                      onChange={(e) => setEditingSubAgent(prev => ({ ...prev, max_iterations: parseInt(e.target.value) || 5 }))}
                      className="w-full px-4 py-2 bg-white border border-slate-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/20"
                      min={1}
                      max={20}
                    />
                  </div>
                  <div className="space-y-2">
                    <label className="text-xs font-medium text-slate-600">{t('orchestrator.timeoutMs')}</label>
                    <input
                      type="number"
                      value={editingSubAgent.timeout_ms}
                      onChange={(e) => setEditingSubAgent(prev => ({ ...prev, timeout_ms: parseInt(e.target.value) || 120000 }))}
                      className="w-full px-4 py-2 bg-white border border-slate-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/20"
                      min={1000}
                    />
                  </div>
                </div>
              </div>
            </div>

            {/* Modal Footer */}
            <div className="p-6 border-t border-slate-100 flex justify-end gap-3 sticky bottom-0 bg-white">
              <button
                onClick={() => setIsSubAgentModalOpen(false)}
                className="px-6 py-2.5 bg-slate-100 text-slate-700 rounded-xl text-sm font-bold hover:bg-slate-200 transition-all"
              >
                {t('common.cancel')}
              </button>
              <button
                onClick={handleSaveSubAgent}
                className="px-6 py-2.5 bg-brand-500 text-white rounded-xl text-sm font-bold hover:bg-brand-600 transition-all shadow-lg shadow-brand-500/20"
              >
                {t('orchestrator.saveSubAgent')}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Deploy Agent Modal */}
      {isDeployModalOpen && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white rounded-2xl w-[480px] shadow-2xl">
            {/* Modal Header */}
            <div className="p-6 border-b border-slate-100 flex items-center justify-between">
              <h2 className="text-lg font-bold text-slate-900">部署为智能体</h2>
              <button
                onClick={() => setIsDeployModalOpen(false)}
                className="text-slate-400 hover:text-slate-600 transition-all"
              >
                <X size={20} />
              </button>
            </div>

            {/* Modal Content */}
            <div className="p-6 space-y-6">
              {/* Name */}
              <div className="space-y-2">
                <label className="text-xs font-bold text-slate-400 uppercase tracking-wider">智能体名称</label>
                <input
                  type="text"
                  value={deployForm.name}
                  onChange={(e) => setDeployForm(prev => ({ ...prev, name: e.target.value }))}
                  className="w-full px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl text-sm focus:ring-2 focus:ring-brand-500/20 outline-none"
                  placeholder="输入智能体名称"
                />
              </div>

              {/* Description */}
              <div className="space-y-2">
                <label className="text-xs font-bold text-slate-400 uppercase tracking-wider">描述</label>
                <textarea
                  value={deployForm.description}
                  onChange={(e) => setDeployForm(prev => ({ ...prev, description: e.target.value }))}
                  className="w-full px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl text-sm focus:ring-2 focus:ring-brand-500/20 outline-none resize-none"
                  rows={3}
                  placeholder="输入智能体描述"
                />
              </div>

              {/* Icon Selection */}
              <div className="space-y-3">
                <label className="text-xs font-bold text-slate-400 uppercase tracking-wider">选择图标</label>
                <div className="grid grid-cols-5 gap-3">
                  {AGENT_ICONS.map((iconName) => {
                    const IconComponent = {
                      Bot, User, Sparkles, Brain, Zap, Workflow, MessageSquare, Globe, Terminal, Code
                    }[iconName] as React.ElementType;
                    return (
                      <button
                        key={iconName}
                        onClick={() => setDeployForm(prev => ({ ...prev, icon: iconName }))}
                        className={cn(
                          "p-3 rounded-xl border-2 transition-all",
                          deployForm.icon === iconName
                            ? "bg-brand-50 border-brand-500 text-brand-500"
                            : "bg-slate-50 border-slate-200 text-slate-500 hover:border-slate-300"
                        )}
                      >
                        <IconComponent size={24} />
                      </button>
                    );
                  })}
                </div>
              </div>
            </div>

            {/* Modal Footer */}
            <div className="p-6 border-t border-slate-100 flex justify-end gap-3">
              <button
                onClick={() => setIsDeployModalOpen(false)}
                className="px-6 py-2.5 bg-slate-100 text-slate-700 rounded-xl text-sm font-bold hover:bg-slate-200 transition-all"
              >
                取消
              </button>
              <button
                onClick={handleConfirmDeploy}
                className="px-6 py-2.5 bg-brand-500 text-white rounded-xl text-sm font-bold hover:bg-brand-600 transition-all shadow-lg shadow-brand-500/20"
              >
                确认部署
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
