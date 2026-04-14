import { useEffect, useState } from 'react';
import {
  Users,
  Zap,
  Database,
  ArrowUpRight,
  ArrowDownRight,
  Clock,
  Plus,
  MessageSquare,
  ShieldCheck,
  Cpu,
  AlertCircle,
  ChevronRight,
  RefreshCw,
  CheckCircle2,
  Globe
} from 'lucide-react';
import { cn } from '../lib/utils';
import { useTranslation } from 'react-i18next';
import { View } from '../types';
import { motion } from 'motion/react';
import {
  agentApi,
  knowledgeBaseApi,
  chatApi,
  dashboardApi,
  type DashboardOverview,
  type AgentUsageItem,
  type ChannelActivityItem,
  type ChatSession,
  type ChatApproval
} from '../lib/api';

interface DashboardProps {
  onViewChange: (view: View) => void;
}

// Format number to K/M shorthand
const formatNumber = (num: number): string => {
  if (num >= 1000000) return `${(num / 1000000).toFixed(1)}M`;
  if (num >= 1000) return `${(num / 1000).toFixed(1)}K`;
  return num.toString();
};

// Format time ago
const formatTimeAgo = (timestamp: number): string => {
  const now = Date.now();
  const diff = now - timestamp;
  const minutes = Math.floor(diff / 60000);
  const hours = Math.floor(diff / 3600000);
  const days = Math.floor(diff / 86400000);

  if (minutes < 1) return 'Just now';
  if (minutes < 60) return `${minutes}m ago`;
  if (hours < 24) return `${hours}h ago`;
  return `${days}d ago`;
};

