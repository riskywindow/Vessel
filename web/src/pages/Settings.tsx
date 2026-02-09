import { useState, useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import { ping, fetchSystemInfo } from '../api/client';
import { Settings as SettingsIcon, Key, Server, CheckCircle, XCircle } from 'lucide-react';

export function SettingsPage() {
  const [apiKey, setApiKey] = useState('');
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    const stored = localStorage.getItem('vessel_api_key');
    if (stored) setApiKey(stored);
  }, []);

  const { data: pingResult, error: pingError } = useQuery({
    queryKey: ['ping'],
    queryFn: ping,
    refetchInterval: 10000,
    retry: false,
  });

  const { data: system } = useQuery({
    queryKey: ['system'],
    queryFn: fetchSystemInfo,
    refetchInterval: 30000,
    retry: false,
  });

  const connected = !!pingResult && !pingError;

  function handleSave() {
    if (apiKey.trim()) {
      localStorage.setItem('vessel_api_key', apiKey.trim());
    } else {
      localStorage.removeItem('vessel_api_key');
    }
    setSaved(true);
    setTimeout(() => setSaved(false), 2000);
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-gray-900">Settings</h1>

      {/* Connection Status */}
      <div className="bg-white shadow rounded-lg p-6">
        <div className="flex items-center gap-3 mb-4">
          <Server className="h-5 w-5 text-gray-400" />
          <h2 className="text-lg font-medium text-gray-900">
            Connection Status
          </h2>
        </div>
        <div className="space-y-3">
          <div className="flex items-center gap-3">
            {connected ? (
              <>
                <CheckCircle className="h-5 w-5 text-green-500" />
                <span className="text-sm text-green-700">
                  Connected to Vessel API
                </span>
              </>
            ) : (
              <>
                <XCircle className="h-5 w-5 text-red-500" />
                <span className="text-sm text-red-700">
                  Not connected to Vessel API
                </span>
              </>
            )}
          </div>
          {system && (
            <div className="text-sm text-gray-500 space-y-1">
              <p>Version: {system.version}</p>
              <p>
                Apps: {system.apps} | Containers: {system.containers} (
                {system.running_containers} running)
              </p>
            </div>
          )}
        </div>
      </div>

      {/* API Key */}
      <div className="bg-white shadow rounded-lg p-6">
        <div className="flex items-center gap-3 mb-4">
          <Key className="h-5 w-5 text-gray-400" />
          <h2 className="text-lg font-medium text-gray-900">API Key</h2>
        </div>
        <p className="text-sm text-gray-500 mb-4">
          Configure the API key to authenticate with the Vessel API. Generate
          one with <code className="bg-gray-100 px-1.5 py-0.5 rounded">vessel api key-generate</code>.
        </p>
        <div className="flex gap-3">
          <input
            type="password"
            value={apiKey}
            onChange={(e) => setApiKey(e.target.value)}
            placeholder="Enter your API key"
            className="flex-1 rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-vessel-500 focus:ring-vessel-500 focus:outline-none"
          />
          <button
            onClick={handleSave}
            className="px-4 py-2 text-sm font-medium text-white bg-vessel-600 rounded-md hover:bg-vessel-700"
          >
            {saved ? 'Saved!' : 'Save'}
          </button>
        </div>
      </div>

      {/* About */}
      <div className="bg-white shadow rounded-lg p-6">
        <div className="flex items-center gap-3 mb-4">
          <SettingsIcon className="h-5 w-5 text-gray-400" />
          <h2 className="text-lg font-medium text-gray-900">About</h2>
        </div>
        <div className="text-sm text-gray-500 space-y-2">
          <p>
            <span className="font-medium text-gray-700">Vessel</span> &mdash; A
            single-binary container orchestrator for the rest of us.
          </p>
          <p>
            Deploy apps on any Linux server using raw Linux primitives. No
            Docker required.
          </p>
        </div>
      </div>
    </div>
  );
}
