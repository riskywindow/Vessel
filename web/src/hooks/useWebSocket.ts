import { useEffect, useRef, useState, useCallback } from 'react';

interface WSMessage {
  type: string;
  timestamp: string;
  data: unknown;
}

export function useWebSocket(path: string, enabled = true) {
  const [connected, setConnected] = useState(false);
  const [messages, setMessages] = useState<WSMessage[]>([]);
  const [lastMessage, setLastMessage] = useState<WSMessage | null>(null);
  const ws = useRef<WebSocket | null>(null);
  const reconnectTimeout = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  const connect = useCallback(() => {
    if (!enabled) return;

    const apiKey = localStorage.getItem('vessel_api_key');
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host =
      import.meta.env.VITE_API_URL?.replace(/^https?:\/\//, '') ||
      window.location.host;
    const url = `${protocol}//${host}${path}${apiKey ? `?api_key=${apiKey}` : ''}`;

    ws.current = new WebSocket(url);

    ws.current.onopen = () => {
      setConnected(true);
    };

    ws.current.onclose = () => {
      setConnected(false);
      reconnectTimeout.current = setTimeout(connect, 3000);
    };

    ws.current.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data) as WSMessage;
        setLastMessage(msg);
        setMessages((prev) => [...prev.slice(-199), msg]);
      } catch {
        // ignore malformed messages
      }
    };

    ws.current.onerror = () => {
      // onclose will handle reconnect
    };
  }, [path, enabled]);

  useEffect(() => {
    connect();
    return () => {
      clearTimeout(reconnectTimeout.current);
      ws.current?.close();
    };
  }, [connect]);

  const send = useCallback((data: unknown) => {
    if (ws.current?.readyState === WebSocket.OPEN) {
      ws.current.send(JSON.stringify(data));
    }
  }, []);

  const clear = useCallback(() => {
    setMessages([]);
  }, []);

  return { connected, messages, lastMessage, send, clear };
}
