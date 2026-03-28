import React from 'react';
import { 
  Users, 
  Zap, 
  Database, 
  Cpu, 
  TrendingUp, 
  ArrowUpRight, 
  ArrowDownRight,
  Clock
} from 'lucide-react';
import { cn } from '../lib/utils';
import { useTranslation } from 'react-i18next';

export function Dashboard() {
  const { t } = useTranslation();
  
  const stats = [
    { label: t('dashboard.activeAgents'), value: '12', change: '+2', trend: 'up', icon: Users, color: 'text-blue-500', bg: 'bg-blue-50' },
    { label: t('dashboard.tasksCompleted'), value: '1,428', change: '+12%', trend: 'up', icon: Zap, color: 'text-orange-500', bg: 'bg-orange-50' },
    { label: t('knowledge.activeSources'), value: '1', change: '+1', trend: 'up', icon: Database, color: 'text-green-500', bg: 'bg-green-50' },
    { label: t('dashboard.modelLatency'), value: '0.8s', change: '-5%', trend: 'down', icon: Cpu, color: 'text-purple-500', bg: 'bg-purple-50' },
  ];

  return (
    <div className="p-8 max-w-7xl mx-auto space-y-8">
      <div>
        <h1 className="text-2xl font-bold text-slate-900">{t('dashboard.welcome')}</h1>
        <p className="text-slate-500 mt-1">{t('dashboard.subtitle')}</p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        {stats.map((stat) => (
          <div key={stat.label} className="bg-white p-6 rounded-2xl border border-slate-200 shadow-sm hover:shadow-md transition-all">
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
          </div>
        ))}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
        <div className="lg:col-span-2 bg-white border border-slate-200 rounded-2xl p-6">
          <div className="flex items-center justify-between mb-6">
            <h3 className="font-bold text-slate-900">{t('dashboard.systemActivity')}</h3>
            <select className="text-xs font-bold text-slate-500 bg-slate-50 border-none rounded-lg focus:ring-0">
              <option>Last 7 Days</option>
              <option>Last 30 Days</option>
            </select>
          </div>
          <div className="h-64 w-full flex items-end gap-2 px-2">
            {[40, 65, 45, 90, 55, 70, 85, 60, 75, 50, 80, 95].map((h, i) => (
              <div key={i} className="flex-1 bg-brand-500/10 rounded-t-md relative group">
                <div 
                  className="absolute bottom-0 left-0 right-0 bg-brand-500 rounded-t-md transition-all duration-500 group-hover:bg-brand-600" 
                  style={{ height: `${h}%` }}
                />
              </div>
            ))}
          </div>
          <div className="flex justify-between mt-4 px-2">
            {['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'].map(d => (
              <span key={d} className="text-[10px] font-bold text-slate-400 uppercase tracking-wider">{d}</span>
            ))}
          </div>
        </div>

        <div className="bg-white border border-slate-200 rounded-2xl p-6">
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
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
        <div className="bg-white border border-slate-200 rounded-2xl p-6">
          <h3 className="font-bold text-slate-900 mb-6">{t('dashboard.channelActivity')}</h3>
          <div className="grid grid-cols-2 gap-4">
            {[
              { name: 'Feishu', status: 'Active', color: 'bg-blue-500', messages: '1,240' },
              { name: 'DingTalk', status: 'Active', color: 'bg-blue-600', messages: '850' },
              { name: 'WeChat', status: 'Inactive', color: 'bg-green-500', messages: '0' },
              { name: 'API', status: 'Active', color: 'bg-slate-900', messages: '4,500' },
            ].map((channel, i) => (
              <div key={i} className="p-4 rounded-xl border border-slate-100 bg-slate-50/50">
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

        <div className="bg-white border border-slate-200 rounded-2xl p-6">
          <h3 className="font-bold text-slate-900 mb-6">{t('dashboard.sandboxMonitoring')}</h3>
          <div className="space-y-6">
            <div className="grid grid-cols-3 gap-4">
              <div className="text-center">
                <p className="text-2xl font-bold text-brand-500">12</p>
                <p className="text-[10px] text-slate-400 uppercase font-bold tracking-wider">{t('dashboard.activeContainers')}</p>
              </div>
              <div className="text-center">
                <p className="text-2xl font-bold text-slate-900">24%</p>
                <p className="text-[10px] text-slate-400 uppercase font-bold tracking-wider">{t('dashboard.cpuUsage')}</p>
              </div>
              <div className="text-center">
                <p className="text-2xl font-bold text-slate-900">1.2GB</p>
                <p className="text-[10px] text-slate-400 uppercase font-bold tracking-wider">{t('dashboard.memoryUsage')}</p>
              </div>
            </div>
            <div className="h-24 w-full flex items-end gap-1 px-2">
              {[20, 25, 22, 30, 28, 35, 32, 40, 38, 45, 42, 50, 48, 55, 52, 60, 58, 65, 62, 70].map((h, i) => (
                <div key={i} className="flex-1 bg-brand-500/20 rounded-t-sm" style={{ height: `${h}%` }} />
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

const Box = ({ size, className }: { size: number, className?: string }) => (
  <svg 
    width={size} 
    height={size} 
    viewBox="0 0 24 24" 
    fill="none" 
    stroke="currentColor" 
    strokeWidth="2" 
    strokeLinecap="round" 
    strokeLinejoin="round" 
    className={className}
  >
    <path d="M21 8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16Z" />
    <path d="m3.3 7 8.7 5 8.7-5" />
    <path d="M12 22V12" />
  </svg>
);
