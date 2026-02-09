import axios from 'axios';

const API_BASE = import.meta.env.VITE_API_URL || '';

export const api = axios.create({
  baseURL: API_BASE,
  headers: {
    'Content-Type': 'application/json',
  },
});

api.interceptors.request.use((config) => {
  const apiKey = localStorage.getItem('vessel_api_key');
  if (apiKey) {
    config.headers['X-API-Key'] = apiKey;
  }
  return config;
});

// Types matching the Go REST API responses

export interface App {
  app: {
    id: string;
    name: string;
    image: string;
    instances: number;
    state: 'running' | 'stopped' | 'deploying' | 'failed';
    created_at: string;
    updated_at: string;
  };
  containers: Container[];
}

export interface Container {
  id: string;
  app_id: string;
  image: string;
  state: 'created' | 'running' | 'stopped' | 'failed';
  ip: string;
  pid: number;
  restart_count: number;
  created_at: string;
  started_at?: string;
}

export interface Deploy {
  id: string;
  app_id: string;
  image: string;
  version: number;
  strategy: string;
  status: 'pending' | 'active' | 'failed' | 'rolled_back';
  created_at: string;
  finished_at?: string;
  error?: string;
}

export interface HealthStatus {
  container_id: string;
  app_id: string;
  status: 'healthy' | 'unhealthy' | 'unknown';
  last_check: string;
  consecutive_fails: number;
  message?: string;
}

export interface MetricPoint {
  timestamp: string;
  cpu_percent: number;
  memory_bytes: number;
  memory_limit: number;
  pids_count: number;
  network_rx_bytes: number;
  network_tx_bytes: number;
}

export interface SystemInfo {
  version: string;
  apps: number;
  containers: number;
  running_containers: number;
}

// API response wrappers
interface APIResponse<T> {
  data: T;
  error: null;
}

interface APIError {
  data: null;
  error: { code: string; message: string };
}

type APIResult<T> = APIResponse<T> | APIError;

// Unwrap API responses
function unwrap<T>(response: { data: APIResult<T> }): T {
  if (response.data.error) {
    throw new Error(response.data.error.message);
  }
  return response.data.data;
}

// App API
export const fetchApps = () => api.get<APIResult<App[]>>('/api/apps').then(unwrap);
export const fetchApp = (name: string) => api.get<APIResult<App>>(`/api/apps/${name}`).then(unwrap);
export const stopApp = (name: string) => api.post(`/api/apps/${name}/stop`);
export const removeApp = (name: string) => api.delete(`/api/apps/${name}`);
export const restartApp = (name: string) => api.post(`/api/apps/${name}/restart`);
export const deployApp = (name: string, config: Record<string, unknown>) =>
  api.post(`/api/apps/${name}/deploy`, config);
export const rollbackApp = (name: string, version: number) =>
  api.post(`/api/apps/${name}/rollback`, { version });
export const fetchDeployHistory = (name: string) =>
  api.get<APIResult<Deploy[]>>(`/api/apps/${name}/history`).then(unwrap);

// Container API
export const fetchContainers = (appName?: string) => {
  const params = appName ? `?app=${appName}` : '';
  return api.get<APIResult<Container[]>>(`/api/containers${params}`).then(unwrap);
};
export const fetchContainer = (id: string) =>
  api.get<APIResult<Container>>(`/api/containers/${id}`).then(unwrap);
export const fetchContainerStats = (id: string) =>
  api.get<APIResult<MetricPoint>>(`/api/containers/${id}/stats`).then(unwrap);

// Health API
export const fetchAllHealth = () =>
  api.get<APIResult<Record<string, HealthStatus>>>('/api/health').then(unwrap);
export const fetchHealthHistory = (containerId: string) =>
  api.get<APIResult<unknown[]>>(`/api/health/${containerId}/history`).then(unwrap);
export const checkHealthNow = (containerId: string) =>
  api.post(`/api/health/${containerId}/check`);

// Metrics API
export const fetchMetrics = (containerId: string, limit = 100) =>
  api.get<APIResult<MetricPoint[]>>(`/api/metrics/${containerId}?limit=${limit}`).then(unwrap);
export const fetchLatestMetrics = (containerId: string) =>
  api.get<APIResult<MetricPoint>>(`/api/metrics/${containerId}/latest`).then(unwrap);

// Secrets API
export const fetchSecrets = () =>
  api.get<APIResult<string[]>>('/api/secrets').then(unwrap);
export const setSecret = (key: string, value: string) =>
  api.post('/api/secrets', { key, value });
export const deleteSecret = (key: string) =>
  api.delete(`/api/secrets/${key}`);

// Network API
export const fetchNetworkRoutes = () =>
  api.get<APIResult<Record<string, unknown[]>>>('/api/network/routes').then(unwrap);

// Certs API
export const fetchCerts = () =>
  api.get<APIResult<unknown>>('/api/certs').then(unwrap);

// System API
export const fetchSystemInfo = () =>
  api.get<APIResult<SystemInfo>>('/api/system').then(unwrap);

export const ping = () => api.get('/api/ping').then((r) => r.data);
