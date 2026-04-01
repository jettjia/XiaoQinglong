/**
 * @license
 * SPDX-License-Identifier: Apache-2.0
 */

import React from 'react';
import { Sidebar } from './components/Sidebar';
import { Dashboard } from './components/Dashboard';
import { AgentManager } from './components/AgentManager';
import { ChatInterface } from './components/ChatInterface';
import { KnowledgeBaseManager } from './components/KnowledgeBaseManager';
import { SkillManager } from './components/SkillManager';
import { AgentOrchestrator } from './components/AgentOrchestrator';
import { ModelManager } from './components/ModelManager';
import { Settings } from './components/Settings';
import { Inbox } from './components/Inbox';
import { CommandCenter } from './components/CommandCenter';
import { View, Agent } from './types';
import { AnimatePresence, motion } from 'motion/react';
import { Toaster } from 'sonner';

export default function App() {
  const [activeView, setActiveView] = React.useState<View>('dashboard');
  const [preselectedAgent, setPreselectedAgent] = React.useState<Agent | null>(null);
  const [editingAgent, setEditingAgent] = React.useState<Agent | null>(null);

  const handleEditAgent = (agent: Agent) => {
    setEditingAgent(agent);
    setActiveView('orchestrator');
  };

  const renderView = () => {
    switch (activeView) {
      case 'dashboard':
        return <Dashboard onViewChange={setActiveView} />;
      case 'agents':
        return <AgentManager onViewChange={setActiveView} onPlayAgent={(agent) => {
          setPreselectedAgent(agent);
        }} onEditAgent={handleEditAgent} />;
      case 'chat':
        return <ChatInterface preselectedAgent={preselectedAgent} onAgentUsed={() => setPreselectedAgent(null)} />;
      case 'knowledge':
        return <KnowledgeBaseManager />;
      case 'skills':
        return <SkillManager initialTab="skills" />;
      case 'orchestrator':
        return <AgentOrchestrator editingAgent={editingAgent} onSaved={() => {
          setEditingAgent(null);
        }} />
      case 'models':
        return <ModelManager />;
      case 'settings':
        return <Settings />;
      case 'inbox':
        return <Inbox />;
      default:
        return <Dashboard onViewChange={setActiveView} />;
    }
  };

  return (
    <div className="flex h-screen bg-slate-50 overflow-hidden">
      <Sidebar activeView={activeView} onViewChange={setActiveView} />
      
      <main className="flex-1 overflow-y-auto relative scrollbar-hide">
        <AnimatePresence mode="wait">
          <motion.div
            key={activeView}
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -10 }}
            transition={{ duration: 0.2 }}
            className="h-full"
          >
            {renderView()}
          </motion.div>
        </AnimatePresence>
      </main>

      <CommandCenter onViewChange={setActiveView} />
      <Toaster position="top-right" richColors />
    </div>
  );
}
