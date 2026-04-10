import React, { useState, useRef, useEffect, useCallback } from 'react';
import {
  Zap,
  Box,
  Code,
  Globe,
  Database,
  Plus,
  Settings2,
  X,
  Upload,
  Trash2,
  Power,
  Eye,
  Terminal,
  Link,
  Wrench,
  ChevronDown,
  Info,
  ShieldAlert,
  Loader2,
  AlertCircle
} from 'lucide-react';
import { motion, AnimatePresence } from 'motion/react';
import { cn } from '../lib/utils';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Skill } from '../types';
import { skillApi, Skill as BackendSkill } from '../lib/api';

interface SkillManagerProps {
  initialTab?: 'skills' | 'a2a' | 'tools' | 'mcp';
}

export function SkillManager({ initialTab = 'skills' }: SkillManagerProps) {
  const { t } = useTranslation();
  const [activeTab, setActiveTab] = useState<'skills' | 'a2a' | 'tools' | 'mcp'>(initialTab);
  const [isAddModalOpen, setIsAddModalOpen] = useState(false);
  const [isViewModalOpen, setIsViewModalOpen] = useState(false);
  const [selectedSkill, setSelectedSkill] = useState<Skill | null>(null);
  const [loading, setLoading] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const [skills, setSkills] = useState<Skill[]>([]);

  // Confirmation dialog state
  const [confirmDialog, setConfirmDialog] = useState<{
    open: boolean;
    title: string;
    message: string;
    onConfirm: () => void;
  }>({ open: false, title: '', message: '', onConfirm: () => {} });

  // 从后端加载 skills
  const loadSkills = useCallback(async () => {
    try {
      setLoading(true);
      const data = await skillApi.findAll();
      // 将后端数据映射到前端 Skill 类型
      const mapped: Skill[] = data.map((s: BackendSkill) => ({
        id: s.ulid,
        name: s.name,
        description: s.description || '',
        type: (s.skill_type || 'custom') as Skill['type'],
        category: (s.skill_type || 'tool') as Skill['category'],
        enabled: s.enabled ?? true,
        is_system: s.is_system ?? false,
        icon: s.skill_type === 'mcp' ? 'Terminal' : s.skill_type === 'a2a' ? 'Link' : 'Wrench',
        mcpUrl: s.skill_type === 'mcp' ? s.path : undefined,
        endpoint: s.skill_type === 'a2a' ? s.path : undefined,
        riskLevel: (s.risk_level as 'low' | 'medium' | 'high') || 'low',
      }));
      setSkills(mapped);
      console.log('Skills loaded:', mapped.length, 'types:', [...new Set(mapped.map(s => s.type))]);
    } catch (err) {
      console.error('Failed to load skills:', err);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadSkills();
  }, [loadSkills]);

  // 通用的错误提示函数，显示 message + cause
  const showError = (err: any, defaultMsg: string) => {
    const message = err?.message || defaultMsg;
    const cause = err?.response?.data?.cause || '';
    const fullMessage = cause ? `${message}\n${cause}` : message;
    toast.error(fullMessage);
  };

  const handleUploadZip = () => {
    fileInputRef.current?.click();
  };

  const handleFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    try {
      setLoading(true);
      await skillApi.upload(file);
      await loadSkills();
    } catch (err: any) {
      console.error('Failed to upload skill:', err);
      toast.error(err.message || 'Upload failed');
    } finally {
      setLoading(false);
      if (fileInputRef.current) fileInputRef.current.value = '';
    }
  };

  const [formData, setFormData] = useState<Partial<Skill & { mcpMode: 'remote' | 'local'; description: string }>>({
    name: '',
    description: '',
    content: '',
    mcpUrl: '',
    mcpMode: 'remote',
    command: '',
    args: [],
    env: {},
    endpoint: '',
    token: '',
    sandboxEndpoint: '',
    method: 'POST',
    timeout: 5000,
    riskLevel: 'low',
    sandboxToken: ''
  });

  const handleAdd = async () => {
    try {
      setLoading(true);
      // 转换 activeTab 为后端接受的 skillType
      const skillTypeMap: Record<string, 'mcp' | 'tool' | 'a2a' | 'skill'> = {
        'mcp': 'mcp',
        'a2a': 'a2a',
        'skills': 'skill',
        'tools': 'tool'
      };
      const skillType = skillTypeMap[activeTab] || 'tool';
      console.log('Creating skill - activeTab:', activeTab, 'skillType:', skillType);
      // 调用后端 API 创建 skill
      const result = await skillApi.create({
        name: formData.name || 'Unnamed',
        description: formData.description || activeTab === 'skills' ? 'Custom user defined skill' : `${activeTab.toUpperCase()} service`,
        skillType: skillType,
        path: formData.mcpUrl || formData.endpoint || '',
        enabled: true,
      });
      console.log('Created skill:', result);
      await loadSkills();
      setIsAddModalOpen(false);
      resetForm();
    } catch (err: any) {
      console.error('Failed to create skill:', err);
      showError(err, 'Failed to create skill');
    } finally {
      setLoading(false);
    }
  };

  const resetForm = () => {
    setFormData({
      name: '',
      description: '',
      content: '',
      mcpUrl: '',
      mcpMode: 'remote',
      command: '',
      args: [],
      env: {},
      endpoint: '',
      token: '',
      sandboxEndpoint: '',
      method: 'POST',
      timeout: 5000,
      riskLevel: 'low',
      sandboxToken: ''
    });
  };

  const toggleSkill = (id: string) => {
    setSkills(skills.map(s => s.id === id ? { ...s, enabled: !s.enabled } : s));
  };

  const deleteSkill = (id: string, name: string) => {
    setConfirmDialog({
      open: true,
      title: t('skills.confirmDeleteTitle') || '确认删除',
      message: t('skills.confirmDeleteMessage', { name }) || `确定要删除技能 "${name}" 吗？`,
      onConfirm: async () => {
        try {
          setLoading(true);
          await skillApi.delete(id);
          setConfirmDialog(prev => ({ ...prev, open: false }));
          await loadSkills();
          toast.success(t('skills.deleteSuccess') || '删除成功');
        } catch (err: any) {
          setConfirmDialog(prev => ({ ...prev, open: false }));
          console.error('Failed to delete skill:', err);
          showError(err, 'Delete failed');
        } finally {
          setLoading(false);
        }
      }
    });
  };

  const viewSkill = (skill: Skill) => {
    setSelectedSkill(skill);
    setIsViewModalOpen(true);
  };

  const getIcon = (iconName?: string) => {
    switch (iconName) {
      case 'Globe': return Globe;
      case 'Code': return Code;
      case 'Database': return Database;
      case 'Terminal': return Terminal;
      case 'Link': return Link;
      case 'Wrench': return Wrench;
      default: return Zap;
    }
  };

  const filteredSkills = skills.filter(s => {
    if (activeTab === 'mcp') return s.type === 'mcp';
    if (activeTab === 'a2a') return s.type === 'a2a';
    if (activeTab === 'tools') return s.type === 'tool';
    return s.type === 'built-in' || s.type === 'custom' || s.type === 'skill';
  });

  console.log('Filtering - activeTab:', activeTab, 'total:', skills.length, 'filtered:', filteredSkills.length);

  const enabledCount = filteredSkills.filter(s => s.enabled).length;

  return (
    <div className="p-8 max-w-7xl mx-auto">
      <div className="flex items-center justify-between mb-8">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-lg bg-orange-50 flex items-center justify-center text-orange-500">
            <Zap size={24} />
          </div>
          <div>
            <h1 className="text-2xl font-bold text-slate-900">
              {activeTab === 'mcp' ? 'mcp' : activeTab === 'a2a' ? 'A2A' : activeTab === 'tools' ? 'tool' : t('skills.title')}
            </h1>
            <p className="text-slate-500 text-sm">
              {activeTab === 'mcp' ? t('skills.mcpAutoLoad') : t('skills.subtitle')}
            </p>
          </div>
        </div>

        <div className="flex items-center gap-3">
          <div className="px-4 py-2 bg-slate-50 border border-slate-200 rounded-xl flex flex-col items-center justify-center min-w-[80px]">
            <span className="text-lg font-bold text-slate-900 leading-none">{enabledCount}</span>
            <span className="text-[10px] font-bold text-slate-400 uppercase mt-1">Enabled</span>
          </div>

          {activeTab === 'skills' && (
            <>
              <input
                type="file"
                ref={fileInputRef}
                onChange={handleFileChange}
                accept=".zip"
                className="hidden"
              />
              <button
                onClick={handleUploadZip}
                className="flex items-center gap-2 px-6 py-3 border border-slate-200 rounded-2xl font-bold text-slate-700 hover:bg-slate-50 transition-all"
              >
                <Upload size={20} />
                {t('skills.uploadZip')}
              </button>
            </>
          )}

          <button
            onClick={() => setIsAddModalOpen(true)}
            className="flex items-center gap-2 bg-slate-900 hover:bg-slate-800 text-white px-8 py-3 rounded-2xl font-bold transition-all shadow-sm"
          >
            <Plus size={20} />
            {t('skills.add')}
          </button>
        </div>
      </div>

      <div className="flex gap-1 p-1 bg-slate-100 rounded-xl mb-8 w-fit">
        {(['skills', 'mcp', 'tools', 'a2a'] as const).map((tab) => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab)}
            className={cn(
              "px-6 py-2 rounded-lg text-sm font-bold transition-all",
              activeTab === tab ? "bg-white text-slate-900 shadow-sm" : "text-slate-500 hover:text-slate-700"
            )}
          >
            {t(`skills.${tab}Tab`)}
          </button>
        ))}
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {filteredSkills.map((skill) => {
          const Icon = getIcon(skill.icon);
          return (
            <div key={skill.id} className={cn(
              "bg-white border border-slate-200 rounded-2xl p-6 hover:shadow-md transition-all group relative",
              !skill.enabled && "opacity-60 grayscale-[0.5]"
            )}>
              <div className="flex items-start justify-between mb-4">
                <div className={cn(
                  "w-12 h-12 rounded-xl flex items-center justify-center",
                  skill.enabled ? "bg-brand-50 text-brand-500" : "bg-slate-100 text-slate-400"
                )}>
                  <Icon size={24} />
                </div>
                <div className="flex items-center gap-2">
                  <div className={cn(
                    "flex items-center gap-1 px-2 py-0.5 rounded-full text-[9px] font-bold uppercase tracking-wider",
                    skill.riskLevel === 'high' ? "bg-red-50 text-red-500 border border-red-100" :
                      skill.riskLevel === 'medium' ? "bg-amber-50 text-amber-500 border border-amber-100" :
                        "bg-emerald-50 text-emerald-500 border border-emerald-100"
                  )}>
                    <ShieldAlert size={10} />
                    {skill.riskLevel ? t(`orchestrator.riskLevels.${skill.riskLevel}`) : t('orchestrator.riskLevels.low')}
                  </div>
                  <div className={cn("w-2 h-2 rounded-full", skill.enabled ? "bg-green-500" : "bg-slate-300")} />
                  <span className="text-[10px] font-bold text-slate-400 uppercase tracking-wider">
                    {skill.enabled ? 'Active' : 'Inactive'}
                  </span>
                </div>
              </div>
              <h3 className="font-bold text-slate-900 mb-1">{skill.name}</h3>
              <p className="text-sm text-slate-500 mb-4 line-clamp-2">
                {skill.description}
              </p>

              {(skill.mcpUrl || skill.endpoint) && (
                <div className="mb-4 p-2 bg-slate-50 rounded-lg border border-slate-100 font-mono text-[10px] text-slate-500 truncate">
                  {skill.mcpUrl || skill.endpoint}
                </div>
              )}

              <div className="flex items-center justify-between pt-4 border-t border-slate-50">
                <span className="text-xs font-medium text-slate-400">{t('skills.usedBy', { count: 12 })}</span>
                <div className="flex items-center gap-1">
                  <button
                    onClick={() => viewSkill(skill)}
                    className="p-2 hover:bg-slate-50 rounded-lg text-slate-400 hover:text-brand-500 transition-colors"
                    title={t('skills.view')}
                  >
                    <Eye size={16} />
                  </button>
                  <button
                    onClick={() => toggleSkill(skill.id)}
                    className={cn(
                      "p-2 hover:bg-slate-50 rounded-lg transition-colors",
                      skill.enabled ? "text-slate-400 hover:text-orange-500" : "text-green-500 hover:text-green-600"
                    )}
                    title={skill.enabled ? t('skills.stop') : t('skills.start')}
                  >
                    <Power size={16} />
                  </button>
                  {!skill.is_system && (
                    <button
                      onClick={() => deleteSkill(skill.id, skill.name)}
                      className="p-2 hover:bg-slate-50 rounded-lg text-slate-400 hover:text-red-500 transition-colors"
                      title={t('skills.delete')}
                    >
                      <Trash2 size={16} />
                    </button>
                  )}
                </div>
              </div>
            </div>
          );
        })}

        {loading && (
          <div className="flex items-center justify-center py-12 col-span-3">
            <Loader2 className="animate-spin text-brand-500" size={32} />
          </div>
        )}

        {!loading && filteredSkills.length === 0 && (
          <div className="col-span-3 text-center py-12 text-slate-400">
            <Box className="mx-auto mb-3 opacity-30" size={48} />
            <p>{t('skills.noSkills', 'No skills found')}</p>
          </div>
        )}
      </div>

      {/* Add Modal */}
      {isAddModalOpen && (
        <div className="fixed inset-0 bg-slate-900/50 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className={cn(
            "bg-white rounded-2xl w-full shadow-2xl overflow-hidden flex flex-col max-h-[90vh]",
            activeTab === 'tools' ? "max-w-xl" : "max-w-3xl"
          )}>
            <div className="p-6 border-b border-slate-100 flex items-center justify-between shrink-0">
              <h2 className="text-xl font-bold text-slate-900">
                {activeTab === 'skills' ? t('skills.createSkill') :
                  activeTab === 'mcp' ? t('skills.createMcp') :
                    activeTab === 'a2a' ? t('skills.createA2A') :
                      t('skills.createTool')}
              </h2>
              <button onClick={() => setIsAddModalOpen(false)} className="text-slate-400 hover:text-slate-600">
                <X size={24} />
              </button>
            </div>

            <div className="p-8 space-y-6 max-h-[70vh] overflow-y-auto">
              {/* Common Name Field */}
              <div>
                <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-2">
                  {activeTab === 'skills' ? t('skills.skillName') :
                    activeTab === 'mcp' ? t('skills.mcpName') :
                      activeTab === 'a2a' ? t('skills.a2aName') :
                        t('skills.toolName')} *
                </label>
                <input
                  type="text"
                  value={formData.name}
                  onChange={e => setFormData({ ...formData, name: e.target.value })}
                  placeholder={activeTab === 'skills' ? t('skills.placeholderName') :
                    activeTab === 'mcp' ? t('skills.placeholderMcpName') :
                      activeTab === 'a2a' ? t('skills.placeholderA2AName') :
                        t('skills.placeholderToolName')}
                  className="w-full px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 outline-none transition-all font-medium"
                />
              </div>

              {/* Common Risk Level Field */}
              <div>
                <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-2">
                  {t('skills.toolRisk')}
                </label>
                <div className="relative">
                  <select
                    value={formData.riskLevel}
                    onChange={e => setFormData({ ...formData, riskLevel: e.target.value as any })}
                    className="w-full px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 outline-none transition-all font-medium appearance-none"
                  >
                    <option value="low">{t('orchestrator.riskLevels.low')}</option>
                    <option value="medium">{t('orchestrator.riskLevels.medium')}</option>
                    <option value="high">{t('orchestrator.riskLevels.high')}</option>
                  </select>
                  <ChevronDown className="absolute right-4 top-1/2 -translate-y-1/2 text-slate-400 pointer-events-none" size={18} />
                </div>
              </div>

              {/* Skills Specific */}
              {activeTab === 'skills' && (
                <div>
                  <div className="flex items-center justify-between mb-2">
                    <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider">
                      {t('skills.skillContent')} *
                    </label>
                    <span className="text-[10px] text-slate-400 bg-slate-100 px-2 py-0.5 rounded">
                      {t('skills.frontmatterSupport')}
                    </span>
                  </div>
                  <textarea
                    value={formData.content}
                    onChange={e => setFormData({ ...formData, content: e.target.value })}
                    placeholder={t('skills.placeholderContent')}
                    className="w-full h-64 px-4 py-4 bg-slate-50 border border-slate-200 rounded-xl focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 outline-none transition-all font-mono text-sm leading-relaxed resize-none"
                  />
                </div>
              )}

              {/* MCP Specific */}
              {activeTab === 'mcp' && (
                <>
                  <div>
                    <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-2">
                      {t('skills.description')}
                    </label>
                    <textarea
                      value={formData.description}
                      onChange={e => setFormData({ ...formData, description: e.target.value })}
                      placeholder="MCP connector description"
                      className="w-full h-20 px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 outline-none transition-all text-sm resize-none"
                    />
                  </div>

                  <div>
                    <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-2">
                      {t('skills.mcpMode')}
                    </label>
                    <div className="flex gap-2 p-1 bg-slate-100 rounded-xl">
                      <button
                        onClick={() => setFormData({ ...formData, mcpMode: 'remote' })}
                        className={cn(
                          "flex-1 py-2 rounded-lg text-xs font-bold transition-all",
                          formData.mcpMode === 'remote' ? "bg-white text-slate-900 shadow-sm" : "text-slate-500 hover:text-slate-700"
                        )}
                      >
                        {t('skills.mcpModeRemote')}
                      </button>
                      <button
                        onClick={() => setFormData({ ...formData, mcpMode: 'local' })}
                        className={cn(
                          "flex-1 py-2 rounded-lg text-xs font-bold transition-all",
                          formData.mcpMode === 'local' ? "bg-white text-slate-900 shadow-sm" : "text-slate-500 hover:text-slate-700"
                        )}
                      >
                        {t('skills.mcpModeLocal')}
                      </button>
                    </div>
                  </div>

                  {formData.mcpMode === 'remote' ? (
                    <>
                      <div>
                        <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-2">
                          {t('skills.mcpEndpoint')} *
                        </label>
                        <input
                          type="text"
                          value={formData.mcpUrl}
                          onChange={e => setFormData({ ...formData, mcpUrl: e.target.value })}
                          placeholder={t('skills.placeholderMcpEndpoint')}
                          className="w-full px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 outline-none transition-all font-mono text-sm"
                        />
                      </div>
                      <div>
                        <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-2">
                          {t('skills.mcpToken')}
                        </label>
                        <input
                          type="password"
                          value={formData.token}
                          onChange={e => setFormData({ ...formData, token: e.target.value })}
                          placeholder="Bearer Token"
                          className="w-full px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 outline-none transition-all font-mono text-sm"
                        />
                      </div>
                    </>
                  ) : (
                    <>
                      <div>
                        <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-2">
                          {t('skills.mcpCommand')} *
                        </label>
                        <input
                          type="text"
                          value={formData.command}
                          onChange={e => setFormData({ ...formData, command: e.target.value })}
                          placeholder={t('skills.placeholderMcpCommand')}
                          className="w-full px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 outline-none transition-all font-mono text-sm"
                        />
                      </div>
                      <div>
                        <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-2">
                          {t('skills.mcpArgs')}
                        </label>
                        <input
                          type="text"
                          value={formData.args?.join(', ')}
                          onChange={e => setFormData({ ...formData, args: e.target.value.split(',').map(s => s.trim()).filter(s => s !== '') })}
                          placeholder={t('skills.placeholderMcpArgs')}
                          className="w-full px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 outline-none transition-all font-mono text-sm"
                        />
                      </div>
                      <div>
                        <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-2">
                          {t('skills.mcpEnv')}
                        </label>
                        <textarea
                          value={JSON.stringify(formData.env, null, 2)}
                          onChange={e => {
                            try {
                              setFormData({ ...formData, env: JSON.parse(e.target.value) });
                            } catch (err) {
                              // Ignore invalid JSON during typing
                            }
                          }}
                          placeholder={t('skills.placeholderMcpEnv')}
                          className="w-full h-32 px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 outline-none transition-all font-mono text-sm"
                        />
                      </div>
                    </>
                  )}
                </>
              )}

              {/* A2A Specific */}
              {activeTab === 'a2a' && (
                <>
                  <div>
                    <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-2">
                      {t('skills.description')}
                    </label>
                    <textarea
                      value={formData.description}
                      onChange={e => setFormData({ ...formData, description: e.target.value })}
                      placeholder="A2A service description"
                      className="w-full h-20 px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 outline-none transition-all text-sm resize-none"
                    />
                  </div>

                  <div>
                    <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-2">
                      {t('skills.a2aEndpoint')} *
                    </label>
                    <input
                      type="text"
                      value={formData.endpoint}
                      onChange={e => setFormData({ ...formData, endpoint: e.target.value })}
                      placeholder={t('skills.placeholderA2AEndpoint')}
                      className="w-full px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 outline-none transition-all font-mono text-sm"
                    />
                  </div>
                  <div>
                    <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-2">
                      {t('skills.a2aToken')}
                    </label>
                    <input
                      type="password"
                      value={formData.token}
                      onChange={e => setFormData({ ...formData, token: e.target.value })}
                      placeholder="xxxxxx"
                      className="w-full px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 outline-none transition-all font-mono text-sm"
                    />
                  </div>
                </>
              )}

              {/* Tool Specific */}
              {activeTab === 'tools' && (
                <div className="space-y-4">
                  <div>
                    <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-2">
                      {t('skills.description')}
                    </label>
                    <textarea
                      value={formData.description}
                      onChange={e => setFormData({ ...formData, description: e.target.value })}
                      placeholder="Tool description"
                      className="w-full h-20 px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 outline-none transition-all text-sm resize-none"
                    />
                  </div>

                  <div>
                    <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-1">
                      {t('skills.toolEndpoint')}
                    </label>
                    <input
                      type="text"
                      value={formData.endpoint}
                      onChange={e => setFormData({ ...formData, endpoint: e.target.value })}
                      placeholder={t('skills.placeholderToolEndpoint')}
                      className="w-full px-3 py-2 bg-slate-50 border border-slate-200 rounded-lg focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 outline-none transition-all text-sm"
                    />
                  </div>
                  <div>
                    <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-1">
                      {t('skills.toolToken')}
                    </label>
                    <input
                      type="password"
                      value={formData.token}
                      onChange={e => setFormData({ ...formData, token: e.target.value })}
                      placeholder="Bearer Token"
                      className="w-full px-3 py-2 bg-slate-50 border border-slate-200 rounded-lg focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 outline-none transition-all text-sm"
                    />
                  </div>
                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-1">
                        {t('skills.toolMethod')}
                      </label>
                      <div className="relative">
                        <select
                          value={formData.method}
                          onChange={e => setFormData({ ...formData, method: e.target.value as any })}
                          className="w-full px-3 py-2 bg-slate-50 border border-slate-200 rounded-lg focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 outline-none transition-all text-sm appearance-none"
                        >
                          <option value="POST">POST</option>
                          <option value="GET">GET</option>
                        </select>
                        <ChevronDown className="absolute right-3 top-1/2 -translate-y-1/2 text-slate-400 pointer-events-none" size={16} />
                      </div>
                    </div>
                    <div>
                      <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-1">
                        {t('skills.toolTimeout')}
                      </label>
                      <input
                        type="number"
                        value={formData.timeout}
                        onChange={e => setFormData({ ...formData, timeout: parseInt(e.target.value) })}
                        className="w-full px-3 py-2 bg-slate-50 border border-slate-200 rounded-lg focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 outline-none transition-all text-sm"
                      />
                    </div>
                  </div>
                </div>
              )}
            </div>

            <div className="p-6 bg-slate-50 flex justify-end gap-3">
              <button
                onClick={() => setIsAddModalOpen(false)}
                className="px-8 py-3 text-sm font-bold text-slate-500 hover:text-slate-700 transition-all bg-white border border-slate-200 rounded-2xl"
              >
                {t('skills.cancel')}
              </button>
              <button
                onClick={handleAdd}
                className="px-12 py-3 bg-slate-900 text-white rounded-2xl font-bold text-sm hover:bg-slate-800 transition-all shadow-lg shadow-slate-900/10"
              >
                {activeTab === 'skills' ? t('skills.create') : t('skills.add')}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* View Modal */}
      {isViewModalOpen && selectedSkill && (
        <div className="fixed inset-0 bg-slate-900/50 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className={cn(
            "bg-white rounded-2xl w-full shadow-2xl overflow-hidden flex flex-col max-h-[90vh]",
            selectedSkill.type === 'tool' ? "max-w-xl" : "max-w-3xl"
          )}>
            {/* Modal Header */}
            <div className="p-6 border-b border-slate-100 flex items-center justify-between bg-white sticky top-0 z-10">
              <h2 className="text-xl font-bold text-slate-900">
                {selectedSkill.type === 'custom' || selectedSkill.type === 'built-in' ? t('skills.view') + ' ' + t('skills.skillsTab') :
                  selectedSkill.type === 'mcp' ? t('skills.view') + ' ' + t('skills.mcpTab') :
                    selectedSkill.type === 'a2a' ? t('skills.view') + ' ' + t('skills.a2aTab') :
                      t('skills.view') + ' ' + t('skills.toolsTab')}
              </h2>
              <button
                onClick={() => setIsViewModalOpen(false)}
                className="text-slate-400 hover:text-slate-600 transition-all"
              >
                <X size={24} />
              </button>
            </div>

            {/* Modal Content */}
            <div className="flex-1 overflow-y-auto p-8 space-y-6">
              {/* Common Name Field */}
              <div>
                <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-2">
                  {selectedSkill.type === 'custom' || selectedSkill.type === 'built-in' ? t('skills.skillName') :
                    selectedSkill.type === 'mcp' ? t('skills.mcpName') :
                      selectedSkill.type === 'a2a' ? t('skills.a2aName') :
                        t('skills.toolName')}
                </label>
                <div className="w-full px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl text-slate-900 font-medium">
                  {selectedSkill.name}
                </div>
              </div>

              {/* Common Description Field */}
              <div>
                <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-2">
                  {t('skills.description')}
                </label>
                <div className="w-full px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl text-slate-700 text-sm">
                  {selectedSkill.description || 'No description provided.'}
                </div>
              </div>

              {/* Common Risk Level Field */}
              <div>
                <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-2">
                  {t('skills.toolRisk')}
                </label>
                <div className="flex items-center gap-2 px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl">
                  <div className={cn(
                    "w-2 h-2 rounded-full",
                    selectedSkill.riskLevel === 'high' ? "bg-red-500" :
                      selectedSkill.riskLevel === 'medium' ? "bg-amber-500" : "bg-emerald-500"
                  )} />
                  <span className="text-slate-900 font-medium capitalize">
                    {selectedSkill.riskLevel ? t(`orchestrator.riskLevels.${selectedSkill.riskLevel}`) : t('orchestrator.riskLevels.low')}
                  </span>
                </div>
              </div>

              {/* Skill / Custom Specific */}
              {(selectedSkill.type === 'custom' || selectedSkill.type === 'built-in') && (
                <div>
                  <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-2">
                    {t('skills.skillContent')}
                  </label>
                  <div className="w-full h-64 px-4 py-4 bg-slate-50 border border-slate-200 rounded-xl font-mono text-sm leading-relaxed overflow-y-auto whitespace-pre-wrap text-slate-700">
                    {selectedSkill.content || 'No content provided.'}
                  </div>
                </div>
              )}

              {/* MCP Specific */}
              {selectedSkill.type === 'mcp' && (
                <>
                  {selectedSkill.mcpUrl ? (
                    <>
                      <div>
                        <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-2">
                          {t('skills.mcpEndpoint')}
                        </label>
                        <div className="w-full px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl font-mono text-sm text-brand-600 break-all">
                          {selectedSkill.mcpUrl}
                        </div>
                      </div>
                      <div>
                        <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-2">
                          {t('skills.mcpToken')}
                        </label>
                        <div className="w-full px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl font-mono text-sm text-slate-500">
                          {selectedSkill.token ? '••••••••••••••••' : 'None'}
                        </div>
                      </div>
                    </>
                  ) : (
                    <>
                      <div>
                        <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-2">
                          {t('skills.mcpCommand')}
                        </label>
                        <div className="w-full px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl font-mono text-sm text-brand-600 break-all">
                          {selectedSkill.command}
                        </div>
                      </div>
                      {selectedSkill.args && selectedSkill.args.length > 0 && (
                        <div>
                          <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-2">
                            {t('skills.mcpArgs')}
                          </label>
                          <div className="w-full px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl font-mono text-sm text-slate-500">
                            {selectedSkill.args.join(' ')}
                          </div>
                        </div>
                      )}
                      {selectedSkill.env && Object.keys(selectedSkill.env).length > 0 && (
                        <div>
                          <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-2">
                            {t('skills.mcpEnv')}
                          </label>
                          <div className="w-full px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl font-mono text-sm text-slate-500 whitespace-pre">
                            {JSON.stringify(selectedSkill.env, null, 2)}
                          </div>
                        </div>
                      )}
                    </>
                  )}
                </>
              )}

              {/* A2A Specific */}
              {selectedSkill.type === 'a2a' && (
                <>
                  <div>
                    <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-2">
                      {t('skills.a2aEndpoint')}
                    </label>
                    <div className="w-full px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl font-mono text-sm text-brand-600 break-all">
                      {selectedSkill.endpoint}
                    </div>
                  </div>
                  <div>
                    <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-2">
                      {t('skills.a2aToken')}
                    </label>
                    <div className="w-full px-4 py-3 bg-slate-50 border border-slate-200 rounded-xl font-mono text-sm text-slate-500">
                      {selectedSkill.token ? '••••••••••••••••' : 'None'}
                    </div>
                  </div>
                </>
              )}

              {/* Tool Specific */}
              {selectedSkill.type === 'tool' && (
                <div className="space-y-4">
                  <div>
                    <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-1">
                      {t('skills.toolEndpoint')}
                    </label>
                    <div className="w-full px-3 py-2 bg-slate-50 border border-slate-200 rounded-lg text-sm text-brand-600 break-all">
                      {selectedSkill.endpoint}
                    </div>
                  </div>
                  <div>
                    <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-1">
                      {t('skills.toolSandbox')}
                    </label>
                    <div className="w-full px-3 py-2 bg-slate-50 border border-slate-200 rounded-lg text-sm text-slate-500 break-all">
                      {selectedSkill.sandboxEndpoint || 'Not configured'}
                    </div>
                  </div>
                  <div>
                    <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-1">
                      {t('skills.toolToken')}
                    </label>
                    <div className="w-full px-3 py-2 bg-slate-50 border border-slate-200 rounded-lg text-sm text-slate-500">
                      {selectedSkill.token ? '••••••••' : 'None'}
                    </div>
                  </div>
                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-1">
                        {t('skills.toolMethod')}
                      </label>
                      <div className="w-full px-3 py-2 bg-slate-50 border border-slate-200 rounded-lg text-sm font-bold">
                        {selectedSkill.method || 'POST'}
                      </div>
                    </div>
                    <div>
                      <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-1">
                        {t('skills.toolTimeout')}
                      </label>
                      <div className="w-full px-3 py-2 bg-slate-50 border border-slate-200 rounded-lg text-sm font-bold">
                        {selectedSkill.timeout} ms
                      </div>
                    </div>
                  </div>
                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-1">
                        {t('skills.toolRisk')}
                      </label>
                      <div className={cn(
                        "w-full px-3 py-2 border rounded-lg text-sm font-bold uppercase",
                        selectedSkill.riskLevel === 'high' ? "bg-red-50 border-red-100 text-red-600" : "bg-blue-50 border-blue-100 text-blue-600"
                      )}>
                        {selectedSkill.riskLevel || 'low'}
                      </div>
                    </div>
                    <div>
                      <label className="block text-[10px] font-bold text-slate-400 uppercase tracking-wider mb-1">
                        {t('skills.toolSandboxToken')}
                      </label>
                      <div className="w-full px-3 py-2 bg-slate-50 border border-slate-200 rounded-lg text-sm text-slate-500">
                        {selectedSkill.sandboxToken ? '••••••••' : 'None'}
                      </div>
                    </div>
                  </div>
                </div>
              )}
            </div>

            <div className="p-6 bg-slate-50 flex justify-end">
              <button
                onClick={() => setIsViewModalOpen(false)}
                className="px-12 py-3 bg-slate-900 text-white rounded-2xl font-bold text-sm hover:bg-slate-800 transition-all shadow-lg shadow-slate-900/10"
              >
                {t('skills.close')}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Confirmation Dialog */}
      <AnimatePresence>
        {confirmDialog.open && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
            onClick={() => setConfirmDialog(prev => ({ ...prev, open: false }))}
          >
            <motion.div
              initial={{ scale: 0.9, opacity: 0 }}
              animate={{ scale: 1, opacity: 1 }}
              exit={{ scale: 0.9, opacity: 0 }}
              className="bg-white rounded-2xl p-6 w-full max-w-md shadow-xl"
              onClick={(e) => e.stopPropagation()}
            >
              <div className="flex items-center gap-3 mb-4">
                <div className="w-12 h-12 rounded-full bg-red-100 flex items-center justify-center">
                  <AlertCircle className="w-6 h-6 text-red-500" />
                </div>
                <h3 className="text-lg font-bold text-slate-900">{confirmDialog.title}</h3>
              </div>
              <p className="text-slate-600 mb-6">{confirmDialog.message}</p>
              <div className="flex justify-end gap-3">
                <button
                  onClick={() => setConfirmDialog(prev => ({ ...prev, open: false }))}
                  className="px-4 py-2 text-sm font-medium text-slate-700 bg-slate-100 rounded-lg hover:bg-slate-200 transition-colors"
                >
                  {t('common.cancel') || '取消'}
                </button>
                <button
                  onClick={confirmDialog.onConfirm}
                  className="px-4 py-2 text-sm font-medium text-white bg-red-500 rounded-lg hover:bg-red-600 transition-colors"
                >
                  {t('common.confirmDelete') || '确认删除'}
                </button>
              </div>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}