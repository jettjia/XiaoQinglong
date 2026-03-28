import React from 'react';
import { Inbox as InboxIcon, CheckCircle2, XCircle, Clock, ShieldAlert, UserCheck, ArrowRight } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { cn } from '../lib/utils';
import { ApprovalTask } from '../types';

const MOCK_TASKS: ApprovalTask[] = [
  {
    id: 't1',
    agentId: 'a1',
    agentName: '财务助手',
    toolName: 'send_payment',
    description: '向供应商支付 5,000 元货款',
    params: { vendor: 'TechCorp', amount: 5000, currency: 'CNY', invoice_id: 'INV-2024-001' },
    timestamp: new Date(Date.now() - 1000 * 60 * 15),
    status: 'pending'
  },
  {
    id: 't2',
    agentId: 'a2',
    agentName: '系统管理员',
    toolName: 'delete_user',
    description: '删除离职员工账号: test_user_01',
    params: { username: 'test_user_01', backup: true },
    timestamp: new Date(Date.now() - 1000 * 60 * 60 * 2),
    status: 'pending'
  },
  {
    id: 't3',
    agentId: 'a3',
    agentName: '市场专员',
    toolName: 'post_to_social',
    description: '发布全员营销推文到官方微博',
    params: { content: '我们的新产品上线啦！快来体验吧！', platforms: ['weibo'] },
    timestamp: new Date(Date.now() - 1000 * 60 * 60 * 24),
    status: 'approved'
  }
];

export function Inbox() {
  const { t } = useTranslation();
  const [tasks, setTasks] = React.useState<ApprovalTask[]>(MOCK_TASKS);
  const [filter, setFilter] = React.useState<'pending' | 'all'>('pending');

  const filteredTasks = tasks.filter(t => filter === 'all' || t.status === 'pending');

  const handleAction = (id: string, action: 'approved' | 'rejected') => {
    setTasks(prev => prev.map(t => t.id === id ? { ...t, status: action } : t));
  };

  return (
    <div className="h-full flex flex-col bg-slate-50 overflow-hidden">
      {/* Header */}
      <header className="h-16 border-b border-slate-200 bg-white flex items-center justify-between px-6 shrink-0 z-20">
        <div className="flex items-center gap-4">
          <div className="w-10 h-10 rounded-xl bg-brand-500/10 flex items-center justify-center text-brand-500">
            <InboxIcon size={20} />
          </div>
          <div>
            <h1 className="text-lg font-bold text-slate-900">{t('inbox.title')}</h1>
            <p className="text-xs text-slate-500 font-medium uppercase tracking-wider">{t('inbox.subtitle')}</p>
          </div>
        </div>
        <div className="flex p-1 bg-slate-100 rounded-lg">
          <button 
            onClick={() => setFilter('pending')}
            className={cn(
              "px-4 py-1.5 text-xs font-bold rounded-md transition-all",
              filter === 'pending' ? "bg-white text-slate-900 shadow-sm" : "text-slate-500 hover:text-slate-700"
            )}
          >
            {t('inbox.pending')} ({tasks.filter(t => t.status === 'pending').length})
          </button>
          <button 
            onClick={() => setFilter('all')}
            className={cn(
              "px-4 py-1.5 text-xs font-bold rounded-md transition-all",
              filter === 'all' ? "bg-white text-slate-900 shadow-sm" : "text-slate-500 hover:text-slate-700"
            )}
          >
            {t('inbox.all')}
          </button>
        </div>
      </header>

      <div className="flex-1 overflow-y-auto p-6 scrollbar-hide">
        <div className="max-w-4xl mx-auto space-y-4">
          {filteredTasks.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-20 text-center">
              <div className="w-16 h-16 rounded-full bg-slate-100 flex items-center justify-center text-slate-300 mb-4">
                <CheckCircle2 size={32} />
              </div>
              <h3 className="text-lg font-bold text-slate-900 mb-1">{t('inbox.emptyTitle')}</h3>
              <p className="text-sm text-slate-500">{t('inbox.emptySubtitle')}</p>
            </div>
          ) : (
            filteredTasks.map(task => (
              <div 
                key={task.id}
                className={cn(
                  "bg-white rounded-2xl border p-6 transition-all shadow-sm",
                  task.status === 'pending' ? "border-slate-200" : "border-slate-100 opacity-75"
                )}
              >
                <div className="flex items-start justify-between mb-6">
                  <div className="flex items-center gap-3">
                    <div className="w-10 h-10 rounded-full bg-brand-500 flex items-center justify-center text-white font-bold">
                      {task.agentName[0]}
                    </div>
                    <div>
                      <h4 className="font-bold text-slate-900">{task.agentName}</h4>
                      <div className="flex items-center gap-2 text-[10px] text-slate-400 font-bold uppercase tracking-wider">
                        <Clock size={10} />
                        {task.timestamp.toLocaleString()}
                      </div>
                    </div>
                  </div>
                  <div className={cn(
                    "px-3 py-1 rounded-full text-[10px] font-bold uppercase tracking-wider",
                    task.status === 'pending' ? "bg-amber-50 text-amber-600 border border-amber-100" :
                    task.status === 'approved' ? "bg-green-50 text-green-600 border border-green-100" :
                    "bg-red-50 text-red-600 border border-red-100"
                  )}>
                    {t(`inbox.status.${task.status}`)}
                  </div>
                </div>

                <div className="bg-slate-50 rounded-xl p-4 mb-6 border border-slate-100">
                  <div className="flex items-center gap-2 text-slate-900 mb-2">
                    <ShieldAlert size={16} className="text-amber-500" />
                    <span className="text-sm font-bold">{t('inbox.requestAction')}: {task.toolName}</span>
                  </div>
                  <p className="text-sm text-slate-600 mb-4">{task.description}</p>
                  
                  <div className="space-y-2">
                    <p className="text-[10px] font-bold text-slate-400 uppercase tracking-widest">{t('inbox.parameters')}</p>
                    <div className="bg-white rounded-lg border border-slate-200 p-3 font-mono text-xs text-slate-700">
                      <pre>{JSON.stringify(task.params, null, 2)}</pre>
                    </div>
                  </div>
                </div>

                {task.status === 'pending' && (
                  <div className="flex items-center justify-end gap-3">
                    <button 
                      onClick={() => handleAction(task.id, 'rejected')}
                      className="flex items-center gap-2 px-6 py-2 bg-white border border-slate-200 rounded-lg text-sm font-bold text-slate-600 hover:bg-red-50 hover:text-red-600 hover:border-red-200 transition-all"
                    >
                      <XCircle size={16} />
                      {t('inbox.reject')}
                    </button>
                    <button 
                      onClick={() => handleAction(task.id, 'approved')}
                      className="flex items-center gap-2 px-6 py-2 bg-brand-500 text-white rounded-lg text-sm font-bold hover:bg-brand-600 transition-all shadow-lg shadow-brand-500/20"
                    >
                      <UserCheck size={16} />
                      {t('inbox.approve')}
                    </button>
                  </div>
                )}
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  );
}
