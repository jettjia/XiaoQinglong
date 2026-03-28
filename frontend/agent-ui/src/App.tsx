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
import { View } from './types';
import { AnimatePresence, motion } from 'motion/react';

export default function App() {
  const [activeView, setActiveView] = React.useState<View>('dashboard');

  const renderView = () => {
    switch (activeView) {
      case 'dashboard':
        return <Dashboard />;
      case 'agents':
        return <AgentManager onViewChange={setActiveView} />;
      case 'chat':
        return <ChatInterface />;
      case 'knowledge':
        return <KnowledgeBaseManager />;
      case 'skills':
        return <SkillManager initialTab="skills" />;
      case 'orchestrator':
        return <AgentOrchestrator />;
      case 'models':
        return <ModelManager />;
      case 'settings':
        return <Settings />;
      case 'inbox':
        return <Inbox />;
      default:
        return <Dashboard />;
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
    </div>
  );
}
