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
        "flex flex-col h-screen transition-all duration-300 border-r border-gray-200",
        isCollapsed ? "w-16" : "w-64",
        // 背景色 #F1F4F9，文字色 #1F2937（深灰）
        "bg-[#F1F4F9] text-gray-700"
      )}
    >
      <div className="p-4 flex items-center justify-between border-b border-gray-200">
        {!isCollapsed && (
          <div className="flex items-center gap-2">
            <div className="w-8 h-8 rounded-lg bg-brand-500 flex items-center justify-center">
              <Zap className="w-5 h-5 text-white fill-white" />
            </div>
            <span className="font-bold text-gray-900 tracking-tight">{t('sidebar.brand')}</span>
          </div>
        )}
        <button
          onClick={() => setIsCollapsed(!isCollapsed)}
          className="p-1 hover:bg-gray-200 rounded-md transition-colors text-gray-500"
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
                  ? "bg-slate-200 text-slate-800 shadow-sm"
                  : "hover:bg-gray-200 text-gray-700"
              )}
            >
              <Icon className={cn(
                "w-5 h-5 shrink-0",
                isActive ? "text-slate-700" : "text-gray-500 group-hover:text-gray-700"
              )} />
              {!isCollapsed && (
                <span className="text-sm font-medium">{item.label}</span>
              )}
              {!isCollapsed && item.badge && item.badge > 0 && (
                <div className="ml-auto px-2 py-0.5 rounded-full bg-red-500 text-white text-xs font-bold">
                  {item.badge}
                </div>
              )}
            </button>
          );
        })}
      </nav>

      <div className="p-4 border-t border-gray-200 space-y-2">
        <button
          onClick={toggleLanguage}
          className="w-full flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-gray-200 text-gray-700 transition-all"
        >
          <Globe className="w-5 h-5 shrink-0 text-gray-500" />
          {!isCollapsed && <span className="text-sm font-medium">{i18n.language === 'en' ? 'English' : '中文'}</span>}
        </button>
        <button
          onClick={() => onViewChange('settings')}
          className={cn(
            "w-full flex items-center gap-3 px-3 py-2 rounded-lg transition-all group",
            activeView === 'settings'
              ? "bg-slate-200 text-slate-800 shadow-sm"
              : "hover:bg-gray-200 text-gray-700"
          )}
        >
          <Settings className={cn(
            "w-5 h-5 shrink-0",
            activeView === 'settings' ? "text-slate-700" : "text-gray-500 group-hover:text-gray-700"
          )} />
          {!isCollapsed && <span className="text-sm font-medium">{t('sidebar.settings')}</span>}
        </button>
      </div>
    </aside>
  );
}
