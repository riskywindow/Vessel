import { useQuery } from '@tanstack/react-query';
import { fetchMetrics, type MetricPoint } from '../api/client';
import { useWebSocket } from '../hooks/useWebSocket';
import { useState, useEffect } from 'react';
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts';

interface MetricsChartProps {
  containerId: string;
}

function formatBytes(bytes: number) {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
  return `${(bytes / 1024 / 1024 / 1024).toFixed(1)} GB`;
}

export function MetricsChart({ containerId }: MetricsChartProps) {
  const [liveMetrics, setLiveMetrics] = useState<MetricPoint[]>([]);
  const { lastMessage } = useWebSocket(
    `/api/ws/containers/${containerId}/metrics`
  );

  // Historical metrics
  const { data: history = [] } = useQuery({
    queryKey: ['metrics', containerId],
    queryFn: () => fetchMetrics(containerId, 100),
  });

  // Append live metrics
  useEffect(() => {
    if (lastMessage?.type === 'metric' && lastMessage.data) {
      setLiveMetrics((prev) => [...prev.slice(-50), lastMessage.data as MetricPoint]);
    }
  }, [lastMessage]);

  // Reset live metrics when container changes
  useEffect(() => {
    setLiveMetrics([]);
  }, [containerId]);

  const allMetrics = [...history, ...liveMetrics];

  const chartData = allMetrics.map((m) => ({
    time: new Date(m.timestamp).toLocaleTimeString(),
    timestamp: m.timestamp,
    cpu_percent: m.cpu_percent,
    memory_bytes: m.memory_bytes,
  }));

  if (chartData.length === 0) {
    return (
      <div className="bg-white rounded-lg shadow p-6">
        <div className="h-64 flex items-center justify-center text-gray-500">
          No metrics available yet. Metrics are collected every 15 seconds.
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* CPU Chart */}
      <div className="bg-white rounded-lg shadow p-6">
        <h3 className="text-lg font-medium mb-4">CPU Usage</h3>
        <ResponsiveContainer width="100%" height={200}>
          <LineChart data={chartData}>
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis dataKey="time" tick={{ fontSize: 10 }} />
            <YAxis unit="%" domain={[0, 100]} tick={{ fontSize: 10 }} />
            <Tooltip
              labelFormatter={(_, payload) => {
                if (payload?.[0]?.payload?.timestamp) {
                  return new Date(payload[0].payload.timestamp).toLocaleString();
                }
                return '';
              }}
              formatter={(value: number | undefined) => [`${(value ?? 0).toFixed(1)}%`, 'CPU']}
            />
            <Line
              type="monotone"
              dataKey="cpu_percent"
              stroke="#3b82f6"
              strokeWidth={2}
              dot={false}
            />
          </LineChart>
        </ResponsiveContainer>
      </div>

      {/* Memory Chart */}
      <div className="bg-white rounded-lg shadow p-6">
        <h3 className="text-lg font-medium mb-4">Memory Usage</h3>
        <ResponsiveContainer width="100%" height={200}>
          <LineChart data={chartData}>
            <CartesianGrid strokeDasharray="3 3" />
            <XAxis dataKey="time" tick={{ fontSize: 10 }} />
            <YAxis tickFormatter={formatBytes} tick={{ fontSize: 10 }} />
            <Tooltip
              labelFormatter={(_, payload) => {
                if (payload?.[0]?.payload?.timestamp) {
                  return new Date(payload[0].payload.timestamp).toLocaleString();
                }
                return '';
              }}
              formatter={(value: number | undefined) => [formatBytes(value ?? 0), 'Memory']}
            />
            <Line
              type="monotone"
              dataKey="memory_bytes"
              stroke="#10b981"
              strokeWidth={2}
              dot={false}
            />
          </LineChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}
