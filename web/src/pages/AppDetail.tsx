import { useParams, Link } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  fetchApp,
  fetchDeployHistory,
  stopApp,
  restartApp,
  rollbackApp,
} from '../api/client';
import { StatusBadge } from '../components/StatusBadge';
import { ContainerCard } from '../components/ContainerCard';
import { MetricsChart } from '../components/MetricsChart';
import { LogViewer } from '../components/LogViewer';
import { DeployHistory } from '../components/DeployHistory';
import { useState } from 'react';
import { ArrowLeft, RotateCw, Square } from 'lucide-react';

type Tab = 'overview' | 'logs' | 'metrics' | 'history';

export function AppDetailPage() {
  const { name } = useParams<{ name: string }>();
  const queryClient = useQueryClient();
  const [activeTab, setActiveTab] = useState<Tab>('overview');
  const [selectedContainer, setSelectedContainer] = useState<string | null>(null);

  const { data: appData, isLoading } = useQuery({
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

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-gray-500">Loading...</div>
      </div>
    );
  }

  if (!appData) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-gray-500">App not found</div>
      </div>
    );
  }

  const { app, containers } = appData;
  const firstRunningContainer = containers?.find((c) => c.state === 'running');

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
                disabled={restartMutation.isPending}
                className="inline-flex items-center gap-2 px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50 disabled:opacity-50"
              >
                <RotateCw className="h-4 w-4" />
                Restart
              </button>
              <button
                onClick={() => stopMutation.mutate()}
                disabled={stopMutation.isPending}
                className="inline-flex items-center gap-2 px-4 py-2 text-sm font-medium text-red-700 bg-white border border-red-300 rounded-md hover:bg-red-50 disabled:opacity-50"
              >
                <Square className="h-4 w-4" />
                Stop
              </button>
            </>
          )}
        </div>
      </div>

      {/* Tabs */}
      <div className="border-b border-gray-200">
        <nav className="-mb-px flex space-x-8">
          {(['overview', 'logs', 'metrics', 'history'] as Tab[]).map((tab) => (
            <button
              key={tab}
              onClick={() => setActiveTab(tab)}
              className={`py-4 px-1 border-b-2 font-medium text-sm ${
                activeTab === tab
                  ? 'border-vessel-500 text-vessel-600'
                  : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
              }`}
            >
              {tab.charAt(0).toUpperCase() + tab.slice(1)}
            </button>
          ))}
        </nav>
      </div>

      {/* Tab Content */}
      {activeTab === 'overview' && (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          {(containers ?? []).map((container) => (
            <ContainerCard
              key={container.id}
              container={container}
              onSelect={() => setSelectedContainer(container.id)}
              isSelected={selectedContainer === container.id}
            />
          ))}
          {(!containers || containers.length === 0) && (
            <div className="col-span-2 bg-white shadow rounded-lg px-6 py-8 text-center text-gray-500">
              No containers
            </div>
          )}
        </div>
      )}

      {activeTab === 'logs' && (
        firstRunningContainer ? (
          <LogViewer containerId={firstRunningContainer.id} />
        ) : (
          <div className="bg-white shadow rounded-lg px-6 py-12 text-center text-gray-500">
            No running containers to stream logs from
          </div>
        )
      )}

      {activeTab === 'metrics' && (
        firstRunningContainer ? (
          <MetricsChart containerId={firstRunningContainer.id} />
        ) : (
          <div className="bg-white shadow rounded-lg px-6 py-12 text-center text-gray-500">
            No running containers to display metrics for
          </div>
        )
      )}

      {activeTab === 'history' && (
        <DeployHistory
          deploys={deploys}
          onRollback={(version) => rollbackMutation.mutate(version)}
          isRollingBack={rollbackMutation.isPending}
        />
      )}
    </div>
  );
}
