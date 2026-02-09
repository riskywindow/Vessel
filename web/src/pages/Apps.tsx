import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import {
  fetchApps,
  stopApp,
  removeApp,
  restartApp,
  type App,
} from '../api/client';
import { StatusBadge, StatusDot } from '../components/StatusBadge';
import {
  Square,
  Trash2,
  RotateCw,
  ChevronRight,
} from 'lucide-react';
import { timeAgo } from '../lib/utils';

export function AppsPage() {
  const queryClient = useQueryClient();
  const { data: apps = [], isLoading } = useQuery({
    queryKey: ['apps'],
    queryFn: fetchApps,
    refetchInterval: 5000,
  });

  const stopMutation = useMutation({
    mutationFn: (name: string) => stopApp(name),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['apps'] }),
  });

  const removeMutation = useMutation({
    mutationFn: (name: string) => removeApp(name),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['apps'] }),
  });

  const restartMutation = useMutation({
    mutationFn: (name: string) => restartApp(name),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['apps'] }),
  });

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-gray-500">Loading apps...</div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-gray-900">Applications</h1>
        <span className="text-sm text-gray-500">{apps.length} apps</span>
      </div>

      {apps.length === 0 ? (
        <div className="bg-white shadow rounded-lg p-12 text-center">
          <p className="text-gray-500">No applications deployed yet.</p>
          <p className="text-sm text-gray-400 mt-2">
            Use <code className="bg-gray-100 px-1.5 py-0.5 rounded">vessel deploy</code> to get started.
          </p>
        </div>
      ) : (
        <div className="grid gap-4">
          {apps.map((app) => (
            <AppCard
              key={app.app.id}
              app={app}
              onStop={() => stopMutation.mutate(app.app.name)}
              onRemove={() => removeMutation.mutate(app.app.name)}
              onRestart={() => restartMutation.mutate(app.app.name)}
            />
          ))}
        </div>
      )}
    </div>
  );
}

function AppCard({
  app,
  onStop,
  onRemove,
  onRestart,
}: {
  app: App;
  onStop: () => void;
  onRemove: () => void;
  onRestart: () => void;
}) {
  const running = app.containers?.filter((c) => c.state === 'running').length ?? 0;
  const total = app.containers?.length ?? 0;

  return (
    <div className="bg-white shadow rounded-lg p-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <StatusDot status={app.app.state} />
          <div>
            <Link
              to={`/apps/${app.app.name}`}
              className="text-lg font-medium text-gray-900 hover:text-vessel-600 flex items-center gap-1"
            >
              {app.app.name}
              <ChevronRight className="h-4 w-4 text-gray-400" />
            </Link>
            <p className="text-sm text-gray-500 truncate max-w-md">
              {app.app.image}
            </p>
          </div>
        </div>

        <div className="flex items-center gap-3">
          <StatusBadge status={app.app.state} />
          <span className="text-sm text-gray-500">
            {running}/{total} instances
          </span>
        </div>
      </div>

      <div className="mt-4 flex items-center justify-between">
        <div className="text-xs text-gray-400">
          Updated {timeAgo(app.app.updated_at)}
        </div>
        <div className="flex gap-2">
          {app.app.state === 'running' && (
            <>
              <button
                onClick={onRestart}
                className="inline-flex items-center gap-1 px-3 py-1.5 text-xs font-medium text-gray-700 bg-gray-100 rounded-md hover:bg-gray-200"
              >
                <RotateCw className="h-3.5 w-3.5" />
                Restart
              </button>
              <button
                onClick={onStop}
                className="inline-flex items-center gap-1 px-3 py-1.5 text-xs font-medium text-yellow-700 bg-yellow-50 rounded-md hover:bg-yellow-100"
              >
                <Square className="h-3.5 w-3.5" />
                Stop
              </button>
            </>
          )}
          {app.app.state === 'stopped' && (
            <button
              onClick={onRemove}
              className="inline-flex items-center gap-1 px-3 py-1.5 text-xs font-medium text-red-700 bg-red-50 rounded-md hover:bg-red-100"
            >
              <Trash2 className="h-3.5 w-3.5" />
              Remove
            </button>
          )}
        </div>
      </div>

      {/* Container list */}
      {app.containers && app.containers.length > 0 && (
        <div className="mt-4 border-t pt-3">
          <div className="grid gap-2">
            {app.containers.map((c) => (
              <div
                key={c.id}
                className="flex items-center justify-between text-sm"
              >
                <div className="flex items-center gap-2">
                  <StatusDot status={c.state} />
                  <code className="text-xs text-gray-600">
                    {c.id.substring(0, 12)}
                  </code>
                </div>
                <div className="flex items-center gap-4 text-xs text-gray-500">
                  {c.ip && <span>{c.ip}</span>}
                  <span>PID {c.pid}</span>
                  {c.restart_count > 0 && (
                    <span className="text-yellow-600">
                      {c.restart_count} restarts
                    </span>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
