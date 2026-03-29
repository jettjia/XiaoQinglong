import React from 'react';
import { Settings as SettingsIcon, Save, RotateCcw, FileCode, Info, FileJson, Loader2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { cn } from '../lib/utils';
import { configApi } from '../lib/api';

const DEFAULT_APP_CONFIG = `# Application Configuration
server:
  lang: "zh-CN"
  public_port: 9292
  private_port: 9293
  server_name: "agent-frame"
  mode: "debug"
  dev: true
  enable_event: false
  enable_job: true
  enable_grpc: false

db:
  db_type: "postgres"
  username: "root"
  password: "admin123"
  db_host: "127.0.0.1"
  db_port: 5432
  db_name: "xiaoqinglong"
  charset: "utf8mb4"
  max_open_conn: 50
  max_idle_conn: 10
  conn_max_lifetime: 500
  log_mode: 4
  slow_threshold: 10
`;

const DEFAULT_SKILLS_CONFIG = `# Skill Global Configuration

# Global Settings
aws.region: us-east-1
aws.timeout: 30
log.level: info

# S3 Skill
s3-skill.bucket: my-bucket
s3-skill.region: cn-north-1
s3-skill.secret: your-aws-secret-key
s3-skill.access_key: your-aws-access-key
s3-skill.endpoint: https://s3.cn-north-1.amazonaws.com.cn

# PDF Skill
pdf-skill.output_dir: /tmp/pdf-output
pdf-skill.quality: high

# PPTX Skill
pptx-skill.template_dir: /workspace/templates
pptx-skill.output_dir: /workspace/outputs

# Sandbox Environment
skills:
  s3-skill:
    env:
      AWS_REGION: cn-north-1
      AWS_ACCESS_KEY_ID: your-access-key
      AWS_SECRET_ACCESS_KEY: your-secret-key
      S3_BUCKET: my-bucket
      S3_ENDPOINT: https://s3.cn-north-1.amazonaws.com.cn
  pdf-skill:
    env:
      OUTPUT_DIR: /tmp/pdf-output
      PDF_QUALITY: high
`;

type ConfigTab = 'app' | 'skills';

export function Settings() {
  const { t } = useTranslation();
  const [activeTab, setActiveTab] = React.useState<ConfigTab>('app');
  const [appConfig, setAppConfig] = React.useState(DEFAULT_APP_CONFIG);
  const [skillsConfig, setSkillsConfig] = React.useState(DEFAULT_SKILLS_CONFIG);
  const [isSaved, setIsSaved] = React.useState(false);
  const [isLoading, setIsLoading] = React.useState(true);
  const [isSaving, setIsSaving] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  // Load configs on mount
  React.useEffect(() => {
    loadConfigs();
  }, []);

  const loadConfigs = async () => {
    setIsLoading(true);
    setError(null);
    try {
      const [app, skills] = await Promise.all([
        configApi.getAppConfig().catch(() => DEFAULT_APP_CONFIG),
        configApi.getSkillsConfig().catch(() => DEFAULT_SKILLS_CONFIG),
      ]);
      setAppConfig(app);
      setSkillsConfig(skills);
    } catch (e: any) {
      setError(e.message || 'Failed to load configs');
    } finally {
      setIsLoading(false);
    }
  };

  const handleSave = async () => {
    setIsSaving(true);
    setError(null);
    try {
      await Promise.all([
        configApi.saveAppConfig(appConfig),
        configApi.saveSkillsConfig(skillsConfig),
      ]);
      setIsSaved(true);
      setTimeout(() => setIsSaved(false), 3000);
    } catch (e: any) {
      setError(e.message || 'Failed to save configs');
    } finally {
      setIsSaving(false);
    }
  };

  const handleReset = () => {
    if (window.confirm(t('settings.confirmReset'))) {
      if (activeTab === 'app') {
        setAppConfig(DEFAULT_APP_CONFIG);
      } else {
        setSkillsConfig(DEFAULT_SKILLS_CONFIG);
      }
    }
  };

  const handleResetAll = () => {
    if (window.confirm(t('settings.confirmResetAll'))) {
      setAppConfig(DEFAULT_APP_CONFIG);
      setSkillsConfig(DEFAULT_SKILLS_CONFIG);
    }
  };

  const currentConfig = activeTab === 'app' ? appConfig : skillsConfig;
  const setCurrentConfig = (value: string) => {
    if (activeTab === 'app') {
      setAppConfig(value);
    } else {
      setSkillsConfig(value);
    }
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100vh', backgroundColor: '#f8fafc', overflow: 'hidden' }}>
      {/* Header */}
      <header style={{ height: '64px', borderBottom: '1px solid #e2e8f0', backgroundColor: 'white', display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '0 24px', flexShrink: 0 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
          <div style={{ width: '40px', height: '40px', borderRadius: '12px', backgroundColor: '#f1f5f9', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <SettingsIcon size={20} color="#64748b" />
          </div>
          <div>
            <h1 style={{ fontSize: '18px', fontWeight: 'bold', color: '#0f172a' }}>{t('settings.title')}</h1>
            <p style={{ fontSize: '12px', color: '#64748b' }}>{t('settings.subtitle')}</p>
          </div>
        </div>
        <div style={{ display: 'flex', gap: '12px' }}>
          <button
            onClick={handleResetAll}
            style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '8px 16px', backgroundColor: 'white', border: '1px solid #e2e8f0', borderRadius: '8px', fontSize: '14px', fontWeight: 'bold', color: '#475569', cursor: 'pointer' }}
          >
            <RotateCcw size={16} />
            {t('settings.resetAll')}
          </button>
          <button
            onClick={handleSave}
            disabled={isSaving}
            style={{
              display: 'flex', alignItems: 'center', gap: '8px', padding: '8px 24px', borderRadius: '8px', fontSize: '14px', fontWeight: 'bold', cursor: isSaving ? 'not-allowed' : 'pointer',
              backgroundColor: isSaved ? '#22c55e' : isSaving ? '#94a3b8' : '#3b82f6',
              color: 'white', border: 'none', boxShadow: '0 4px 6px -1px rgba(0, 0, 0, 0.1)'
            }}
          >
            <Save size={16} />
            {isSaving ? t('settings.saving') : isSaved ? t('settings.saved') : t('settings.save')}
          </button>
        </div>
      </header>

      {error && (
        <div style={{ margin: '16px 24px', padding: '12px', backgroundColor: '#fef2f2', border: '1px solid #fecaca', borderRadius: '8px', fontSize: '14px', color: '#dc2626' }}>
          {error}
        </div>
      )}

      <div style={{ flex: 1, padding: '24px', display: 'flex', gap: '24px', overflow: 'hidden' }}>
        {/* Main Editor Area */}
        <div style={{ flex: 1, display: 'flex', flexDirection: 'column', backgroundColor: 'white', borderRadius: '16px', border: '1px solid #e2e8f0', overflow: 'hidden' }}>
          {/* Tabs */}
          <div style={{ padding: '8px', backgroundColor: '#f1f5f9', display: 'flex', gap: '4px', borderBottom: '1px solid #e2e8f0' }}>
            <button
              onClick={() => setActiveTab('app')}
              style={{
                display: 'flex', alignItems: 'center', gap: '8px', padding: '8px 16px', borderRadius: '8px', fontSize: '14px', fontWeight: 'bold',
                backgroundColor: activeTab === 'app' ? 'white' : 'transparent',
                color: activeTab === 'app' ? '#0f172a' : '#64748b',
                border: 'none', cursor: 'pointer'
              }}
            >
              <FileCode size={16} />
              {t('settings.appConfig')}
            </button>
            <button
              onClick={() => setActiveTab('skills')}
              style={{
                display: 'flex', alignItems: 'center', gap: '8px', padding: '8px 16px', borderRadius: '8px', fontSize: '14px', fontWeight: 'bold',
                backgroundColor: activeTab === 'skills' ? 'white' : 'transparent',
                color: activeTab === 'skills' ? '#0f172a' : '#64748b',
                border: 'none', cursor: 'pointer'
              }}
            >
              <FileJson size={16} />
              {t('settings.skillsConfig')}
            </button>
          </div>

          {/* Editor Header */}
          <div style={{ padding: '12px 16px', borderBottom: '1px solid #f1f5f9', display: 'flex', alignItems: 'center', justifyContent: 'space-between', backgroundColor: '#f8fafc' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              {activeTab === 'app' ? <FileCode size={16} color="#94a3b8" /> : <FileJson size={16} color="#94a3b8" />}
              <span style={{ fontSize: '12px', fontWeight: 'bold', color: '#475569', textTransform: 'uppercase' }}>
                {activeTab === 'app' ? 'config.yaml' : 'skills-config.yaml'}
              </span>
            </div>
            <button
              onClick={handleReset}
              style={{ display: 'flex', alignItems: 'center', gap: '6px', padding: '4px 8px', fontSize: '12px', color: '#64748b', backgroundColor: 'transparent', border: 'none', borderRadius: '6px', cursor: 'pointer' }}
            >
              <RotateCcw size={12} />
              {t('settings.reset')}
            </button>
          </div>

          {/* Editor Content */}
          <div style={{ flex: 1, minHeight: '300px', position: 'relative' }}>
            {isLoading ? (
              <div style={{ position: 'absolute', inset: 0, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                <Loader2 size={24} color="#94a3b8" style={{ animation: 'spin 1s linear infinite' }} />
              </div>
            ) : (
              <textarea
                value={currentConfig}
                onChange={(e) => setCurrentConfig(e.target.value)}
                spellCheck={false}
                style={{
                  position: 'absolute', top: 0, left: 0, right: 0, bottom: 0,
                  width: '100%', height: '100%', padding: '24px',
                  fontFamily: 'monospace', fontSize: '14px', color: '#1e293b',
                  backgroundColor: 'transparent', resize: 'none', border: 'none', outline: 'none'
                }}
              />
            )}
          </div>
        </div>

        {/* Info Panel */}
        <div style={{ width: '320px', flexShrink: 0, display: 'flex', flexDirection: 'column', gap: '24px' }}>
          <div style={{ backgroundColor: 'white', borderRadius: '16px', border: '1px solid #e2e8f0', padding: '24px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', color: '#0f172a', marginBottom: '16px' }}>
              <Info size={18} color="#3b82f6" />
              <h2 style={{ fontWeight: 'bold' }}>{t('settings.infoTitle')}</h2>
            </div>
            <p style={{ fontSize: '12px', color: '#64748b', marginBottom: '16px' }}>
              {t('settings.infoDescription')}
            </p>
            <div style={{ padding: '12px', backgroundColor: '#eff6ff', borderRadius: '12px', border: '1px solid #dbeafe' }}>
              <p style={{ fontSize: '10px', fontWeight: 'bold', color: '#2563eb', textTransform: 'uppercase', marginBottom: '4px' }}>
                {t('settings.tipTitle')}
              </p>
              <p style={{ fontSize: '11px', color: '#1d4ed8' }}>
                {t('settings.tipDescription')}
              </p>
            </div>
          </div>

          <div style={{ backgroundColor: '#0f172a', borderRadius: '16px', padding: '24px', boxShadow: '0 25px 50px -12px rgba(0, 0, 0, 0.25)' }}>
            <h3 style={{ fontSize: '14px', fontWeight: 'bold', color: 'white', marginBottom: '12px' }}>{t('settings.helpTitle')}</h3>
            <ul style={{ listStyle: 'none', padding: 0, margin: 0 }}>
              <li style={{ display: 'flex', alignItems: 'flex-start', gap: '8px', fontSize: '11px', color: '#94a3b8', marginBottom: '8px' }}>
                <div style={{ width: '4px', height: '4px', borderRadius: '50%', backgroundColor: '#3b82f6', marginTop: '6px', flexShrink: 0 }} />
                <span>{t('settings.help1')}</span>
              </li>
              <li style={{ display: 'flex', alignItems: 'flex-start', gap: '8px', fontSize: '11px', color: '#94a3b8', marginBottom: '8px' }}>
                <div style={{ width: '4px', height: '4px', borderRadius: '50%', backgroundColor: '#3b82f6', marginTop: '6px', flexShrink: 0 }} />
                <span>{t('settings.help2')}</span>
              </li>
              <li style={{ display: 'flex', alignItems: 'flex-start', gap: '8px', fontSize: '11px', color: '#94a3b8' }}>
                <div style={{ width: '4px', height: '4px', borderRadius: '50%', backgroundColor: '#3b82f6', marginTop: '6px', flexShrink: 0 }} />
                <span>{t('settings.help3')}</span>
              </li>
            </ul>
          </div>
        </div>
      </div>

      <style>{`
        @keyframes spin {
          from { transform: rotate(0deg); }
          to { transform: rotate(360deg); }
        }
      `}</style>
    </div>
  );
}