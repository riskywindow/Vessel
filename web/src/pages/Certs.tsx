import { useQuery } from '@tanstack/react-query';
import { fetchCerts } from '../api/client';
import { Shield, Lock } from 'lucide-react';

export function CertsPage() {
  const { data: certInfo, isLoading } = useQuery({
    queryKey: ['certs'],
    queryFn: fetchCerts,
    refetchInterval: 30000,
  });

  const info = certInfo as {
    tls_enabled?: boolean;
    acme_enabled?: boolean;
    custom_certs?: string[];
  } | null;

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-gray-900">Certificates</h1>

      <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
        <div className="bg-white shadow rounded-lg p-6">
          <div className="flex items-center gap-3 mb-4">
            <Lock className="h-5 w-5 text-gray-400" />
            <h2 className="text-lg font-medium text-gray-900">TLS Status</h2>
          </div>
          {isLoading ? (
            <p className="text-gray-500">Loading...</p>
          ) : (
            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <span className="text-sm text-gray-600">TLS Enabled</span>
                <span
                  className={`text-sm font-medium ${
                    info?.tls_enabled ? 'text-green-600' : 'text-gray-400'
                  }`}
                >
                  {info?.tls_enabled ? 'Yes' : 'No'}
                </span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-sm text-gray-600">ACME (Let's Encrypt)</span>
                <span
                  className={`text-sm font-medium ${
                    info?.acme_enabled ? 'text-green-600' : 'text-gray-400'
                  }`}
                >
                  {info?.acme_enabled ? 'Active' : 'Inactive'}
                </span>
              </div>
            </div>
          )}
        </div>

        <div className="bg-white shadow rounded-lg p-6">
          <div className="flex items-center gap-3 mb-4">
            <Shield className="h-5 w-5 text-gray-400" />
            <h2 className="text-lg font-medium text-gray-900">
              Custom Certificates
            </h2>
          </div>
          {isLoading ? (
            <p className="text-gray-500">Loading...</p>
          ) : info?.custom_certs && info.custom_certs.length > 0 ? (
            <ul className="space-y-2">
              {info.custom_certs.map((cert) => (
                <li
                  key={cert}
                  className="flex items-center gap-2 text-sm text-gray-700"
                >
                  <Lock className="h-3.5 w-3.5 text-green-500" />
                  {cert}
                </li>
              ))}
            </ul>
          ) : (
            <p className="text-sm text-gray-500">
              No custom certificates loaded.
              <br />
              <span className="text-xs text-gray-400">
                Use <code>vessel cert import</code> to add custom certificates.
              </span>
            </p>
          )}
        </div>
      </div>
    </div>
  );
}
