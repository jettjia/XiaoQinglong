import React from 'react';
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
  X
} from 'lucide-react';
import { Agent, View, AgentLog } from '../types';
import { INITIAL_AGENTS } from '../constants';
import { cn } from '../lib/utils';
import { useTranslation } from 'react-i18next';
import { motion, AnimatePresence } from 'motion/react';

interface AgentManagerProps {
  onViewChange: (view: View) => void;
}

export function AgentManager({ onViewChange }: AgentManagerProps) {
  const { t } = useTranslation();
  const [agents, setAgents] = React.useState<Agent[]>([
    ...INITIAL_AGENTS,
    {
      id: 'custom-1',
      name: '我的智能体',
      description: '这是一个由我编排的自定义智能体',
      model: 'gemini-3.1-pro-preview',
      skills: ['deep-research', 'coding'],
      tools: ['web-search'],
      icon: 'Users',
      isBuiltIn: false,
      channels: ['api', 'web'],
      isPeriodic: true,
      cronRule: '0 * * * *',
      logs: [
        { id: 'l1', timestamp: new Date(Date.now() - 1000 * 60 * 60), status: 'success', message: '任务执行成功：已同步 12 条数据', duration: '1.2s' },
        { id: 'l2', timestamp: new Date(Date.now() - 1000 * 60 * 120), status: 'failed', message: '任务执行失败：API 响应超时', duration: '0.5s' },
        { id: 'l3', timestamp: new Date(Date.now() - 1000 * 60 * 180), status: 'success', message: '任务执行成功：清理缓存完成', duration: '0.8s' },
      ]
    }
  ]);
  const [searchQuery, setSearchQuery] = React.useState('');
  const [selectedAgentForLogs, setSelectedAgentForLogs] = React.useState<Agent | null>(null);

  const filteredAgents = agents.filter(a => 
    a.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
    a.description.toLowerCase().includes(searchQuery.toLowerCase())
  );

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
        <button className="flex items-center gap-2 px-4 py-2 bg-white border border-slate-200 rounded-lg text-slate-600 hover:bg-slate-50 transition-all">
          <Filter size={18} />
          {t('agents.filter')}
        </button>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {filteredAgents.map((agent) => (
          <div 
            key={agent.id} 
            onClick={() => !agent.isBuiltIn && onViewChange('orchestrator')}
            className={cn(
              "bg-white border border-slate-200 rounded-xl p-6 transition-all group",
              !agent.isBuiltIn ? "hover:shadow-md cursor-pointer border-brand-100 hover:border-brand-200" : "hover:shadow-sm"
            )}
          >
            <div className="flex items-start justify-between mb-4">
              <div className="w-12 h-12 rounded-xl bg-slate-100 flex items-center justify-center text-slate-600 group-hover:bg-brand-50 group-hover:text-brand-500 transition-colors">
                <Users size={24} />
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
                {!agent.isBuiltIn && (
                  <button 
                    onClick={(e) => {
                      e.stopPropagation();
                      onViewChange('orchestrator');
                    }}
                    className="p-2 hover:bg-slate-100 rounded-lg text-slate-400 transition-colors"
                  >
                    <Edit2 size={16} />
                  </button>
                )}
                <button 
                  onClick={(e) => e.stopPropagation()}
                  className="p-2 hover:bg-slate-100 rounded-lg text-slate-400 transition-colors"
                >
                  <MoreVertical size={16} />
                </button>
              </div>
            </div>

            <h3 className="font-bold text-slate-900 mb-1 flex items-center gap-2">
              {agent.name}
              {agent.isBuiltIn && (
                <span className="text-[10px] font-bold uppercase tracking-wider bg-slate-100 text-slate-500 px-1.5 py-0.5 rounded">{t('agents.builtIn')}</span>
              )}
              {agent.isPeriodic && (
                <span className="text-[10px] font-bold uppercase tracking-wider bg-amber-100 text-amber-600 px-1.5 py-0.5 rounded flex items-center gap-1">
                  <Clock size={10} />
                  {t('agents.periodic')}
                </span>
              )}
            </h3>
            <p className="text-sm text-slate-500 line-clamp-2 mb-4">
              {agent.description}
            </p>

            <div className="flex flex-wrap gap-2 mb-4">
              {agent.channels?.map(channel => (
                <span key={channel} className="text-[10px] font-bold uppercase tracking-wider bg-slate-100 text-slate-500 px-2 py-0.5 rounded flex items-center gap-1">
                  <Globe size={10} />
                  {channel}
                </span>
              ))}
            </div>

            <div className="flex flex-wrap gap-2 mb-4">
              {agent.skills.map(skill => (
                <span key={skill} className="text-[11px] font-medium bg-brand-50 text-brand-600 px-2 py-0.5 rounded-full">
                  {skill}
                </span>
              ))}
            </div>

            <div className="pt-4 border-t border-slate-100 flex items-center justify-between">
              <div className="flex items-center gap-2 text-xs text-slate-400">
                <Cpu size={14} />
                {agent.model}
              </div>
              <div className="flex items-center gap-3">
                {agent.isPeriodic && (
                  <button 
                    onClick={(e) => {
                      e.stopPropagation();
                      setSelectedAgentForLogs(agent);
                    }}
                    className="text-xs font-semibold text-slate-500 hover:text-slate-900 flex items-center gap-1"
                  >
                    <Clock size={12} />
                    {t('agents.viewLogs')}
                  </button>
                )}
                <button 
                  onClick={(e) => e.stopPropagation()}
                  className="text-xs font-semibold text-brand-500 hover:text-brand-600 flex items-center gap-1"
                >
                  {t('agents.viewDetails')}
                  <ExternalLink size={12} />
                </button>
              </div>
            </div>
          </div>
        ))}
      </div>

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
