import { cn } from '../lib/utils';

const statusColors: Record<string, string> = {
  running: 'bg-green-100 text-green-800',
  stopped: 'bg-gray-100 text-gray-800',
  deploying: 'bg-yellow-100 text-yellow-800',
  failed: 'bg-red-100 text-red-800',
  created: 'bg-blue-100 text-blue-800',
  healthy: 'bg-green-100 text-green-800',
  unhealthy: 'bg-red-100 text-red-800',
  unknown: 'bg-gray-100 text-gray-800',
  active: 'bg-green-100 text-green-800',
  pending: 'bg-gray-100 text-gray-800',
  rolled_back: 'bg-yellow-100 text-yellow-800',
};

const dotColors: Record<string, string> = {
  running: 'bg-green-500',
  stopped: 'bg-gray-400',
  deploying: 'bg-yellow-500',
  failed: 'bg-red-500',
  healthy: 'bg-green-500',
  unhealthy: 'bg-red-500',
  unknown: 'bg-gray-400',
};

export function StatusBadge({ status }: { status: string }) {
  return (
    <span
      className={cn(
        'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium',
        statusColors[status] || 'bg-gray-100 text-gray-800'
      )}
    >
      {status}
    </span>
  );
}

export function StatusDot({ status }: { status: string }) {
  return (
    <span
      className={cn(
        'inline-block w-2 h-2 rounded-full',
        dotColors[status] || 'bg-gray-400'
      )}
    />
  );
}
