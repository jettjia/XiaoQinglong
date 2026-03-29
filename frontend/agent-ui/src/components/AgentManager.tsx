import React, { useState } from 'react';
import {
  Plus,
  Search,
  Filter,
  MoreVertical,
  Play,
  Edit2,
  Trash2,
  ExternalLink,
  Users,
  Cpu,
  Globe,
  Clock,
  CheckCircle2,
  XCircle,
  AlertCircle,
  X,
  Download,
  Copy,
  Bot,
  Sparkles,
  Brain,
  Zap,
  Workflow,
  MessageSquare,
  Terminal,
  Code,
  Power
} from 'lucide-react';
import { Agent, View } from '../types';
import { INITIAL_AGENTS } from '../constants';
import { cn } from '../lib/utils';
import { useTranslation } from 'react-i18next';
import { motion, AnimatePresence } from 'motion/react';
import { agentApi, modelApi, skillApi, knowledgeBaseApi } from '../lib/api';

interface AgentManagerProps {
  onViewChange: (view: View) => void;
}

const AGENT_ICONS = ['Bot', 'Users', 'Sparkles', 'Brain', 'Zap', 'Workflow', 'MessageSquare', 'Globe', 'Terminal', 'Code'];

export function AgentManager({ onViewChange }: AgentManagerProps) {
  const { t } = useTranslation();
  const [agents, setAgents] = React.useState<Agent[]>(INITIAL_AGENTS);
  const [searchQuery, setSearchQuery] = React.useState('');
  const [filterStatus, setFilterStatus] = React.useState<'all' | 'enabled' | 'disabled' | 'system'>('all');
  const [selectedAgentForLogs, setSelectedAgentForLogs] = React.useState<Agent | null>(null);
  const [isImportModalOpen, setIsImportModalOpen] = React.useState(false);
  const [isViewModalOpen, setIsViewModalOpen] = useState(false);
  const [selectedAgent, setSelectedAgent] = React.useState<Agent | null>(null);
  const [importJson, setImportJson] = React.useState('');
  const [loading, setLoading] = React.useState(false);

  // 用于解析配置的数据
  const [backendModels, setBackendModels] = React.useState<any[]>([]);
  const [backendSkills, setBackendSkills] = React.useState<any[]>([]);
  const [backendKBs, setBackendKBs] = React.useState<any[]>([]);

  // 加载辅助数据
  React.useEffect(() => {
    const loadHelperData = async () => {
      try {
        const [models, skills, kbs] = await Promise.all([
          modelApi.findAll ? modelApi.findAll() : Promise.resolve([]),
          skillApi.findAll ? skillApi.findAll() : Promise.resolve([]),
          knowledgeBaseApi.findAll ? knowledgeBaseApi.findAll() : Promise.resolve([]),
        ]);
        setBackendModels(models || []);
        setBackendSkills(skills || []);
        setBackendKBs(kbs || []);
      } catch (err) {
        console.error('Failed to load helper data:', err);
      }
    };
    loadHelperData();
  }, []);

  // 根据 ID 获取名称
  const getModelName = (id: string) => {
    const model = backendModels.find(m => m.ulid === id || m.id === id);
    return model ? `${model.name} (${model.provider})` : id;
  };

  const getSkillName = (id: string) => {
    const skill = backendSkills.find(s => s.ulid === id || s.id === id);
    return skill ? skill.name : id;
  };

  const getKBName = (id: string) => {
    const kb = backendKBs.find(k => k.ulid === id || k.id === id);
    return kb ? kb.name : id;
  };

  // 解析配置并返回友好显示 (基于ID的版本)
  const parseAgentConfig = (config: any) => {
    if (!config) return null;
    const cfg = typeof config === 'string' ? JSON.parse(config) : config;

    return {
      systemPrompt: cfg.systemPrompt || '-',
      models: cfg.models ? {
        default: getModelName(cfg.models.default),
        rewrite: getModelName(cfg.models.rewrite),
        skill: getModelName(cfg.models.skill),
        summarize: getModelName(cfg.models.summarize),
      } : null,
      temperature: cfg.temperature,
      maxTokens: cfg.maxTokens,
      topK: cfg.topK,
      rerank: cfg.rerank ? '是' : '否',
      selectedSkills: cfg.selectedSkills?.map(getSkillName).join(', ') || '-',
      selectedKBs: cfg.selectedKBs?.map(getKBName).join(', ') || '-',
      requireApproval: cfg.requireApproval ? '是' : '否',
      approvalThreshold: cfg.approvalThreshold,
      channels: cfg.channels?.join(', ') || '-',
      isPeriodic: cfg.isPeriodic ? '是' : '否',
      cronRule: cfg.cronRule || '-',
      memoryLimit: cfg.memoryLimit,
      longTermMemory: cfg.longTermMemory ? '是' : '否',
      variables: cfg.variables?.map((v: any) => `${v.name} (${v.type}${v.required ? ', 必填' : ''})`).join('; ') || '-',
      retryCount: cfg.retryCount,
      retryInterval: cfg.retryInterval,
      timeout: cfg.timeout,
      endpoint: cfg.endpoint || '-',
      maxIterations: cfg.maxIterations,
      stream: cfg.stream ? '是' : '否',
      sandbox: cfg.sandbox?.enabled ? `已启用 (${cfg.sandbox.mode})` : '未启用',
    };
  };

  // 解析 config_json (可运行的完整版本)
  const parseConfigJson = (configJson: any) => {
    if (!configJson) return null;
    const cfg = typeof configJson === 'string' ? JSON.parse(configJson) : configJson;

    return {
      endpoint: cfg.endpoint || '-',
      models: cfg.models || {},
      systemPrompt: cfg.system_prompt || '-',
      toolsCount: cfg.tools?.length || 0,
      tools: cfg.tools?.map((t: any) => t.name).join(', ') || '-',
      mcpsCount: cfg.mcps?.length || 0,
      mcps: cfg.mcps?.map((m: any) => m.name).join(', ') || '-',
      a2aCount: cfg.a2a?.length || 0,
      a2a: cfg.a2a?.map((a: any) => a.name).join(', ') || '-',
      skillsCount: cfg.skills?.length || 0,
      skills: cfg.skills?.map((s: any) => s.name).join(', ') || '-',
      knowledgeCount: cfg.knowledge?.length || 0,
      knowledge: cfg.knowledge?.map((k: any) => k.name).join(', ') || '-',
      temperature: cfg.options?.temperature,
      maxTokens: cfg.options?.max_tokens,
      maxIterations: cfg.options?.max_iterations,
      stream: cfg.options?.stream ? '是' : '否',
      sandbox: cfg.sandbox?.enabled ? `已启用 (${cfg.sandbox.mode})` : '未启用',
    };
  };

  // 从后端加载 agents
  const loadAgents = React.useCallback(async () => {
    try {
      setAgents([]); // 先清空
      const data = await agentApi.findAll();
      setAgents(data || []);
    } catch (err) {
      console.error('Failed to load agents:', err);
      setAgents([]);
    }
  }, []);

  React.useEffect(() => {
    loadAgents();
  }, [loadAgents]);

  const filteredAgents = agents.filter(a => {
    const matchesSearch = a.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      a.description.toLowerCase().includes(searchQuery.toLowerCase());

    if (filterStatus === 'enabled') return matchesSearch && a.enabled;
    if (filterStatus === 'disabled') return matchesSearch && !a.enabled;
    if (filterStatus === 'system') return matchesSearch && (a.is_system || a.isBuiltIn);
    return matchesSearch;
  });

  // 删除 Agent
  const handleDelete = async (agent: Agent) => {
    if (agent.is_system || agent.isBuiltIn) {
      alert('系统内置 Agent 不能删除');
      return;
    }
    if (!confirm(`确定要删除 Agent "${agent.name}" 吗？`)) {
      return;
    }
    try {
      await agentApi.delete(agent.ulid || agent.id);
      await loadAgents();
    } catch (err: any) {
      alert(err.message || '删除失败');
    }
  };

  // 启用/停用 Agent
  const handleToggleEnabled = async (agent: Agent) => {
    if (agent.is_system || agent.isBuiltIn) {
      alert('系统内置 Agent 不能停用');
      return;
    }
    try {
      await agentApi.updateEnabled(agent.ulid || agent.id, !agent.enabled);
      await loadAgents();
    } catch (err: any) {
      alert(err.message || '操作失败');
    }
  };

  // 导出 Agent JSON
  const handleExport = (agent: Agent) => {
    const exportData = {
      name: agent.name,
      description: agent.description,
      icon: agent.icon,
      model: agent.model,
      config: agent.config,
      enabled: agent.enabled,
    };
    const jsonStr = JSON.stringify(exportData, null, 2);
    navigator.clipboard.writeText(jsonStr);
    alert('Agent JSON 已复制到剪贴板');
  };

  // 导入 Agent
  const handleImport = async () => {
    if (!importJson.trim()) {
      alert('请输入 JSON 数据');
      return;
    }
    try {
      const config = JSON.parse(importJson);
      if (!config.name) {
        alert('JSON 数据缺少 name 字段');
        return;
      }
      setLoading(true);
      await agentApi.upload({
        name: config.name,
        description: config.description || '',
        icon: config.icon || 'Bot',
        model: config.model || '',
        config: typeof config.config === 'string' ? config.config : JSON.stringify(config.config || {}),
        enabled: config.enabled ?? true,
      });
      await loadAgents();
      setIsImportModalOpen(false);
      setImportJson('');
      alert('导入成功');
    } catch (err: any) {
      alert(err.message || '导入失败');
    } finally {
      setLoading(false);
    }
  };

  // 查看 Agent
  const handleView = (agent: Agent) => {
    setSelectedAgent(agent);
    setIsViewModalOpen(true);
  };

  const getAgentIcon = (iconName: string) => {
    const icons: Record<string, React.ElementType> = {
      Bot, Users, Sparkles, Brain, Zap, Workflow, MessageSquare, Globe, Terminal, Code
    };
    const Icon = icons[iconName] || Bot;
    return <Icon size={24} />;
  };

  return (
    <div className="p-8 max-w-7xl mx-auto">
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">{t('agents.title')}</h1>
          <p className="text-slate-500 mt-1">{t('agents.subtitle')}</p>
        </div>
      </div>

      <div className="flex items-center gap-4 mb-6">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 w-4 h-4" />
          <input
            type="text"
            placeholder={t('agents.search')}
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="w-full pl-10 pr-4 py-2 bg-white border border-slate-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 transition-all"
          />
        </div>
        <select
          value={filterStatus}
          onChange={(e) => setFilterStatus(e.target.value as any)}
          className="px-4 py-2 bg-white border border-slate-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 transition-all appearance-none"
        >
          <option value="all">{t('common.all')}</option>
          <option value="enabled">{t('common.enabled')}</option>
          <option value="disabled">{t('common.disabled')}</option>
          <option value="system">{t('agents.builtIn')}</option>
        </select>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {filteredAgents.map((agent) => (
          <div
            key={agent.id}
            className={cn(
              "bg-white border border-slate-200 rounded-2xl p-6 transition-all group relative",
              agent.enabled ? "shadow-sm hover:shadow-md" : "opacity-60 grayscale-[0.5]"
            )}
          >
            <div className="flex items-start justify-between mb-4">
              <div className="w-12 h-12 rounded-xl bg-slate-100 flex items-center justify-center text-slate-600 group-hover:bg-brand-50 group-hover:text-brand-500 transition-colors">
                {getAgentIcon(agent.icon)}
              </div>
              <div className="flex items-center gap-1">
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    // Handle play
                  }}
                  className="p-2 hover:bg-slate-100 rounded-lg text-slate-400 transition-colors"
                >
                  <Play size={16} className="fill-current" />
                </button>
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    handleView(agent);
                  }}
                  className="p-2 hover:bg-slate-100 rounded-lg text-slate-400 transition-colors"
                >
                  <ExternalLink size={16} />
                </button>
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    handleExport(agent);
                  }}
                  className="p-2 hover:bg-slate-100 rounded-lg text-slate-400 transition-colors"
                >
                  <Download size={16} />
                </button>
                {!agent.isBuiltIn && !agent.is_system && (
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      // Handle edit - navigate to orchestrator
                      onViewChange('orchestrator');
                    }}
                    className="p-2 hover:bg-slate-100 rounded-lg text-slate-400 transition-colors"
                  >
                    <Edit2 size={16} />
                  </button>
                )}
              </div>
            </div>

            <h3 className="font-bold text-slate-900 mb-1 flex items-center gap-2">
              {agent.name}
              {agent.isBuiltIn || agent.is_system ? (
                <span className="text-[10px] font-bold uppercase tracking-wider bg-slate-100 text-slate-500 px-1.5 py-0.5 rounded">{t('agents.builtIn')}</span>
              ) : null}
              {agent.isPeriodic ? (
                <span className="text-[10px] font-bold uppercase tracking-wider bg-amber-100 text-amber-600 px-1.5 py-0.5 rounded flex items-center gap-1">
                  <Clock size={10} />
                  {t('agents.periodic')}
                </span>
              ) : null}
            </h3>
            <p className="text-sm text-slate-500 line-clamp-2 mb-4">
              {agent.description}
            </p>

            <div className="flex flex-wrap gap-2 mb-4">
              {(() => {
                let channels = agent.channels;
                if (typeof channels === 'string' && channels) {
                  try { channels = JSON.parse(channels); } catch { channels = []; }
                }
                if (!Array.isArray(channels)) channels = [];
                return channels.map(channel => (
                  <span key={channel} className="text-[10px] font-bold uppercase tracking-wider bg-slate-100 text-slate-500 px-2 py-0.5 rounded flex items-center gap-1">
                    <Globe size={10} />
                    {channel}
                  </span>
                ));
              })()}
            </div>

            <div className="flex flex-wrap gap-2 mb-4">
              {agent.skills?.map(skill => (
                <span key={skill} className="text-[11px] font-medium bg-brand-50 text-brand-600 px-2 py-0.5 rounded-full">
                  {skill}
                </span>
              ))}
            </div>

            <div className="pt-4 border-t border-slate-100 flex items-center justify-between">
              <div className="flex items-center gap-2 text-xs text-slate-400">
                <Cpu size={14} />
                {getModelName(agent.model)}
              </div>
              <div className="flex items-center gap-1">
                {agent.isPeriodic && (
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      setSelectedAgentForLogs(agent);
                    }}
                    className="p-2 hover:bg-slate-50 rounded-lg text-slate-400 hover:text-brand-500 transition-colors"
                    title={t('agents.viewLogs')}
                  >
                    <Clock size={16} />
                  </button>
                )}
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    handleToggleEnabled(agent);
                  }}
                  className={cn(
                    "p-2 hover:bg-slate-50 rounded-lg transition-colors",
                    agent.enabled ? "text-slate-400 hover:text-orange-500" : "text-green-500 hover:text-green-600"
                  )}
                  title={agent.enabled ? '停用' : '启用'}
                >
                  <Power size={16} />
                </button>
                {!agent.isBuiltIn && !agent.is_system && (
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      handleDelete(agent);
                    }}
                    className="p-2 hover:bg-slate-50 rounded-lg text-slate-400 hover:text-red-500 transition-colors"
                    title={t('agents.delete')}
                  >
                    <Trash2 size={16} />
                  </button>
                )}
              </div>
            </div>
          </div>
        ))}
      </div>

      {/* Import Modal */}
      {isImportModalOpen && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white rounded-2xl w-[600px] shadow-2xl">
            <div className="p-6 border-b border-slate-100 flex items-center justify-between">
              <h2 className="text-lg font-bold text-slate-900">导入 Agent JSON</h2>
              <button
                onClick={() => setIsImportModalOpen(false)}
                className="text-slate-400 hover:text-slate-600 transition-all"
              >
                <X size={20} />
              </button>
            </div>
            <div className="p-6 space-y-4">
              <p className="text-sm text-slate-500">
                请粘贴 Agent 的 JSON 配置数据（参考 backend/runner/example/test-all.json 格式）
              </p>
              <textarea
                value={importJson}
                onChange={(e) => setImportJson(e.target.value)}
                className="w-full h-64 px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl text-sm font-mono focus:ring-2 focus:ring-brand-500/20 outline-none resize-none"
                placeholder={'{\n  "name": "Agent名称",\n  "description": "描述",\n  "icon": "Bot",\n  "model": "模型",\n  "config": {}\n}'}
              />
            </div>
            <div className="p-6 border-t border-slate-100 flex justify-end gap-3">
              <button
                onClick={() => setIsImportModalOpen(false)}
                className="px-6 py-2.5 bg-slate-100 text-slate-700 rounded-xl text-sm font-bold hover:bg-slate-200 transition-all"
              >
                取消
              </button>
              <button
                onClick={handleImport}
                disabled={loading}
                className="px-6 py-2.5 bg-brand-500 text-white rounded-xl text-sm font-bold hover:bg-brand-600 transition-all shadow-lg shadow-brand-500/20 disabled:opacity-50"
              >
                {loading ? '导入中...' : '确认导入'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* View Modal */}
      {isViewModalOpen && selectedAgent && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white rounded-2xl w-[600px] shadow-2xl max-h-[80vh] overflow-hidden flex flex-col">
            <div className="p-6 border-b border-slate-100 flex items-center justify-between shrink-0">
              <h2 className="text-lg font-bold text-slate-900">{t('agents.viewAgent')}</h2>
              <button
                onClick={() => setIsViewModalOpen(false)}
                className="text-slate-400 hover:text-slate-600 transition-all"
              >
                <X size={20} />
              </button>
            </div>
            <div className="flex-1 overflow-y-auto p-6 space-y-6">
              <div className="flex items-center gap-4">
                <div className="w-16 h-16 rounded-xl bg-slate-100 flex items-center justify-center text-slate-600">
                  {getAgentIcon(selectedAgent.icon)}
                </div>
                <div>
                  <h3 className="font-bold text-slate-900 text-lg">{selectedAgent.name}</h3>
                  <p className="text-sm text-slate-500">{selectedAgent.description}</p>
                </div>
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div className="p-4 bg-slate-50 rounded-xl">
                  <label className="text-[10px] font-bold text-slate-400 uppercase tracking-wider">{t('agents.icon')}</label>
                  <p className="text-sm text-slate-900 font-medium">{selectedAgent.icon}</p>
                </div>
                <div className="p-4 bg-slate-50 rounded-xl">
                  <label className="text-[10px] font-bold text-slate-400 uppercase tracking-wider">{t('agents.model')}</label>
                  <p className="text-sm text-slate-900 font-medium">{selectedAgent.model}</p>
                </div>
                <div className="p-4 bg-slate-50 rounded-xl">
                  <label className="text-[10px] font-bold text-slate-400 uppercase tracking-wider">{t('agents.status')}</label>
                  <p className={cn("text-sm font-medium", selectedAgent.enabled ? "text-green-600" : "text-slate-500")}>
                    {selectedAgent.enabled ? t('agents.enabled') : t('agents.disabled')}
                  </p>
                </div>
                <div className="p-4 bg-slate-50 rounded-xl">
                  <label className="text-[10px] font-bold text-slate-400 uppercase tracking-wider">{t('agents.type')}</label>
                  <p className="text-sm text-slate-900 font-medium">
                    {selectedAgent.is_system || selectedAgent.isBuiltIn ? t('agents.systemBuiltIn') : t('agents.userCreated')}
                  </p>
                </div>
              </div>

              {selectedAgent.config && (() => {
                const cfg = parseAgentConfig(selectedAgent.config);
                const cfgJson = parseConfigJson(selectedAgent.config_json);
                if (!cfg) return null;
                return (
                  <div className="space-y-4">
                    <label className="text-[10px] font-bold text-slate-400 uppercase tracking-wider">{t('agents.relatedConfig')}</label>

                    {/* System Prompt */}
                    <div className="p-3 bg-slate-50 rounded-xl">
                      <label className="text-[9px] font-bold text-slate-400 uppercase">{t('agents.systemPrompt')}</label>
                      <p className="text-xs text-slate-700 mt-1 whitespace-pre-wrap">{cfg.systemPrompt}</p>
                    </div>

                    {/* Models */}
                    {cfg.models && (
                      <div className="p-3 bg-slate-50 rounded-xl">
                        <label className="text-[9px] font-bold text-slate-400 uppercase">{t('agents.modelConfig')}</label>
                        <div className="grid grid-cols-2 gap-2 mt-2">
                          <div><span className="text-[9px] text-slate-500">{t('agents.default')}:</span> <span className="text-xs text-slate-700">{cfg.models.default}</span></div>
                          <div><span className="text-[9px] text-slate-500">{t('agents.rewrite')}:</span> <span className="text-xs text-slate-700">{cfg.models.rewrite}</span></div>
                          <div><span className="text-[9px] text-slate-500">{t('agents.skill')}:</span> <span className="text-xs text-slate-700">{cfg.models.skill}</span></div>
                          <div><span className="text-[9px] text-slate-500">{t('agents.summarize')}:</span> <span className="text-xs text-slate-700">{cfg.models.summarize}</span></div>
                        </div>
                      </div>
                    )}

                    {/* Skills & KBs */}
                    <div className="grid grid-cols-2 gap-3">
                      <div className="p-3 bg-slate-50 rounded-xl">
                        <label className="text-[9px] font-bold text-slate-400 uppercase">{t('agents.skills')} ({selectedAgent.skills?.length || 0})</label>
                        <p className="text-xs text-slate-700 mt-1 break-all">{cfg.selectedSkills}</p>
                      </div>
                      <div className="p-3 bg-slate-50 rounded-xl">
                        <label className="text-[9px] font-bold text-slate-400 uppercase">{t('agents.knowledgeBases')} ({selectedAgent.knowledgeBases?.length || 0})</label>
                        <p className="text-xs text-slate-700 mt-1 break-all">{cfg.selectedKBs}</p>
                      </div>
                    </div>

                    {/* Basic Config */}
                    <div className="grid grid-cols-3 gap-3">
                      <div className="p-3 bg-slate-50 rounded-xl text-center">
                        <label className="text-[9px] font-bold text-slate-400 uppercase">{t('agents.temperature')}</label>
                        <p className="text-sm text-slate-700 font-medium mt-1">{cfg.temperature}</p>
                      </div>
                      <div className="p-3 bg-slate-50 rounded-xl text-center">
                        <label className="text-[9px] font-bold text-slate-400 uppercase">{t('agents.maxTokens')}</label>
                        <p className="text-sm text-slate-700 font-medium mt-1">{cfg.maxTokens}</p>
                      </div>
                      <div className="p-3 bg-slate-50 rounded-xl text-center">
                        <label className="text-[9px] font-bold text-slate-400 uppercase">{t('agents.topK')}</label>
                        <p className="text-sm text-slate-700 font-medium mt-1">{cfg.topK}</p>
                      </div>
                    </div>

                    {/* Sandbox & Stream */}
                    <div className="grid grid-cols-2 gap-3">
                      <div className="p-3 bg-slate-50 rounded-xl">
                        <label className="text-[9px] font-bold text-slate-400 uppercase">{t('agents.sandbox')}</label>
                        <p className="text-xs text-slate-700 mt-1">{cfg.sandbox}</p>
                      </div>
                      <div className="p-3 bg-slate-50 rounded-xl">
                        <label className="text-[9px] font-bold text-slate-400 uppercase">{t('agents.periodicTask')}</label>
                        <p className="text-xs text-slate-700 mt-1">{cfg.isPeriodic} {cfg.cronRule !== '-' ? `(${cfg.cronRule})` : ''}</p>
                      </div>
                    </div>

                    {/* Runnable Config */}
                    {cfgJson && (
                      <>
                        <div className="border-t border-slate-200 pt-4">
                          <label className="text-[10px] font-bold text-brand-500 uppercase tracking-wider">{t('agents.runnableConfig')}</label>
                        </div>

                        {/* Models */}
                        {Object.keys(cfgJson.models).length > 0 && (
                          <div className="p-3 bg-brand-50 rounded-xl">
                            <label className="text-[9px] font-bold text-brand-600 uppercase">{t('agents.modelConfig')}</label>
                            <div className="grid grid-cols-2 gap-2 mt-2">
                              {Object.entries(cfgJson.models).map(([key, model]: [string, any]) => (
                                <div key={key}><span className="text-[9px] text-slate-500">{key}:</span> <span className="text-xs text-slate-700">{model.provider}/{model.name}</span></div>
                              ))}
                            </div>
                          </div>
                        )}

                        {/* Tools/MCP/A2A/Skills Summary */}
                        <div className="grid grid-cols-2 gap-3">
                          {(cfgJson.toolsCount > 0 || cfgJson.mcpsCount > 0 || cfgJson.a2aCount > 0) && (
                            <>
                              {cfgJson.toolsCount > 0 && (
                                <div className="p-3 bg-brand-50 rounded-xl">
                                  <label className="text-[9px] font-bold text-brand-600 uppercase">{t('agents.tools')} ({cfgJson.toolsCount})</label>
                                  <p className="text-xs text-slate-700 mt-1">{cfgJson.tools}</p>
                                </div>
                              )}
                              {cfgJson.mcpsCount > 0 && (
                                <div className="p-3 bg-brand-50 rounded-xl">
                                  <label className="text-[9px] font-bold text-brand-600 uppercase">{t('agents.mcp')} ({cfgJson.mcpsCount})</label>
                                  <p className="text-xs text-slate-700 mt-1">{cfgJson.mcps}</p>
                                </div>
                              )}
                              {cfgJson.a2aCount > 0 && (
                                <div className="p-3 bg-brand-50 rounded-xl">
                                  <label className="text-[9px] font-bold text-brand-600 uppercase">{t('agents.a2a')} ({cfgJson.a2aCount})</label>
                                  <p className="text-xs text-slate-700 mt-1">{cfgJson.a2a}</p>
                                </div>
                              )}
                            </>
                          )}
                          {cfgJson.skillsCount > 0 && (
                            <div className="p-3 bg-brand-50 rounded-xl">
                              <label className="text-[9px] font-bold text-brand-600 uppercase">{t('skills.skillsTab')} ({cfgJson.skillsCount})</label>
                              <p className="text-xs text-slate-700 mt-1">{cfgJson.skills}</p>
                            </div>
                          )}
                          {cfgJson.knowledgeCount > 0 && (
                            <div className="p-3 bg-brand-50 rounded-xl">
                              <label className="text-[9px] font-bold text-brand-600 uppercase">{t('agents.knowledge')} ({cfgJson.knowledgeCount})</label>
                              <p className="text-xs text-slate-700 mt-1">{cfgJson.knowledge}</p>
                            </div>
                          )}
                        </div>
                      </>
                    )}

                    {/* Raw JSON Toggle */}
                    <details className="cursor-pointer">
                      <summary className="text-[10px] font-bold text-slate-400 uppercase tracking-wider">{t('agents.rawJson')}</summary>
                      <pre className="w-full h-48 mt-2 px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl text-xs font-mono overflow-auto whitespace-pre-wrap text-slate-700">
                        {typeof selectedAgent.config === 'string' ? selectedAgent.config : JSON.stringify(selectedAgent.config, null, 2)}
                      </pre>
                    </details>

                    {selectedAgent.config_json && (
                      <details className="cursor-pointer">
                        <summary className="text-[10px] font-bold text-brand-500 uppercase tracking-wider">{t('agents.runnableJson')}</summary>
                        <pre className="w-full h-64 mt-2 px-4 py-3 bg-brand-50 border border-brand-200 rounded-xl text-xs font-mono overflow-auto whitespace-pre-wrap text-slate-700">
                          {typeof selectedAgent.config_json === 'string' ? selectedAgent.config_json : JSON.stringify(selectedAgent.config_json, null, 2)}
                        </pre>
                      </details>
                    )}
                  </div>
                );
              })()}
            </div>
            <div className="p-6 border-t border-slate-100 flex justify-end">
              <button
                onClick={() => {
                  handleExport(selectedAgent);
                }}
                className="px-6 py-2.5 bg-slate-100 text-slate-700 rounded-xl text-sm font-bold hover:bg-slate-200 transition-all flex items-center gap-2"
              >
                <Copy size={16} />
                {t('agents.copyJson')}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Logs Modal */}
      <AnimatePresence>
        {selectedAgentForLogs && (
          <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              onClick={() => setSelectedAgentForLogs(null)}
              className="absolute inset-0 bg-slate-900/40 backdrop-blur-sm"
            />
            <motion.div
              initial={{ opacity: 0, scale: 0.95, y: 20 }}
              animate={{ opacity: 1, scale: 1, y: 0 }}
              exit={{ opacity: 0, scale: 0.95, y: 20 }}
              className="relative w-full max-w-2xl bg-white rounded-2xl shadow-2xl overflow-hidden flex flex-col max-h-[80vh]"
            >
              <div className="p-6 border-b border-slate-100 flex items-center justify-between shrink-0">
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 rounded-xl bg-amber-50 text-amber-500 flex items-center justify-center">
                    <Clock size={20} />
                  </div>
                  <div>
                    <h3 className="text-lg font-bold text-slate-900">{selectedAgentForLogs.name} - {t('agents.logsTitle')}</h3>
                    <p className="text-xs text-slate-500 font-medium uppercase tracking-wider">{t('orchestrator.cronRule')}: {selectedAgentForLogs.cronRule}</p>
                  </div>
                </div>
                <button
                  onClick={() => setSelectedAgentForLogs(null)}
                  className="p-2 hover:bg-slate-100 rounded-lg text-slate-400 transition-colors"
                >
                  <X size={20} />
                </button>
              </div>

              <div className="flex-1 overflow-y-auto p-6 space-y-4 scrollbar-hide">
                {!selectedAgentForLogs.logs || selectedAgentForLogs.logs.length === 0 ? (
                  <div className="py-12 text-center">
                    <AlertCircle size={32} className="mx-auto text-slate-300 mb-3" />
                    <p className="text-sm text-slate-500">{t('agents.noLogs')}</p>
                  </div>
                ) : (
                  selectedAgentForLogs.logs.map((log) => (
                    <div key={log.id} className="flex gap-4 p-4 rounded-xl border border-slate-100 bg-slate-50/50">
                      <div className={cn(
                        "w-8 h-8 rounded-full flex items-center justify-center shrink-0",
                        log.status === 'success' ? "bg-green-50 text-green-500" :
                        log.status === 'failed' ? "bg-red-50 text-red-500" :
                        "bg-blue-50 text-blue-500"
                      )}>
                        {log.status === 'success' && <CheckCircle2 size={16} />}
                        {log.status === 'failed' && <XCircle size={16} />}
                        {log.status === 'running' && <div className="w-4 h-4 border-2 border-blue-500 border-t-transparent rounded-full animate-spin" />}
                      </div>
                      <div className="flex-1">
                        <div className="flex items-center justify-between mb-1">
                          <span className={cn(
                            "text-[10px] font-bold uppercase tracking-wider",
                            log.status === 'success' ? "text-green-600" :
                            log.status === 'failed' ? "text-red-600" :
                            "text-blue-600"
                          )}>
                            {log.status}
                          </span>
                          <span className="text-[10px] font-medium text-slate-400">{log.timestamp.toLocaleString()}</span>
                        </div>
                        <p className="text-sm text-slate-700">{log.message}</p>
                        {log.duration && (
                          <div className="mt-2 flex items-center gap-1 text-[10px] font-bold text-slate-400 uppercase tracking-widest">
                            <Clock size={10} />
                            Duration: {log.duration}
                          </div>
                        )}
                      </div>
                    </div>
                  ))
                )}
              </div>
            </motion.div>
          </div>
        )}
      </AnimatePresence>
    </div>
  );
}
