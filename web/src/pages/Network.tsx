import { useQuery } from '@tanstack/react-query';
import { fetchNetworkRoutes } from '../api/client';
import { Network as NetworkIcon, Globe } from 'lucide-react';

export function NetworkPage() {
  const { data: routes, isLoading } = useQuery({
    queryKey: ['routes'],
    queryFn: fetchNetworkRoutes,
    refetchInterval: 10000,
  });

  const routeEntries = routes ? Object.entries(routes) : [];

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-gray-900">Network</h1>

      <div className="bg-white shadow rounded-lg overflow-hidden">
        <div className="px-6 py-4 border-b border-gray-200 flex items-center gap-2">
          <NetworkIcon className="h-5 w-5 text-gray-400" />
          <h2 className="text-lg font-medium text-gray-900">Routes</h2>
        </div>

        {isLoading ? (
          <div className="p-8 text-center text-gray-500">Loading routes...</div>
        ) : routeEntries.length > 0 ? (
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                    Hostname
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                    Backends
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200">
                {routeEntries.map(([hostname, backends]) => (
                  <tr key={hostname} className="hover:bg-gray-50">
                    <td className="px-6 py-4">
                      <div className="flex items-center gap-2">
                        <Globe className="h-4 w-4 text-gray-400" />
                        <span className="text-sm font-medium text-gray-900">
                          {hostname}
                        </span>
                      </div>
                    </td>
                    <td className="px-6 py-4">
                      <div className="flex flex-wrap gap-2">
                        {(backends as { ip?: string; port?: number; container_id?: string }[]).map(
                          (b, i) => (
                            <span
                              key={i}
                              className="inline-flex items-center px-2 py-1 rounded text-xs bg-gray-100 text-gray-700 font-mono"
                            >
                              {b.ip}:{b.port}
                              {b.container_id && (
                                <span className="ml-1 text-gray-400">
                                  ({(b.container_id as string).substring(0, 8)})
                                </span>
                              )}
                            </span>
                          )
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <div className="p-12 text-center text-gray-500">
            <NetworkIcon className="h-12 w-12 mx-auto mb-3 text-gray-300" />
            <p>No routes configured</p>
            <p className="text-sm text-gray-400 mt-1">
              Routes are created automatically when apps are deployed with
              domain configuration.
            </p>
          </div>
        )}
      </div>
    </div>
  );
}
