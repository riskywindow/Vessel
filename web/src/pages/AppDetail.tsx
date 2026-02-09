import { useParams, Link } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  fetchApp,
  fetchDeployHistory,
  stopApp,
  restartApp,
  rollbackApp,
  fetchMetrics,
  type Deploy,
  type MetricPoint,
} from '../api/client';
import { StatusBadge, StatusDot } from '../components/StatusBadge';
import { useWebSocket } from '../hooks/useWebSocket';
import {
  ArrowLeft,
  RotateCw,
  Square,
  Undo2,
  Terminal,
} from 'lucide-react';
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts';
import { timeAgo } from '../lib/utils';

export function AppDetailPage() {
  const { name } = useParams<{ name: string }>();
  const queryClient = useQueryClient();

  const { data: appData } = useQuery({
    queryKey: ['app', name],
    queryFn: () => fetchApp(name!),
    refetchInterval: 5000,
    enabled: !!name,
  });

  const { data: deploys = [] } = useQuery({
    queryKey: ['deploys', name],
    queryFn: () => fetchDeployHistory(name!),
    enabled: !!name,
  });

  // Get metrics for first running container
  const firstContainer = appData?.containers?.find(
    (c) => c.state === 'running'
  );
  const { data: metrics = [] } = useQuery({
    queryKey: ['metrics', firstContainer?.id],
    queryFn: () => fetchMetrics(firstContainer!.id, 60),
    refetchInterval: 15000,
    enabled: !!firstContainer,
  });

  // Live log streaming
  const { messages: logMessages } = useWebSocket(
    firstContainer ? `/api/ws/containers/${firstContainer.id}/logs` : '',
    !!firstContainer
  );

  const stopMutation = useMutation({
    mutationFn: () => stopApp(name!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['app', name] });
      queryClient.invalidateQueries({ queryKey: ['apps'] });
    },
  });

  const restartMutation = useMutation({
    mutationFn: () => restartApp(name!),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['app', name] });
    },
  });

  const rollbackMutation = useMutation({
    mutationFn: (version: number) => rollbackApp(name!, version),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['app', name] });
      queryClient.invalidateQueries({ queryKey: ['deploys', name] });
    },
  });

  if (!appData) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-gray-500">Loading...</div>
      </div>
    );
  }

  const { app, containers } = appData;

  const chartData = metrics.map((m: MetricPoint) => ({
    time: new Date(m.timestamp).toLocaleTimeString(),
    cpu: m.cpu_percent,
    memory: m.memory_bytes / (1024 * 1024),
  }));

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <Link to="/apps" className="text-gray-400 hover:text-gray-600">
            <ArrowLeft className="h-5 w-5" />
          </Link>
          <div>
            <div className="flex items-center gap-3">
              <h1 className="text-2xl font-bold text-gray-900">{app.name}</h1>
              <StatusBadge status={app.state} />
            </div>
            <p className="text-sm text-gray-500 mt-1">{app.image}</p>
          </div>
        </div>

        <div className="flex gap-2">
          {app.state === 'running' && (
            <>
              <button
                onClick={() => restartMutation.mutate()}
                className="inline-flex items-center gap-2 px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50"
              >
                <RotateCw className="h-4 w-4" />
                Restart
              </button>
              <button
                onClick={() => stopMutation.mutate()}
                className="inline-flex items-center gap-2 px-4 py-2 text-sm font-medium text-red-700 bg-white border border-red-300 rounded-md hover:bg-red-50"
              >
                <Square className="h-4 w-4" />
                Stop
              </button>
            </>
          )}
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Containers */}
        <div className="bg-white shadow rounded-lg">
          <div className="px-6 py-4 border-b border-gray-200">
            <h2 className="text-lg font-medium text-gray-900">Containers</h2>
          </div>
          <div className="divide-y divide-gray-200">
            {(containers ?? []).map((c) => (
              <div key={c.id} className="px-6 py-4">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <StatusDot status={c.state} />
                    <code className="text-sm font-mono">
                      {c.id.substring(0, 12)}
                    </code>
                  </div>
                  <StatusBadge status={c.state} />
                </div>
                <div className="mt-2 grid grid-cols-3 gap-4 text-xs text-gray-500">
                  <div>
                    <span className="font-medium text-gray-700">IP:</span>{' '}
                    {c.ip || 'N/A'}
                  </div>
                  <div>
                    <span className="font-medium text-gray-700">PID:</span>{' '}
                    {c.pid}
                  </div>
                  <div>
                    <span className="font-medium text-gray-700">Restarts:</span>{' '}
                    {c.restart_count}
                  </div>
                </div>
              </div>
            ))}
            {(!containers || containers.length === 0) && (
              <div className="px-6 py-8 text-center text-gray-500">
                No containers
              </div>
            )}
          </div>
        </div>

        {/* Metrics Chart */}
        <div className="bg-white shadow rounded-lg">
          <div className="px-6 py-4 border-b border-gray-200">
            <h2 className="text-lg font-medium text-gray-900">
              Resource Usage
            </h2>
          </div>
          <div className="p-6">
            {chartData.length > 0 ? (
              <div className="space-y-6">
                <div>
                  <h3 className="text-sm font-medium text-gray-700 mb-2">
                    CPU %
                  </h3>
                  <ResponsiveContainer width="100%" height={150}>
                    <LineChart data={chartData}>
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis dataKey="time" tick={{ fontSize: 10 }} />
                      <YAxis tick={{ fontSize: 10 }} />
                      <Tooltip />
                      <Line
                        type="monotone"
                        dataKey="cpu"
                        stroke="#0ea5e9"
                        strokeWidth={2}
                        dot={false}
                      />
                    </LineChart>
                  </ResponsiveContainer>
                </div>
                <div>
                  <h3 className="text-sm font-medium text-gray-700 mb-2">
                    Memory (MB)
                  </h3>
                  <ResponsiveContainer width="100%" height={150}>
                    <LineChart data={chartData}>
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis dataKey="time" tick={{ fontSize: 10 }} />
                      <YAxis tick={{ fontSize: 10 }} />
                      <Tooltip />
                      <Line
                        type="monotone"
                        dataKey="memory"
                        stroke="#10b981"
                        strokeWidth={2}
                        dot={false}
                      />
                    </LineChart>
                  </ResponsiveContainer>
                </div>
              </div>
            ) : (
              <div className="h-64 flex items-center justify-center text-gray-500">
                No metrics available
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Logs */}
      <div className="bg-white shadow rounded-lg">
        <div className="px-6 py-4 border-b border-gray-200 flex items-center gap-2">
          <Terminal className="h-5 w-5 text-gray-400" />
          <h2 className="text-lg font-medium text-gray-900">Logs</h2>
          <span className="text-xs text-gray-400">
            {logMessages.length > 0 ? 'Live' : 'Waiting for logs...'}
          </span>
        </div>
        <div className="bg-gray-900 p-4 rounded-b-lg max-h-80 overflow-y-auto font-mono text-xs">
          {logMessages.length > 0 ? (
            logMessages.map((msg, i) => (
              <div key={i} className="text-gray-300 leading-5">
                {typeof msg.data === 'string' ? msg.data : JSON.stringify(msg.data)}
              </div>
            ))
          ) : (
            <div className="text-gray-600">No log output yet</div>
          )}
        </div>
      </div>

      {/* Deploy History */}
      <div className="bg-white shadow rounded-lg overflow-hidden">
        <div className="px-6 py-4 border-b border-gray-200">
          <h2 className="text-lg font-medium text-gray-900">Deploy History</h2>
        </div>
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                  Version
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                  Status
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                  Strategy
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                  Image
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                  Created
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200">
              {deploys.map((d: Deploy) => (
                <tr key={d.id} className="hover:bg-gray-50">
                  <td className="px-6 py-4 text-sm font-medium text-gray-900">
                    v{d.version}
                  </td>
                  <td className="px-6 py-4">
                    <StatusBadge status={d.status} />
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-500">
                    {d.strategy}
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-500 truncate max-w-48">
                    {d.image}
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-500">
                    {timeAgo(d.created_at)}
                  </td>
                  <td className="px-6 py-4">
                    {d.status === 'active' || d.status === 'rolled_back' ? (
                      <button
                        onClick={() => rollbackMutation.mutate(d.version)}
                        className="inline-flex items-center gap-1 text-xs text-vessel-600 hover:text-vessel-700"
                      >
                        <Undo2 className="h-3.5 w-3.5" />
                        Rollback
                      </button>
                    ) : null}
                  </td>
                </tr>
              ))}
              {deploys.length === 0 && (
                <tr>
                  <td
                    colSpan={6}
                    className="px-6 py-8 text-center text-gray-500"
                  >
                    No deploys yet
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
