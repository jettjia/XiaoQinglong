import React, { useState } from 'react';
import { 
  Plus, 
  Search, 
  Database, 
  Clock, 
  RefreshCw,
  Trash2,
  X,
  Eye,
  EyeOff,
  Check,
  Play,
  HelpCircle,
  Info
} from 'lucide-react';
import { INITIAL_KNOWLEDGE_BASES } from '../constants';
import { cn } from '../lib/utils';
import { useTranslation } from 'react-i18next';
import { KnowledgeBase, RecallTestRecord } from '../types';

export function KnowledgeBaseManager() {
  const { t } = useTranslation();
  const [knowledgeBases, setKnowledgeBases] = useState<KnowledgeBase[]>(INITIAL_KNOWLEDGE_BASES);
  const [testRecords, setTestRecords] = useState<RecallTestRecord[]>([
    {
      id: 'demo-1',
      kbId: 'kb-1',
      kbName: 'External Search API',
      query: 'What is the company policy on remote work?',
      timestamp: '10:45 AM',
      results: [
        { title: 'Remote Work Policy', score: 0.98, content: 'Employees are eligible for remote work after 3 months of tenure.' }
      ]
    },
    {
      id: 'demo-2',
      kbId: 'kb-1',
      kbName: 'External Search API',
      query: 'How to reset my password?',
      timestamp: '09:30 AM',
      results: [
        { title: 'IT Support Guide', score: 0.95, content: 'Visit the self-service portal at https://reset.company.com' }
      ]
    }
  ]);
  const [isAddModalOpen, setIsAddModalOpen] = useState(false);
  const [isTestModalOpen, setIsTestModalOpen] = useState(false);
  const [isSpecModalOpen, setIsSpecModalOpen] = useState(false);
  const [selectedKB, setSelectedKB] = useState<KnowledgeBase | null>(null);
  const [showToken, setShowToken] = useState(false);
  const [testQuery, setTestQuery] = useState('');
  const [testResults, setTestResults] = useState<any[]>([]);

  const [newKB, setNewKB] = useState<Partial<KnowledgeBase>>({
    name: '',
    description: '',
    retrievalUrl: '',
    token: '',
    enabled: true
  });

  const handleAddKB = () => {
    const kb: KnowledgeBase = {
      id: `kb-${Date.now()}`,
      name: newKB.name || '',
      description: newKB.description,
      lastUpdated: new Date().toISOString().split('T')[0],
      retrievalUrl: newKB.retrievalUrl || '',
      token: newKB.token,
      enabled: newKB.enabled,
    };
    setKnowledgeBases([...knowledgeBases, kb]);
    setIsAddModalOpen(false);
    setNewKB({
      name: '',
      description: '',
      retrievalUrl: '',
      token: '',
      enabled: true
    });
  };

  const handleDeleteKB = (id: string) => {
    setKnowledgeBases(knowledgeBases.filter(kb => kb.id !== id));
  };

  const handleTestRecall = (kb: KnowledgeBase) => {
    setSelectedKB(kb);
    setIsTestModalOpen(true);
    setTestResults([]);
    setTestQuery('');
  };

  const runTest = () => {
    if (!selectedKB) return;
    
    // Mock test results
    const results = [
      { title: 'Document A', score: 0.92, content: 'This is a sample content from document A that matches the query.' },
      { title: 'Document B', score: 0.85, content: 'Another piece of information found in the knowledge base.' },
      { title: 'Document C', score: 0.78, content: 'Relevant data extracted from the indexed sources.' },
    ];
    
    setTestResults(results);

    const record: RecallTestRecord = {
      id: `test-${Date.now()}`,
      kbId: selectedKB.id,
      kbName: selectedKB.name,
      query: testQuery,
      timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
      results
    };

    setTestRecords([record, ...testRecords]);
  };

  const deleteTestRecord = (id: string) => {
    setTestRecords(testRecords.filter(r => r.id !== id));
  };

  return (
    <div className="p-8 max-w-7xl mx-auto">
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">{t('knowledge.title')}</h1>
          <p className="text-slate-500 mt-1">{t('knowledge.subtitle')}</p>
        </div>
        <div className="flex items-center gap-3">
          <button 
            onClick={() => setIsSpecModalOpen(true)}
            className="flex items-center gap-2 bg-white border border-slate-200 hover:bg-slate-50 text-slate-600 px-4 py-2 rounded-lg font-medium transition-all shadow-sm"
          >
            <HelpCircle size={20} />
            {t('knowledge.apiSpec')}
          </button>
          <button 
            onClick={() => setIsAddModalOpen(true)}
            className="flex items-center gap-2 bg-brand-500 hover:bg-brand-600 text-white px-4 py-2 rounded-lg font-medium transition-all shadow-sm"
          >
            <Plus size={20} />
            {t('knowledge.connect')}
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
        {/* Stats */}
        <div className="lg:col-span-3 grid grid-cols-1 md:grid-cols-1 gap-4">
          <div className="bg-white p-6 rounded-xl border border-slate-200 shadow-sm">
            <div className="flex items-center gap-4">
              <div className="w-12 h-12 rounded-lg bg-blue-50 text-blue-500 flex items-center justify-center">
                <Database size={24} />
              </div>
              <div>
                <p className="text-sm text-slate-500">{t('knowledge.activeSources')}</p>
                <p className="text-2xl font-bold text-slate-900">
                  {knowledgeBases.length}
                </p>
              </div>
            </div>
          </div>
        </div>

        {/* List */}
        <div className="lg:col-span-2 space-y-4">
          <div className="flex items-center justify-between mb-2">
            <h2 className="font-bold text-slate-800">{t('knowledge.activeSources')}</h2>
            <div className="relative">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 w-4 h-4" />
              <input 
                type="text"
                placeholder={t('knowledge.search')}
                className="pl-9 pr-4 py-1.5 bg-slate-100 border-none rounded-lg text-sm focus:ring-2 focus:ring-brand-500/20 w-64"
              />
            </div>
          </div>
          
          {knowledgeBases.map((kb) => (
            <div key={kb.id} className="bg-white border border-slate-200 rounded-xl p-5 hover:border-brand-500/30 transition-all group">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-4">
                  <div className="w-10 h-10 rounded-lg bg-slate-100 flex items-center justify-center text-slate-500 group-hover:bg-brand-50 group-hover:text-brand-500 transition-colors">
                    <Database size={20} />
                  </div>
                  <div>
                    <h3 className="font-bold text-slate-900">{kb.name}</h3>
                    <div className="flex items-center gap-3 mt-1">
                      <span className="text-xs text-slate-400 flex items-center gap-1">
                        <Clock size={12} />
                        Updated {kb.lastUpdated}
                      </span>
                      <span className="text-xs text-slate-400 font-mono truncate max-w-[200px]">
                        {kb.retrievalUrl}
                      </span>
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <button 
                    onClick={() => handleTestRecall(kb)}
                    className="p-2 hover:bg-slate-100 rounded-lg text-slate-400 hover:text-brand-500 transition-colors"
                    title={t('knowledge.recallTest')}
                  >
                    <Play size={16} />
                  </button>
                  <button className="p-2 hover:bg-slate-100 rounded-lg text-slate-400 transition-colors">
                    <RefreshCw size={16} />
                  </button>
                  <button 
                    onClick={() => handleDeleteKB(kb.id)}
                    className="p-2 hover:bg-slate-100 rounded-lg text-slate-400 hover:text-red-500 transition-colors"
                  >
                    <Trash2 size={16} />
                  </button>
                </div>
              </div>
            </div>
          ))}

          {knowledgeBases.length === 0 && (
            <div className="text-center py-12 bg-slate-50 rounded-xl border border-dashed border-slate-200">
              <Database className="mx-auto text-slate-300 mb-3" size={48} />
              <p className="text-slate-500 font-medium">No knowledge bases found.</p>
            </div>
          )}
        </div>

        {/* Sidebar / Quick Actions */}
        <div className="space-y-6">
          <div className="bg-white border border-slate-200 rounded-2xl p-6">
            <h3 className="font-bold text-slate-900 mb-4">{t('knowledge.recentActivity')}</h3>
            <div className="space-y-4">
              {testRecords.map(record => (
                <div key={record.id} className="flex gap-3 group">
                  <div className="w-1.5 h-1.5 rounded-full bg-brand-500 mt-1.5 shrink-0" />
                  <div className="flex-1">
                    <div className="flex items-center justify-between">
                      <p className="text-xs font-bold text-slate-700">{record.kbName}</p>
                      <button 
                        onClick={() => deleteTestRecord(record.id)}
                        className="opacity-0 group-hover:opacity-100 text-slate-400 hover:text-red-500 transition-all"
                      >
                        <Trash2 size={12} />
                      </button>
                    </div>
                    <p className="text-[10px] text-slate-500 italic mt-0.5 line-clamp-1">"{record.query}"</p>
                    <p className="text-[10px] text-slate-400 mt-0.5">{record.timestamp}</p>
                  </div>
                </div>
              ))}
              {testRecords.length === 0 && (
                <p className="text-xs text-slate-400 italic text-center py-4">No recent test activity</p>
              )}
            </div>
          </div>
          
          <div className="bg-brand-50 border border-brand-100 rounded-2xl p-6">
            <h3 className="font-bold text-brand-900 mb-2 flex items-center gap-2">
              <Check size={18} className="text-brand-500" />
              System Status
            </h3>
            <p className="text-xs text-brand-700 leading-relaxed">
              All knowledge bases are currently active and ready for agent retrieval.
            </p>
          </div>
        </div>
      </div>

      {/* Add KB Modal */}
      {isAddModalOpen && (
        <div className="fixed inset-0 bg-slate-900/50 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl w-full max-w-md shadow-2xl overflow-hidden flex flex-col max-h-[90vh]">
            <div className="p-6 border-b border-slate-100 flex items-center justify-between shrink-0">
              <h2 className="text-xl font-bold text-slate-900">{t('knowledge.addKB')}</h2>
              <button onClick={() => setIsAddModalOpen(false)} className="text-slate-400 hover:text-slate-600">
                <X size={24} />
              </button>
            </div>
            <div className="p-6 space-y-4 flex-1 overflow-y-auto">
              <div>
                <label className="block text-sm font-bold text-slate-700 mb-1">
                  {t('knowledge.name')} *
                </label>
                <input 
                  type="text"
                  value={newKB.name}
                  onChange={e => setNewKB({...newKB, name: e.target.value})}
                  placeholder={t('knowledge.placeholderName')}
                  className="w-full px-4 py-2 bg-slate-50 border border-slate-200 rounded-lg focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 outline-none transition-all"
                />
              </div>
              <div>
                <label className="block text-sm font-bold text-slate-700 mb-1">
                  {t('knowledge.description')}
                </label>
                <input 
                  type="text"
                  value={newKB.description}
                  onChange={e => setNewKB({...newKB, description: e.target.value})}
                  placeholder={t('knowledge.placeholderDesc')}
                  className="w-full px-4 py-2 bg-slate-50 border border-slate-200 rounded-lg focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 outline-none transition-all"
                />
              </div>
              
              <div>
                <label className="block text-sm font-bold text-slate-700 mb-1 flex items-center justify-between">
                  <span>{t('knowledge.retrievalUrl')} *</span>
                  <button 
                    onClick={() => setIsSpecModalOpen(true)}
                    className="text-brand-500 hover:text-brand-600 flex items-center gap-1 text-xs font-medium"
                  >
                    <HelpCircle size={14} />
                    {t('knowledge.apiSpec')}
                  </button>
                </label>
                <input 
                  type="text"
                  value={newKB.retrievalUrl}
                  onChange={e => setNewKB({...newKB, retrievalUrl: e.target.value})}
                  placeholder={t('knowledge.placeholderUrl')}
                  className="w-full px-4 py-2 bg-slate-50 border border-slate-200 rounded-lg focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 outline-none transition-all font-mono text-sm"
                />
              </div>
              <div>
                <label className="block text-sm font-bold text-slate-700 mb-1">
                  {t('knowledge.token')}
                </label>
                <div className="relative">
                  <input 
                    type={showToken ? "text" : "password"}
                    value={newKB.token}
                    onChange={e => setNewKB({...newKB, token: e.target.value})}
                    placeholder={t('knowledge.placeholderToken')}
                    className="w-full pl-4 pr-10 py-2 bg-slate-50 border border-slate-200 rounded-lg focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 outline-none transition-all font-mono text-sm"
                  />
                  <button 
                    onClick={() => setShowToken(!showToken)}
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600"
                  >
                    {showToken ? <EyeOff size={16} /> : <Eye size={16} />}
                  </button>
                </div>
              </div>
              <div className="flex items-center gap-2 pt-2">
                <button 
                  onClick={() => setNewKB({...newKB, enabled: !newKB.enabled})}
                  className={cn(
                    "w-5 h-5 rounded border flex items-center justify-center transition-all",
                    newKB.enabled ? "bg-brand-500 border-brand-500 text-white" : "bg-white border-slate-300"
                  )}
                >
                  {newKB.enabled && <Check size={14} />}
                </button>
                <span className="text-sm font-medium text-slate-600">{t('knowledge.enabled')}</span>
              </div>
            </div>
            <div className="p-6 bg-slate-50 flex gap-3 shrink-0">
              <button 
                onClick={() => setIsAddModalOpen(false)}
                className="flex-1 py-2 text-sm font-bold text-slate-500 hover:text-slate-700 transition-all"
              >
                {t('knowledge.cancel')}
              </button>
              <button 
                onClick={handleAddKB}
                disabled={!newKB.name || !newKB.retrievalUrl}
                className="flex-1 py-2 bg-slate-900 text-white rounded-lg font-bold text-sm hover:bg-slate-800 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {t('knowledge.save')}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Recall Test Modal */}
      {isTestModalOpen && selectedKB && (
        <div className="fixed inset-0 bg-slate-900/50 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl w-full max-w-2xl shadow-2xl overflow-hidden">
            <div className="p-6 border-b border-slate-100 flex items-center justify-between">
              <div>
                <h2 className="text-xl font-bold text-slate-900">{t('knowledge.recallTest')}</h2>
                <p className="text-sm text-slate-500">{selectedKB.name}</p>
              </div>
              <button onClick={() => setIsTestModalOpen(false)} className="text-slate-400 hover:text-slate-600">
                <X size={24} />
              </button>
            </div>
            <div className="p-6 space-y-6">
              <div className="flex gap-2">
                <input 
                  type="text"
                  value={testQuery}
                  onChange={e => setTestQuery(e.target.value)}
                  placeholder={t('knowledge.testQuery')}
                  className="flex-1 px-4 py-2 bg-slate-50 border border-slate-200 rounded-lg focus:ring-2 focus:ring-brand-500/20 focus:border-brand-500 outline-none transition-all"
                />
                <button 
                  onClick={runTest}
                  disabled={!testQuery}
                  className="px-6 bg-brand-500 text-white rounded-lg font-bold text-sm hover:bg-brand-600 transition-all disabled:opacity-50"
                >
                  {t('knowledge.recallTest')}
                </button>
              </div>

              <div className="space-y-4">
                <h3 className="text-sm font-bold text-slate-700 uppercase tracking-wider">{t('knowledge.testResults')}</h3>
                <div className="space-y-3 max-h-96 overflow-y-auto pr-2">
                  {testResults.map((result, i) => (
                    <div key={i} className="p-4 bg-slate-50 rounded-xl border border-slate-100">
                      <div className="flex items-center justify-between mb-2">
                        <span className="font-bold text-slate-900 text-sm">{result.title}</span>
                        <span className="text-xs font-bold px-2 py-0.5 bg-brand-50 text-brand-600 rounded-full">
                          Score: {result.score}
                        </span>
                      </div>
                      <p className="text-sm text-slate-600 leading-relaxed italic">
                        "{result.content}"
                      </p>
                    </div>
                  ))}
                  {testResults.length === 0 && (
                    <div className="text-center py-12 text-slate-400">
                      <Search className="mx-auto mb-2 opacity-20" size={48} />
                      <p>Enter a query to test retrieval performance</p>
                    </div>
                  )}
                </div>
              </div>
            </div>
          </div>
        </div>
      )}
      {/* API Spec Modal */}
      {isSpecModalOpen && (
        <div className="fixed inset-0 bg-slate-900/50 backdrop-blur-sm z-[60] flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl w-full max-w-2xl shadow-2xl overflow-hidden flex flex-col max-h-[90vh]">
            <div className="p-6 border-b border-slate-100 flex items-center justify-between shrink-0">
              <div className="flex items-center gap-2">
                <div className="w-8 h-8 rounded-lg bg-brand-50 text-brand-500 flex items-center justify-center">
                  <Info size={18} />
                </div>
                <h2 className="text-lg font-bold text-slate-900">{t('knowledge.apiSpecTitle')}</h2>
              </div>
              <button onClick={() => setIsSpecModalOpen(false)} className="text-slate-400 hover:text-slate-600">
                <X size={24} />
              </button>
            </div>
            <div className="p-8 space-y-8 flex-1 overflow-y-auto">
              <div className="bg-blue-50 border border-blue-100 rounded-xl p-4">
                <p className="text-sm text-blue-800 leading-relaxed">
                  {t('knowledge.apiSpecIntro')}
                </p>
              </div>

              <div>
                <div className="flex items-center justify-between mb-2">
                  <h3 className="text-sm font-bold text-slate-800 uppercase tracking-wider">{t('knowledge.apiSpecInput')}</h3>
                  <button 
                    onClick={() => {
                      navigator.clipboard.writeText(JSON.stringify({ query: "example query", top_k: 5 }, null, 2));
                    }}
                    className="text-[10px] font-bold text-brand-500 hover:text-brand-600 uppercase"
                  >
                    {t('knowledge.copy')}
                  </button>
                </div>
                <div className="bg-slate-900 rounded-lg p-4 font-mono text-xs text-blue-300 relative group">
                  <p>{"{"}</p>
                  <p className="pl-4">"query": <span className="text-green-400">"string"</span>, <span className="text-slate-500">// {t('knowledge.apiSpecQuery')}</span></p>
                  <p className="pl-4">"top_k": <span className="text-orange-400">number</span> <span className="text-slate-500">// {t('knowledge.apiSpecTopK')}</span></p>
                  <p>{"}"}</p>
                </div>
              </div>

              <div>
                <div className="flex items-center justify-between mb-2">
                  <h3 className="text-sm font-bold text-slate-800 uppercase tracking-wider">{t('knowledge.apiSpecOutput')}</h3>
                  <button 
                    onClick={() => {
                      navigator.clipboard.writeText(JSON.stringify([{ title: "Example Title", content: "Example content snippet...", score: 0.95 }], null, 2));
                    }}
                    className="text-[10px] font-bold text-brand-500 hover:text-brand-600 uppercase"
                  >
                    {t('knowledge.copy')}
                  </button>
                </div>
                <div className="bg-slate-900 rounded-lg p-4 font-mono text-xs text-blue-300">
                  <p>{"["}</p>
                  <p className="pl-4">{"{"}</p>
                  <p className="pl-8">"title": <span className="text-green-400">"string"</span>, <span className="text-slate-500">// {t('knowledge.apiSpecResultTitle')}</span></p>
                  <p className="pl-8">"content": <span className="text-green-400">"string"</span>, <span className="text-slate-500">// {t('knowledge.apiSpecResultContent')}</span></p>
                  <p className="pl-8">"score": <span className="text-orange-400">number</span> <span className="text-slate-500">// {t('knowledge.apiSpecResultScore')}</span></p>
                  <p className="pl-4">{"}"},</p>
                  <p className="pl-4">...</p>
                  <p>{"]"}</p>
                </div>
              </div>

              <div>
                <h3 className="text-sm font-bold text-slate-800 mb-2 uppercase tracking-wider">{t('knowledge.apiSpecAuth')}</h3>
                <p className="text-sm text-slate-600 mb-3">
                  {t('knowledge.apiSpecAuthDesc')}
                </p>
                <div className="bg-slate-100 rounded-lg p-4 font-mono text-xs text-slate-700">
                  <p>Authorization: Bearer YOUR_TOKEN</p>
                </div>
              </div>
            </div>
            <div className="p-6 border-t border-slate-100 flex justify-end shrink-0">
              <button 
                onClick={() => setIsSpecModalOpen(false)}
                className="px-8 py-2 bg-slate-900 text-white rounded-xl font-bold text-sm hover:bg-slate-800 transition-all shadow-lg shadow-slate-900/20"
              >
                {t('knowledge.close')}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
