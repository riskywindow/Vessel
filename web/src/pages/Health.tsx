import { useQuery } from '@tanstack/react-query';
import { fetchAllHealth, fetchApps } from '../api/client';
import { StatusBadge } from '../components/StatusBadge';
import { useWebSocket } from '../hooks/useWebSocket';
import { Activity, CheckCircle, XCircle, AlertCircle } from 'lucide-react';
import { timeAgo } from '../lib/utils';

export function HealthPage() {
  const { data: health = {} } = useQuery({
    queryKey: ['health'],
    queryFn: fetchAllHealth,
    refetchInterval: 5000,
  });

  const { data: apps = [] } = useQuery({
    queryKey: ['apps'],
    queryFn: fetchApps,
    refetchInterval: 10000,
  });

  // Subscribe to live health events
  const { lastMessage } = useWebSocket('/api/ws/health');

  const entries = Object.entries(health);
  const healthyCount = entries.filter(([, h]) => h.status === 'healthy').length;
  const unhealthyCount = entries.filter(
    ([, h]) => h.status === 'unhealthy'
  ).length;
  const unknownCount = entries.filter(([, h]) => h.status === 'unknown').length;

  // Build container ID to app name lookup
  const containerToApp: Record<string, string> = {};
  for (const app of apps) {
    for (const c of app.containers ?? []) {
      containerToApp[c.id] = app.app.name;
    }
  }

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-gray-900">Health Status</h1>

      {/* Summary */}
      <div className="grid grid-cols-1 gap-6 sm:grid-cols-3">
        <div className="bg-white shadow rounded-lg p-6 flex items-center gap-4">
          <div className="p-3 rounded-lg bg-green-100">
            <CheckCircle className="h-6 w-6 text-green-600" />
          </div>
          <div>
            <p className="text-sm text-gray-500">Healthy</p>
            <p className="text-2xl font-semibold text-gray-900">
              {healthyCount}
            </p>
          </div>
        </div>
        <div className="bg-white shadow rounded-lg p-6 flex items-center gap-4">
          <div className="p-3 rounded-lg bg-red-100">
            <XCircle className="h-6 w-6 text-red-600" />
          </div>
          <div>
            <p className="text-sm text-gray-500">Unhealthy</p>
            <p className="text-2xl font-semibold text-gray-900">
              {unhealthyCount}
            </p>
          </div>
        </div>
        <div className="bg-white shadow rounded-lg p-6 flex items-center gap-4">
          <div className="p-3 rounded-lg bg-gray-100">
            <AlertCircle className="h-6 w-6 text-gray-600" />
          </div>
          <div>
            <p className="text-sm text-gray-500">Unknown</p>
            <p className="text-2xl font-semibold text-gray-900">
              {unknownCount}
            </p>
          </div>
        </div>
      </div>

      {/* Health Table */}
      <div className="bg-white shadow rounded-lg overflow-hidden">
        <div className="px-6 py-4 border-b border-gray-200 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Activity className="h-5 w-5 text-gray-400" />
            <h2 className="text-lg font-medium text-gray-900">
              Container Health
            </h2>
          </div>
          {lastMessage && (
            <span className="text-xs text-green-500 flex items-center gap-1">
              <span className="w-1.5 h-1.5 bg-green-500 rounded-full animate-pulse" />
              Live
            </span>
          )}
        </div>
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                  Container
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                  App
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                  Status
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                  Consecutive Fails
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                  Last Check
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                  Message
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200">
              {entries.map(([id, h]) => (
                <tr key={id} className="hover:bg-gray-50">
                  <td className="px-6 py-4">
                    <code className="text-sm font-mono text-gray-700">
                      {id.substring(0, 12)}
                    </code>
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-500">
                    {containerToApp[id] || h.app_id || 'unknown'}
                  </td>
                  <td className="px-6 py-4">
                    <StatusBadge status={h.status} />
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-500">
                    {h.consecutive_fails > 0 ? (
                      <span className="text-red-600 font-medium">
                        {h.consecutive_fails}
                      </span>
                    ) : (
                      <span className="text-gray-400">0</span>
                    )}
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-500">
                    {h.last_check ? timeAgo(h.last_check) : 'never'}
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-500 truncate max-w-48">
                    {h.message || '\u2014'}
                  </td>
                </tr>
              ))}
              {entries.length === 0 && (
                <tr>
                  <td
                    colSpan={6}
                    className="px-6 py-12 text-center text-gray-500"
                  >
                    No health checks registered
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
