'use client';

import { useState, useEffect, useCallback, useRef } from 'react';

export type SSEStatus = 'idle' | 'connecting' | 'connected' | 'disconnected' | 'error';

export interface SSEEvent<T = unknown> {
  type: string;
  data: T;
  timestamp: Date;
}

export interface ProgressEvent {
  type: 'started' | 'progress' | 'warning' | 'complete' | 'error' | 'info' | 'debug';
  job_id: string;
  progress: number;
  phase: string;
  message: string;
  
  // Chunk info
  total_chunks?: number;
  completed_chunks?: number;
  current_chunk?: number;
  
  // Detailed info for logs UI
  detail?: string;           // Additional detail (e.g., page range, chunk content preview)
  page_range?: string;       // e.g., "pages 1-4"
  duration?: string;         // Human-readable duration
  bytes_size?: number;       // Size in bytes (for downloads)
  token_count?: number;      // Token count for LLM calls
  subjects_found?: number;   // Subjects found in chunk
  
  // Error info
  error_type?: string;
  error_message?: string;
  retry_count?: number;
  max_retries?: number;
  recoverable?: boolean;
  
  // Result info
  result_syllabus_ids?: number[];
  result_subjects?: Array<{
    id: number;
    name: string;
    code: string;
    credits: number;
  }>;
  
  // Database stats
  units_created?: number;
  topics_created?: number;
  books_created?: number;
  
  // Timing
  elapsed_ms?: number;
  timestamp: string;
}

export interface UseSSEOptions {
  url: string;
  token?: string;
  onOpen?: () => void;
  onMessage?: (event: ProgressEvent) => void;
  onError?: (error: Event) => void;
  onClose?: () => void;
  autoReconnect?: boolean;
  reconnectInterval?: number;
  maxReconnectAttempts?: number;
}

export interface UseSSEResult {
  status: SSEStatus;
  events: ProgressEvent[];
  latestEvent: ProgressEvent | null;
  progress: number;
  phase: string;
  message: string;
  error: string | null;
  jobId: string | null;
  isComplete: boolean;
  connect: () => void;
  disconnect: () => void;
  reset: () => void;
}

