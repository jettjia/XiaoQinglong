import React from 'react';
import { Settings as SettingsIcon, Save, RotateCcw, FileCode, Info } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { cn } from '../lib/utils';

const DEFAULT_YAML = `# Global Configuration for OpenClaw Integrations
# -------------------------------------------

# Feishu (Lark) Integration
feishu:
  enabled: true
  app_id: "cli_a1b2c3d4e5f6"
  app_secret: "****************"
  verification_token: "v_12345678"
  encrypt_key: ""

# DingTalk Integration
dingtalk:
  enabled: false
  app_key: "ding123456"
  app_secret: "****************"

# MCP (Model Context Protocol) Settings
mcp:
  enabled: true
  servers:
    - name: "default"
      url: "http://localhost:8080"
      token: ""

# Orchestrator Defaults
orchestrator:
  default_reasoning_model: "gemini-3-pro-preview"
  default_generation_model: "gemini-3-flash-preview"
  max_steps: 15
  timeout_seconds: 300

# UI Preferences
ui:
  theme: "light"
  language: "zh"
  sidebar_collapsed: false
`;

export function Settings() {
  const { t } = useTranslation();
  const [yamlContent, setYamlContent] = React.useState(DEFAULT_YAML);
  const [isSaved, setIsSaved] = React.useState(false);

  const handleSave = () => {
    // In a real app, this would send the YAML to the backend
    console.log('Saving YAML configuration:', yamlContent);
    setIsSaved(true);
    setTimeout(() => setIsSaved(false), 3000);
  };

  const handleReset = () => {
    if (window.confirm(t('settings.confirmReset'))) {
      setYamlContent(DEFAULT_YAML);
    }
  };

  return (
    <div className="h-full flex flex-col bg-slate-50 overflow-hidden">
      {/* Header */}
      <header className="h-16 border-b border-slate-200 bg-white flex items-center justify-between px-6 shrink-0 z-20">
        <div className="flex items-center gap-4">
          <div className="w-10 h-10 rounded-xl bg-slate-100 flex items-center justify-center text-slate-600">
            <SettingsIcon size={20} />
          </div>
          <div>
            <h1 className="text-lg font-bold text-slate-900">{t('settings.title')}</h1>
            <p className="text-xs text-slate-500 font-medium uppercase tracking-wider">{t('settings.subtitle')}</p>
          </div>
        </div>
        <div className="flex items-center gap-3">
          <button 
            onClick={handleReset}
            className="flex items-center gap-2 px-4 py-2 bg-white border border-slate-200 rounded-lg text-sm font-bold text-slate-600 hover:bg-slate-50 transition-all"
          >
            <RotateCcw size={16} />
            {t('settings.reset')}
          </button>
          <button 
            onClick={handleSave}
            className={cn(
              "flex items-center gap-2 px-6 py-2 rounded-lg text-sm font-bold transition-all shadow-lg",
              isSaved 
                ? "bg-green-500 text-white shadow-green-500/20" 
                : "bg-brand-500 text-white hover:bg-brand-600 shadow-brand-500/20"
            )}
          >
            <Save size={16} />
            {isSaved ? t('settings.saved') : t('settings.save')}
          </button>
        </div>
      </header>

      <div className="flex-1 overflow-hidden p-6 flex gap-6">
        {/* YAML Editor */}
        <div className="flex-1 flex flex-col bg-white rounded-2xl border border-slate-200 shadow-sm overflow-hidden">
          <div className="px-4 py-3 border-b border-slate-100 bg-slate-50/50 flex items-center justify-between">
            <div className="flex items-center gap-2">
              <FileCode size={16} className="text-slate-400" />
              <span className="text-xs font-bold text-slate-600 uppercase tracking-wider">config.yaml</span>
            </div>
            <div className="flex gap-1.5">
              <div className="w-2.5 h-2.5 rounded-full bg-slate-200" />
              <div className="w-2.5 h-2.5 rounded-full bg-slate-200" />
              <div className="w-2.5 h-2.5 rounded-full bg-slate-200" />
            </div>
          </div>
          <div className="flex-1 relative">
            <textarea
              value={yamlContent}
              onChange={(e) => setYamlContent(e.target.value)}
              spellCheck={false}
              className="absolute inset-0 w-full h-full p-6 font-mono text-sm text-slate-800 bg-transparent resize-none focus:ring-0 border-none leading-relaxed"
            />
          </div>
        </div>

        {/* Info Panel */}
        <div className="w-80 shrink-0 space-y-6">
          <div className="bg-white rounded-2xl border border-slate-200 p-6 shadow-sm">
            <div className="flex items-center gap-2 text-slate-900 mb-4">
              <Info size={18} className="text-brand-500" />
              <h2 className="font-bold">{t('settings.infoTitle')}</h2>
            </div>
            <div className="space-y-4">
              <p className="text-xs text-slate-500 leading-relaxed">
                {t('settings.infoDescription')}
              </p>
              <div className="p-3 bg-brand-50 rounded-xl border border-brand-100">
                <p className="text-[10px] font-bold text-brand-600 uppercase tracking-wider mb-1">
                  {t('settings.tipTitle')}
                </p>
                <p className="text-[11px] text-brand-700 leading-relaxed">
                  {t('settings.tipDescription')}
                </p>
              </div>
            </div>
          </div>

          <div className="bg-slate-900 rounded-2xl p-6 text-white shadow-xl shadow-slate-900/20">
            <h3 className="text-sm font-bold mb-3">{t('settings.helpTitle')}</h3>
            <ul className="space-y-2">
              <li className="text-[11px] text-slate-400 flex items-start gap-2">
                <div className="w-1 h-1 rounded-full bg-brand-500 mt-1.5 shrink-0" />
                <span>{t('settings.help1')}</span>
              </li>
              <li className="text-[11px] text-slate-400 flex items-start gap-2">
                <div className="w-1 h-1 rounded-full bg-brand-500 mt-1.5 shrink-0" />
                <span>{t('settings.help2')}</span>
              </li>
              <li className="text-[11px] text-slate-400 flex items-start gap-2">
                <div className="w-1 h-1 rounded-full bg-brand-500 mt-1.5 shrink-0" />
                <span>{t('settings.help3')}</span>
              </li>
            </ul>
          </div>
        </div>
      </div>
    </div>
  );
}
