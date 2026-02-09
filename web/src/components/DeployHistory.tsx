import { type Deploy } from '../api/client';
import { StatusBadge } from './StatusBadge';
import { Undo2 } from 'lucide-react';
import { timeAgo } from '../lib/utils';

interface DeployHistoryProps {
  deploys: Deploy[];
  onRollback: (version: number) => void;
  isRollingBack?: boolean;
}

export function DeployHistory({ deploys, onRollback, isRollingBack }: DeployHistoryProps) {
  return (
    <div className="bg-white shadow rounded-lg overflow-hidden">
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
            {deploys.map((d) => (
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
                  {(d.status === 'active' || d.status === 'rolled_back') && (
                    <button
                      onClick={() => onRollback(d.version)}
                      disabled={isRollingBack}
                      className="inline-flex items-center gap-1 text-xs text-vessel-600 hover:text-vessel-700 disabled:opacity-50"
                    >
                      <Undo2 className="h-3.5 w-3.5" />
                      Rollback
                    </button>
                  )}
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
  );
}