export function useSSE(options: UseSSEOptions): UseSSEResult {
  const {
    url,
    token,
    onOpen,
    onMessage,
    onError,
    onClose,
    autoReconnect = false,
    reconnectInterval = 3000,
    maxReconnectAttempts = 3,
  } = options;

  const [status, setStatus] = useState<SSEStatus>('idle');
  const [events, setEvents] = useState<ProgressEvent[]>([]);
  const [latestEvent, setLatestEvent] = useState<ProgressEvent | null>(null);
  const [progress, setProgress] = useState(0);
  const [phase, setPhase] = useState('');
  const [message, setMessage] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [jobId, setJobId] = useState<string | null>(null);
  const [isComplete, setIsComplete] = useState(false);

  const eventSourceRef = useRef<EventSource | null>(null);
  const reconnectAttemptsRef = useRef(0);
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  
  // Use refs for values that need to be current in callbacks
  // Update refs synchronously during render to ensure they're current when connect() is called
  const urlRef = useRef(url);
  const tokenRef = useRef(token);
  
  // Synchronously update refs during render (before effects run)
  urlRef.current = url;
  tokenRef.current = token;

  const cleanup = useCallback(() => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }
  }, []);

  const handleEvent = useCallback((event: MessageEvent) => {
    try {
      const data: ProgressEvent = JSON.parse(event.data);
      
      setLatestEvent(data);
      setEvents(prev => [...prev, data]);
      setProgress(data.progress);
      setPhase(data.phase);
      setMessage(data.message);
      
      if (data.job_id) {
        setJobId(data.job_id);
      }

      if (data.type === 'complete') {
        setIsComplete(true);
        setStatus('disconnected');
        cleanup();
      }

      if (data.type === 'error') {
        setError(data.error_message || data.message);
        setStatus('error');
        cleanup();
      }

      onMessage?.(data);
    } catch (e) {
      console.error('Failed to parse SSE event:', e);
    }
  }, [cleanup, onMessage]);

  const connect = useCallback(() => {
    cleanup();
    setStatus('connecting');
    setError(null);
    setIsComplete(false);
    setEvents([]);
    setProgress(0);
    setPhase('');
    setMessage('');

    // Use refs to get current URL and token values
    const currentUrl = urlRef.current;
    const currentToken = tokenRef.current;
    
    if (!currentUrl) {
      console.error('SSE: No URL provided');
      setStatus('error');
      setError('No URL provided for SSE connection');
      return;
    }

    // Build URL with token if provided
    const sseUrl = currentToken ? `${currentUrl}${currentUrl.includes('?') ? '&' : '?'}token=${currentToken}` : currentUrl;
    
    console.log('SSE connecting to:', sseUrl);
    console.log('SSE token present:', !!currentToken);

    try {
      const eventSource = new EventSource(sseUrl, { withCredentials: true });
      eventSourceRef.current = eventSource;

      eventSource.onopen = () => {
        setStatus('connected');
        reconnectAttemptsRef.current = 0;
        onOpen?.();
      };

      // Listen for specific event types (SSE custom events from server)
      eventSource.addEventListener('started', (e) => handleEvent(e as MessageEvent));
      eventSource.addEventListener('progress', (e) => handleEvent(e as MessageEvent));
      eventSource.addEventListener('warning', (e) => handleEvent(e as MessageEvent));
      eventSource.addEventListener('complete', (e) => handleEvent(e as MessageEvent));
      eventSource.addEventListener('info', (e) => handleEvent(e as MessageEvent));
      eventSource.addEventListener('debug', (e) => handleEvent(e as MessageEvent));
      // Note: 'error' as SSE event type is for server-sent error events, not connection errors
      eventSource.addEventListener('error', (e) => {
        const messageEvent = e as MessageEvent;
        // Only handle if it's a server-sent error event with data, not a connection error
        if (messageEvent.data) {
          handleEvent(messageEvent);
        }
      });

      // Generic message handler for fallback
      eventSource.onmessage = (e) => handleEvent(e);

      eventSource.onerror = (e) => {
        console.error('SSE error:', e);
        console.log('SSE readyState:', eventSource.readyState, 'CLOSED:', EventSource.CLOSED, 'CONNECTING:', EventSource.CONNECTING, 'OPEN:', EventSource.OPEN);
        
        // Don't treat CONNECTING state as an error - this happens during normal reconnection
        if (eventSource.readyState === EventSource.CONNECTING) {
          console.log('SSE is reconnecting (normal behavior), not treating as error');
          return;
        }
        
        if (eventSource.readyState === EventSource.CLOSED) {
          console.log('SSE connection closed');
          setStatus('disconnected');
          
          if (autoReconnect && reconnectAttemptsRef.current < maxReconnectAttempts && !isComplete) {
            reconnectAttemptsRef.current += 1;
            console.log(`SSE reconnecting (attempt ${reconnectAttemptsRef.current}/${maxReconnectAttempts})`);
            reconnectTimeoutRef.current = setTimeout(() => {
              connect();
            }, reconnectInterval);
          } else if (reconnectAttemptsRef.current >= maxReconnectAttempts) {
            setError('Connection failed after maximum reconnection attempts');
            setStatus('error');
          }
        } else {
          console.log('SSE error in OPEN state - actual error');
          setStatus('error');
          setError('Connection error');
        }

        onError?.(e);
      };
    } catch (e) {
      console.error('Failed to create EventSource:', e);
      setStatus('error');
      setError('Failed to establish connection');
    }
  }, [cleanup, handleEvent, onOpen, onError, autoReconnect, reconnectInterval, maxReconnectAttempts, isComplete]);

  const disconnect = useCallback(() => {
    cleanup();
    setStatus('disconnected');
    onClose?.();
  }, [cleanup, onClose]);

  // Reset all state to initial values (for reuse)
  const reset = useCallback(() => {
    cleanup();
    setStatus('idle');
    setEvents([]);
    setLatestEvent(null);
    setProgress(0);
    setPhase('');
    setMessage('');
    setError(null);
    setJobId(null);
    setIsComplete(false);
  }, [cleanup]);

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      cleanup();
    };
  }, [cleanup]);

  return {
    status,
    events,
    latestEvent,
    progress,
    phase,
    message,
    error,
    jobId,
    isComplete,
    connect,
    disconnect,
    reset,
  };
}
