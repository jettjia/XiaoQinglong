import React from 'react';
import {
  Send,
  Paperclip,
  Image as ImageIcon,
  Mic,
  MoreHorizontal,
  Plus,
  History,
  Trash2,
  ChevronDown,
  MessageSquare,
  Search,
  Zap,
  FileText,
  FileSearch,
  BarChart3,
  MapPin,
  Sun,
  Code as CodeIcon,
  Languages,
  Check,
  ArrowUp,
  ChevronRight,
  LayoutGrid,
  PenTool,
  Podcast,
  CircleDot,
  Music,
  CheckSquare,
  PieChart,
  Eye,
  Settings,
  Play,
  Volume2,
  Box,
  Terminal,
  Brain,
  Wrench,
  Video,
  ExternalLink,
  ChevronUp,
  PanelLeftClose,
  PanelLeftOpen,
  ShieldAlert,
  UserCheck,
  XCircle,
  Clock
} from 'lucide-react';
import { useDropzone } from 'react-dropzone';
import { motion, AnimatePresence } from 'motion/react';
import ReactMarkdown from 'react-markdown';
import { cn } from '../lib/utils';
import { Message, FileInfo, Agent, Conversation, PendingApproval, ChatSession } from '../types';
import { INITIAL_AGENTS } from '../constants';
import { agentApi, chatApi } from '../lib/api';
import { useTranslation } from 'react-i18next';

const API_BASE = '/api/xiaoqinglong/agent-frame/v1';
const CURRENT_USER_ID = 'user-1'; // TODO: Get from auth context

interface ChatInterfaceProps {
  preselectedAgent?: Agent | null;
  onAgentUsed?: () => void;
}

