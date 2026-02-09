import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import {
  fetchApps,
  fetchSystemInfo,
  fetchAllHealth,
  type App,
  type HealthStatus,
} from '../api/client';
import { StatusBadge, StatusDot } from '../components/StatusBadge';
import {
  Box,
  Server,
  CheckCircle,
  AlertCircle,
  XCircle,
} from 'lucide-react';
import { timeAgo } from '../lib/utils';

function StatCard({
  title,
  value,
  icon: Icon,
  color,
}: {
  title: string;
  value: string | number;
  icon: React.ElementType;
  color: string;
}) {
  return (
    <div className="bg-white rounded-lg shadow p-6">
      <div className="flex items-center">
        <div className={`p-3 rounded-lg ${color}`}>
          <Icon className="h-6 w-6 text-white" />
        </div>
        <div className="ml-4">
          <p className="text-sm font-medium text-gray-500">{title}</p>
          <p className="text-2xl font-semibold text-gray-900">{value}</p>
        </div>
      </div>
    </div>
  );
}

function AppRow({
  app,
  health,
}: {
  app: App;
  health: Record<string, HealthStatus>;
}) {
  const runningContainers = app.containers?.filter(
    (c) => c.state === 'running'
  ).length ?? 0;
  const totalContainers = app.containers?.length ?? 0;
  const containerHealth = (app.containers ?? [])
    .map((c) => health[c.id])
    .filter(Boolean);
  const healthyCount = containerHealth.filter(
    (h) => h.status === 'healthy'
  ).length;
  const unhealthyCount = containerHealth.filter(
    (h) => h.status === 'unhealthy'
  ).length;

  return (
    <tr className="hover:bg-gray-50">
      <td className="px-6 py-4 whitespace-nowrap">
        <div className="flex items-center gap-3">
          <StatusDot status={app.app.state} />
          <div>
            <Link
              to={`/apps/${app.app.name}`}
              className="text-sm font-medium text-gray-900 hover:text-vessel-600"
            >
              {app.app.name}
            </Link>
            <div className="text-xs text-gray-500 truncate max-w-64">
              {app.app.image}
            </div>
          </div>
        </div>
      </td>
      <td className="px-6 py-4 whitespace-nowrap">
        <StatusBadge status={app.app.state} />
      </td>
      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
        {runningContainers}/{totalContainers}
      </td>
      <td className="px-6 py-4 whitespace-nowrap">
        <div className="flex items-center gap-2">
          {healthyCount > 0 && (
            <span className="flex items-center text-green-600 text-sm">
              <CheckCircle className="h-4 w-4 mr-1" />
              {healthyCount}
            </span>
          )}
          {unhealthyCount > 0 && (
            <span className="flex items-center text-red-600 text-sm">
              <XCircle className="h-4 w-4 mr-1" />
              {unhealthyCount}
            </span>
          )}
          {healthyCount === 0 && unhealthyCount === 0 && (
            <span className="text-gray-400 text-sm">&mdash;</span>
          )}
        </div>
      </td>
      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
        {timeAgo(app.app.updated_at)}
      </td>
    </tr>
  );
}

export function Dashboard() {
  const { data: apps = [] } = useQuery({
    queryKey: ['apps'],
    queryFn: fetchApps,
    refetchInterval: 5000,
  });
  const { data: system } = useQuery({
    queryKey: ['system'],
    queryFn: fetchSystemInfo,
    refetchInterval: 10000,
  });
  const { data: health = {} } = useQuery({
    queryKey: ['health'],
    queryFn: fetchAllHealth,
    refetchInterval: 5000,
  });

  const healthyContainers = Object.values(health).filter(
    (h) => h.status === 'healthy'
  ).length;
  const unhealthyContainers = Object.values(health).filter(
    (h) => h.status === 'unhealthy'
  ).length;

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-gray-900">Dashboard</h1>

      {/* Stats */}
      <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          title="Total Apps"
          value={system?.apps ?? 0}
          icon={Box}
          color="bg-blue-500"
        />
        <StatCard
          title="Running Containers"
          value={system?.running_containers ?? 0}
          icon={Server}
          color="bg-green-500"
        />
        <StatCard
          title="Healthy"
          value={healthyContainers}
          icon={CheckCircle}
          color="bg-emerald-500"
        />
        <StatCard
          title="Unhealthy"
          value={unhealthyContainers}
          icon={AlertCircle}
          color={unhealthyContainers > 0 ? 'bg-red-500' : 'bg-gray-400'}
        />
      </div>

      {/* Apps Table */}
      <div className="bg-white shadow rounded-lg overflow-hidden">
        <div className="px-6 py-4 border-b border-gray-200">
          <h2 className="text-lg font-medium text-gray-900">Applications</h2>
        </div>
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  App
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Status
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Instances
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Health
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Updated
                </th>
              </tr>
            </thead>
            <tbody className="bg-white divide-y divide-gray-200">
              {apps.map((app) => (
                <AppRow key={app.app.id} app={app} health={health} />
              ))}
              {apps.length === 0 && (
                <tr>
                  <td
                    colSpan={5}
                    className="px-6 py-12 text-center text-gray-500"
                  >
                    No applications deployed yet
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
