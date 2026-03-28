import React from 'react';
import { 
  Plus, 
  Workflow, 
  Zap, 
  Database, 
  Cpu, 
  Play, 
  Save,
  ChevronRight,
  Settings2,
  Layers,
  Search,
  Box,
  Share2,
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
  RefreshCw,
  Code,
  Layout,
  Activity
} from 'lucide-react';
import { motion, AnimatePresence } from 'motion/react';
import { cn } from '../lib/utils';
import { useTranslation } from 'react-i18next';
import { INITIAL_SKILLS, INITIAL_KNOWLEDGE_BASES, MOCK_MODELS } from '../constants';
import { Agent, Skill, KnowledgeBase, Message, Variable } from '../types';

export function AgentOrchestrator() {
  const { t } = useTranslation();
  
  // Agent Configuration State
  const [agentConfig, setAgentConfig] = React.useState({
    name: 'New Agent',
    description: 'A custom orchestrated agent',
    systemPrompt: '',
    reasoningModel: MOCK_MODELS[0].id,
    generationModel: MOCK_MODELS[1].id,
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
      type: 'a2ui',
      version: '1.0',
      strict: true,
      schema: '{}',
    },
  });

  // Test Chat State
  const [testMessages, setTestMessages] = React.useState<Message[]>([]);
  const [testInput, setTestInput] = React.useState('');
  const [isTesting, setIsTesting] = React.useState(false);
  const [isTestPanelCollapsed, setIsTestPanelCollapsed] = React.useState(false);
  const testScrollRef = React.useRef<HTMLDivElement>(null);
  const testAbortControllerRef = React.useRef<AbortController | null>(null);

  // Skill Category State
  const [skillCategory, setSkillCategory] = React.useState<'all' | 'built-in' | 'mcp' | 'tool' | 'a2a'>('all');

  const filteredSkills = INITIAL_SKILLS.filter(skill => 
    skillCategory === 'all' || skill.type === skillCategory
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

  const handleApprove = (messageId: string) => {
    setTestMessages(prev => prev.map(msg => {
      if (msg.id === messageId) {
        return {
          ...msg,
          status: 'completed',
          content: msg.content + "\n\n✅ **已批准**。工具已成功执行。",
          trace: msg.trace?.map(t => t.type === 'tool' ? { ...t, status: 'success', content: '工具执行已完成。' } : t)
        };
      }
      return msg;
    }));
  };

  const handleReject = (messageId: string) => {
    setTestMessages(prev => prev.map(msg => {
      if (msg.id === messageId) {
        return {
          ...msg,
          status: 'failed',
          content: msg.content + "\n\n❌ **已拒绝**。操作已被用户取消。",
          trace: msg.trace?.map(t => t.type === 'tool' ? { ...t, status: 'error', content: '用户拒绝了该操作。' } : t)
        };
      }
      return msg;
    }));
  };

  const handleSendMessage = () => {
    if (!testInput.trim()) return;
    
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

    // Mock AI Response with Trace Data
    const timerId = setTimeout(() => {
      // Check for risk-based intervention
      const selectedSkillsData = INITIAL_SKILLS.filter(s => agentConfig.selectedSkills.includes(s.id));
      
      // For demo: if user mentions a tool name, simulate calling it
      const mentionedSkill = selectedSkillsData.find(s => 
        testInput.toLowerCase().includes(s.name.toLowerCase()) || 
        testInput.toLowerCase().includes(s.id.toLowerCase())
      );

      if (mentionedSkill && agentConfig.requireApproval) {
        const isIntercepted = 
          (agentConfig.approvalThreshold === 'low') ||
          (agentConfig.approvalThreshold === 'medium' && (mentionedSkill.riskLevel === 'medium' || mentionedSkill.riskLevel === 'high')) ||
          (agentConfig.approvalThreshold === 'high' && mentionedSkill.riskLevel === 'high');

        if (isIntercepted) {
          const approvalMsg: Message = {
            id: (Date.now() + 1).toString(),
            role: 'assistant',
            content: `我需要使用 **${mentionedSkill.name}** 工具来处理您的请求。由于这是一个 **${mentionedSkill.riskLevel}** 风险的操作，需要您的审批。`,
            timestamp: new Date(),
            status: 'pending_approval',
            trace: [
              { id: '1', type: 'thought' as const, label: '思考', content: `用户请求：${testInput}。需要调用 ${mentionedSkill.name}。`, status: 'success' as const, timestamp: new Date() },
              { id: '2', type: 'tool' as const, label: mentionedSkill.name, content: '等待人工审批...', status: 'pending' as const, timestamp: new Date() }
            ]
          };
          setTestMessages(prev => [...prev, approvalMsg]);
          setIsTesting(false);
          testAbortControllerRef.current = null;
          return;
        }
      }

      const aiMsg: Message = {
        id: (Date.now() + 1).toString(),
        role: 'assistant',
        content: `我已根据以下配置完成测试：\n- 推理模型：${agentConfig.reasoningModel}\n- 生成模型：${agentConfig.generationModel}\n- 知识库：${agentConfig.selectedKBs.length} 个\n- 技能：${agentConfig.selectedSkills.length} 个`,
        timestamp: new Date(),
        trace: [
          { id: '1', type: 'thought' as const, label: '推理', content: '正在分析用户查询并识别意图...', status: 'success' as const, timestamp: new Date() },
          ...(agentConfig.selectedKBs.length > 0 ? [{ id: '2', type: 'retrieval' as const, label: '检索', content: `正在 ${agentConfig.selectedKBs.length} 个知识库中搜索... 找到 3 个相关片段。`, status: 'success' as const, timestamp: new Date() }] : []),
          ...(agentConfig.selectedSkills.length > 0 ? [{ id: '3', type: 'tool' as const, label: '工具执行', content: `正在执行工具：${INITIAL_SKILLS.find(s => agentConfig.selectedSkills.includes(s.id))?.name}...`, status: 'success' as const, timestamp: new Date() }] : []),
          { id: '4', type: 'thought' as const, label: '综合', content: '基于检索数据和工具输出综合最终回复。', status: 'success' as const, timestamp: new Date() }
        ]
      };
      setTestMessages(prev => [...prev, aiMsg]);
      setIsTesting(false);
      testAbortControllerRef.current = null;
    }, 1500);

    testAbortControllerRef.current.signal.addEventListener('abort', () => {
      clearTimeout(timerId);
      setIsTesting(false);
      testAbortControllerRef.current = null;
    });
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
          <button className="flex items-center gap-2 px-4 py-2 bg-white border border-slate-200 rounded-lg text-sm font-bold text-slate-600 hover:bg-slate-50 transition-all">
            <Save size={16} />
            {t('orchestrator.saveDraft')}
          </button>
          <button className="flex items-center gap-2 px-4 py-2 bg-brand-500 text-white rounded-lg text-sm font-bold hover:bg-brand-600 transition-all shadow-lg shadow-brand-500/20">
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
                          !arr[idx+1].active && "hidden"
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
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <label className="text-[10px] font-bold text-slate-400 uppercase tracking-widest">{t('orchestrator.reasoningStage')}</label>
                <select 
                  value={agentConfig.reasoningModel}
                  onChange={(e) => setAgentConfig(prev => ({ ...prev, reasoningModel: e.target.value }))}
                  className="w-full bg-white border border-slate-200 rounded-xl px-4 py-2.5 text-sm focus:ring-2 focus:ring-brand-500/20 outline-none"
                >
                  {MOCK_MODELS.map(m => <option key={m.id} value={m.id}>{m.name}</option>)}
                </select>
              </div>
              <div className="space-y-2">
                <label className="text-[10px] font-bold text-slate-400 uppercase tracking-widest">{t('orchestrator.generationStage')}</label>
                <select 
                  value={agentConfig.generationModel}
                  onChange={(e) => setAgentConfig(prev => ({ ...prev, generationModel: e.target.value }))}
                  className="w-full bg-white border border-slate-200 rounded-xl px-4 py-2.5 text-sm focus:ring-2 focus:ring-brand-500/20 outline-none"
                >
                  {MOCK_MODELS.map(m => <option key={m.id} value={m.id}>{m.name}</option>)}
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
                  onClick={() => setAgentConfig(prev => ({ ...prev, selectedKBs: INITIAL_KNOWLEDGE_BASES.map(kb => kb.id) }))}
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
              {INITIAL_KNOWLEDGE_BASES.map(kb => (
                <button
                  key={kb.id}
                  onClick={() => handleToggleKB(kb.id)}
                  className={cn(
                    "flex items-center gap-3 p-3 rounded-xl border-2 transition-all text-left group",
                    agentConfig.selectedKBs.includes(kb.id)
                      ? "bg-brand-50 border-brand-500"
                      : "bg-white border-slate-100 hover:border-slate-200"
                  )}
                >
                  <div className={cn(
                    "p-2 rounded-lg transition-colors",
                    agentConfig.selectedKBs.includes(kb.id) ? "bg-brand-500 text-white" : "bg-slate-100 text-slate-500 group-hover:bg-slate-200"
                  )}>
                    <Database size={16} />
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-bold text-slate-900 truncate">{kb.name}</p>
                    <p className="text-[10px] text-slate-400 line-clamp-1">{kb.description}</p>
                  </div>
                  {agentConfig.selectedKBs.includes(kb.id) && (
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
                {(['all', 'built-in', 'mcp', 'tool', 'a2a'] as const).map((cat) => (
                  <button
                    key={cat}
                    onClick={() => setSkillCategory(cat)}
                    className={cn(
                      "px-3 py-1 text-[9px] font-bold uppercase tracking-wider rounded-md transition-all",
                      skillCategory === cat ? "bg-white text-slate-900 shadow-sm" : "text-slate-500 hover:text-slate-700"
                    )}
                  >
                    {cat === 'built-in' ? t('skills.skillsTab') : (cat === 'all' ? 'All' : t(`skills.${cat}Tab`))}
                  </button>
                ))}
              </div>
            </div>

            <div className="grid grid-cols-3 gap-3">
              {filteredSkills.map(skill => (
                <button
                  key={skill.id}
                  onClick={() => handleToggleSkill(skill.id)}
                  className={cn(
                    "flex flex-col gap-2 p-3 rounded-xl border-2 transition-all text-left",
                    agentConfig.selectedSkills.includes(skill.id)
                      ? "bg-brand-50 border-brand-500"
                      : "bg-white border-slate-100 hover:border-slate-200"
                  )}
                >
                  <div className="flex items-center justify-between">
                    <div className={cn(
                      "p-1.5 rounded-lg",
                      agentConfig.selectedSkills.includes(skill.id) ? "bg-brand-500 text-white" : "bg-slate-100 text-slate-500"
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
                      {INITIAL_SKILLS
                        .filter(s => agentConfig.selectedSkills.includes(s.id))
                        .map(skill => {
                          const isIntercepted = 
                            (agentConfig.approvalThreshold === 'low') ||
                            (agentConfig.approvalThreshold === 'medium' && (skill.riskLevel === 'medium' || skill.riskLevel === 'high')) ||
                            (agentConfig.approvalThreshold === 'high' && skill.riskLevel === 'high');

                          return (
                            <div 
                              key={skill.id} 
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
                {['api', 'web', 'feishu', 'dingtalk'].map(channel => (
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
                    <p className="text-sm font-medium text-slate-500">Configure your agent and start testing here.</p>
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
                        {msg.content}
                        
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
    </div>
  );
}
