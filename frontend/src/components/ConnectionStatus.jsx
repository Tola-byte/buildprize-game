import { useState, useEffect } from 'react';
import { wsService } from '../services/websocket';
import './ConnectionStatus.css';

export default function ConnectionStatus() {
  const [status, setStatus] = useState(wsService.getConnectionStatus());

  useEffect(() => {
    const handleStatusChange = (data) => {
      setStatus(data.status);
    };

    const handleConnected = () => setStatus('connected');
    const handleDisconnected = () => setStatus('disconnected');
    const handleError = () => setStatus('error');

    wsService.on('status', handleStatusChange);
    wsService.on('connected', handleConnected);
    wsService.on('disconnected', handleDisconnected);
    wsService.on('error', handleError);

    // Set initial status - check actual WebSocket state
    setStatus(wsService.getConnectionStatus());

    // Also poll periodically to catch any state mismatches (every 500ms)
    const statusCheckInterval = setInterval(() => {
      const currentStatus = wsService.getConnectionStatus();
      setStatus(prevStatus => {
        // Only update if status actually changed
        if (prevStatus !== currentStatus) {
          return currentStatus;
        }
        return prevStatus;
      });
    }, 500);

    return () => {
      clearInterval(statusCheckInterval);
      wsService.off('status', handleStatusChange);
      wsService.off('connected', handleConnected);
      wsService.off('disconnected', handleDisconnected);
      wsService.off('error', handleError);
    };
  }, []);

  const getStatusText = () => {
    switch (status) {
      case 'connected':
        return 'Connected';
      case 'connecting':
        return 'Connecting...';
      case 'disconnected':
        return 'Disconnected';
      case 'error':
        return 'Connection Error';
      default:
        return 'Unknown';
    }
  };

  const getStatusClass = () => {
    switch (status) {
      case 'connected':
        return 'status-connected';
      case 'connecting':
        return 'status-connecting';
      case 'disconnected':
        return 'status-disconnected';
      case 'error':
        return 'status-error';
      default:
        return '';
    }
  };

  return (
    <div className={`connection-status ${getStatusClass()}`}>
      <span className="status-dot"></span>
      <span className="status-text">{getStatusText()}</span>
    </div>
  );
}

