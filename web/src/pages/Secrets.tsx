import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { fetchSecrets, setSecret, deleteSecret } from '../api/client';
import { Key, Plus, Trash2 } from 'lucide-react';

export function SecretsPage() {
  const queryClient = useQueryClient();
  const [showForm, setShowForm] = useState(false);
  const [newKey, setNewKey] = useState('');
  const [newValue, setNewValue] = useState('');

  const { data: secrets = [], isLoading } = useQuery({
    queryKey: ['secrets'],
    queryFn: fetchSecrets,
    refetchInterval: 10000,
  });

  const addMutation = useMutation({
    mutationFn: () => setSecret(newKey, newValue),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['secrets'] });
      setNewKey('');
      setNewValue('');
      setShowForm(false);
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (key: string) => deleteSecret(key),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['secrets'] }),
  });

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold text-gray-900">Secrets</h1>
        <button
          onClick={() => setShowForm(!showForm)}
          className="inline-flex items-center gap-2 px-4 py-2 text-sm font-medium text-white bg-vessel-600 rounded-md hover:bg-vessel-700"
        >
          <Plus className="h-4 w-4" />
          Add Secret
        </button>
      </div>

      {showForm && (
        <div className="bg-white shadow rounded-lg p-6">
          <h3 className="text-sm font-medium text-gray-900 mb-4">
            New Secret
          </h3>
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700">
                Key
              </label>
              <input
                type="text"
                value={newKey}
                onChange={(e) => setNewKey(e.target.value)}
                placeholder="MY_SECRET_KEY"
                className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-vessel-500 focus:ring-vessel-500 focus:outline-none"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700">
                Value
              </label>
              <input
                type="password"
                value={newValue}
                onChange={(e) => setNewValue(e.target.value)}
                placeholder="secret value"
                className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-vessel-500 focus:ring-vessel-500 focus:outline-none"
              />
            </div>
            <div className="flex gap-2">
              <button
                onClick={() => addMutation.mutate()}
                disabled={!newKey || !newValue}
                className="px-4 py-2 text-sm font-medium text-white bg-vessel-600 rounded-md hover:bg-vessel-700 disabled:opacity-50"
              >
                Save
              </button>
              <button
                onClick={() => setShowForm(false)}
                className="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50"
              >
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}

      <div className="bg-white shadow rounded-lg overflow-hidden">
        <div className="px-6 py-4 border-b border-gray-200 flex items-center gap-2">
          <Key className="h-5 w-5 text-gray-400" />
          <h2 className="text-lg font-medium text-gray-900">
            Stored Secrets
          </h2>
          <span className="text-sm text-gray-400">({secrets.length})</span>
        </div>

        {isLoading ? (
          <div className="p-8 text-center text-gray-500">
            Loading secrets...
          </div>
        ) : secrets.length > 0 ? (
          <ul className="divide-y divide-gray-200">
            {secrets.map((key) => (
              <li
                key={key}
                className="px-6 py-4 flex items-center justify-between hover:bg-gray-50"
              >
                <div className="flex items-center gap-3">
                  <Key className="h-4 w-4 text-gray-400" />
                  <code className="text-sm font-mono text-gray-900">{key}</code>
                </div>
                <div className="flex items-center gap-3">
                  <code className="text-xs text-gray-400">
                    {'${'}secret:{key}{'}'}
                  </code>
                  <button
                    onClick={() => deleteMutation.mutate(key)}
                    className="p-1 text-gray-400 hover:text-red-600 rounded"
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>
              </li>
            ))}
          </ul>
        ) : (
          <div className="p-12 text-center text-gray-500">
            <Key className="h-12 w-12 mx-auto mb-3 text-gray-300" />
            <p>No secrets stored</p>
            <p className="text-sm text-gray-400 mt-1">
              Secrets are encrypted with AES-256-GCM and referenced via{' '}
              <code className="bg-gray-100 px-1 rounded">
                {'${'}secret:key{'}'}
              </code>
            </p>
          </div>
        )}
      </div>
    </div>
  );
}
