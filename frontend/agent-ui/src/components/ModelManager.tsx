import React from 'react';
import { 
  Cpu, 
  CheckCircle2, 
  AlertCircle, 
  Settings2, 
  ChevronRight,
  BarChart3,
  Activity,
  Plus,
  X,
  Database,
  Globe,
  Key
} from 'lucide-react';
import { cn } from '../lib/utils';
import { useTranslation } from 'react-i18next';
import { Model } from '../types';
import { motion, AnimatePresence } from 'motion/react';

export function ModelManager() {
  const { t } = useTranslation();
  const [activeTab, setActiveTab] = React.useState<'llm' | 'embedding'>('llm');
  const [isModalOpen, setIsModalOpen] = React.useState(false);
  const [newModelType, setNewModelType] = React.useState<'llm' | 'embedding'>('llm');
  const [editingModelId, setEditingModelId] = React.useState<string | null>(null);
  
  const [models, setModels] = React.useState<Model[]>([
    { 
      id: '1',
      name: 'Gemini 3.1 Pro', 
      provider: 'Google', 
      status: 'active', 
      latency: '1.2s', 
      contextWindow: '128k',
      usage: 65,
      type: 'llm'
    },
    { 
      id: '2',
      name: 'Gemini 3 Flash', 
      provider: 'Google', 
      status: 'active', 
      latency: '0.4s', 
      contextWindow: '1M',
      usage: 24,
      type: 'llm'
    },
    { 
      id: '3',
      name: 'GPT-4o', 
      provider: 'OpenAI', 
      status: 'configured', 
      latency: '1.8s', 
      contextWindow: '128k',
      usage: 12,
      type: 'llm'
    },
    { 
      id: '4',
      name: 'text-embedding-004', 
      provider: 'Google', 
      status: 'active', 
      latency: '0.1s', 
      contextWindow: '2k',
      usage: 45,
      type: 'embedding'
    },
    { 
      id: '5',
      name: 'text-embedding-3-small', 
      provider: 'OpenAI', 
      status: 'active', 
      latency: '0.2s', 
      contextWindow: '8k',
      usage: 15,
      type: 'embedding'
    }
  ]);

  const [formData, setFormData] = React.useState({
    name: '',
    provider: 'OpenAI',
    baseUrl: 'https://api.openai.com/v1',
    apiKey: '',
    category: 'default' as 'default' | 'rewrite' | 'skill' | 'summarize'
  });

  const handleAddModel = (e: React.FormEvent) => {
    e.preventDefault();
    if (editingModelId) {
      setModels(prev => prev.map(m => m.id === editingModelId ? {
        ...m,
        name: formData.name,
        provider: formData.provider,
        baseUrl: formData.baseUrl,
        apiKey: formData.apiKey,
        type: newModelType,
        category: newModelType === 'llm' ? formData.category : undefined
      } : m));
    } else {
      const model: Model = {
        id: Date.now().toString(),
        name: formData.name,
        provider: formData.provider,
        baseUrl: formData.baseUrl,
        apiKey: formData.apiKey,
        status: 'configured',
        latency: 'N/A',
        contextWindow: 'N/A',
        usage: 0,
        type: newModelType,
        category: newModelType === 'llm' ? formData.category : undefined
      };
      setModels(prev => [...prev, model]);
    }
    setIsModalOpen(false);
    setEditingModelId(null);
    setFormData({ name: '', provider: 'OpenAI', baseUrl: 'https://api.openai.com/v1', apiKey: '', category: 'default' });
  };

  const handleEditClick = (model: Model) => {
    setEditingModelId(model.id);
    setNewModelType(model.type);
    setFormData({
      name: model.name,
      provider: model.provider,
      baseUrl: model.baseUrl || 'https://api.openai.com/v1',
      apiKey: model.apiKey || '',
      category: model.category || 'default'
    });
    setIsModalOpen(true);
  };

  const handleDeleteModel = (id: string) => {
    setModels(prev => prev.filter(m => m.id !== id));
    setIsModalOpen(false);
    setEditingModelId(null);
  };

  const filteredModels = models.filter(m => m.type === activeTab);

  return (
    <div className="p-8 max-w-7xl mx-auto">
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">{t('models.title')}</h1>
          <p className="text-slate-500 mt-1">{t('models.subtitle')}</p>
        </div>
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2 px-3 py-1.5 bg-green-50 text-green-600 rounded-lg text-xs font-bold border border-green-100">
            <Activity size={14} />
            {t('models.systemStatus')}
          </div>
          <button 
            onClick={() => {
              setEditingModelId(null);
              setNewModelType(activeTab);
              setFormData({ name: '', provider: 'OpenAI', baseUrl: 'https://api.openai.com/v1', apiKey: '', category: 'default' });
              setIsModalOpen(true);
            }}
            className="flex items-center gap-2 bg-brand-500 hover:bg-brand-600 text-white px-4 py-2 rounded-lg font-medium transition-all shadow-sm"
          >
            <Plus size={20} />
            {activeTab === 'llm' ? t('models.addModel') : t('models.addEmbedding')}
          </button>
        </div>
      </div>

      <div className="flex gap-1 p-1 bg-slate-100 rounded-xl mb-8 w-fit">
        <button 
          onClick={() => setActiveTab('llm')}
          className={cn(
            "px-6 py-2 rounded-lg text-sm font-bold transition-all",
            activeTab === 'llm' ? "bg-white text-slate-900 shadow-sm" : "text-slate-500 hover:text-slate-700"
          )}
        >
          {t('models.llmTab')}
        </button>
        <button 
          onClick={() => setActiveTab('embedding')}
          className={cn(
            "px-6 py-2 rounded-lg text-sm font-bold transition-all",
            activeTab === 'embedding' ? "bg-white text-slate-900 shadow-sm" : "text-slate-500 hover:text-slate-700"
          )}
        >
          {t('models.embeddingTab')}
        </button>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
        {/* Model Cards */}
        <div className="lg:col-span-2 space-y-6">
          {filteredModels.map((model) => (
            <div 
              key={model.id} 
              onClick={() => handleEditClick(model)}
              className="bg-white border border-slate-200 rounded-2xl p-6 hover:border-brand-500/30 transition-all group cursor-pointer"
            >
              <div className="flex items-start justify-between mb-6">
                <div className="flex items-center gap-4">
                  <div className={cn(
                    "w-12 h-12 rounded-xl flex items-center justify-center transition-colors",
                    model.type === 'llm' ? "bg-slate-100 text-slate-600 group-hover:bg-brand-50 group-hover:text-brand-500" : "bg-blue-50 text-blue-500 group-hover:bg-blue-100"
                  )}>
                    {model.type === 'llm' ? <Cpu size={24} /> : <Database size={24} />}
                  </div>
                  <div>
                    <div className="flex items-center gap-2">
                      <h3 className="font-bold text-slate-900">{model.name}</h3>
                      <span className={cn(
                        "px-1.5 py-0.5 rounded text-[8px] font-black uppercase tracking-tighter",
                        model.type === 'llm' ? "bg-brand-100 text-brand-600" : "bg-blue-100 text-blue-600"
                      )}>
                        {model.type}
                      </span>
                      {model.category && (
                        <span className="px-1.5 py-0.5 rounded bg-slate-100 text-slate-600 text-[8px] font-black uppercase tracking-tighter">
                          {t(`models.cat${model.category.charAt(0).toUpperCase() + model.category.slice(1)}`)}
                        </span>
                      )}
                    </div>
                    <p className="text-xs text-slate-400 font-medium uppercase tracking-wider">{model.provider}</p>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <div className={cn(
                    "px-2 py-1 rounded text-[10px] font-bold uppercase tracking-wider",
                    model.status === 'active' ? "bg-green-50 text-green-600" : "bg-slate-100 text-slate-500"
                  )}>
                    {model.status}
                  </div>
                  <button className="p-2 hover:bg-slate-50 rounded-lg text-slate-400">
                    <Settings2 size={18} />
                  </button>
                </div>
              </div>

              <div className="grid grid-cols-3 gap-4 mb-6">
                <div className="p-3 bg-slate-50 rounded-xl">
                  <p className="text-[10px] text-slate-400 font-bold uppercase mb-1">{t('models.latency')}</p>
                  <p className="text-sm font-bold text-slate-700">{model.latency}</p>
                </div>
                <div className="p-3 bg-slate-50 rounded-xl">
                  <p className="text-[10px] text-slate-400 font-bold uppercase mb-1">{t('models.context')}</p>
                  <p className="text-sm font-bold text-slate-700">{model.contextWindow}</p>
                </div>
                <div className="p-3 bg-slate-50 rounded-xl">
                  <p className="text-[10px] text-slate-400 font-bold uppercase mb-1">{t('models.successRate')}</p>
                  <p className="text-sm font-bold text-slate-700">99.9%</p>
                </div>
              </div>

              <div className="space-y-2">
                <div className="flex justify-between text-[10px] font-bold text-slate-400 uppercase">
                  <span>Usage (Last 24h)</span>
                  <span>{model.usage}%</span>
                </div>
                <div className="h-1.5 w-full bg-slate-100 rounded-full overflow-hidden">
                  <div 
                    className={cn(
                      "h-full rounded-full transition-all duration-500",
                      model.type === 'llm' ? "bg-brand-500" : "bg-blue-500"
                    )} 
                    style={{ width: `${model.usage}%` }}
                  />
                </div>
              </div>
            </div>
          ))}
          
          {filteredModels.length === 0 && (
            <div className="bg-white border border-dashed border-slate-200 rounded-2xl p-12 text-center">
              <div className="w-16 h-16 rounded-full bg-slate-50 flex items-center justify-center text-slate-300 mx-auto mb-4">
                <Cpu size={32} />
              </div>
              <h3 className="text-lg font-bold text-slate-900 mb-2">No models configured</h3>
              <p className="text-sm text-slate-500 mb-6">Add your first {activeTab === 'llm' ? 'LLM' : 'embedding'} model to get started.</p>
              <button 
                onClick={() => {
                  setEditingModelId(null);
                  setNewModelType(activeTab);
                  setFormData({ name: '', provider: 'OpenAI', baseUrl: 'https://api.openai.com/v1', apiKey: '', category: 'default' });
                  setIsModalOpen(true);
                }}
                className="inline-flex items-center gap-2 text-brand-500 font-bold hover:text-brand-600"
              >
                <Plus size={20} />
                {activeTab === 'llm' ? t('models.addModel') : t('models.addEmbedding')}
              </button>
            </div>
          )}
        </div>

        {/* Sidebar */}
        <div className="space-y-6">
          <div className="bg-white border border-slate-200 rounded-2xl p-6">
            <h3 className="font-bold text-slate-900 mb-4 flex items-center gap-2">
              <BarChart3 size={18} className="text-brand-500" />
              {t('models.usage')}
            </h3>
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <span className="text-sm text-slate-500">{t('models.tokens')}</span>
                <span className="text-sm font-bold text-slate-900">4.2M</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-sm text-slate-500">{t('models.cost')}</span>
                <span className="text-sm font-bold text-slate-900">$12.45</span>
              </div>
              <div className="pt-4 border-t border-slate-100">
                <button className="w-full py-2 bg-slate-900 text-white rounded-lg text-sm font-bold hover:bg-slate-800 transition-all">
                  {t('models.viewBilling')}
                </button>
              </div>
            </div>
          </div>

          <div className="bg-white border border-slate-200 rounded-2xl p-6">
            <h3 className="font-bold text-slate-900 mb-4">{t('models.apiKeys')}</h3>
            <div className="space-y-3">
              {[
                { name: 'Google Gemini', status: 'valid' },
                { name: 'OpenAI', status: 'valid' },
                { name: 'Anthropic', status: 'missing' },
              ].map(key => (
                <div key={key.name} className="flex items-center justify-between p-3 bg-slate-50 rounded-xl">
                  <span className="text-xs font-medium text-slate-700">{key.name}</span>
                  {key.status === 'valid' ? (
                    <CheckCircle2 size={14} className="text-green-500" />
                  ) : (
                    <AlertCircle size={14} className="text-red-500" />
                  )}
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>

      {/* Add Model Modal */}
      <AnimatePresence>
        {isModalOpen && (
          <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/50 backdrop-blur-sm">
            <motion.div 
              initial={{ opacity: 0, scale: 0.95 }}
              animate={{ opacity: 1, scale: 1 }}
              exit={{ opacity: 0, scale: 0.95 }}
              className="bg-white rounded-2xl shadow-2xl w-full max-w-lg overflow-hidden flex flex-col max-h-[90vh]"
            >
              <div className="p-6 border-b border-slate-100 flex items-center justify-between shrink-0">
                <h2 className="text-xl font-bold text-slate-900">
                  {editingModelId 
                    ? (newModelType === 'llm' ? t('models.editModel') : t('models.editEmbedding'))
                    : (newModelType === 'llm' ? t('models.addModel') : t('models.addEmbedding'))
                  }
                </h2>
                <button 
                  onClick={() => setIsModalOpen(false)}
                  className="p-2 hover:bg-slate-100 rounded-lg text-slate-400"
                >
                  <X size={20} />
                </button>
              </div>
              <form onSubmit={handleAddModel} className="flex-1 flex flex-col overflow-hidden">
                <div className="flex-1 overflow-y-auto p-6 space-y-4">
                  <div className="space-y-1">
                  <label className="text-xs font-bold text-slate-400 uppercase">{t('models.modelType')}</label>
                  <div className="flex gap-2 p-1 bg-slate-100 rounded-xl">
                    <button 
                      type="button"
                      onClick={() => setNewModelType('llm')}
                      className={cn(
                        "flex-1 flex items-center justify-center gap-2 py-2 rounded-lg text-sm font-bold transition-all",
                        newModelType === 'llm' ? "bg-white text-slate-900 shadow-sm" : "text-slate-500 hover:text-slate-700"
                      )}
                    >
                      <Cpu size={16} />
                      {t('models.llm')}
                    </button>
                    <button 
                      type="button"
                      onClick={() => setNewModelType('embedding')}
                      className={cn(
                        "flex-1 flex items-center justify-center gap-2 py-2 rounded-lg text-sm font-bold transition-all",
                        newModelType === 'embedding' ? "bg-white text-slate-900 shadow-sm" : "text-slate-500 hover:text-slate-700"
                      )}
                    >
                      <Database size={16} />
                      {t('models.embedding')}
                    </button>
                  </div>
                </div>

                <div className="space-y-1">
                  <label className="text-xs font-bold text-slate-400 uppercase">{t('models.modelName')}</label>
                  <div className="relative">
                    <Cpu className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 w-4 h-4" />
                    <input 
                      required
                      type="text"
                      value={formData.name}
                      onChange={e => setFormData({...formData, name: e.target.value})}
                      placeholder="e.g. gpt-4o"
                      className="w-full pl-10 pr-4 py-2 bg-slate-50 border border-slate-200 rounded-xl focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 outline-none transition-all"
                    />
                  </div>
                </div>

                {newModelType === 'llm' && (
                  <div className="space-y-1">
                    <label className="text-xs font-bold text-slate-400 uppercase">{t('models.category')}</label>
                    <select 
                      value={formData.category}
                      onChange={e => setFormData({...formData, category: e.target.value as any})}
                      className="w-full px-4 py-2 bg-slate-50 border border-slate-200 rounded-xl focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 outline-none transition-all"
                    >
                      <option value="default">{t('models.catDefault')}</option>
                      <option value="rewrite">{t('models.catRewrite')}</option>
                      <option value="skill">{t('models.catSkill')}</option>
                      <option value="summarize">{t('models.catSummarize')}</option>
                    </select>
                  </div>
                )}

                <div className="space-y-1">
                  <label className="text-xs font-bold text-slate-400 uppercase">{t('models.provider')}</label>
                  <select 
                    value={formData.provider}
                    onChange={e => setFormData({...formData, provider: e.target.value})}
                    className="w-full px-4 py-2 bg-slate-50 border border-slate-200 rounded-xl focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 outline-none transition-all"
                  >
                    <option value="OpenAI">OpenAI</option>
                    <option value="Anthropic">Anthropic</option>
                    <option value="Google">Google</option>
                    <option value="Custom">Custom (OpenAI Compatible)</option>
                  </select>
                </div>

                <div className="space-y-1">
                  <label className="text-xs font-bold text-slate-400 uppercase">{t('models.baseUrl')}</label>
                  <div className="relative">
                    <Globe className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 w-4 h-4" />
                    <input 
                      required
                      type="url"
                      value={formData.baseUrl}
                      onChange={e => setFormData({...formData, baseUrl: e.target.value})}
                      className="w-full pl-10 pr-4 py-2 bg-slate-50 border border-slate-200 rounded-xl focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 outline-none transition-all"
                    />
                  </div>
                </div>

                <div className="space-y-1">
                  <label className="text-xs font-bold text-slate-400 uppercase">{t('models.apiKey')}</label>
                  <div className="relative">
                    <Key className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 w-4 h-4" />
                    <input 
                      required
                      type="password"
                      value={formData.apiKey}
                      onChange={e => setFormData({...formData, apiKey: e.target.value})}
                      placeholder="sk-..."
                      className="w-full pl-10 pr-4 py-2 bg-slate-50 border border-slate-200 rounded-xl focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 outline-none transition-all"
                    />
                  </div>
                </div>

                </div>

                <div className="p-6 border-t border-slate-100 flex gap-3 shrink-0">
                  {editingModelId && (
                    <button 
                      type="button"
                      onClick={() => handleDeleteModel(editingModelId)}
                      className="px-4 py-2 bg-red-50 text-red-600 rounded-xl font-bold hover:bg-red-100 transition-all mr-auto"
                    >
                      {t('models.delete')}
                    </button>
                  )}
                  <button 
                    type="button"
                    onClick={() => setIsModalOpen(false)}
                    className="px-4 py-2 bg-slate-100 text-slate-600 rounded-xl font-bold hover:bg-slate-200 transition-all"
                  >
                    {t('models.cancel')}
                  </button>
                  <button 
                    type="submit"
                    className="flex-1 py-2 bg-brand-500 text-white rounded-xl font-bold hover:bg-brand-600 transition-all shadow-lg shadow-brand-500/20"
                  >
                    {t('models.save')}
                  </button>
                </div>
              </form>
            </motion.div>
          </div>
        )}
      </AnimatePresence>
    </div>
  );
}
