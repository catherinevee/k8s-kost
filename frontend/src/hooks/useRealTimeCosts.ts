import { useState, useEffect, useCallback } from 'react';

interface CostData {
  namespace: string;
  costs: any[];
  summary: {
    total: number;
    average_daily: number;
    projected_monthly: number;
  };
  breakdown: {
    compute: number;
    storage: number;
    network: number;
    other: number;
  };
}

interface WebSocketMessage {
  type: string;
  namespace?: string;
  data: any;
  timestamp: string;
}

export const useRealTimeCosts = (namespace: string) => {
  const [costs, setCosts] = useState<CostData | null>(null);
  const [isConnected, setIsConnected] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const connectWebSocket = useCallback(() => {
    const ws = new WebSocket(`ws://${window.location.host}/ws`);
    
    ws.onopen = () => {
      setIsConnected(true);
      setError(null);
      
      // Subscribe to namespace updates
      ws.send(JSON.stringify({
        type: 'subscribe',
        namespace: namespace
      }));
    };

    ws.onmessage = (event) => {
      try {
        const message: WebSocketMessage = JSON.parse(event.data);
        
        if (message.type === 'cost_update' && message.namespace === namespace) {
          setCosts(message.data);
        }
      } catch (err) {
        console.error('Error parsing WebSocket message:', err);
      }
    };

    ws.onclose = () => {
      setIsConnected(false);
      // Attempt to reconnect after 5 seconds
      setTimeout(() => {
        if (!isConnected) {
          connectWebSocket();
        }
      }, 5000);
    };

    ws.onerror = (err) => {
      setError('WebSocket connection error');
      console.error('WebSocket error:', err);
    };

    return ws;
  }, [namespace, isConnected]);

  useEffect(() => {
    const ws = connectWebSocket();

    return () => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.close();
      }
    };
  }, [connectWebSocket]);

  // Fallback to REST API if WebSocket is not available
  useEffect(() => {
    if (!isConnected) {
      const fetchCosts = async () => {
        try {
          const response = await fetch(`/api/costs/namespace/${namespace}`);
          if (response.ok) {
            const data = await response.json();
            setCosts(data);
          }
        } catch (err) {
          setError('Failed to fetch cost data');
        }
      };

      fetchCosts();
      const interval = setInterval(fetchCosts, 30000); // Poll every 30 seconds

      return () => clearInterval(interval);
    }
  }, [namespace, isConnected]);

  return { costs, isConnected, error };
}; 