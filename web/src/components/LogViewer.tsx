import { useEffect, useRef, useState } from 'react';
import { useWebSocket } from '../hooks/useWebSocket';

interface LogViewerProps {
  containerId: string;
}

export function LogViewer({ containerId }: LogViewerProps) {
  const { connected, messages, clear } = useWebSocket(
    `/api/ws/containers/${containerId}/logs`
  );
  const logRef = useRef<HTMLDivElement>(null);
  const [autoScroll, setAutoScroll] = useState(true);

  useEffect(() => {
    if (autoScroll && logRef.current) {
      logRef.current.scrollTop = logRef.current.scrollHeight;
    }
  }, [messages, autoScroll]);

  return (
    <div className="bg-gray-900 rounded-lg overflow-hidden">
      <div className="flex items-center justify-between px-4 py-2 bg-gray-800 border-b border-gray-700">
        <div className="flex items-center gap-2">
          <div
            className={`w-2 h-2 rounded-full ${
              connected ? 'bg-green-500' : 'bg-red-500'
            }`}
          />
          <span className="text-sm text-gray-300">
            {connected ? 'Connected' : 'Disconnected'}
          </span>
        </div>
        <div className="flex items-center gap-4">
          <label className="flex items-center gap-2 text-sm text-gray-300">
            <input
              type="checkbox"
              checked={autoScroll}
              onChange={(e) => setAutoScroll(e.target.checked)}
              className="rounded"
            />
            Auto-scroll
          </label>
          <button
            onClick={clear}
            className="text-sm text-gray-400 hover:text-white"
          >
            Clear
          </button>
        </div>
      </div>
      <div
        ref={logRef}
        className="h-[500px] overflow-auto p-4 font-mono text-sm text-gray-100"
      >
        {messages.map((msg, i) => (
          <div key={i} className="hover:bg-gray-800 px-1 -mx-1">
            <span className="text-gray-500">
              {new Date(msg.timestamp).toLocaleTimeString()}
            </span>{' '}
            <span>
              {typeof msg.data === 'object' && msg.data !== null && 'line' in msg.data
                ? (msg.data as { line: string }).line
                : typeof msg.data === 'string'
                  ? msg.data
                  : JSON.stringify(msg.data)}
            </span>
          </div>
        ))}
        {messages.length === 0 && (
          <div className="text-gray-500">Waiting for logs...</div>
        )}
      </div>
    </div>
  );
}
