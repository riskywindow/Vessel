import { type Container } from '../api/client';
import { StatusBadge, StatusDot } from './StatusBadge';

interface ContainerCardProps {
  container: Container;
  onSelect?: () => void;
  isSelected?: boolean;
}

export function ContainerCard({ container, onSelect, isSelected }: ContainerCardProps) {
  return (
    <div
      onClick={onSelect}
      className={`bg-white shadow rounded-lg p-5 cursor-pointer transition-all ${
        isSelected
          ? 'ring-2 ring-vessel-500 shadow-md'
          : 'hover:shadow-md'
      }`}
    >
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-3">
          <StatusDot status={container.state} />
          <code className="text-sm font-mono font-medium">
            {container.id.substring(0, 12)}
          </code>
        </div>
        <StatusBadge status={container.state} />
      </div>
      <div className="grid grid-cols-3 gap-4 text-xs text-gray-500">
        <div>
          <span className="font-medium text-gray-700">IP:</span>{' '}
          {container.ip || 'N/A'}
        </div>
        <div>
          <span className="font-medium text-gray-700">PID:</span>{' '}
          {container.pid}
        </div>
        <div>
          <span className="font-medium text-gray-700">Restarts:</span>{' '}
          {container.restart_count}
        </div>
      </div>
      {container.image && (
        <div className="mt-2 text-xs text-gray-400 truncate">
          {container.image}
        </div>
      )}
    </div>
  );
}