export function ChatInterface({ preselectedAgent, onAgentUsed }: ChatInterfaceProps) {
  const { t } = useTranslation();
  const [messages, setMessages] = React.useState<Message[]>([]);
  const [input, setInput] = React.useState('');
  const [files, setFiles] = React.useState<FileInfo[]>([]);
  const filesRef = React.useRef<FileInfo[]>([]); // 用于跟踪当前文件，异步更新
  const pendingFilesRef = React.useRef<File[]>([]); // 保存待上传的原始 File 对象
  const [agents, setAgents] = React.useState<Agent[]>([]);
  const [activeAgent, setActiveAgent] = React.useState<Agent | null>(null);
  const [isLoading, setIsLoading] = React.useState(false);
  const [conversations, setConversations] = React.useState<Conversation[]>([]);
  const [activeConversationId, setActiveConversationId] = React.useState<string | null>(null);
  const [currentSession, setCurrentSession] = React.useState<ChatSession | null>(null);
  const [isMoreAgentsOpen, setIsMoreAgentsOpen] = React.useState(false);
  const [searchQuery, setSearchQuery] = React.useState('');
  const [isSidebarOpen, setIsSidebarOpen] = React.useState(true);
  const [isTraceOpen, setIsTraceOpen] = React.useState(false);
  const [selectedMessageId, setSelectedMessageId] = React.useState<string | null>(null);
  const [showThinking, setShowThinking] = React.useState<Record<string, boolean>>({});
  const [collapsedTools, setCollapsedTools] = React.useState<Record<string, boolean>>({});
  const [pendingApprovals, setPendingApprovals] = React.useState<PendingApproval[]>([]);
  const scrollRef = React.useRef<HTMLDivElement>(null);
  const abortControllerRef = React.useRef<AbortController | null>(null);
  const pollingRef = React.useRef<NodeJS.Timeout | null>(null);

  // Load agents from backend
  React.useEffect(() => {
    const loadAgents = async () => {
      try {
        const backendAgents = await agentApi.findAll();
        // 系统 Agent 排在前面，用户 Agent 排在后面
        const sortedAgents = backendAgents.sort((a, b) => {
          if (a.is_system === b.is_system) return 0;
          return a.is_system ? -1 : 1;
        });
        setAgents(sortedAgents);
      } catch (err) {
        console.error('Failed to load agents:', err);
      }
    };
    loadAgents();
  }, []);

  // Load user sessions
  React.useEffect(() => {
    const loadSessions = async () => {
      try {
        const sessions = await chatApi.getSessionsByUserId(CURRENT_USER_ID);
        const convs: Conversation[] = sessions.map(s => ({
          id: s.ulid,
          title: s.title || '新会话',
          lastMessage: '',
          timestamp: new Date(s.updated_at || s.created_at),
          agentId: s.agent_id
        }));
        setConversations(convs);
      } catch (err) {
        console.error('Failed to load sessions:', err);
      }
    };
    loadSessions();
  }, []);

  // Handle preselected agent from AgentManager
  React.useEffect(() => {
    if (preselectedAgent) {
      setActiveAgent(preselectedAgent);
      if (onAgentUsed) {
        onAgentUsed();
      }
    }
  }, [preselectedAgent, onAgentUsed]);

  // Poll for pending approvals
  React.useEffect(() => {
    const pollApprovals = async () => {
      try {
        const approvals = await chatApi.getPendingApprovals();
        const mapped: PendingApproval[] = approvals.map(a => ({
          id: a.ulid,
          sessionId: a.session_id,
          messageId: a.message_id,
          toolName: a.tool_name,
          toolType: a.tool_type,
          riskLevel: a.risk_level,
          parameters: a.parameters ? JSON.parse(a.parameters) : {},
          status: a.status as 'pending' | 'approved' | 'rejected',
          timestamp: new Date(a.created_at)
        }));
        setPendingApprovals(mapped);
      } catch (err) {
        console.error('Failed to poll approvals:', err);
      }
    };

    pollApprovals();
    pollingRef.current = setInterval(pollApprovals, 5000);

    return () => {
      if (pollingRef.current) {
        clearInterval(pollingRef.current);
      }
    };
  }, []);

  // Load session messages
  const loadSessionMessages = async (sessionId: string) => {
    try {
      const msgs = await chatApi.getMessagesBySessionId(sessionId);
      const mapped: Message[] = msgs.map(m => ({
        id: m.ulid,
        role: m.role as 'user' | 'assistant',
        content: m.content,
        timestamp: new Date(m.created_at),
        status: m.status as 'pending_approval' | 'completed' | 'failed' | undefined,
        thinking: m.trace ? JSON.parse(m.trace)?.thinking : undefined,
        trace: m.trace ? JSON.parse(m.trace)?.trace : undefined
      }));
      setMessages(mapped);
    } catch (err) {
      console.error('Failed to load messages:', err);
    }
  };

  // Handle approval
  const handleApproval = async (approvalId: string, action: 'approved' | 'rejected', reason?: string) => {
    try {
      if (action === 'approved') {
        await chatApi.approveApproval(approvalId, CURRENT_USER_ID, reason);
      } else {
        await chatApi.rejectApproval(approvalId, CURRENT_USER_ID, reason);
      }
      // Remove from pending list
      setPendingApprovals(prev => prev.filter(a => a.id !== approvalId));
    } catch (err) {
      console.error('Failed to handle approval:', err);
    }
  };

  const getAgentIcon = (iconName: string, size: number = 16) => {
    switch (iconName) {
      case 'Zap': return <Zap size={size} />;
      case 'ImageIcon': return <ImageIcon size={size} />;
      case 'PenTool': return <PenTool size={size} />;
      case 'Languages': return <Languages size={size} />;
      case 'Code': return <CodeIcon size={size} />;
      case 'Search': return <Search size={size} />;
      case 'Podcast': return <Podcast size={size} />;
      case 'CircleDot': return <CircleDot size={size} />;
      case 'Music': return <Music size={size} />;
      case 'CheckSquare': return <CheckSquare size={size} />;
      case 'PieChart': return <PieChart size={size} />;
      default: return <MessageSquare size={size} />;
    }
  };

  const visibleAgents = agents.length > 0 ? agents.slice(0, 6) : INITIAL_AGENTS.slice(0, 6);
  const moreAgents = agents.length > 0 ? agents.slice(6) : INITIAL_AGENTS.slice(6);

  const onDrop = React.useCallback((acceptedFiles: File[]) => {
    console.log('[onDrop] called, acceptedFiles:', acceptedFiles.length);
    // 只保存文件信息到 state，不上传
    const newFiles = acceptedFiles.map(file => ({
      name: file.name,
      size: file.size,
      type: file.type,
      url: URL.createObjectURL(file)
    }));
    filesRef.current = [...filesRef.current, ...newFiles];
    pendingFilesRef.current = [...pendingFilesRef.current, ...acceptedFiles];
    console.log('[onDrop] after - filesRef.current.length:', filesRef.current.length, 'pendingFilesRef.current.length:', pendingFilesRef.current.length);
    setFiles(filesRef.current);
  }, []);

  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    onDrop,
    noClick: true
  } as any);

  const handleSend = async () => {
    const currentFiles = filesRef.current;
    console.log('[handleSend] called, currentFiles:', currentFiles, 'input:', input);
    if ((!input.trim() && currentFiles.length === 0) || isLoading || !activeAgent) return;

    const userMessage: Message = {
      id: Date.now().toString(),
      role: 'user',
      content: input,
      timestamp: new Date(),
      files: [...currentFiles]
    };

    setMessages(prev => [...prev, userMessage]);
    setInput('');
    setFiles([]);
    filesRef.current = [];
    setIsLoading(true);
    abortControllerRef.current = new AbortController();

    try {
      // Get session ID - create session if needed
      let sessionId = currentSession?.ulid || activeConversationId;
      console.log('[handleSend] sessionId:', sessionId, 'currentFiles:', currentFiles.length);
      // Use first 50 chars of input as session title
      const sessionTitle = input.length > 50 ? input.substring(0, 50) + '...' : input;
      if (!sessionId) {
        const result = await chatApi.createSession({
          user_id: CURRENT_USER_ID,
          agent_id: activeAgent.ulid || activeAgent.id,
          title: sessionTitle,
          channel: 'web',
          model: activeAgent.model,
          status: 'active'
        });
        sessionId = result.ulid;
        setCurrentSession({
          ulid: result.ulid,
          user_id: CURRENT_USER_ID,
          agent_id: activeAgent.ulid || activeAgent.id,
          title: sessionTitle,
          channel: 'web',
          model: activeAgent.model || '',
          status: 'active',
          created_at: Date.now(),
          updated_at: Date.now(),
          created_by: CURRENT_USER_ID,
          updated_by: CURRENT_USER_ID
        });
        setActiveConversationId(result.ulid);
        // Add to conversations list for sidebar display
        const newConv: Conversation = {
          id: result.ulid,
          title: sessionTitle,
          timestamp: new Date(),
          agentId: activeAgent.ulid || activeAgent.id
        };
        setConversations(prev => [newConv, ...prev]);
      }

      // 如果有待上传的文件，先上传
      let filesToSend = currentFiles;
      console.log('[handleSend] pendingFilesRef.current.length:', pendingFilesRef.current.length);
      if (pendingFilesRef.current.length > 0) {
        console.log('[handleSend] Uploading pending files first, count:', pendingFilesRef.current.length);
        try {
          const result = await chatApi.uploadFiles(sessionId, pendingFilesRef.current);
          console.log('[handleSend] Pending files uploaded:', result);
          // 更新 files 中的 virtual_path
          const updatedFiles = currentFiles.map((f, idx) => {
            if (!f.virtual_path && result.files[idx]) {
              return { ...f, virtual_path: result.files[idx]?.virtual_path || '' };
            }
            return f;
          });
          console.log('[handleSend] updatedFiles:', updatedFiles);
          filesToSend = updatedFiles;
          filesRef.current = updatedFiles;
          setFiles(updatedFiles);
          pendingFilesRef.current = [];
        } catch (err) {
          console.error('Failed to upload pending files:', err);
          // 上传失败，只发送有 virtual_path 的文件
          filesToSend = currentFiles.filter(f => f.virtual_path);
        }
      } else {
        console.log('[handleSend] No pending files to upload, using currentFiles:', currentFiles);
      }

      // Save user message to database
      let userMessageUlid: string | null = null;
      try {
        const userMsgResult = await chatApi.createMessage({
          session_id: sessionId,
          role: 'user',
          content: input,
          status: 'completed'
        });
        userMessageUlid = userMsgResult.ulid;
      } catch (err) {
        console.error('Failed to save user message:', err);
      }

      // 调用 runner API
      console.log('[handleSend] Calling runner with filesToSend:', filesToSend);
      const runResponse = await chatApi.runAgentStream({
        agent_id: activeAgent.ulid || activeAgent.id,
        user_id: CURRENT_USER_ID,
        session_id: sessionId || undefined,
        input: input,
        files: filesToSend.length > 0 ? filesToSend : undefined,
        is_test: false
      });

      // 检查是否流式响应
      const contentType = runResponse.headers.get('content-type') || '';
      const isStreaming = contentType.includes('text/event-stream');

      if (isStreaming) {
        // 流式响应处理
        const reader = runResponse.body?.getReader();
        if (!reader) {
          throw new Error('No reader available');
        }

        const assistantMsgId = (Date.now() + 1).toString();
        let accumulatedContent = '';
        let streamTokenUsage: { prompt_tokens?: number; completion_tokens?: number; total_tokens?: number } = {};
        let toolCalls: Message['toolCalls'] = [];
        let pendingToolCall: { name: string; args: any } | null = null;
        let recallInfo: Message['recallInfo'] = { status: 'running' };

        // 先创建一条空消息用于流式更新
        const assistantMessage: Message = {
          id: assistantMsgId,
          role: 'assistant',
          content: '',
          timestamp: new Date(),
          status: 'streaming',
          toolCalls: [],
          recallInfo: { status: 'running' }
        };
        setMessages(prev => [...prev, assistantMessage]);

        const decoder = new TextDecoder();
        let buffer = '';
        let currentEventType = '';

        // 更新消息内容和方法调用的辅助函数
        const updateMessage = (updates: Partial<Message>) => {
          setMessages(prev => prev.map(m =>
            m.id === assistantMsgId
              ? { ...m, ...updates }
              : m
          ));
        };

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
                    updateMessage({ content: accumulatedContent });
                  }

                  // 处理 recall_complete 事件 - 显示知识召回完成
                  if (currentEventType === 'recall_complete') {
                    recallInfo = { status: 'completed', count: data.count, message: data.message };
                    updateMessage({ recallInfo });
                  }

                  // 处理 tool_call 事件 - 显示正在调用的工具
                  if (currentEventType === 'tool_call') {
                    const toolName = data.tool || data.name || 'unknown';
                    pendingToolCall = { name: toolName, args: data.arguments || {} };
                    toolCalls = [...toolCalls, { name: toolName, args: data.arguments, result: '执行中...', status: 'running' }];
                    updateMessage({ toolCalls: [...toolCalls] });
                  }

                  // 处理 tool 事件 - 显示工具执行结果（可能没有前置 tool_call）
                  if (currentEventType === 'tool') {
                    const toolName = data.tool || pendingToolCall?.name || 'unknown';
                    if (pendingToolCall && toolCalls.length > 0) {
                      // 更新最后一个 toolCall 的结果
                      toolCalls = toolCalls.map((tc, idx) =>
                        idx === toolCalls.length - 1
                          ? { ...tc, result: data.output || '(无输出)', status: 'completed' }
                          : tc
                      );
                    } else {
                      // 没有 pendingToolCall，说明是独立的 tool 事件，直接添加
                      toolCalls = [...toolCalls, { name: toolName, args: {}, result: data.output || '(无输出)', status: 'completed' }];
                    }
                    pendingToolCall = null;
                    updateMessage({ toolCalls: [...toolCalls] });
                  }

                  if (currentEventType === 'done') {
                    streamTokenUsage = {
                      prompt_tokens: data.prompt_tokens,
                      completion_tokens: data.completion_tokens,
                      total_tokens: data.total_tokens
                    };
                    updateMessage({ content: data.content || accumulatedContent, status: 'completed', toolCalls: [...toolCalls] });
                  }

                  if (currentEventType === 'error') {
                    console.error('Stream error:', data.error);
                    // 将错误信息添加到消息内容中显示给用户
                    const errorMsg = `\n\n⚠️ 执行错误: ${data.error}\n`;
                    accumulatedContent += errorMsg;
                    updateMessage({ content: accumulatedContent });
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
                    updateMessage({ content: accumulatedContent });
                  }

                  // 处理 recall_complete 事件
                  if (currentEventType === 'recall_complete') {
                    recallInfo = { status: 'completed', count: data.count, message: data.message };
                    updateMessage({ recallInfo });
                  }

                  // 处理 tool_call 事件
                  if (currentEventType === 'tool_call') {
                    pendingToolCall = { name: data.tool || data.name, args: data.arguments || {} };
                    toolCalls = [...toolCalls, { name: data.tool || data.name, args: data.arguments, result: '执行中...', status: 'running' }];
                    updateMessage({ toolCalls: [...toolCalls] });
                  }

                  // 处理 tool 事件（可能没有前置 tool_call）
                  if (currentEventType === 'tool') {
                    const toolName = data.tool || pendingToolCall?.name || 'unknown';
                    if (pendingToolCall && toolCalls.length > 0) {
                      toolCalls = toolCalls.map((tc, idx) =>
                        idx === toolCalls.length - 1
                          ? { ...tc, result: data.output || '(无输出)', status: 'completed' }
                          : tc
                      );
                    } else {
                      toolCalls = [...toolCalls, { name: toolName, args: {}, result: data.output || '(无输出)', status: 'completed' }];
                    }
                    pendingToolCall = null;
                    updateMessage({ toolCalls: [...toolCalls] });
                  }

                  if (currentEventType === 'done') {
                    streamTokenUsage = {
                      prompt_tokens: data.prompt_tokens,
                      completion_tokens: data.completion_tokens,
                      total_tokens: data.total_tokens
                    };
                    updateMessage({ content: data.content || accumulatedContent, status: 'completed', toolCalls: [...toolCalls] });
                  }

                  if (currentEventType === 'error') {
                    console.error('Stream error:', data.error);
                    const errorMsg = `\n\n⚠️ 执行错误: ${data.error}\n`;
                    accumulatedContent += errorMsg;
                    updateMessage({ content: accumulatedContent });
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

        // 保存消息到数据库
        try {
          await chatApi.createMessage({
            session_id: sessionId,
            role: 'assistant',
            content: accumulatedContent,
            model: activeAgent.model || '',
            tokens: streamTokenUsage.total_tokens || 0,
            metadata: streamTokenUsage.prompt_tokens || streamTokenUsage.completion_tokens
              ? JSON.stringify({
                prompt_tokens: streamTokenUsage.prompt_tokens,
                completion_tokens: streamTokenUsage.completion_tokens
              })
              : undefined,
            status: 'completed'
          });
        } catch (err) {
          console.error('Failed to save assistant message:', err);
        }

        setIsLoading(false);
        return
      }

      // 非流式响应处理
      const data = await runResponse.json();

      // Check if response indicates pending approval
      if (data.pending_approvals && data.pending_approvals.length > 0) {
        // Handle pending approvals from the response
        for (const approval of data.pending_approvals) {
          const pendingApproval: PendingApproval = {
            id: approval.interrupt_id,
            sessionId: data.checkpoint_id || sessionId,
            messageId: userMessageUlid || '',
            toolName: approval.tool_name,
            toolType: approval.tool_type || '',
            riskLevel: approval.risk_level || 'high',
            parameters: approval.arguments ? JSON.parse(approval.arguments) : {},
            status: 'pending',
            timestamp: new Date()
          };
          setPendingApprovals(prev => [...prev, pendingApproval]);
        }
      }

      const assistantMessage: Message = {
        id: (Date.now() + 1).toString(),
        role: 'assistant',
        content: data.content || data.output || "I'm sorry, I couldn't generate a response.",
        timestamp: new Date(),
        thinking: data.thinking,
        trace: data.trace,
        status: data.status
      };
      setMessages(prev => [...prev, assistantMessage]);

      // Save assistant message to database
      try {
        await chatApi.createMessage({
          session_id: sessionId,
          role: 'assistant',
          content: data.content || data.output || '',
          model: data.metadata?.model || activeAgent.model || '',
          tokens: data.metadata?.tokens_used || 0,
          latency_ms: data.metadata?.latency_ms || 0,
          status: data.pending_approvals?.length > 0 ? 'pending_approval' : 'completed',
          trace: data.trace ? JSON.stringify({ thinking: data.thinking, trace: data.trace }) : undefined
        });
      } catch (err) {
        console.error('Failed to save assistant message:', err);
      }
    } catch (error: any) {
      if (error.name === 'AbortError') {
        console.log("Generation stopped by user");
        return;
      }
      console.error("Runner Error:", error);
      const errorMessage: Message = {
        id: (Date.now() + 1).toString(),
        role: 'assistant',
        content: "Sorry, I encountered an error while processing your request. Please try again later.",
        timestamp: new Date()
      };
      setMessages(prev => [...prev, errorMessage]);
    } finally {
      setIsLoading(false);
      abortControllerRef.current = null;
    }
  };

  const stopGeneration = () => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
      setIsLoading(false);
    }
  };

  const runDemo = () => {
    const demoMessages: Message[] = [
      {
        id: 'demo-1',
        role: 'assistant',
        content: "# 研究报告：2024年人工智能趋势\n\n## 1. 核心发现\n人工智能正在从单一任务向**多模态协作**演进。研究表明，集成视觉、听觉 and 文本处理能力的模型在复杂任务中的表现提升了40%。\n\n## 2. 关键技术\n- **RAG (检索增强生成)**: 减少幻觉，提高事实准确性。\n- **Agentic Workflows**: 自主决策与工具调用。\n- **On-device AI**: 隐私保护与低延迟。",
        timestamp: new Date(),
        thinking: "用户询问了AI趋势。我需要生成一份结构化的研究报告，涵盖核心发现和关键技术。使用Markdown格式以提高可读性。",
        trace: [
          { id: 't1', type: 'thought', label: '分析意图', content: '用户对AI未来趋势感兴趣，需要深度分析。', status: 'success', timestamp: new Date() },
          { id: 't2', type: 'skill', label: '知识检索', content: '正在从知识库检索 2024 AI Trends...', status: 'success', duration: '1.2s', timestamp: new Date() },
          { id: 't3', type: 'observation', label: '检索结果', content: '找到 3 篇相关文档，重点在于多模态和 Agent。', status: 'success', timestamp: new Date() },
          { id: 't4', type: 'thought', label: '生成大纲', content: '按核心发现、关键技术、未来展望组织内容。', status: 'success', timestamp: new Date() }
        ]
      },
      {
        id: 'demo-2',
        role: 'assistant',
        content: "我已为您查询了最新的天气和交通状况。",
        timestamp: new Date(),
        toolCalls: [
          { name: 'get_weather', args: { location: '北京' }, result: { temp: '15°C', condition: '晴' } },
          { name: 'get_traffic', args: { route: '东三环' }, result: { status: '拥堵', delay: '15min' } }
        ],
        trace: [
          { id: 't5', type: 'thought', label: '识别工具', content: '需要调用天气和交通 API。', status: 'success', timestamp: new Date() },
          { id: 't6', type: 'tool', label: 'get_weather', content: '调用天气接口: { location: "北京" }', status: 'success', duration: '0.8s', timestamp: new Date() },
          { id: 't7', type: 'tool', label: 'get_traffic', content: '调用交通接口: { route: "东三环" }', status: 'success', duration: '1.1s', timestamp: new Date() }
        ]
      },
      {
        id: 'demo-3',
        role: 'assistant',
        content: "这是为您生成的创意素材：",
        timestamp: new Date(),
        imageUrl: "https://picsum.photos/seed/ai-art/800/450",
        audioUrl: "https://www.soundhelix.com/examples/mp3/SoundHelix-Song-1.mp3",
        videoUrl: "https://www.w3schools.com/html/mov_bbb.mp4"
      },
      {
        id: 'demo-4',
        role: 'assistant',
        content: "为您准备了交互式数据看板：",
        timestamp: new Date(),
        a2ui: {
          type: 'dashboard',
          data: {
            title: '月度增长概览',
            metrics: [
              { label: '活跃用户', value: '1.2M', trend: '+12%' },
              { label: '转化率', value: '3.4%', trend: '+0.5%' },
              { label: '留存率', value: '68%', trend: '-2%' }
            ]
          }
        }
      }
    ];

    setMessages(prev => [...prev, ...demoMessages]);
  };

  const startNewConversation = () => {
    setMessages([]);
    setActiveConversationId(null);
    setCurrentSession(null);
  };

  const deleteConversation = async (id: string) => {
    try {
      await chatApi.deleteSession(id);
    } catch (err) {
      console.error('Failed to delete session:', err);
    }
    setConversations(prev => prev.filter(c => c.id !== id));
    if (activeConversationId === id) {
      startNewConversation();
    }
  };

  // Handle conversation selection
  const handleSelectConversation = async (conv: Conversation) => {
    setActiveConversationId(conv.id);
    await loadSessionMessages(conv.id);
  };

  React.useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages]);

  return (
    <div className="flex h-full bg-white overflow-hidden" {...getRootProps()}>
      <input {...getInputProps()} />

      {/* History Sidebar */}
      <motion.div
        initial={false}
        animate={{ width: isSidebarOpen ? 280 : 0, opacity: isSidebarOpen ? 1 : 0 }}
        className={cn(
          "border-r border-slate-100 flex flex-col bg-slate-50/50 overflow-hidden shrink-0",
          !isSidebarOpen && "border-none"
        )}
      >
        <div className="p-4 border-b border-slate-100 flex items-center justify-between min-w-[280px]">
          <div className="flex items-center gap-2">
            <button
              onClick={() => setIsSidebarOpen(false)}
              className="p-1.5 hover:bg-slate-200 rounded-md transition-colors text-slate-500"
              title="收起侧边栏"
            >
              <PanelLeftClose size={18} />
            </button>
            <h2 className="font-bold text-slate-800 flex items-center gap-2">
              <History size={18} className="text-slate-400" />
              {t('chat.history')}
            </h2>
          </div>
          <button
            onClick={startNewConversation}
            className="p-1.5 hover:bg-slate-200 rounded-md transition-colors"
          >
            <Plus size={18} className="text-slate-600" />
          </button>
        </div>
        <div className="p-3">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 w-3.5 h-3.5" />
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder={t('agents.search')}
              className="w-full pl-9 pr-4 py-1.5 bg-white border border-slate-200 rounded-lg text-xs focus:ring-2 focus:ring-brand-500/20"
            />
          </div>
        </div>
        <div className="flex-1 overflow-y-auto p-2 space-y-1 min-w-[280px]">
          {(() => {
            const filteredConvs = conversations.filter(c =>
              c.title.toLowerCase().includes(searchQuery.toLowerCase())
            );
            if (filteredConvs.length === 0) {
              return (
                <div className="p-4 text-center">
                  <p className="text-xs text-slate-400">{searchQuery ? t('chat.noConversations') : t('chat.noConversations')}</p>
                </div>
              );
            }
            return filteredConvs.map(conv => (
              <div key={conv.id} className="group relative">
                <button
                  onClick={() => handleSelectConversation(conv)}
                  className={cn(
                    "w-full text-left p-3 rounded-lg transition-all flex flex-col gap-1",
                    activeConversationId === conv.id ? "bg-white shadow-sm border border-slate-200" : "hover:bg-white/50"
                  )}
                >
                  <p className="text-sm font-medium text-slate-700 truncate pr-6">{conv.title}</p>
                  <p className="text-[10px] text-slate-400 truncate">{conv.lastMessage}</p>
                </button>
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    deleteConversation(conv.id);
                  }}
                  className="absolute right-2 top-1/2 -translate-y-1/2 p-1.5 opacity-0 group-hover:opacity-100 hover:bg-red-50 text-red-400 rounded-md transition-all"
                >
                  <Trash2 size={14} />
                </button>
              </div>
            ));
          })()}
        </div>
      </motion.div>

      {/* Main Chat Area */}
      <div className="flex-1 flex flex-col relative min-w-0">
        {/* Header */}
        <header className="h-14 border-b border-slate-100 flex items-center justify-between px-4 lg:px-6 bg-white/80 backdrop-blur-sm z-10">
          <div className="flex items-center gap-3">
            {!isSidebarOpen && (
              <button
                onClick={() => setIsSidebarOpen(true)}
                className="p-2 hover:bg-slate-100 rounded-lg text-slate-500 transition-colors"
                title="展开侧边栏"
              >
                <PanelLeftOpen size={18} />
              </button>
            )}
            <div className="w-8 h-8 rounded-full bg-brand-500/10 flex items-center justify-center text-brand-500">
              <div className="w-6 h-6 rounded-full bg-brand-500 flex items-center justify-center text-white font-bold text-[10px]">
                {activeAgent?.name?.[0] || 'A'}
              </div>
            </div>
            <div>
              <h3 className="font-bold text-slate-900 text-xs lg:text-sm">{activeAgent?.name || '选择智能体'}</h3>
              <div className="flex items-center gap-1.5">
                <div className="w-1 h-1 rounded-full bg-green-500" />
                <span className="text-[8px] lg:text-[10px] font-medium text-slate-400 uppercase tracking-wider">{t('chat.active')}</span>
              </div>
            </div>
          </div>
          <div className="flex items-center gap-2">
            {pendingApprovals.length > 0 && (
              <div className="flex items-center gap-1 px-2 py-1 bg-amber-50 text-amber-600 rounded-full text-xs font-bold">
                <ShieldAlert size={14} />
                <span>{pendingApprovals.length}</span>
              </div>
            )}
            <button className="p-2 hover:bg-slate-100 rounded-lg text-slate-400 transition-colors">
              <MoreHorizontal size={20} />
            </button>
          </div>
        </header>

        {/* Messages */}
        <div
          ref={scrollRef}
          className="flex-1 overflow-y-auto p-3 lg:p-4 space-y-3 lg:space-y-4 scrollbar-hide bg-slate-50/30"
        >
          {messages.length === 0 && (
            <div className="h-full flex flex-col items-center justify-center text-center max-w-md mx-auto">
              <div className="w-16 h-16 rounded-2xl bg-brand-500/10 flex items-center justify-center text-brand-500 mb-4">
                <MessageSquare size={32} />
              </div>
              <h2 className="text-xl font-bold text-slate-900 mb-2">{t('chat.startNew')}</h2>
              <p className="text-sm text-slate-500">
                Ask {activeAgent?.name || 'an agent'} anything. You can upload documents, images, or just start typing.
              </p>
            </div>
          )}

          {messages.map((msg) => (
            <motion.div
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
              key={msg.id}
              className={cn(
                "flex gap-4 max-w-4xl",
                msg.role === 'user' ? "ml-auto flex-row-reverse" : ""
              )}
            >
              <div className={cn(
                "w-8 h-8 rounded-full shrink-0 flex items-center justify-center text-xs font-bold",
                msg.role === 'user' ? "bg-slate-900 text-white" : "bg-brand-500 text-white"
              )}>
                {msg.role === 'user' ? 'U' : 'A'}
              </div>
              <div className={cn(
                "flex flex-col gap-3",
                msg.role === 'user' ? "items-end" : "items-start w-full"
              )}>
                {/* Thinking Process */}
                {msg.thinking && (
                  <div className="w-full max-w-2xl">
                    <button
                      onClick={() => setShowThinking(prev => ({ ...prev, [msg.id]: !prev[msg.id] }))}
                      className="flex items-center gap-2 text-[10px] font-bold text-slate-400 uppercase tracking-widest hover:text-brand-500 transition-colors mb-2"
                    >
                      <Brain size={12} />
                      {showThinking[msg.id] ? "隐藏思考过程" : "查看思考过程"}
                      {showThinking[msg.id] ? <ChevronUp size={12} /> : <ChevronDown size={12} />}
                    </button>
                    <AnimatePresence>
                      {showThinking[msg.id] && (
                        <motion.div
                          initial={{ opacity: 0, height: 0 }}
                          animate={{ opacity: 1, height: 'auto' }}
                          exit={{ opacity: 0, height: 0 }}
                          className="overflow-hidden"
                        >
                          <div className="p-4 bg-slate-50 border border-slate-100 rounded-xl text-xs text-slate-500 italic leading-relaxed mb-3">
                            {msg.thinking}
                          </div>
                        </motion.div>
                      )}
                    </AnimatePresence>
                  </div>
                )}

                {/* Recall Status */}
                {msg.recallInfo && (
                  <div className="w-full max-w-2xl mb-2">
                    <div className="flex items-center gap-2 text-[10px] text-slate-400">
                      {msg.recallInfo.status === 'running' ? (
                        <>
                          <div className="w-1.5 h-1.5 bg-amber-500 rounded-full animate-pulse" />
                          <span>正在召回知识...</span>
                        </>
                      ) : (
                        <>
                          <Check size={10} className="text-green-500" />
                          <span>已召回 {msg.recallInfo.count || 0} 条相关知识</span>
                        </>
                      )}
                    </div>
                  </div>
                )}

                {/* Tool Calls */}
                {msg.toolCalls && msg.toolCalls.length > 0 && (
                  <div className="w-full max-w-2xl space-y-2 mb-2">
                    {msg.toolCalls.map((tool, idx) => {
                      const toolKey = `${msg.id}-${idx}`;
                      const isCollapsed = collapsedTools[toolKey];
                      const isRunning = tool.status === 'running' || tool.result === '执行中...';
                      const isError = tool.result?.startsWith('错误:') || tool.status === 'error';
                      return (
                        <div key={idx} className="bg-slate-50 border border-slate-100 rounded-xl overflow-hidden">
                          <div
                            className="px-4 py-2 border-b border-slate-100 flex items-center justify-between bg-slate-100/50 cursor-pointer hover:bg-slate-100/70 transition-colors"
                            onClick={() => {
                              setCollapsedTools(prev => ({
                                ...prev,
                                [toolKey]: !prev[toolKey]
                              }));
                            }}
                          >
                            <div className="flex items-center gap-2">
                              <ChevronRight size={12} className={cn("text-slate-400 transition-transform", !isCollapsed && "rotate-90")} />
                              <Wrench size={12} className="text-brand-500" />
                              <span className="text-[10px] font-bold text-slate-600">
                                {tool.name}
                              </span>
                            </div>
                            {isRunning ? (
                              <div className="flex items-center gap-1 text-amber-500">
                                <div className="w-1.5 h-1.5 bg-amber-500 rounded-full animate-bounce" />
                                <span className="text-[10px]">执行中</span>
                              </div>
                            ) : isError ? (
                              <XCircle size={12} className="text-red-500" />
                            ) : (
                              <Check size={12} className="text-green-500" />
                            )}
                          </div>
                          {!isCollapsed && (
                            <div className="p-3 space-y-2">
                              <div className="space-y-1">
                                <div className="flex items-center gap-1 text-[9px] text-slate-400 uppercase font-medium">
                                  <Terminal size={10} /> 输入
                                </div>
                                <code className="block text-[10px] text-slate-600 bg-white px-2 py-1.5 rounded border border-slate-100 font-mono">
                                  {JSON.stringify(tool.args, null, 2)}
                                </code>
                              </div>
                              {tool.result && (
                                <div className="space-y-1 pt-2 border-t border-slate-100">
                                  <div className="flex items-center gap-1 text-[9px] text-slate-400 uppercase font-medium">
                                    {isRunning ? (
                                      <Clock size={10} className="text-amber-500" />
                                    ) : isError ? (
                                      <XCircle size={10} className="text-red-500" />
                                    ) : (
                                      <CheckSquare size={10} className="text-green-500" />
                                    )}
                                    输出
                                  </div>
                                  <code className="block text-[10px] text-slate-600 bg-white px-2 py-1.5 rounded border border-slate-100 font-mono max-h-32 overflow-auto">
                                    {typeof tool.result === 'string' ? tool.result : JSON.stringify(tool.result, null, 2)}
                                  </code>
                                </div>
                              )}
                            </div>
                          )}
                        </div>
                      );
                    })}
                  </div>
                )}

                <div className={cn(
                  "p-3 lg:p-4 rounded-2xl text-sm leading-relaxed shadow-sm",
                  msg.role === 'user'
                    ? "bg-slate-900 text-white rounded-tr-none max-w-[95%]"
                    : "bg-white border border-slate-200 text-slate-800 rounded-tl-none w-fit max-w-[85%]"
                )}>
                  {msg.role === 'assistant' ? (
                    msg.status === 'streaming' ? (
                      <div className="text-slate-800 whitespace-pre-wrap">{msg.content}</div>
                    ) : (
                      <div className="markdown-body prose prose-slate prose-sm max-w-none">
                        <ReactMarkdown>{msg.content}</ReactMarkdown>
                      </div>
                    )
                  ) : (
                    msg.content
                  )}

                  {/* Media Content */}
                  <div className="space-y-3 mt-3">
                    {msg.imageUrl && (
                      <div className="rounded-xl overflow-hidden border border-slate-100">
                        <img src={msg.imageUrl} alt="Generated content" className="w-full h-auto" referrerPolicy="no-referrer" />
                      </div>
                    )}

                    {msg.videoUrl && (
                      <div className="rounded-xl overflow-hidden border border-slate-100 bg-black aspect-video">
                        <video src={msg.videoUrl} controls className="w-full h-full" />
                      </div>
                    )}

                    {msg.audioUrl && (
                      <div className="flex items-center gap-3 p-3 bg-slate-50 rounded-xl border border-slate-100">
                        <div className="w-8 h-8 rounded-full bg-brand-500 flex items-center justify-center text-white">
                          <Volume2 size={16} />
                        </div>
                        <audio src={msg.audioUrl} controls className="h-8 flex-1" />
                      </div>
                    )}
                  </div>

                  {/* a2ui Dashboard Demo */}
                  {msg.a2ui?.type === 'dashboard' && (
                    <div className="mt-4 p-4 bg-slate-50 rounded-xl border border-slate-100">
                      <div className="flex items-center justify-between mb-4">
                        <h4 className="text-xs font-bold text-slate-700 flex items-center gap-2">
                          <BarChart3 size={14} className="text-brand-500" />
                          {msg.a2ui.data.title}
                        </h4>
                        <div className="flex items-center gap-2">
                          {msg.trace && (
                            <button
                              onClick={() => {
                                setSelectedMessageId(msg.id);
                                setIsTraceOpen(true);
                              }}
                              className="p-1 hover:bg-slate-200 rounded text-slate-400 transition-colors"
                              title="查看执行追踪"
                            >
                              <Search size={12} />
                            </button>
                          )}
                          <ExternalLink size={12} className="text-slate-400" />
                        </div>
                      </div>
                      <div className="grid grid-cols-3 gap-3">
                        {msg.a2ui.data.metrics.map((m: any, i: number) => (
                          <div key={i} className="bg-white p-2 rounded-lg border border-slate-100 shadow-sm">
                            <p className="text-[8px] text-slate-400 uppercase font-bold mb-1">{m.label}</p>
                            <p className="text-sm font-bold text-slate-900">{m.value}</p>
                            <p className={cn(
                              "text-[8px] font-bold mt-1",
                              m.trend.startsWith('+') ? "text-green-500" : "text-red-500"
                            )}>{m.trend}</p>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}

                  {msg.files && msg.files.length > 0 && (
                    <div className="mt-3 flex flex-wrap gap-2">
                      {msg.files.map((file, i) => (
                        <div key={i} className="flex items-center gap-2 p-2 bg-white/10 rounded-lg border border-white/10">
                          <Paperclip size={12} />
                          <span className="text-[10px] truncate max-w-[100px]">{file.name}</span>
                        </div>
                      ))}
                    </div>
                  )}

                  {/* Trace Trigger for non-a2ui messages */}
                  {msg.role === 'assistant' && msg.trace && !msg.a2ui && (
                    <div className="mt-3 pt-3 border-t border-slate-100 flex justify-end">
                      <button
                        onClick={() => {
                          setSelectedMessageId(msg.id);
                          setIsTraceOpen(true);
                        }}
                        className="flex items-center gap-1.5 text-[10px] font-bold text-brand-500 hover:text-brand-600 uppercase tracking-wider"
                      >
                        <Search size={12} />
                        查看执行追踪
                      </button>
                    </div>
                  )}
                </div>
                {isLoading && msg.id === messages[messages.length - 1]?.id && msg.role === 'user' && (
                  <div className="flex gap-1 mt-1">
                    <div className="w-1.5 h-1.5 bg-brand-500 rounded-full animate-bounce" style={{ animationDelay: '0ms' }} />
                    <div className="w-1.5 h-1.5 bg-brand-500 rounded-full animate-bounce" style={{ animationDelay: '150ms' }} />
                    <div className="w-1.5 h-1.5 bg-brand-500 rounded-full animate-bounce" style={{ animationDelay: '300ms' }} />
                  </div>
                )}
                <span className="text-[10px] text-slate-400 font-medium uppercase tracking-tighter">
                  {msg.timestamp.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                </span>
              </div>
            </motion.div>
          ))}

          {/* Pending Approvals Cards */}
          {pendingApprovals.length > 0 && (
            <div className="space-y-4 mt-4">
              {pendingApprovals.map(approval => (
                <div
                  key={approval.id}
                  className="bg-white rounded-2xl border border-amber-200 p-6 shadow-lg shadow-amber-100/50"
                >
                  <div className="flex items-start justify-between mb-4">
                    <div className="flex items-center gap-3">
                      <div className="w-10 h-10 rounded-full bg-amber-500 flex items-center justify-center text-white font-bold">
                        <ShieldAlert size={20} />
                      </div>
                      <div>
                        <h4 className="font-bold text-slate-900">待审批请求</h4>
                        <div className="flex items-center gap-2 text-[10px] text-slate-400 font-medium">
                          <Clock size={10} />
                          {approval.timestamp.toLocaleString()}
                        </div>
                      </div>
                    </div>
                    <div className="px-3 py-1 rounded-full text-[10px] font-bold uppercase tracking-wider bg-amber-50 text-amber-600 border border-amber-100">
                      待审批
                    </div>
                  </div>

                  <div className="bg-slate-50 rounded-xl p-4 mb-4 border border-slate-100">
                    <div className="flex items-center gap-2 text-slate-900 mb-2">
                      <ShieldAlert size={16} className="text-amber-500" />
                      <span className="text-sm font-bold">工具: {approval.toolName}</span>
                    </div>
                    <p className="text-sm text-slate-600 mb-4">
                      风险等级: <span className={approval.riskLevel === 'high' ? 'text-red-500 font-bold' : 'text-amber-500 font-bold'}>{approval.riskLevel.toUpperCase()}</span>
                    </p>

                    <div className="space-y-2">
                      <p className="text-[10px] font-bold text-slate-400 uppercase tracking-widest">参数</p>
                      <div className="bg-white rounded-lg border border-slate-200 p-3 font-mono text-xs text-slate-700">
                        <pre>{JSON.stringify(approval.parameters, null, 2)}</pre>
                      </div>
                    </div>
                  </div>

                  <div className="flex items-center justify-end gap-3">
                    <button
                      onClick={() => handleApproval(approval.id, 'rejected')}
                      className="flex items-center gap-2 px-6 py-2 bg-white border border-slate-200 rounded-lg text-sm font-bold text-slate-600 hover:bg-red-50 hover:text-red-600 hover:border-red-200 transition-all"
                    >
                      <XCircle size={16} />
                      拒绝
                    </button>
                    <button
                      onClick={() => handleApproval(approval.id, 'approved')}
                      className="flex items-center gap-2 px-6 py-2 bg-brand-500 text-white rounded-lg text-sm font-bold hover:bg-brand-600 transition-all shadow-lg shadow-brand-500/20"
                    >
                      <UserCheck size={16} />
                      批准
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Input Area */}
        <div className="p-4 lg:p-6 bg-white border-t border-slate-100">
          <AnimatePresence>
            {files.length > 0 && (
              <motion.div
                initial={{ opacity: 0, height: 0 }}
                animate={{ opacity: 1, height: 'auto' }}
                exit={{ opacity: 0, height: 0 }}
                className="mb-3 flex flex-wrap gap-2"
              >
                {files.map((file, i) => (
                  <div key={i} className="group relative flex items-center gap-2 p-2 bg-slate-50 rounded-lg border border-slate-200">
                    <Paperclip size={14} className="text-slate-400" />
                    <span className="text-xs text-slate-600 truncate max-w-[150px]">{file.name}</span>
                    <button
                      onClick={() => setFiles(prev => prev.filter((_, idx) => idx !== i))}
                      className="p-1 hover:bg-slate-200 rounded-full text-slate-400 hover:text-red-500 transition-colors"
                    >
                      <Trash2 size={12} />
                    </button>
                  </div>
                ))}
              </motion.div>
            )}
          </AnimatePresence>

          <div className="relative group w-full max-w-4xl mx-auto">
            <div className="relative bg-white border border-slate-200 rounded-[24px] shadow-[0_4px_20px_rgb(0,0,0,0.03)] transition-all">
              <textarea
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && !e.shiftKey) {
                    e.preventDefault();
                    handleSend();
                  }
                }}
                placeholder={t('chat.placeholder')}
                className="w-full bg-transparent border-none focus:ring-0 outline-none focus:outline-none text-base p-4 pb-1 resize-none min-h-[50px] max-h-32 scrollbar-hide"
              />

              <div className="flex items-center justify-between px-4 pb-4">
                <div className="flex items-center gap-1 flex-1 mr-2">
                  <button
                    onClick={() => document.getElementById('file-upload')?.click()}
                    className="p-1.5 hover:bg-slate-50 rounded-full text-slate-600 transition-colors shrink-0"
                  >
                    <Paperclip size={16} />
                  </button>

                  <div className="h-4 w-px bg-slate-100 shrink-0" />

                  <button
                    onClick={runDemo}
                    className="flex items-center gap-1 px-2 py-1 rounded-lg bg-brand-50 text-brand-600 hover:bg-brand-100 transition-all text-[10px] font-bold whitespace-nowrap shrink-0"
                  >
                    <Zap size={12} />
                    演示全场景
                  </button>

                  <div className="h-4 w-px bg-slate-100 shrink-0" />

                  <div className="flex items-center gap-1">
                    {visibleAgents.map(agent => (
                      <button
                        key={agent.ulid || agent.id}
                        onClick={() => {
                          setActiveAgent(agent);
                        }}
                        className={cn(
                          "flex items-center gap-1 px-2 py-1.5 rounded-lg transition-all text-xs font-medium whitespace-nowrap shrink-0 ring-2",
                          activeAgent && (activeAgent.ulid ? activeAgent.ulid === agent.ulid : activeAgent.id === agent.id)
                            ? "ring-brand-500 bg-brand-50 text-brand-700"
                            : "text-slate-600 hover:bg-slate-50 ring-transparent"
                        )}
                      >
                        {getAgentIcon(agent.icon || '', 12)}
                        <span>{agent.name}</span>
                        {agent.id === 'quick' && (
                          <span className="bg-blue-50 text-blue-500 text-[9px] px-1 py-0.5 rounded font-bold">新</span>
                        )}
                      </button>
                    ))}
                    {moreAgents.length > 0 && (
                      <div className="relative shrink-0">
                        <button
                          onClick={() => setIsMoreAgentsOpen(!isMoreAgentsOpen)}
                          className={cn(
                            "flex items-center gap-1 px-2 py-1.5 rounded-lg transition-all text-xs font-medium whitespace-nowrap",
                            isMoreAgentsOpen ? "bg-slate-100 text-slate-900" : "text-slate-600 hover:bg-slate-50"
                          )}
                        >
                          <LayoutGrid size={14} />
                          <span>{t('chat.more')}</span>
                        </button>

                        <AnimatePresence>
                          {isMoreAgentsOpen && (
                            <>
                              <div
                                className="fixed inset-0 z-20"
                                onClick={() => setIsMoreAgentsOpen(false)}
                              />
                              <motion.div
                                initial={{ opacity: 0, y: 10, scale: 0.95 }}
                                animate={{ opacity: 1, y: 0, scale: 1 }}
                                exit={{ opacity: 0, y: 10, scale: 0.95 }}
                                className="absolute bottom-full right-0 mb-2 w-48 bg-white border border-slate-100 rounded-xl shadow-lg z-30 py-1.5 overflow-hidden"
                              >
                                {moreAgents.map(agent => (
                                  <button
                                    key={agent.ulid || agent.id}
                                    onClick={() => {
                                      setActiveAgent(agent);
                                      setIsMoreAgentsOpen(false);
                                    }}
                                    className={cn(
                                      "w-full flex items-center gap-2 px-3 py-2 text-xs transition-colors",
                                      activeAgent && (activeAgent.ulid ? activeAgent.ulid === agent.ulid : activeAgent.id === agent.id) ? "bg-brand-50 text-brand-700 font-semibold ring-2 ring-brand-500" : "text-slate-700 hover:bg-slate-50"
                                    )}
                                  >
                                    {getAgentIcon(agent.icon || '', 14)}
                                    <span className="font-medium">{agent.name}</span>
                                  </button>
                                ))}
                              </motion.div>
                            </>
                          )}
                        </AnimatePresence>
                      </div>
                    )}
                  </div>
                </div>

                <button
                  onClick={isLoading ? stopGeneration : handleSend}
                  disabled={(!input.trim() && files.length === 0) && !isLoading}
                  className={cn(
                    "w-10 h-10 rounded-full flex items-center justify-center transition-all shrink-0",
                    isLoading
                      ? "bg-red-500 text-white shadow-lg shadow-red-500/20 hover:scale-105 active:scale-95"
                      : (input.trim() || files.length > 0)
                        ? "bg-blue-600 text-white shadow-lg shadow-blue-600/20 hover:scale-105 active:scale-95"
                        : "bg-slate-100 text-slate-300 cursor-not-allowed"
                  )}
                >
                  {isLoading ? (
                    <div className="w-3 h-3 bg-white rounded-sm" />
                  ) : (
                    <ArrowUp size={18} strokeWidth={2.5} />
                  )}
                </button>
              </div>
            </div>
          </div>

          <input
            id="file-upload"
            type="file"
            multiple
            className="hidden"
            onChange={(e) => {
              if (e.target.files) {
                onDrop(Array.from(e.target.files));
              }
            }}
          />
        </div>

        {isDragActive && (
          <div className="absolute inset-0 bg-brand-500/10 backdrop-blur-sm border-2 border-dashed border-brand-500 z-50 flex flex-col items-center justify-center">
            <div className="w-20 h-20 rounded-full bg-brand-500 text-white flex items-center justify-center mb-4 animate-bounce">
              <Plus size={40} />
            </div>
            <h2 className="text-2xl font-bold text-brand-600">{t('chat.dropFiles')}</h2>
            <p className="text-brand-500/70">{t('chat.dropSubtitle')}</p>
          </div>
        )}
      </div>

      {/* Trace Sidebar */}
      <AnimatePresence>
        {isTraceOpen && (
          <motion.div
            initial={{ x: 400 }}
            animate={{ x: 0 }}
            exit={{ x: 400 }}
            className="w-[400px] border-l border-slate-200 bg-white flex flex-col shrink-0 z-30 shadow-2xl"
          >
            <div className="h-14 border-b border-slate-100 flex items-center justify-between px-4">
              <div className="flex items-center gap-2">
                <Search size={18} className="text-brand-500" />
                <h3 className="font-bold text-slate-900">执行追踪 (Tracing)</h3>
              </div>
              <button
                onClick={() => setIsTraceOpen(false)}
                className="p-1.5 hover:bg-slate-100 rounded-md text-slate-400"
              >
                <Plus size={20} className="rotate-45" />
              </button>
            </div>
            <div className="flex-1 overflow-y-auto p-6 space-y-6 scrollbar-hide">
              {messages.find(m => m.id === selectedMessageId)?.trace?.map((step, idx) => (
                <div key={step.id} className="relative pl-8">
                  {/* Timeline Line */}
                  {idx !== (messages.find(m => m.id === selectedMessageId)?.trace?.length || 0) - 1 && (
                    <div className="absolute left-[11px] top-6 bottom-[-24px] w-0.5 bg-slate-100" />
                  )}

                  {/* Step Icon */}
                  <div className={cn(
                    "absolute left-0 top-0 w-6 h-6 rounded-full flex items-center justify-center z-10",
                    step.status === 'success' ? "bg-green-500 text-white" : "bg-brand-500 text-white"
                  )}>
                    {step.type === 'thought' && <Brain size={12} />}
                    {step.type === 'tool' && <Wrench size={12} />}
                    {step.type === 'skill' && <Zap size={12} />}
                    {step.type === 'observation' && <Eye size={12} />}
                  </div>

                  <div className="space-y-1">
                    <div className="flex items-center justify-between">
                      <span className="text-[10px] font-bold text-slate-400 uppercase tracking-widest">
                        {step.label}
                      </span>
                      {step.duration && (
                        <span className="text-[10px] font-medium text-slate-400 bg-slate-100 px-1.5 py-0.5 rounded">
                          {step.duration}
                        </span>
                      )}
                    </div>
                    <div className="p-3 bg-slate-50 rounded-xl border border-slate-100 text-xs text-slate-700 leading-relaxed">
                      {step.content}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