export function Dashboard({ onViewChange }: DashboardProps) {
  const { t } = useTranslation();

  // Loading states
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Data states
  const [overview, setOverview] = useState<DashboardOverview | null>(null);
  const [agentRanking, setAgentRanking] = useState<AgentUsageItem[]>([]);
  const [channelActivity, setChannelActivity] = useState<ChannelActivityItem[]>([]);
  const [pendingApprovals, setPendingApprovals] = useState<ChatApproval[]>([]);
  const [recentSessions, setRecentSessions] = useState<ChatSession[]>([]);

  // Fetch all dashboard data
  const fetchData = async (isRefresh = false) => {
    if (isRefresh) setRefreshing(true);
    else setLoading(true);
    setError(null);

    try {
      // Fetch all data in parallel
      const [
        overviewData,
        agentRankingData,
        channelActivityData,
        approvalsData,
        agentsData,
        knowledgeData,
        sessionsData,
      ] = await Promise.allSettled([
        dashboardApi.getOverview().catch(() => null),
        dashboardApi.getAgentUsageRanking().catch(() => []),
        dashboardApi.getChannelActivity().catch(() => []),
        chatApi.getPendingApprovals().catch(() => []),
        agentApi.findAll().catch(() => []),
        knowledgeBaseApi.findAll().catch(() => []),
        dashboardApi.getRecentSessions().catch(() => []),
      ]);

      // Handle overview data or calculate from other APIs
      if (overviewData.status === 'fulfilled' && overviewData.value) {
        setOverview(overviewData.value);
      } else {
        // Calculate from other APIs if dashboard API not available
        const agents = agentsData.status === 'fulfilled' ? agentsData.value : [];
        const knowledge = knowledgeData.status === 'fulfilled' ? knowledgeData.value : [];

        setOverview({
          active_agents: Array.isArray(agents) ? agents.filter((a: any) => a.enabled).length : 0,
          periodic_agents: Array.isArray(agents) ? agents.filter((a: any) => a.isPeriodic || a.cronRule).length : 0,
          tasks_completed: 0, // No direct API available
          total_tokens: 0,    // No direct API available
          active_knowledge_sources: Array.isArray(knowledge) ? knowledge.filter((k: any) => k.enabled).length : 0,
        });
      }

      // Set other data
      if (agentRankingData.status === 'fulfilled' && agentRankingData.value) {
        const tr = agentRankingData.value;
        setAgentRanking(Array.isArray(tr) ? tr : (tr as any)?.rankings || []);
      }
      if (channelActivityData.status === 'fulfilled' && channelActivityData.value) {
        const ca = channelActivityData.value;
        setChannelActivity(Array.isArray(ca) ? ca : (ca as any)?.channels || []);
      }
      if (approvalsData.status === 'fulfilled' && approvalsData.value) {
        const ap = approvalsData.value;
        setPendingApprovals(Array.isArray(ap) ? ap : []);
      }
      if (sessionsData.status === 'fulfilled' && sessionsData.value) {
        const ss = sessionsData.value;
        setRecentSessions(Array.isArray(ss) ? ss : []);
      }

    } catch (err) {
      console.error('Failed to fetch dashboard data:', err);
      setError(err instanceof Error ? err.message : 'Failed to load dashboard');
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };

  // Initial fetch
  useEffect(() => {
    fetchData();
  }, []);

  // Stats with real data
  const stats = [
    {
      label: t('dashboard.activeAgents'),
      value: overview?.active_agents ?? '-',
      change: '',
      trend: 'up',
      icon: Users,
      color: 'text-blue-500',
      bg: 'bg-blue-50',
      view: 'agents' as View
    },
    {
      label: t('dashboard.periodicAgents'),
      value: overview?.periodic_agents ?? '-',
      change: '',
      trend: 'up',
      icon: Clock,
      color: 'text-purple-500',
      bg: 'bg-purple-50',
      view: 'agents' as View
    },
    {
      label: t('dashboard.tasksCompleted'),
      value: overview?.tasks_completed ? formatNumber(overview.tasks_completed) : '-',
      change: '',
      trend: 'up',
      icon: Zap,
      color: 'text-orange-500',
      bg: 'bg-orange-50'
    },
    {
      label: t('dashboard.totalTokens'),
      value: overview?.total_tokens ? formatNumber(overview.total_tokens) : '-',
      change: '',
      trend: 'up',
      icon: Cpu,
      color: 'text-indigo-500',
      bg: 'bg-indigo-50'
    },
    {
      label: t('knowledge.activeSources'),
      value: overview?.active_knowledge_sources ?? '-',
      change: '',
      trend: 'up',
      icon: Database,
      color: 'text-green-500',
      bg: 'bg-green-50',
      view: 'knowledge' as View
    },
  ];

  const quickActions = [
    { label: t('dashboard.createAgent'), icon: Plus, view: 'orchestrator' as View, color: 'bg-brand-500 text-white hover:bg-brand-600' },
    { label: t('dashboard.addKnowledge'), icon: Database, view: 'knowledge' as View, color: 'bg-white text-slate-700 border border-slate-200 hover:bg-slate-50' },
    { label: t('dashboard.configureModel'), icon: Cpu, view: 'models' as View, color: 'bg-white text-slate-700 border border-slate-200 hover:bg-slate-50' },
  ];

  // Loading skeleton
  if (loading) {
    return (
      <div className="p-8 max-w-7xl mx-auto space-y-8">
        <div className="flex justify-between items-center">
          <div className="space-y-2">
            <div className="h-8 w-48 bg-slate-200 rounded animate-pulse" />
            <div className="h-4 w-64 bg-slate-100 rounded animate-pulse" />
          </div>
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-6">
          {[...Array(5)].map((_, i) => (
            <div key={i} className="bg-white p-6 rounded-2xl border border-slate-200 animate-pulse">
              <div className="h-12 w-12 bg-slate-100 rounded-xl mb-4" />
              <div className="h-4 w-24 bg-slate-100 rounded mb-2" />
              <div className="h-8 w-16 bg-slate-100 rounded" />
            </div>
          ))}
        </div>
      </div>
    );
  }

  // Error state
  if (error) {
    return (
      <div className="p-8 max-w-7xl mx-auto">
        <div className="bg-red-50 border border-red-200 rounded-2xl p-6 text-center">
          <AlertCircle className="mx-auto h-12 w-12 text-red-400 mb-4" />
          <h3 className="text-lg font-semibold text-red-800 mb-2">Failed to load dashboard</h3>
          <p className="text-red-600 mb-4">{error}</p>
          <button
            onClick={() => fetchData()}
            className="px-4 py-2 bg-red-100 text-red-700 rounded-lg hover:bg-red-200 transition-colors"
          >
            Try Again
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="p-8 max-w-7xl mx-auto space-y-8">
      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">{t('dashboard.welcome')}</h1>
          <p className="text-slate-500 mt-1">{t('dashboard.subtitle')}</p>
        </div>

        <div className="flex items-center gap-3">
          {/* Refresh button */}
          <button
            onClick={() => fetchData(true)}
            disabled={refreshing}
            className="flex items-center gap-2 px-4 py-2 rounded-xl text-sm font-bold bg-white text-slate-700 border border-slate-200 hover:bg-slate-50 transition-all disabled:opacity-50"
          >
            <RefreshCw size={18} className={refreshing ? 'animate-spin' : ''} />
            {refreshing ? 'Refreshing...' : 'Refresh'}
          </button>

          {/* Quick Actions */}
          {quickActions.map((action) => (
            <button
              key={action.label}
              onClick={() => onViewChange(action.view)}
              className={cn(
                "flex items-center gap-2 px-4 py-2 rounded-xl text-sm font-bold transition-all shadow-sm",
                action.color
              )}
            >
              <action.icon size={18} />
              {action.label}
            </button>
          ))}
        </div>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-6">
        {stats.map((stat, i) => (
          <motion.div
            key={stat.label}
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: i * 0.1 }}
            onClick={() => stat.view && onViewChange(stat.view)}
            className={cn(
              "bg-white p-6 rounded-2xl border border-slate-200 shadow-sm hover:shadow-md transition-all",
              stat.view && "cursor-pointer hover:border-brand-200"
            )}
          >
            <div className="flex items-start justify-between mb-4">
              <div className={cn("w-12 h-12 rounded-xl flex items-center justify-center", stat.bg, stat.color)}>
                <stat.icon size={24} />
              </div>
              {stat.change && (
                <div className={cn(
                  "flex items-center gap-1 text-xs font-bold px-2 py-1 rounded-full",
                  stat.trend === 'up' ? "bg-green-50 text-green-600" : "bg-blue-50 text-blue-600"
                )}>
                  {stat.trend === 'up' ? <ArrowUpRight size={12} /> : <ArrowDownRight size={12} />}
                  {stat.change}
                </div>
              )}
            </div>
            <p className="text-sm font-medium text-slate-500">{stat.label}</p>
            <p className="text-2xl font-bold text-slate-900 mt-1">{stat.value}</p>
          </motion.div>
        ))}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
        {/* Pending Approvals */}
        <div className="lg:col-span-2 bg-white border border-slate-200 rounded-2xl p-6 shadow-sm">
          <div className="flex items-center justify-between mb-6">
            <div className="flex items-center gap-2">
              <ShieldCheck className="text-brand-500" size={20} />
              <h3 className="font-bold text-slate-900">{t('dashboard.pendingApprovals')}</h3>
              {pendingApprovals.length > 0 && (
                <span className="px-2 py-0.5 text-xs font-bold bg-red-100 text-red-600 rounded-full">
                  {pendingApprovals.length}
                </span>
              )}
            </div>
            <button
              onClick={() => onViewChange('inbox')}
              className="text-xs font-bold text-brand-500 hover:text-brand-600 flex items-center gap-1"
            >
              {t('dashboard.viewAll')}
              <ChevronRight size={14} />
            </button>
          </div>

          {pendingApprovals.length === 0 ? (
            <div className="text-center py-8 text-slate-400">
              <CheckCircle2 className="mx-auto h-12 w-12 mb-3 opacity-50" />
              <p>No pending approvals</p>
            </div>
          ) : (
            <div className="space-y-4">
              {pendingApprovals.slice(0, 5).map((item) => (
                <div
                  key={item.ulid}
                  className="flex items-center justify-between p-4 rounded-xl border border-slate-100 hover:border-brand-200 hover:bg-brand-50/30 transition-all cursor-pointer group"
                  onClick={() => onViewChange('inbox')}
                >
                  <div className="flex items-center gap-4">
                    <div className={cn(
                      "w-10 h-10 rounded-lg flex items-center justify-center",
                      item.risk_level === 'high' ? "bg-red-50 text-red-500" :
                        item.risk_level === 'medium' ? "bg-orange-50 text-orange-500" : "bg-blue-50 text-blue-500"
                    )}>
                      <AlertCircle size={20} />
                    </div>
                    <div>
                      <p className="font-bold text-slate-900 group-hover:text-brand-600 transition-colors">{item.tool_name}</p>
                      <div className="flex items-center gap-2 mt-1">
                        <span className="text-xs font-medium text-slate-500">{item.tool_type}</span>
                        <span className="text-[10px] text-slate-300">•</span>
                        <span className="text-xs text-slate-400">{formatTimeAgo(item.created_at)}</span>
                      </div>
                    </div>
                  </div>
                  <div className="flex items-center gap-3">
                    <span className={cn(
                      "text-[10px] font-bold uppercase tracking-wider px-2 py-1 rounded-md",
                      item.risk_level === 'high' ? "bg-red-100 text-red-700" :
                        item.risk_level === 'medium' ? "bg-orange-100 text-orange-700" : "bg-blue-100 text-blue-700"
                    )}>
                      {item.risk_level} Risk
                    </span>
                    <ChevronRight size={16} className="text-slate-300 group-hover:text-brand-500 transition-colors" />
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Recent Conversations */}
        <div className="bg-white border border-slate-200 rounded-2xl p-6 shadow-sm">
          <div className="flex items-center justify-between mb-6">
            <div className="flex items-center gap-2">
              <MessageSquare className="text-slate-400" size={20} />
              <h3 className="font-bold text-slate-900">{t('dashboard.recentConversations')}</h3>
            </div>
          </div>

          {recentSessions.length === 0 ? (
            <div className="text-center py-8 text-slate-400">
              <MessageSquare className="mx-auto h-12 w-12 mb-3 opacity-50" />
              <p>No recent conversations</p>
            </div>
          ) : (
            <div className="space-y-6">
              {recentSessions.slice(0, 5).map((chat) => (
                <div
                  key={chat.ulid}
                  className="flex gap-3 cursor-pointer group"
                  onClick={() => onViewChange('chat')}
                >
                  <div className="w-10 h-10 rounded-full bg-slate-100 flex items-center justify-center shrink-0 group-hover:bg-brand-50 transition-colors">
                    <Users size={18} className="text-slate-400 group-hover:text-brand-500" />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex justify-between items-start mb-0.5">
                      <p className="text-sm font-bold text-slate-900 truncate group-hover:text-brand-600 transition-colors">{chat.title || 'Untitled'}</p>
                      <span className="text-[10px] text-slate-400 shrink-0">{formatTimeAgo(chat.updated_at)}</span>
                    </div>
                    <p className="text-xs text-slate-500 truncate">{chat.channel || 'Direct'}</p>
                  </div>
                </div>
              ))}

              <button
                onClick={() => onViewChange('chat')}
                className="w-full py-2 text-xs font-bold text-slate-400 hover:text-brand-500 transition-colors border-t border-slate-50 mt-2"
              >
                {t('chat.history')}
              </button>
            </div>
          )}
        </div>
      </div>

      {/* Agent Usage Ranking */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
        <div className="bg-white border border-slate-200 rounded-2xl p-6 shadow-sm">
          <h3 className="font-bold text-slate-900 mb-6">{t('dashboard.agentUsageRanking')}</h3>
          {agentRanking.length === 0 ? (
            <div className="text-center py-8 text-slate-400">
              <Users className="mx-auto h-12 w-12 mb-3 opacity-50" />
              <p>No agent usage data</p>
            </div>
          ) : (
            <div className="space-y-4">
              {agentRanking.slice(0, 4).map((item) => {
                const maxSessions = agentRanking[0]?.session_count || 1;
                const percentage = Math.round((item.session_count / maxSessions) * 100);
                return (
                  <div key={item.agent_id} className="space-y-2">
                    <div className="flex justify-between text-xs">
                      <span className="font-bold text-slate-700">{item.agent_name}</span>
                      <span className="text-slate-400">{item.session_count} sessions</span>
                    </div>
                    <div className="h-1.5 w-full bg-slate-100 rounded-full overflow-hidden">
                      <div
                        className="h-full bg-brand-500 rounded-full"
                        style={{ width: `${percentage}%` }}
                      />
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>

        {/* Channel Activity */}
        <div className="lg:col-span-2 bg-white border border-slate-200 rounded-2xl p-6 shadow-sm">
          <h3 className="font-bold text-slate-900 mb-6">{t('dashboard.channelActivity')}</h3>
          {channelActivity.length === 0 ? (
            <div className="text-center py-8 text-slate-400">
              <Globe className="mx-auto h-12 w-12 mb-3 opacity-50" />
              <p>No channel activity data</p>
            </div>
          ) : (
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
              {channelActivity.slice(0, 4).map((channel) => (
                <div key={channel.channel_id} className="p-4 rounded-xl border border-slate-100 bg-slate-50/50 hover:border-brand-100 transition-colors">
                  <div className="flex items-center gap-2 mb-2">
                    <div className={cn("w-2 h-2 rounded-full", channel.status === 'active' ? "bg-green-500" : "bg-slate-300")} />
                    <span className="text-xs font-bold text-slate-700">{channel.channel_name}</span>
                  </div>
                  <p className="text-lg font-bold text-slate-900">{formatNumber(channel.message_count)}</p>
                  <p className="text-[10px] text-slate-400 uppercase font-bold tracking-wider">Messages</p>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
