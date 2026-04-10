import React from 'react';
import { Inbox as InboxIcon, CheckCircle2, XCircle, Clock, ShieldAlert, UserCheck, RefreshCw } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { cn } from '../lib/utils';
import { ChatApproval } from '../types';
import { chatApi } from '../lib/api';

const CURRENT_USER_ID = 'user-1'; // TODO: Get from auth context

export function Inbox() {
  const { t } = useTranslation();
  const [approvals, setApprovals] = React.useState<ChatApproval[]>([]);
  const [filter, setFilter] = React.useState<'pending' | 'all'>('pending');
  const [loading, setLoading] = React.useState(false);

  const loadApprovals = React.useCallback(async () => {
    setLoading(true);
    try {
      const data = await chatApi.getPendingApprovals();
      setApprovals(data);
    } catch (err) {
      console.error('Failed to load approvals:', err);
    } finally {
      setLoading(false);
    }
  }, []);

  // Load on mount
  React.useEffect(() => {
    loadApprovals();
  }, [loadApprovals]);

  const filteredApprovals = approvals.filter(a => filter === 'all' || a.status === 'pending');

  const handleAction = async (id: string, action: 'approved' | 'rejected') => {
    try {
      if (action === 'approved') {
        await chatApi.approveApproval(id, CURRENT_USER_ID);
      } else {
        await chatApi.rejectApproval(id, CURRENT_USER_ID);
      }
      // Update local state
      setApprovals(prev => prev.map(a => a.ulid === id ? { ...a, status: action } : a));
    } catch (err) {
      console.error('Failed to handle approval:', err);
    }
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
        <div className="flex items-center gap-3">
          <button
            onClick={loadApprovals}
            disabled={loading}
            className={cn(
              "flex items-center gap-2 px-3 py-1.5 text-xs font-bold rounded-lg transition-all",
              loading ? "text-slate-400 cursor-not-allowed" : "text-slate-500 hover:text-slate-700 hover:bg-slate-100"
            )}
          >
            <RefreshCw size={14} className={loading ? "animate-spin" : ""} />
            {t('inbox.refresh') || '刷新'}
          </button>
          <div className="flex p-1 bg-slate-100 rounded-lg">
            <button
              onClick={() => setFilter('pending')}
              className={cn(
                "px-4 py-1.5 text-xs font-bold rounded-md transition-all",
                filter === 'pending' ? "bg-white text-slate-900 shadow-sm" : "text-slate-500 hover:text-slate-700"
              )}
            >
              {t('inbox.pending')} ({approvals.filter(a => a.status === 'pending').length})
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
        </div>
      </header>

      <div className="flex-1 overflow-y-auto p-6 scrollbar-hide">
        <div className="max-w-4xl mx-auto space-y-4">
          {filteredApprovals.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-20 text-center">
              <div className="w-16 h-16 rounded-full bg-slate-100 flex items-center justify-center text-slate-300 mb-4">
                <CheckCircle2 size={32} />
              </div>
              <h3 className="text-lg font-bold text-slate-900 mb-1">{t('inbox.emptyTitle')}</h3>
              <p className="text-sm text-slate-500">{t('inbox.emptySubtitle')}</p>
            </div>
          ) : (
            filteredApprovals.map(approval => (
              <div
                key={approval.ulid}
                className={cn(
                  "bg-white rounded-2xl border p-6 transition-all shadow-sm",
                  approval.status === 'pending' ? "border-slate-200" : "border-slate-100 opacity-75"
                )}
              >
                <div className="flex items-start justify-between mb-6">
                  <div className="flex items-center gap-3">
                    <div className="w-10 h-10 rounded-full bg-brand-500 flex items-center justify-center text-white font-bold">
                      {approval.tool_name?.[0] || 'T'}
                    </div>
                    <div>
                      <h4 className="font-bold text-slate-900">{approval.tool_name}</h4>
                      <div className="flex items-center gap-2 text-[10px] text-slate-400 font-bold uppercase tracking-wider">
                        <Clock size={10} />
                        {new Date(approval.created_at).toLocaleString()}
                      </div>
                    </div>
                  </div>
                  <div className={cn(
                    "px-3 py-1 rounded-full text-[10px] font-bold uppercase tracking-wider",
                    approval.status === 'pending' ? "bg-amber-50 text-amber-600 border border-amber-100" :
                      approval.status === 'approved' ? "bg-green-50 text-green-600 border border-green-100" :
                        "bg-red-50 text-red-600 border border-red-100"
                  )}>
                    {t(`inbox.status.${approval.status}`)}
                  </div>
                </div>

                <div className="bg-slate-50 rounded-xl p-4 mb-6 border border-slate-100">
                  <div className="flex items-center gap-2 text-slate-900 mb-2">
                    <ShieldAlert size={16} className="text-amber-500" />
                    <span className="text-sm font-bold">{t('inbox.requestAction')}: {approval.tool_name}</span>
                  </div>
                  <p className="text-sm text-slate-600 mb-4">风险等级: <span className={approval.risk_level === 'high' ? 'text-red-500 font-bold' : 'text-amber-500 font-bold'}>{approval.risk_level?.toUpperCase() || 'MEDIUM'}</span></p>

                  <div className="space-y-2">
                    <p className="text-[10px] font-bold text-slate-400 uppercase tracking-widest">{t('inbox.parameters')}</p>
                    <div className="bg-white rounded-lg border border-slate-200 p-3 font-mono text-xs text-slate-700">
                      <pre>{approval.parameters || '{}'}</pre>
                    </div>
                  </div>
                </div>

                {approval.status === 'pending' && (
                  <div className="flex items-center justify-end gap-3">
                    <button
                      onClick={() => handleAction(approval.ulid, 'rejected')}
                      className="flex items-center gap-2 px-6 py-2 bg-white border border-slate-200 rounded-lg text-sm font-bold text-slate-600 hover:bg-red-50 hover:text-red-600 hover:border-red-200 transition-all"
                    >
                      <XCircle size={16} />
                      {t('inbox.reject')}
                    </button>
                    <button
                      onClick={() => handleAction(approval.ulid, 'approved')}
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
