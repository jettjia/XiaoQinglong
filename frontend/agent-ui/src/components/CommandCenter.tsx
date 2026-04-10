import React, { useState, useRef, useEffect } from 'react';
import {
  Sparkles,
  X,
  Send,
  Bot,
  Zap,
  Database,
  Cpu,
  Inbox as InboxIcon,
  Check,
  AlertCircle,
  Loader2,
  Plus,
  ArrowRight,
  Trash2
} from 'lucide-react';
import { motion, AnimatePresence, useDragControls } from 'motion/react';
import { cn } from '../lib/utils';
import { commandApi, CommandResult } from '../lib/api';
import { toast } from 'sonner';
import { useTranslation } from 'react-i18next';

interface CommandCenterProps {
  onViewChange: (view: any, data?: any) => void;
}

type Intent = 'add_model' | 'create_agent' | 'install_skill' | 'show_inbox' | 'config_kb' | 'test_kb_recall' | 'unknown';

interface CommandAction {
  id: string;
  intent: Intent;
  title: string;
  description: string;
  data: any;
  status: 'pending' | 'executing' | 'completed' | 'failed';
  result?: CommandResult;
}

export function CommandCenter({ onViewChange }: CommandCenterProps) {
  const { t } = useTranslation();
  const [isOpen, setIsOpen] = useState(false);
  const [input, setInput] = useState('');
  const [isProcessing, setIsProcessing] = useState(false);
  const [actions, setActions] = useState<CommandAction[]>([]);
  const [skillGuidanceVisible, setSkillGuidanceVisible] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);
  const dragControls = useDragControls();

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [actions]);

  const handleCommand = async () => {
    if (!input.trim() || isProcessing) return;

    setIsProcessing(true);
    const userQuery = input;
    setInput('');

    try {
      const cmdResult = await commandApi.execute(userQuery);

      // 如果没有 prefilled 数据，直接执行不需要确认
      const needsConfirmation = !!cmdResult.prefilled;

      const newAction: CommandAction = {
        id: Math.random().toString(36).substr(2, 9),
        intent: cmdResult.action as Intent,
        title: cmdResult.action,
        description: cmdResult.message || '',
        data: cmdResult.prefilled || cmdResult.result || {},
        status: needsConfirmation ? 'pending' : 'executing', // 有预填充数据需要确认，否则直接执行
        result: cmdResult
      };

      setActions(prev => [...prev, newAction]);

      // 无需确认的直接执行
      if (!needsConfirmation) {
        const action = newAction;
        // 使用 setTimeout 避免在 setState 过程中调用
        setTimeout(() => executeAction(action), 0);
      }
    } catch (error) {
      console.error('Command Error:', error);
      toast.error('Failed to process command. Please try again.');
    } finally {
      setIsProcessing(false);
    }
  };

  const executeAction = async (action: CommandAction) => {
    setActions(prev => prev.map(a => a.id === action.id ? { ...a, status: 'executing' } : a));

    try {
      const result = action.result;

      if (!result) {
        throw new Error('No result from command execution');
      }

      // Show toast message
      if (result.message) {
        if (result.success) {
          toast.success(result.message);
        } else {
          toast.error(result.message);
        }
      }

      // Handle skill guidance
      if (result.action === 'install_skill' && result.show_guidance) {
        setSkillGuidanceVisible(true);
        setActions(prev => prev.map(a => a.id === action.id ? { ...a, status: 'completed' } : a));
        return;
      }

      // Handle navigation
      if (result.navigate_to) {
        onViewChange(result.navigate_to, result.prefilled);
      }

      setActions(prev => prev.map(a => a.id === action.id ? { ...a, status: 'completed' } : a));
    } catch (error) {
      setActions(prev => prev.map(a => a.id === action.id ? { ...a, status: 'failed' } : a));
      toast.error('Execution failed.');
    }
  };

  const getIcon = (intent: Intent) => {
    switch (intent) {
      case 'add_model': return <Cpu className="text-purple-500" size={18} />;
      case 'create_agent': return <Bot className="text-blue-500" size={18} />;
      case 'install_skill': return <Zap className="text-orange-500" size={18} />;
      case 'show_inbox': return <InboxIcon className="text-emerald-500" size={18} />;
      case 'config_kb': return <Database className="text-indigo-500" size={18} />;
      case 'test_kb_recall': return <Database className="text-indigo-500" size={18} />;
      default: return <Sparkles className="text-slate-400" size={18} />;
    }
  };

  return (
    <>
      {/* Floating Trigger Button */}
      <motion.button
        drag
        dragConstraints={{ left: -window.innerWidth + 80, right: 0, top: -window.innerHeight + 80, bottom: 0 }}
        dragElastic={0.1}
        dragMomentum={false}
        whileHover={{ scale: 1.1 }}
        whileTap={{ scale: 0.9 }}
        onClick={() => setIsOpen(true)}
        className={cn(
          "fixed bottom-60 right-8 w-14 h-14 rounded-2xl bg-slate-900 text-white shadow-2xl flex items-center justify-center transition-opacity z-40 group cursor-move",
          isOpen && "opacity-0 pointer-events-none"
        )}
      >
        <Sparkles size={24} className="group-hover:rotate-12 transition-transform" />
        <div className="absolute -top-1 -right-1 w-4 h-4 bg-brand-500 rounded-full border-2 border-white animate-pulse" />
      </motion.button>

      <AnimatePresence>
        {isOpen && (
          <div className="fixed inset-0 z-50 flex items-center justify-center p-8 pb-32 pointer-events-none">
            {/* Backdrop for closing */}
            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              onClick={() => setIsOpen(false)}
              className="absolute inset-0 bg-slate-900/20 backdrop-blur-[2px] pointer-events-auto"
            />

            {/* Command Dialog */}
            <motion.div
              drag
              dragControls={dragControls}
              dragListener={false}
              dragConstraints={{ left: -window.innerWidth + 400, right: 0, top: -window.innerHeight + 600, bottom: 0 }}
              dragMomentum={false}
              initial={{ opacity: 0, y: 20, scale: 0.95 }}
              animate={{ opacity: 1, y: 0, scale: 1 }}
              exit={{ opacity: 0, y: 20, scale: 0.95 }}
              className="w-full max-w-md bg-white rounded-3xl shadow-2xl border border-slate-200 flex flex-col overflow-hidden pointer-events-auto relative z-10 max-h-[80vh] h-auto"
            >
              {/* Header */}
              <div
                onPointerDown={(e) => dragControls.start(e)}
                className="p-6 border-b border-slate-100 flex items-center justify-between bg-white shrink-0 cursor-move active:bg-slate-50 transition-colors"
              >
                <div className="flex items-center gap-3 pointer-events-none">
                  <div className="w-10 h-10 rounded-xl bg-slate-900 text-white flex items-center justify-center">
                    <Sparkles size={20} />
                  </div>
                  <div>
                    <h2 className="text-lg font-bold text-slate-900">{t('commandCenter.title')}</h2>
                    <p className="text-[10px] font-bold text-slate-400 uppercase tracking-widest">{t('commandCenter.subtitle')}</p>
                  </div>
                </div>
                <div className="flex items-center gap-1 pointer-events-auto">
                  {actions.length > 0 && (
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        setActions([]);
                      }}
                      className="p-2 hover:bg-red-50 rounded-xl text-slate-400 hover:text-red-500 transition-colors"
                      title="清除历史"
                    >
                      <Trash2 size={18} />
                    </button>
                  )}
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      setIsOpen(false);
                    }}
                    className="p-2 hover:bg-slate-100 rounded-xl text-slate-400 transition-colors"
                  >
                    <X size={20} />
                  </button>
                </div>
              </div>

              {/* Actions List */}
              <div
                ref={scrollRef}
                className="flex-1 overflow-y-auto p-6 space-y-4 scrollbar-hide min-h-[200px]"
              >
                {actions.length === 0 ? (
                  <div className="h-full min-h-[200px] flex flex-col items-center justify-center text-center p-8">
                    <div className="w-16 h-16 rounded-3xl bg-slate-50 flex items-center justify-center text-slate-200 mb-6">
                      <Sparkles size={32} />
                    </div>
                    <h3 className="text-sm font-bold text-slate-900 mb-2">{t('commandCenter.howCanIHelp')}</h3>
                    <p className="text-xs text-slate-500 leading-relaxed">
                      {t('commandCenter.helpDesc')}
                    </p>
                  </div>
                ) : (
                  actions.map((action) => (
                    <motion.div
                      key={action.id}
                      initial={{ opacity: 0, x: -20 }}
                      animate={{ opacity: 1, x: 0 }}
                      className={cn(
                        "p-4 rounded-2xl border transition-all",
                        action.status === 'completed' ? "bg-emerald-50 border-emerald-100" :
                          action.status === 'failed' ? "bg-red-50 border-red-100" :
                            "bg-slate-50 border-slate-100"
                      )}
                    >
                      <div className="flex items-start gap-3">
                        <div className="w-8 h-8 rounded-lg bg-white shadow-sm flex items-center justify-center shrink-0">
                          {getIcon(action.intent)}
                        </div>
                        <div className="flex-1 min-w-0">
                          <h4 className="text-sm font-bold text-slate-900 truncate">{action.title}</h4>
                          <p className="text-[11px] text-slate-500 mt-1 leading-relaxed">{action.description}</p>

                          {action.status === 'pending' && (
                            <div className="mt-4 flex gap-2">
                              <button
                                onClick={() => executeAction(action)}
                                className="flex-1 py-2 bg-slate-900 text-white rounded-xl text-[10px] font-bold hover:bg-slate-800 transition-all flex items-center justify-center gap-2"
                              >
                                {t('commandCenter.confirm')}
                                <ArrowRight size={12} />
                              </button>
                              <button
                                onClick={() => setActions(prev => prev.filter(a => a.id !== action.id))}
                                className="px-4 py-2 bg-white border border-slate-200 text-slate-500 rounded-xl text-[10px] font-bold hover:bg-slate-50 transition-all"
                              >
                                {t('commandCenter.cancel')}
                              </button>
                            </div>
                          )}

                          {action.status === 'executing' && (
                            <div className="mt-4 flex items-center gap-2 text-brand-500 text-[10px] font-bold">
                              <Loader2 size={12} className="animate-spin" />
                              {t('commandCenter.executing')}
                            </div>
                          )}

                          {action.status === 'completed' && (
                            <div className="mt-4 flex items-center gap-2 text-emerald-600 text-[10px] font-bold">
                              <Check size={12} />
                              {t('commandCenter.completed')}
                            </div>
                          )}

                          {action.status === 'completed' && action.result?.result?.results && (
                            <div className="mt-3 p-3 bg-white rounded-xl border border-slate-100">
                              <div className="text-[10px] font-bold text-slate-500 mb-2">
                                召回结果 ({action.result.result.results.length})
                              </div>
                              {action.result.result.results.map((r: any, idx: number) => (
                                <div key={idx} className="mb-2 last:mb-0">
                                  <div className="text-[11px] font-bold text-slate-700">{r.title}</div>
                                  <div className="text-[10px] text-slate-500 mt-0.5 line-clamp-2">{r.content}</div>
                                  <div className="text-[9px] text-slate-400 mt-0.5">Score: {r.score?.toFixed(2)}</div>
                                </div>
                              ))}
                            </div>
                          )}
                        </div>
                      </div>
                    </motion.div>
                  ))
                )}

                {/* Skill Guidance Card */}
                {skillGuidanceVisible && (
                  <motion.div
                    initial={{ opacity: 0, y: 20 }}
                    animate={{ opacity: 1, y: 0 }}
                    className="mt-4 p-4 rounded-2xl bg-orange-50 border border-orange-100"
                  >
                    <div className="flex items-center gap-2 mb-2">
                      <Zap className="text-orange-500" size={18} />
                      <span className="font-bold text-orange-700">技能安装需要更多配置</span>
                    </div>
                    <p className="text-xs text-orange-600 mb-3">
                      技能安装需要：技能类型、服务地址、访问凭证等复杂配置，
                      建议去技能市场查找。
                    </p>
                    <div className="flex gap-2">
                      <a
                        href="https://skills.sh"
                        target="_blank"
                        rel="noopener noreferrer"
                        className="flex-1 py-2 bg-orange-500 text-white rounded-xl text-xs font-bold text-center hover:bg-orange-600"
                      >
                        去 skills.sh 看看
                      </a>
                      <button
                        onClick={() => setSkillGuidanceVisible(false)}
                        className="px-4 py-2 bg-white border border-orange-200 text-orange-600 rounded-xl text-xs font-bold hover:bg-orange-50"
                      >
                        关闭
                      </button>
                    </div>
                  </motion.div>
                )}
              </div>

              {/* Input Area */}
              <div className="p-6 border-t border-slate-100 bg-slate-50/50 shrink-0">
                <div className="relative">
                  <input
                    type="text"
                    value={input}
                    onChange={(e) => setInput(e.target.value)}
                    onKeyDown={(e) => e.key === 'Enter' && handleCommand()}
                    placeholder={t('commandCenter.placeholder')}
                    className="w-full pl-4 pr-12 py-4 bg-white border border-slate-200 rounded-2xl focus:ring-4 focus:ring-brand-500/10 focus:border-brand-500 outline-none transition-all text-sm font-medium shadow-sm"
                  />
                  <button
                    onClick={handleCommand}
                    disabled={!input.trim() || isProcessing}
                    className="absolute right-2 top-1/2 -translate-y-1/2 w-10 h-10 bg-slate-900 text-white rounded-xl flex items-center justify-center hover:bg-slate-800 disabled:opacity-50 disabled:cursor-not-allowed transition-all"
                  >
                    {isProcessing ? <Loader2 size={18} className="animate-spin" /> : <Send size={18} />}
                  </button>
                </div>
                <div className="mt-3 flex items-center justify-center gap-4">
                  <span className="text-[9px] font-bold text-slate-400 uppercase tracking-widest flex items-center gap-1">
                    <Cpu size={10} />
                    {t('commandCenter.models')}
                  </span>
                  <span className="text-[9px] font-bold text-slate-400 uppercase tracking-widest flex items-center gap-1">
                    <Bot size={10} />
                    {t('commandCenter.agents')}
                  </span>
                  <span className="text-[9px] font-bold text-slate-400 uppercase tracking-widest flex items-center gap-1">
                    <Zap size={10} />
                    {t('commandCenter.skills')}
                  </span>
                </div>
              </div>
            </motion.div>
          </div>
        )}
      </AnimatePresence>
    </>
  );
}
