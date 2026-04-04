import React from 'react';
import {
  LayoutDashboard,
  Workflow,
  Users,
  Zap,
  Box,
  Database,
  Cpu,
  MessageSquare,
  Settings,
  ChevronLeft,
  ChevronRight,
  Globe,
  Inbox
} from 'lucide-react';
import { cn } from '../lib/utils';
import { View } from '../types';
import { useTranslation } from 'react-i18next';

interface SidebarProps {
  activeView: View;
  onViewChange: (view: View) => void;
}

export function Sidebar({ activeView, onViewChange }: SidebarProps) {
  const [isCollapsed, setIsCollapsed] = React.useState(false);
  const { t, i18n } = useTranslation();

  const navItems = [
    { id: 'dashboard', label: t('sidebar.dashboard'), icon: LayoutDashboard },
    { id: 'inbox', label: t('sidebar.inbox'), icon: Inbox },
    { id: 'orchestrator', label: t('sidebar.orchestrator'), icon: Workflow },
    { id: 'agents', label: t('sidebar.agents'), icon: Users },
    { id: 'chat', label: t('sidebar.chat'), icon: MessageSquare },
    { id: 'skills', label: t('sidebar.skills'), icon: Zap },
    { id: 'knowledge', label: t('sidebar.knowledge'), icon: Database },
    { id: 'models', label: t('sidebar.models'), icon: Cpu },
  ];

  const toggleLanguage = () => {
    const nextLang = i18n.language === 'en' ? 'zh' : 'en';
    i18n.changeLanguage(nextLang);
  };

  return (
    <aside 
      className={cn(
        "flex flex-col h-screen bg-slate-900 text-slate-300 transition-all duration-300 border-r border-slate-800",
        isCollapsed ? "w-16" : "w-64"
      )}
    >
      <div className="p-4 flex items-center justify-between border-b border-slate-800">
        {!isCollapsed && (
          <div className="flex items-center gap-2">
            <div className="w-8 h-8 rounded-lg bg-brand-500 flex items-center justify-center">
              <Zap className="w-5 h-5 text-white fill-white" />
            </div>
            <span className="font-bold text-white tracking-tight">{t('sidebar.brand')}</span>
          </div>
        )}
        <button 
          onClick={() => setIsCollapsed(!isCollapsed)}
          className="p-1 hover:bg-slate-800 rounded-md transition-colors"
        >
          {isCollapsed ? <ChevronRight size={20} /> : <ChevronLeft size={20} />}
        </button>
      </div>

      <nav className="flex-1 overflow-y-auto py-4 px-2 space-y-1 scrollbar-hide">
        {navItems.map((item) => {
          const Icon = item.icon;
          const isActive = activeView === item.id;
          
          return (
            <button
              key={item.id}
              onClick={() => onViewChange(item.id as View)}
              className={cn(
                "w-full flex items-center gap-3 px-3 py-2 rounded-lg transition-all group",
                isActive
                  ? "bg-brand-500/10 text-brand-500"
                  : "hover:bg-slate-800 hover:text-white"
              )}
            >
              <Icon className={cn(
                "w-5 h-5 shrink-0",
                isActive ? "text-brand-500" : "text-slate-400 group-hover:text-white"
              )} />
              {!isCollapsed && (
                <span className="text-sm font-medium">{item.label}</span>
              )}
              {!isCollapsed && item.badge && item.badge > 0 && (
                <div className="ml-auto px-2 py-0.5 rounded-full bg-red-500 text-white text-xs font-bold">
                  {item.badge}
                </div>
              )}
              {isActive && !isCollapsed && !item.badge && (
                <div className="ml-auto w-1.5 h-1.5 rounded-full bg-brand-500" />
              )}
            </button>
          );
        })}
      </nav>

      <div className="p-4 border-t border-slate-800 space-y-2">
        <button 
          onClick={toggleLanguage}
          className="w-full flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-slate-800 hover:text-white transition-all"
        >
          <Globe className="w-5 h-5 text-slate-400" />
          {!isCollapsed && <span className="text-sm font-medium">{i18n.language === 'en' ? 'English' : '中文'}</span>}
        </button>
        <button 
          onClick={() => onViewChange('settings')}
          className={cn(
            "w-full flex items-center gap-3 px-3 py-2 rounded-lg transition-all group",
            activeView === 'settings' 
              ? "bg-brand-500/10 text-brand-500" 
              : "hover:bg-slate-800 hover:text-white"
          )}
        >
          <Settings className={cn(
            "w-5 h-5 shrink-0",
            activeView === 'settings' ? "text-brand-500" : "text-slate-400 group-hover:text-white"
          )} />
          {!isCollapsed && <span className="text-sm font-medium">{t('sidebar.settings')}</span>}
        </button>
      </div>
    </aside>
  );
}
