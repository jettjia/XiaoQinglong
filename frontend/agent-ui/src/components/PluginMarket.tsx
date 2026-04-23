import React, { useState, useEffect, useCallback } from 'react';
import {
  Plug,
  Check,
  X,
  ExternalLink,
  Copy,
  RefreshCw,
  Trash2,
  Loader2,
  AlertCircle,
  CheckCircle,
  XCircle,
  Clock
} from 'lucide-react';
import { motion, AnimatePresence } from 'motion/react';
import { cn } from '../lib/utils';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { pluginApi, type Plugin, type PluginInstance, type StartAuthResponse, type PollAuthResponse } from '../lib/api';

export function PluginMarket() {
  const { t } = useTranslation();
  const [plugins, setPlugins] = useState<Plugin[]>([]);
  const [instances, setInstances] = useState<PluginInstance[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Auth modal state
  const [authModal, setAuthModal] = useState<{
    open: boolean;
    plugin: Plugin | null;
    authResponse: StartAuthResponse | null;
    polling: boolean;
    pollStatus: 'pending' | 'authorized' | 'expired' | 'denied' | null;
    error: string | null;
  }>({
    open: false,
    plugin: null,
    authResponse: null,
    polling: false,
    pollStatus: null,
    error: null,
  });

  // Load plugins and instances
  const loadData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      const [pluginsRes, instancesRes] = await Promise.all([
        pluginApi.getPlugins(),
        pluginApi.getUserInstances(),
      ]);
      setPlugins(pluginsRes.plugins || []);
      setInstances(instancesRes.instances || []);
    } catch (err: any) {
      console.error('Failed to load plugins:', err);
      setError(err.message || 'Failed to load plugins');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadData();
  }, [loadData]);

  // Start authorization
  const handleStartAuth = async (plugin: Plugin) => {
    try {
      const authResponse = await pluginApi.startAuth(plugin.id);
      setAuthModal({
        open: true,
        plugin,
        authResponse,
        polling: false,
        pollStatus: 'pending',
        error: null,
      });
    } catch (err: any) {
      toast.error(err.message || 'Failed to start authorization');
    }
  };

  // Poll authorization status
  const handlePollAuth = useCallback(async () => {
    if (!authModal.authResponse?.state) return;

    setAuthModal(prev => ({ ...prev, polling: true, error: null }));

    try {
      const pollResponse = await pluginApi.pollAuth(authModal.authResponse.state);
      setAuthModal(prev => ({
        ...prev,
        polling: false,
        pollStatus: pollResponse.status,
      }));

      if (pollResponse.status === 'authorized') {
        toast.success(`${authModal.plugin?.name} connected successfully!`);
        // Don't auto-close - let user click "Done" button
      } else if (pollResponse.status === 'expired' || pollResponse.status === 'denied') {
        setAuthModal(prev => ({
          ...prev,
          error: pollResponse.status === 'expired' ? 'Authorization expired' : 'Authorization denied',
        }));
      }
    } catch (err: any) {
      setAuthModal(prev => ({
        ...prev,
        polling: false,
        error: err.message || 'Polling failed',
      }));
    }
  }, [authModal.authResponse?.state]);

  // Start polling when auth modal opens with authResponse
  useEffect(() => {
    if (!authModal.open || !authModal.authResponse?.state || authModal.pollStatus !== 'pending') {
      return;
    }

    const interval = authModal.authResponse.interval || 5;
    const timeout = (authModal.authResponse.expires_in || 300) * 1000;

    const pollTimeout = setTimeout(() => {
      setAuthModal(prev => {
        if (prev.pollStatus === 'pending') {
          return { ...prev, polling: false, pollStatus: 'expired', error: 'Authorization timed out' };
        }
        return prev;
      });
    }, timeout);

    const pollInterval = setInterval(() => {
      setAuthModal(prev => {
        if (prev.pollStatus === 'pending' && !prev.polling) {
          handlePollAuth();
        }
        return prev;
      });
    }, interval * 1000);

    // Initial poll
    handlePollAuth();

    return () => {
      clearTimeout(pollTimeout);
      clearInterval(pollInterval);
    };
  }, [authModal.open, authModal.authResponse?.state, authModal.pollStatus]);

  // Delete instance
  const handleDeleteInstance = async (ulid: string) => {
    if (!confirm('Are you sure you want to remove this plugin authorization?')) {
      return;
    }
    try {
      await pluginApi.deleteInstance(ulid);
      toast.success('Plugin removed successfully');
      loadData();
    } catch (err: any) {
      toast.error(err.message || 'Failed to remove plugin');
    }
  };

  // Refresh token
  const handleRefreshToken = async (ulid: string) => {
    try {
      const result = await pluginApi.refreshToken(ulid);
      if (result.status === 'active') {
        toast.success('Token refreshed successfully');
      } else {
        toast.error('Token expired, please re-authorize');
      }
      loadData();
    } catch (err: any) {
      toast.error(err.message || 'Failed to refresh token');
    }
  };

  // Copy to clipboard
  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    toast.success('Copied to clipboard');
  };

  // Get plugin instance by plugin ID
  const getInstanceByPluginId = (pluginId: string): PluginInstance | undefined => {
    return instances.find(inst => inst.plugin_id === pluginId);
  };

  // Get plugin status
  const getPluginStatus = (plugin: Plugin): 'available' | 'authorized' => {
    if (plugin.status === 'authorized' || getInstanceByPluginId(plugin.id)) {
      return 'authorized';
    }
    return 'available';
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <Loader2 className="w-8 h-8 animate-spin text-blue-500" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center h-full gap-4">
        <AlertCircle className="w-12 h-12 text-red-500" />
        <p className="text-gray-600">{error}</p>
        <button
          onClick={loadData}
          className="px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600"
        >
          Retry
        </button>
      </div>
    );
  }

  return (
    <div className="p-6 max-w-6xl mx-auto">
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-gray-900">{t('sidebar.plugins')}</h1>
        <p className="text-gray-600 mt-1">Connect your data sources to enable intelligent search and retrieval.</p>
      </div>

      {/* Plugin Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        {plugins.map((plugin) => {
          const instance = getInstanceByPluginId(plugin.id);
          const isAuthorized = getPluginStatus(plugin) === 'authorized';

          return (
            <motion.div
              key={plugin.id}
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              className={cn(
                "bg-white rounded-xl border p-6 transition-all",
                isAuthorized ? "border-green-200 bg-green-50" : "border-gray-200 hover:border-blue-300 hover:shadow-md"
              )}
            >
              <div className="flex items-start justify-between mb-4">
                <div className="flex items-center gap-3">
                  <div className={cn(
                    "w-12 h-12 rounded-xl flex items-center justify-center text-2xl",
                    isAuthorized ? "bg-green-100" : "bg-gray-100"
                  )}>
                    {plugin.icon}
                  </div>
                  <div>
                    <h3 className="font-semibold text-gray-900">{plugin.name}</h3>
                    <p className="text-sm text-gray-500">v{plugin.version}</p>
                  </div>
                </div>
                {isAuthorized && (
                  <span className="flex items-center gap-1 px-2 py-1 bg-green-100 text-green-700 text-xs font-medium rounded-full">
                    <CheckCircle className="w-3 h-3" />
                    Connected
                  </span>
                )}
              </div>

              <p className="text-gray-600 text-sm mb-4">{plugin.description}</p>

              {isAuthorized && instance ? (
                <div className="space-y-3 mb-4">
                  <div className="flex items-center gap-2 text-sm">
                    {instance.user_info?.avatar ? (
                      <img
                        src={instance.user_info.avatar}
                        alt={instance.user_info.name}
                        className="w-6 h-6 rounded-full"
                      />
                    ) : (
                      <div className="w-6 h-6 rounded-full bg-gray-200 flex items-center justify-center">
                        <Plug className="w-3 h-3 text-gray-500" />
                      </div>
                    )}
                    <span className="text-gray-700">{instance.user_info?.name || 'User'}</span>
                    <span className={cn(
                      "px-1.5 py-0.5 text-xs rounded",
                      instance.status === 'active' ? "bg-green-100 text-green-700" : "bg-red-100 text-red-700"
                    )}>
                      {instance.status}
                    </span>
                  </div>
                  {instance.expires_at && (
                    <div className="flex items-center gap-1 text-xs text-gray-500">
                      <Clock className="w-3 h-3" />
                      Expires: {new Date(instance.expires_at).toLocaleDateString()}
                    </div>
                  )}
                </div>
              ) : null}

              <div className="flex gap-2">
                {isAuthorized && instance ? (
                  <>
                    <button
                      onClick={() => handleRefreshToken(instance.ulid)}
                      className="flex-1 flex items-center justify-center gap-1 px-3 py-2 text-sm bg-gray-100 hover:bg-gray-200 text-gray-700 rounded-lg transition-colors"
                    >
                      <RefreshCw className="w-4 h-4" />
                      Refresh
                    </button>
                    <button
                      onClick={() => handleDeleteInstance(instance.ulid)}
                      className="flex items-center justify-center gap-1 px-3 py-2 text-sm bg-red-50 hover:bg-red-100 text-red-600 rounded-lg transition-colors"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </>
                ) : (
                  <button
                    onClick={() => handleStartAuth(plugin)}
                    className="flex-1 flex items-center justify-center gap-2 px-4 py-2 text-sm bg-blue-500 hover:bg-blue-600 text-white rounded-lg transition-colors"
                  >
                    <Plug className="w-4 h-4" />
                    Connect
                  </button>
                )}
              </div>
            </motion.div>
          );
        })}
      </div>

      {/* Auth Modal */}
      <AnimatePresence>
        {authModal.open && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4"
            onClick={() => !authModal.polling && setAuthModal({ open: false, plugin: null, authResponse: null, polling: false, pollStatus: null, error: null })}
          >
            <motion.div
              initial={{ scale: 0.95, opacity: 0 }}
              animate={{ scale: 1, opacity: 1 }}
              exit={{ scale: 0.95, opacity: 0 }}
              className="bg-white rounded-2xl shadow-xl max-w-md w-full p-6"
              onClick={(e) => e.stopPropagation()}
            >
              <div className="flex items-center justify-between mb-6">
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 rounded-xl bg-blue-100 flex items-center justify-center text-xl">
                    {authModal.plugin?.icon}
                  </div>
                  <div>
                    <h3 className="font-semibold text-gray-900">Connect {authModal.plugin?.name}</h3>
                    <p className="text-sm text-gray-500">Device Authorization</p>
                  </div>
                </div>
                <button
                  onClick={() => setAuthModal({ open: false, plugin: null, authResponse: null, polling: false, pollStatus: null, error: null })}
                  disabled={authModal.polling}
                  className="p-2 hover:bg-gray-100 rounded-lg disabled:opacity-50"
                >
                  <X className="w-5 h-5 text-gray-500" />
                </button>
              </div>

              {authModal.pollStatus === 'pending' && (
                <div className="space-y-4">
                  <div className="bg-blue-50 border border-blue-200 rounded-lg p-4">
                    <h4 className="font-medium text-blue-900 mb-3">Step 1: Open Authorization URL</h4>
                    <div className="flex items-center gap-2 mb-2">
                      <code className="flex-1 bg-white px-3 py-2 rounded border text-sm break-all">
                        {authModal.authResponse?.verification_url}
                      </code>
                      <button
                        onClick={() => copyToClipboard(authModal.authResponse?.verification_url || '')}
                        className="p-2 hover:bg-blue-100 rounded-lg"
                      >
                        <Copy className="w-4 h-4 text-blue-600" />
                      </button>
                    </div>
                    <a
                      href={authModal.authResponse?.verification_url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex items-center gap-1 text-sm text-blue-600 hover:text-blue-700"
                    >
                      Open in browser <ExternalLink className="w-3 h-3" />
                    </a>
                  </div>

                  <div className="bg-blue-50 border border-blue-200 rounded-lg p-4">
                    <h4 className="font-medium text-blue-900 mb-3">Step 2: Enter User Code</h4>
                    <div className="flex items-center gap-2">
                      <code className="flex-1 bg-white px-3 py-2 rounded border text-2xl font-mono font-bold text-center tracking-widest">
                        {authModal.authResponse?.user_code}
                      </code>
                      <button
                        onClick={() => copyToClipboard(authModal.authResponse?.user_code || '')}
                        className="p-2 hover:bg-blue-100 rounded-lg"
                      >
                        <Copy className="w-4 h-4 text-blue-600" />
                      </button>
                    </div>
                  </div>

                  <div className="flex items-center justify-center gap-2 py-4">
                    <Loader2 className="w-5 h-5 animate-spin text-blue-500" />
                    <span className="text-gray-600">Waiting for authorization...</span>
                  </div>

                  <p className="text-xs text-gray-500 text-center">
                    This code will expire in {authModal.authResponse?.expires_in} seconds.
                    Polling every {authModal.authResponse?.interval} seconds.
                  </p>
                </div>
              )}

              {authModal.pollStatus === 'authorized' && (
                <div className="text-center py-8">
                  <motion.div
                    initial={{ scale: 0 }}
                    animate={{ scale: 1 }}
                    className="w-20 h-20 bg-green-100 rounded-full flex items-center justify-center mx-auto mb-4"
                  >
                    <motion.div
                      initial={{ scale: 0 }}
                      animate={{ scale: 1 }}
                      transition={{ delay: 0.2, type: "spring" }}
                    >
                      <CheckCircle className="w-10 h-10 text-green-500" />
                    </motion.div>
                  </motion.div>
                  <motion.h3
                    initial={{ opacity: 0, y: 10 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ delay: 0.3 }}
                    className="text-xl font-bold text-gray-900 mb-2"
                  >
                    🎉 Authorization Successful!
                  </motion.h3>
                  <motion.p
                    initial={{ opacity: 0, y: 10 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ delay: 0.4 }}
                    className="text-gray-600 mb-4"
                  >
                    Your <span className="font-semibold">{authModal.plugin?.name}</span> account has been connected!
                  </motion.p>
                  <motion.button
                    initial={{ opacity: 0, y: 10 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ delay: 0.5 }}
                    onClick={() => {
                      setAuthModal({ open: false, plugin: null, authResponse: null, polling: false, pollStatus: null, error: null });
                      loadData();
                    }}
                    className="mt-4 px-6 py-2 bg-green-500 hover:bg-green-600 text-white font-medium rounded-lg transition-colors"
                  >
                    Done
                  </motion.button>
                </div>
              )}

              {authModal.pollStatus === 'expired' && (
                <div className="text-center py-8">
                  <div className="w-16 h-16 bg-red-100 rounded-full flex items-center justify-center mx-auto mb-4">
                    <XCircle className="w-8 h-8 text-red-500" />
                  </div>
                  <h3 className="text-lg font-semibold text-gray-900 mb-2">Authorization Expired</h3>
                  <p className="text-gray-600 mb-4">Please try again.</p>
                  <button
                    onClick={() => authModal.plugin && handleStartAuth(authModal.plugin)}
                    className="px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600"
                  >
                    Try Again
                  </button>
                </div>
              )}

              {authModal.pollStatus === 'denied' && (
                <div className="text-center py-8">
                  <div className="w-16 h-16 bg-red-100 rounded-full flex items-center justify-center mx-auto mb-4">
                    <XCircle className="w-8 h-8 text-red-500" />
                  </div>
                  <h3 className="text-lg font-semibold text-gray-900 mb-2">Authorization Denied</h3>
                  <p className="text-gray-600 mb-4">You denied the authorization request.</p>
                  <button
                    onClick={() => authModal.plugin && handleStartAuth(authModal.plugin)}
                    className="px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600"
                  >
                    Try Again
                  </button>
                </div>
              )}

              {authModal.error && authModal.pollStatus !== 'expired' && authModal.pollStatus !== 'denied' && (
                <div className="mt-4 p-3 bg-red-50 border border-red-200 rounded-lg">
                  <p className="text-sm text-red-600">{authModal.error}</p>
                </div>
              )}
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}