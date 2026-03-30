import React from 'react';
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
  Globe,
  CheckCircle2,
  AlertCircle,
  ChevronRight
} from 'lucide-react';
import { cn } from '../lib/utils';
import { useTranslation } from 'react-i18next';
import { View } from '../types';
import { motion } from 'motion/react';

interface DashboardProps {
  onViewChange: (view: View) => void;
}

export function Dashboard({ onViewChange }: DashboardProps) {
  const { t } = useTranslation();

  const stats = [
    { label: t('dashboard.activeAgents'), value: '12', change: '+2', trend: 'up', icon: Users, color: 'text-blue-500', bg: 'bg-blue-50', view: 'agents' as View },
    { label: t('dashboard.periodicAgents'), value: '4', change: '+1', trend: 'up', icon: Clock, color: 'text-purple-500', bg: 'bg-purple-50', view: 'agents' as View },
    { label: t('dashboard.tasksCompleted'), value: '1,428', change: '+12%', trend: 'up', icon: Zap, color: 'text-orange-500', bg: 'bg-orange-50' },
    { label: t('dashboard.totalTokens'), value: '2.4M', change: '+18%', trend: 'up', icon: Cpu, color: 'text-indigo-500', bg: 'bg-indigo-50' },
    { label: t('knowledge.activeSources'), value: '1', change: '+1', trend: 'up', icon: Database, color: 'text-green-500', bg: 'bg-green-50', view: 'knowledge' as View },
  ];

  const quickActions = [
    { label: t('dashboard.createAgent'), icon: Plus, view: 'orchestrator' as View, color: 'bg-brand-500 text-white hover:bg-brand-600' },
    { label: t('dashboard.addKnowledge'), icon: Database, view: 'knowledge' as View, color: 'bg-white text-slate-700 border border-slate-200 hover:bg-slate-50' },
    { label: t('dashboard.configureModel'), icon: Cpu, view: 'models' as View, color: 'bg-white text-slate-700 border border-slate-200 hover:bg-slate-50' },
  ];

  const pendingApprovals = [
    { id: 1, title: 'Sensitive Data Access', agent: 'Research-Agent', time: '2m ago', risk: 'High', type: 'Tool Call' },
    { id: 2, title: 'External API Execution', agent: 'Data-Analyzer', time: '15m ago', risk: 'Medium', type: 'A2A' },
    { id: 3, title: 'Knowledge Base Update', agent: 'System', time: '1h ago', risk: 'Low', type: 'Knowledge' },
  ];

  const recentConversations = [
    { id: 1, name: 'Research-Agent', lastMessage: 'The analysis of the market trends is complete.', time: '5m ago' },
    { id: 2, name: 'Customer-Support', lastMessage: 'How can I assist you with your order today?', time: '12m ago' },
    { id: 3, name: 'Coding-Assistant', lastMessage: 'I have refactored the authentication logic.', time: '45m ago' },
  ];

  return (
    <div className="p-8 max-w-7xl mx-auto space-y-8">
      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">{t('dashboard.welcome')}</h1>
          <p className="text-slate-500 mt-1">{t('dashboard.subtitle')}</p>
        </div>

        {/* Quick Actions */}
        <div className="flex items-center gap-3">
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
              <div className={cn(
                "flex items-center gap-1 text-xs font-bold px-2 py-1 rounded-full",
                stat.trend === 'up' ? "bg-green-50 text-green-600" : "bg-blue-50 text-blue-600"
              )}>
                {stat.trend === 'up' ? <ArrowUpRight size={12} /> : <ArrowDownRight size={12} />}
                {stat.change}
              </div>
            </div>
            <p className="text-sm font-medium text-slate-500">{stat.label}</p>
            <p className="text-2xl font-bold text-slate-900 mt-1">{stat.value}</p>
          </motion.div>
        ))}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
        {/* Pending Approvals - Core Workflow */}
        <div className="lg:col-span-2 bg-white border border-slate-200 rounded-2xl p-6 shadow-sm">
          <div className="flex items-center justify-between mb-6">
            <div className="flex items-center gap-2">
              <ShieldCheck className="text-brand-500" size={20} />
              <h3 className="font-bold text-slate-900">{t('dashboard.pendingApprovals')}</h3>
            </div>
            <button
              onClick={() => onViewChange('inbox')}
              className="text-xs font-bold text-brand-500 hover:text-brand-600 flex items-center gap-1"
            >
              {t('dashboard.viewAll')}
              <ChevronRight size={14} />
            </button>
          </div>

          <div className="space-y-4">
            {pendingApprovals.map((item) => (
              <div
                key={item.id}
                className="flex items-center justify-between p-4 rounded-xl border border-slate-100 hover:border-brand-200 hover:bg-brand-50/30 transition-all cursor-pointer group"
                onClick={() => onViewChange('inbox')}
              >
                <div className="flex items-center gap-4">
                  <div className={cn(
                    "w-10 h-10 rounded-lg flex items-center justify-center",
                    item.risk === 'High' ? "bg-red-50 text-red-500" :
                      item.risk === 'Medium' ? "bg-orange-50 text-orange-500" : "bg-blue-50 text-blue-500"
                  )}>
                    <AlertCircle size={20} />
                  </div>
                  <div>
                    <p className="font-bold text-slate-900 group-hover:text-brand-600 transition-colors">{item.title}</p>
                    <div className="flex items-center gap-2 mt-1">
                      <span className="text-xs font-medium text-slate-500">{item.agent}</span>
                      <span className="text-[10px] text-slate-300">•</span>
                      <span className="text-xs text-slate-400">{item.time}</span>
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-3">
                  <span className={cn(
                    "text-[10px] font-bold uppercase tracking-wider px-2 py-1 rounded-md",
                    item.risk === 'High' ? "bg-red-100 text-red-700" :
                      item.risk === 'Medium' ? "bg-orange-100 text-orange-700" : "bg-blue-100 text-blue-700"
                  )}>
                    {item.risk} Risk
                  </span>
                  <ChevronRight size={16} className="text-slate-300 group-hover:text-brand-500 transition-colors" />
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Recent Conversations */}
        <div className="bg-white border border-slate-200 rounded-2xl p-6 shadow-sm">
          <div className="flex items-center justify-between mb-6">
            <div className="flex items-center gap-2">
              <MessageSquare className="text-slate-400" size={20} />
              <h3 className="font-bold text-slate-900">{t('dashboard.recentConversations')}</h3>
            </div>
          </div>

          <div className="space-y-6">
            {recentConversations.map((chat) => (
              <div
                key={chat.id}
                className="flex gap-3 cursor-pointer group"
                onClick={() => onViewChange('chat')}
              >
                <div className="w-10 h-10 rounded-full bg-slate-100 flex items-center justify-center shrink-0 group-hover:bg-brand-50 transition-colors">
                  <Users size={18} className="text-slate-400 group-hover:text-brand-500" />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex justify-between items-start mb-0.5">
                    <p className="text-sm font-bold text-slate-900 truncate group-hover:text-brand-600 transition-colors">{chat.name}</p>
                    <span className="text-[10px] text-slate-400 shrink-0">{chat.time}</span>
                  </div>
                  <p className="text-xs text-slate-500 truncate">{chat.lastMessage}</p>
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
        </div>
      </div>

      {/* Token Usage Ranking - Kept as it is a real list */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
        <div className="bg-white border border-slate-200 rounded-2xl p-6 shadow-sm">
          <h3 className="font-bold text-slate-900 mb-6">{t('dashboard.tokenUsageRanking')}</h3>
          <div className="space-y-4">
            {[
              { name: 'Research-Agent', usage: '1.2M', percentage: 85 },
              { name: 'Customer-Support', usage: '850K', percentage: 65 },
              { name: 'Data-Analyzer', usage: '420K', percentage: 35 },
              { name: 'Coding-Assistant', usage: '120K', percentage: 15 },
            ].map((item, i) => (
              <div key={i} className="space-y-2">
                <div className="flex justify-between text-xs">
                  <span className="font-bold text-slate-700">{item.name}</span>
                  <span className="text-slate-400">{item.usage} tokens</span>
                </div>
                <div className="h-1.5 w-full bg-slate-100 rounded-full overflow-hidden">
                  <div
                    className="h-full bg-brand-500 rounded-full"
                    style={{ width: `${item.percentage}%` }}
                  />
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Channel Activity */}
        <div className="lg:col-span-2 bg-white border border-slate-200 rounded-2xl p-6 shadow-sm">
          <h3 className="font-bold text-slate-900 mb-6">{t('dashboard.channelActivity')}</h3>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            {[
              { name: 'Feishu', status: 'Active', messages: '1,240' },
              { name: 'DingTalk', status: 'Active', messages: '850' },
              { name: 'WeChat', status: 'Inactive', messages: '0' },
              { name: 'API', status: 'Active', messages: '4,500' },
            ].map((channel, i) => (
              <div key={i} className="p-4 rounded-xl border border-slate-100 bg-slate-50/50 hover:border-brand-100 transition-colors">
                <div className="flex items-center gap-2 mb-2">
                  <div className={cn("w-2 h-2 rounded-full", channel.status === 'Active' ? "bg-green-500" : "bg-slate-300")} />
                  <span className="text-xs font-bold text-slate-700">{channel.name}</span>
                </div>
                <p className="text-lg font-bold text-slate-900">{channel.messages}</p>
                <p className="text-[10px] text-slate-400 uppercase font-bold tracking-wider">Messages</p>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
