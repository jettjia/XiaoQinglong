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
  PanelLeftOpen
} from 'lucide-react';
import { useDropzone } from 'react-dropzone';
import { motion, AnimatePresence } from 'motion/react';
import ReactMarkdown from 'react-markdown';
import { cn } from '../lib/utils';
import { Message, FileInfo, Agent, Conversation } from '../types';
import { INITIAL_AGENTS } from '../constants';
import { GoogleGenAI } from "@google/genai";
import { useTranslation } from 'react-i18next';

const ai = new GoogleGenAI({ apiKey: process.env.GEMINI_API_KEY || '' });

export function ChatInterface() {
  const { t } = useTranslation();
  const [messages, setMessages] = React.useState<Message[]>([]);
  const [input, setInput] = React.useState('');
  const [files, setFiles] = React.useState<FileInfo[]>([]);
  const [activeAgent, setActiveAgent] = React.useState<Agent>(INITIAL_AGENTS[0]);
  const [isLoading, setIsLoading] = React.useState(false);
  const [conversations, setConversations] = React.useState<Conversation[]>([
    { id: '1', title: 'Research on AI Agents', lastMessage: 'How can I help?', timestamp: new Date() },
    { id: '2', title: 'Data Analysis Project', lastMessage: 'The results are ready.', timestamp: new Date() }
  ]);
  const [activeConversationId, setActiveConversationId] = React.useState<string | null>(null);
  const [isMoreAgentsOpen, setIsMoreAgentsOpen] = React.useState(false);
  const [isSidebarOpen, setIsSidebarOpen] = React.useState(true);
  const [isTraceOpen, setIsTraceOpen] = React.useState(false);
  const [selectedMessageId, setSelectedMessageId] = React.useState<string | null>(null);
  const [showThinking, setShowThinking] = React.useState<Record<string, boolean>>({});
  const scrollRef = React.useRef<HTMLDivElement>(null);
  const abortControllerRef = React.useRef<AbortController | null>(null);

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

  const visibleAgents = INITIAL_AGENTS.slice(0, 6);
  const moreAgents = INITIAL_AGENTS.slice(6);

  const onDrop = React.useCallback((acceptedFiles: File[]) => {
    const newFiles = acceptedFiles.map(file => ({
      name: file.name,
      size: file.size,
      type: file.type,
      url: URL.createObjectURL(file)
    }));
    setFiles(prev => [...prev, ...newFiles]);
  }, []);

  const { getRootProps, getInputProps, isDragActive } = useDropzone({ 
    onDrop,
    noClick: true
  } as any);

  const handleSend = async () => {
    if ((!input.trim() && files.length === 0) || isLoading) return;

    const userMessage: Message = {
      id: Date.now().toString(),
      role: 'user',
      content: input,
      timestamp: new Date(),
      files: [...files]
    };

    setMessages(prev => [...prev, userMessage]);
    setInput('');
    setFiles([]);
    setIsLoading(true);
    abortControllerRef.current = new AbortController();

    try {
      const parts: any[] = [{ text: input }];
      
      // Add image data if present
      for (const file of userMessage.files || []) {
        if (file.type.startsWith('image/')) {
          const base64 = await fetch(file.url).then(r => r.blob()).then(blob => {
            return new Promise((resolve) => {
              const reader = new FileReader();
              reader.onloadend = () => resolve((reader.result as string).split(',')[1]);
              reader.readAsDataURL(blob);
            });
          });
          parts.push({
            inlineData: {
              data: base64,
              mimeType: file.type
            }
          });
        }
      }

      // We wrap the API call to allow cancellation (even if SDK doesn't support it directly)
      const generatePromise = ai.models.generateContent({
        model: activeAgent.model || "gemini-3-flash-preview",
        contents: { parts },
        config: {
          systemInstruction: `You are ${activeAgent.name}, ${activeAgent.description}. Provide helpful, accurate, and concise responses.`
        }
      });

      const response = await Promise.race([
        generatePromise,
        new Promise<null>((_, reject) => {
          abortControllerRef.current?.signal.addEventListener('abort', () => reject(new Error('Aborted')));
        })
      ]);

      if (!response) return;

      const assistantMessage: Message = {
        id: (Date.now() + 1).toString(),
        role: 'assistant',
        content: response.text || "I'm sorry, I couldn't generate a response.",
        timestamp: new Date()
      };
      setMessages(prev => [...prev, assistantMessage]);
    } catch (error: any) {
      if (error.message === 'Aborted') {
        console.log("Generation stopped by user");
        return;
      }
      console.error("Gemini Error:", error);
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
  };

  const deleteConversation = (id: string) => {
    setConversations(prev => prev.filter(c => c.id !== id));
    if (activeConversationId === id) {
      startNewConversation();
    }
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
              placeholder={t('agents.search')}
              className="w-full pl-9 pr-4 py-1.5 bg-white border border-slate-200 rounded-lg text-xs focus:ring-2 focus:ring-brand-500/20"
            />
          </div>
        </div>
        <div className="flex-1 overflow-y-auto p-2 space-y-1 min-w-[280px]">
          {conversations.length === 0 ? (
            <div className="p-4 text-center">
              <p className="text-xs text-slate-400">{t('chat.noConversations')}</p>
            </div>
          ) : (
            conversations.map(conv => (
              <div key={conv.id} className="group relative">
                <button 
                  onClick={() => setActiveConversationId(conv.id)}
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
            ))
          )}
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
                {activeAgent.name[0]}
              </div>
            </div>
            <div>
              <h3 className="font-bold text-slate-900 text-xs lg:text-sm">{activeAgent.name}</h3>
              <div className="flex items-center gap-1.5">
                <div className="w-1 h-1 rounded-full bg-green-500" />
                <span className="text-[8px] lg:text-[10px] font-medium text-slate-400 uppercase tracking-wider">{t('chat.active')}</span>
              </div>
            </div>
          </div>
          <div className="flex items-center gap-2">
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
                Ask {activeAgent.name} anything. You can upload documents, images, or just start typing.
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

                {/* Tool Calls */}
                {msg.toolCalls && msg.toolCalls.length > 0 && (
                  <div className="w-full max-w-2xl space-y-2 mb-2">
                    {msg.toolCalls.map((tool, idx) => (
                      <div key={idx} className="bg-slate-50 border border-slate-100 rounded-xl overflow-hidden">
                        <div className="px-4 py-2 border-b border-slate-100 flex items-center justify-between bg-slate-100/50">
                          <div className="flex items-center gap-2 text-[10px] font-bold text-slate-500 uppercase">
                            <Wrench size={12} className="text-brand-500" />
                            工具调用: {tool.name}
                          </div>
                          <Check size={12} className="text-green-500" />
                        </div>
                        <div className="p-3 space-y-2">
                          <div className="flex items-start gap-2">
                            <Terminal size={12} className="text-slate-400 mt-0.5" />
                            <code className="text-[10px] text-slate-600 bg-white px-1.5 py-0.5 rounded border border-slate-100">
                              {JSON.stringify(tool.args)}
                            </code>
                          </div>
                          {tool.result && (
                            <div className="flex items-start gap-2 pt-2 border-t border-slate-100">
                              <CheckSquare size={12} className="text-green-500 mt-0.5" />
                              <div className="text-[10px] text-slate-500">
                                {JSON.stringify(tool.result)}
                              </div>
                            </div>
                          )}
                        </div>
                      </div>
                    ))}
                  </div>
                )}

                <div className={cn(
                  "p-3 lg:p-4 rounded-2xl text-sm leading-relaxed shadow-sm w-fit max-w-[90%]",
                  msg.role === 'user' 
                    ? "bg-slate-900 text-white rounded-tr-none" 
                    : "bg-white border border-slate-200 text-slate-800 rounded-tl-none"
                )}>
                  {msg.role === 'assistant' ? (
                    <div className="markdown-body prose prose-slate prose-sm max-w-none">
                      <ReactMarkdown>{msg.content}</ReactMarkdown>
                    </div>
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
                <div className="flex items-center gap-3 flex-wrap py-1 flex-1 mr-2">
                  <button 
                    onClick={() => document.getElementById('file-upload')?.click()}
                    className="p-1.5 hover:bg-slate-50 rounded-full text-slate-600 transition-colors shrink-0"
                  >
                    <Paperclip size={18} />
                  </button>
                  
                  <div className="h-5 w-px bg-slate-100 shrink-0" />

                  <button 
                    onClick={runDemo}
                    className="flex items-center gap-2 px-2.5 py-1.5 rounded-lg bg-brand-50 text-brand-600 hover:bg-brand-100 transition-all text-[11px] font-bold whitespace-nowrap shrink-0"
                  >
                    <Zap size={14} />
                    演示全场景
                  </button>
                  
                  <div className="h-5 w-px bg-slate-100 shrink-0" />

                  <div className="flex items-center gap-2">
                    {visibleAgents.map(agent => (
                      <button 
                        key={agent.id}
                        onClick={() => setActiveAgent(agent)}
                        className={cn(
                          "flex items-center gap-2 px-3 py-2 rounded-xl transition-all shrink-0 text-sm font-medium whitespace-nowrap",
                          activeAgent.id === agent.id 
                            ? "bg-slate-50 text-slate-900" 
                            : "text-slate-900 hover:bg-slate-50"
                        )}
                      >
                        <div className={cn(
                          "flex items-center gap-2",
                          agent.id === 'quick' && "text-slate-900"
                        )}>
                          {getAgentIcon(agent.icon || '')}
                          <span>{agent.name}</span>
                          {agent.id === 'quick' && (
                            <div className="flex items-center gap-1">
                              <span className="bg-blue-50 text-blue-500 text-[10px] px-1.5 py-0.5 rounded font-bold">新</span>
                              <ChevronRight size={14} className="text-slate-300" />
                            </div>
                          )}
                        </div>
                      </button>
                    ))}
                    {moreAgents.length > 0 && (
                      <div className="relative shrink-0">
                        <button 
                          onClick={() => setIsMoreAgentsOpen(!isMoreAgentsOpen)}
                          className={cn(
                            "flex items-center gap-2 px-3 py-2 rounded-xl transition-all text-sm font-medium whitespace-nowrap",
                            isMoreAgentsOpen ? "bg-slate-50 text-slate-900" : "text-slate-900 hover:bg-slate-50"
                          )}
                        >
                          <LayoutGrid size={18} />
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
                                className="absolute bottom-full right-0 mb-3 w-56 bg-white border border-slate-100 rounded-2xl shadow-[0_10px_40px_rgba(0,0,0,0.08)] z-30 py-2 overflow-hidden"
                              >
                                {moreAgents.map(agent => (
                                  <button
                                    key={agent.id}
                                    onClick={() => {
                                      setActiveAgent(agent);
                                      setIsMoreAgentsOpen(false);
                                    }}
                                    className={cn(
                                      "w-full flex items-center gap-3 px-4 py-2.5 text-base transition-colors",
                                      activeAgent.id === agent.id ? "bg-slate-50 text-slate-900 font-semibold" : "text-slate-900 hover:bg-slate-50"
                                    )}
                                  >
                                    <div className="text-slate-900 shrink-0">
                                      {getAgentIcon(agent.icon || '', 20)}
                                    </div>
                                    <span className="whitespace-nowrap font-medium">{agent.name}</span>
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
