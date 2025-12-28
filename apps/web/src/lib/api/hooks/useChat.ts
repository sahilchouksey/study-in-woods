import { useMutation, useQuery, useQueryClient, useInfiniteQuery } from '@tanstack/react-query';
import { useState, useCallback, useRef, useEffect } from 'react';
import {
  chatService,
  chatContextService,
  chatHistoryService,
  type ChatMessage,
  type Citation,
  type CreateSessionRequest,
  type ChatContextResponse,
  type StreamDoneEvent,
  type StreamUsage,
  type ToolEvent,
  type PaginatedSessionsResponse,
  type PaginatedMessagesResponse,
  type SessionHistoryResponse,
  type AISettings,
} from '@/lib/api/chat';
import type { InfiniteData } from '@tanstack/react-query';
import { showErrorToast, showSuccessToast } from '@/lib/utils/errors';

// Default page size for chat messages - must match what ChatInterface uses
export const DEFAULT_CHAT_PAGE_SIZE = 50;

// ==================== Chat Context Hooks ====================

/**
 * Hook to get all chat context dropdown data
 */
export function useChatContext() {
  return useQuery({
    queryKey: ['chat', 'context'],
    queryFn: () => chatContextService.getChatContext(),
    staleTime: 5 * 60 * 1000, // 5 minutes - this data doesn't change often
  });
}

/**
 * Hook to get subjects by semester (for lazy loading)
 */
export function useSubjectsBySemester(semesterId: number | null) {
  return useQuery({
    queryKey: ['chat', 'context', 'subjects', semesterId],
    queryFn: () => chatContextService.getSubjects(semesterId!),
    enabled: !!semesterId,
    staleTime: 5 * 60 * 1000,
  });
}

/**
 * Hook to get syllabus context for a subject
 */
export function useSubjectSyllabus(subjectId: number | null) {
  return useQuery({
    queryKey: ['chat', 'context', 'syllabus', subjectId],
    queryFn: () => chatContextService.getSubjectSyllabus(subjectId!),
    enabled: !!subjectId,
    staleTime: 10 * 60 * 1000, // 10 minutes
  });
}

// ==================== Chat Session Hooks ====================

/**
 * Hook to get all chat sessions
 */
export function useChatSessions(options?: { status?: string; subject_id?: string }) {
  return useQuery({
    queryKey: ['chat', 'sessions', options],
    queryFn: () => chatService.getSessions(options),
    staleTime: 30000, // 30 seconds
  });
}

/**
 * Hook to get a specific chat session
 */
export function useChatSession(sessionId: string | null) {
  return useQuery({
    queryKey: ['chat', 'session', sessionId],
    queryFn: () => chatService.getSession(sessionId!),
    enabled: !!sessionId,
    staleTime: 30000,
  });
}

/**
 * Hook to get messages for a session (all messages - legacy)
 */
export function useChatMessages(sessionId: string | null) {
  return useQuery({
    queryKey: ['chat', 'messages', sessionId],
    queryFn: () => chatService.getMessages(sessionId!),
    enabled: !!sessionId,
    staleTime: 10000, // 10 seconds
  });
}

/**
 * Infinite query hook for paginated chat messages
 * Messages are loaded from newest to oldest, so page 1 = most recent messages
 * Use fetchPreviousPage to load older messages (scrolling up)
 */
export function useInfiniteChatMessages(sessionId: string | null, pageSize: number = 50) {
  return useInfiniteQuery({
    queryKey: ['chat', 'messages', 'infinite', sessionId, pageSize],
    queryFn: async ({ pageParam = 1 }) => {
      return chatService.getMessagesPaginated(sessionId!, pageParam, pageSize);
    },
    getNextPageParam: (lastPage: PaginatedMessagesResponse) => {
      // For loading older messages (previous pages)
      if (lastPage.pagination.current_page < lastPage.pagination.total_pages) {
        return lastPage.pagination.current_page + 1;
      }
      return undefined;
    },
    getPreviousPageParam: (firstPage: PaginatedMessagesResponse) => {
      // For loading newer messages (if any)
      if (firstPage.pagination.current_page > 1) {
        return firstPage.pagination.current_page - 1;
      }
      return undefined;
    },
    initialPageParam: 1,
    enabled: !!sessionId,
    staleTime: 10000,
  });
}

/**
 * Hook to fetch full citations for a specific message
 * Citations are truncated by default in message list for performance
 */
export function useMessageCitations(sessionId: string | null, messageId: string | null) {
  return useQuery({
    queryKey: ['chat', 'citations', sessionId, messageId],
    queryFn: () => chatService.getMessageCitations(sessionId!, messageId!),
    enabled: !!sessionId && !!messageId,
    staleTime: 60000, // Cache for 1 minute
  });
}

/**
 * Hook to create a new chat session
 */
export function useCreateChatSession() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateSessionRequest) => chatService.createSession(data),
    onSuccess: (session) => {
      // Invalidate sessions list
      queryClient.invalidateQueries({ queryKey: ['chat', 'sessions'] });
      showSuccessToast(`Chat session created for ${session.subject?.name || 'subject'}`);
    },
    onError: (error) => {
      showErrorToast(error, 'Failed to create session');
    },
  });
}

/**
 * Hook to delete a chat session
 */
export function useDeleteChatSession() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (sessionId: string) => chatService.deleteSession(sessionId),
    onSuccess: (_, sessionId) => {
      // Remove from cache
      queryClient.removeQueries({ queryKey: ['chat', 'session', sessionId] });
      queryClient.removeQueries({ queryKey: ['chat', 'messages', sessionId] });
      // Invalidate sessions list
      queryClient.invalidateQueries({ queryKey: ['chat', 'sessions'] });
      showSuccessToast('Chat session deleted');
    },
    onError: (error) => {
      showErrorToast(error, 'Failed to delete session');
    },
  });
}

/**
 * Hook to archive a chat session
 */
export function useArchiveChatSession() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (sessionId: string) => chatService.archiveSession(sessionId),
    onSuccess: (_, sessionId) => {
      // Invalidate session and sessions list
      queryClient.invalidateQueries({ queryKey: ['chat', 'session', sessionId] });
      queryClient.invalidateQueries({ queryKey: ['chat', 'sessions'] });
      showSuccessToast('Chat session archived');
    },
    onError: (error) => {
      showErrorToast(error, 'Failed to archive session');
    },
  });
}

// ==================== Streaming Chat Hook ====================

export interface UseStreamingChatOptions {
  sessionId: string;
  onComplete?: () => void;
  /** AI settings to use for messages */
  aiSettings?: AISettings;
}

export interface UseStreamingChatReturn {
  sendMessage: (content: string) => void;
  isStreaming: boolean;
  streamingContent: string;
  streamingReasoning: string;       // AI's thinking/reasoning process
  streamingCitations: Citation[];   // Retrieved sources
  streamingUsage: StreamUsage | null;  // Token usage
  streamingToolEvents: ToolEvent[]; // Tool execution events
  isReasoning: boolean;             // True while receiving reasoning chunks
  isToolRunning: boolean;           // True while a tool is executing
  hasCompletedResponse: boolean;    // True after streaming completes (until next message)
  completedMessageId: number | null; // ID of the completed assistant message (for deduplication)
  cancelStream: () => void;
}

/**
 * Hook for streaming chat messages using SSE
 * Enhanced with reasoning and citations support
 * Includes throttled updates for smoother animation
 */
export function useStreamingChat({ sessionId, onComplete, aiSettings }: UseStreamingChatOptions): UseStreamingChatReturn {
  const queryClient = useQueryClient();
  const [isStreaming, setIsStreaming] = useState(false);
  const [isReasoning, setIsReasoning] = useState(false);
  const [isToolRunning, setIsToolRunning] = useState(false);
  const [streamingContent, setStreamingContent] = useState('');
  const [streamingReasoning, setStreamingReasoning] = useState('');
  const [streamingCitations, setStreamingCitations] = useState<Citation[]>([]);
  const [streamingUsage, setStreamingUsage] = useState<StreamUsage | null>(null);
  const [streamingToolEvents, setStreamingToolEvents] = useState<ToolEvent[]>([]);
  // Track if we have completed content that should still be displayed
  // This stays true after streaming completes until the user sends the next message
  const [hasCompletedResponse, setHasCompletedResponse] = useState(false);
  // Track the ID of the completed assistant message for accurate deduplication
  // This prevents filtering out the wrong message during race conditions
  const [completedMessageId, setCompletedMessageId] = useState<number | null>(null);
  const abortControllerRef = useRef<AbortController | null>(null);
  
  // Throttling refs for smooth streaming animation
  const contentBufferRef = useRef<string>('');
  const reasoningBufferRef = useRef<string>('');
  const flushIntervalRef = useRef<NodeJS.Timeout | null>(null);
  const THROTTLE_MS = 30; // ~33 FPS for smooth animation
  
  // Flush buffers at controlled interval for smooth animation
  useEffect(() => {
    if (isStreaming) {
      flushIntervalRef.current = setInterval(() => {
        // Flush content buffer
        if (contentBufferRef.current) {
          setStreamingContent((prev) => prev + contentBufferRef.current);
          contentBufferRef.current = '';
        }
        // Flush reasoning buffer
        if (reasoningBufferRef.current) {
          setStreamingReasoning((prev) => prev + reasoningBufferRef.current);
          reasoningBufferRef.current = '';
        }
      }, THROTTLE_MS);
    }
    
    return () => {
      if (flushIntervalRef.current) {
        clearInterval(flushIntervalRef.current);
        flushIntervalRef.current = null;
      }
    };
  }, [isStreaming]);

  const cancelStream = useCallback(() => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
      abortControllerRef.current = null;
    }
    // Clear buffers
    contentBufferRef.current = '';
    reasoningBufferRef.current = '';
    if (flushIntervalRef.current) {
      clearInterval(flushIntervalRef.current);
      flushIntervalRef.current = null;
    }
    setIsStreaming(false);
    setIsReasoning(false);
    setIsToolRunning(false);
    setHasCompletedResponse(false);
    setCompletedMessageId(null);
    setStreamingContent('');
    setStreamingReasoning('');
    setStreamingCitations([]);
    setStreamingUsage(null);
    setStreamingToolEvents([]);
  }, []);

  const sendMessage = useCallback((content: string) => {
    if (isStreaming) return;

    // Reset state - clear any previous completed response
    setIsStreaming(true);
    setIsReasoning(true); // Start in reasoning phase
    setIsToolRunning(false);
    setHasCompletedResponse(false); // Clear previous completed response
    setCompletedMessageId(null); // Clear previous completed message ID
    setStreamingContent('');
    setStreamingReasoning('');
    setStreamingCitations([]);
    setStreamingUsage(null);
    setStreamingToolEvents([]);

    // Add optimistic user message
    const optimisticUserMessage: ChatMessage = {
      id: `temp-user-${Date.now()}`,
      session_id: sessionId,
      subject_id: '',
      user_id: 'current-user',
      role: 'user',
      content,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    };

    // Update legacy query cache
    queryClient.setQueryData<ChatMessage[]>(
      ['chat', 'messages', sessionId],
      (old = []) => [...old, optimisticUserMessage]
    );

    // Update infinite query cache for optimistic UI
    queryClient.setQueryData<InfiniteData<PaginatedMessagesResponse>>(
      ['chat', 'messages', 'infinite', sessionId, DEFAULT_CHAT_PAGE_SIZE],
      (old) => {
        if (!old) {
          // If no data yet, create initial structure with optimistic message
          return {
            pages: [{
              messages: [optimisticUserMessage],
              pagination: { current_page: 1, per_page: 50, total: 1, total_pages: 1 }
            }],
            pageParams: [1],
          };
        }
        // Add message to the first page (most recent messages)
        const newPages = [...old.pages];
        if (newPages.length > 0) {
          newPages[0] = {
            ...newPages[0],
            messages: [...newPages[0].messages, optimisticUserMessage],
          };
        }
        return { ...old, pages: newPages };
      }
    );

    // Track if we're inside a tool call block to filter it out
    let insideToolCall = false;
    let toolCallBuffer = '';
    
    // Start streaming with enhanced callbacks
    abortControllerRef.current = chatService.sendMessageStream(
      sessionId,
      content,
      {
        // Content chunks - buffer for smooth animation
        // Filter out ##TOOL_CALL##...##END_TOOL_CALL## blocks
        onChunk: (chunk) => {
          // When we receive content, we're no longer reasoning
          setIsReasoning(false);
          
          // Check if this chunk starts or continues a tool call block
          const combinedContent = toolCallBuffer + chunk;
          
          // If we see ##TOOL_CALL##, start buffering (don't show to user)
          if (combinedContent.includes('##TOOL_CALL##')) {
            insideToolCall = true;
            // Keep everything before ##TOOL_CALL## if any
            const beforeToolCall = combinedContent.split('##TOOL_CALL##')[0];
            if (beforeToolCall && !toolCallBuffer) {
              contentBufferRef.current += beforeToolCall;
            }
            toolCallBuffer = combinedContent;
            return;
          }
          
          // If we're inside a tool call, keep buffering until we see the end
          if (insideToolCall) {
            toolCallBuffer += chunk;
            
            // Check if we've reached the end of the tool call
            if (toolCallBuffer.includes('##END_TOOL_CALL##')) {
              // Extract anything after ##END_TOOL_CALL## and add to content
              const afterToolCall = toolCallBuffer.split('##END_TOOL_CALL##')[1];
              if (afterToolCall) {
                contentBufferRef.current += afterToolCall;
              }
              // Reset tool call tracking
              insideToolCall = false;
              toolCallBuffer = '';
            }
            return;
          }
          
          // Normal content - add to buffer
          contentBufferRef.current += chunk;
        },
        // Reasoning/thinking chunks - buffer for smooth animation
        onReasoning: (chunk) => {
          reasoningBufferRef.current += chunk;
        },
        // Citations from knowledge base
        onCitations: (citations) => {
          setStreamingCitations(citations);
        },
        // Token usage info
        onUsage: (usage) => {
          setStreamingUsage(usage);
        },
        // Tool execution events
        onTool: (event) => {
          setStreamingToolEvents((prev) => [...prev, event]);
          // Track running state
          if (event.type === 'tool_start') {
            setIsToolRunning(true);
          } else if (event.type === 'tool_end' || event.type === 'tool_error') {
            setIsToolRunning(false);
          }
        },
        // Stream complete
        onComplete: (data: StreamDoneEvent) => {
          // Flush any remaining buffered content before completing
          if (contentBufferRef.current) {
            setStreamingContent((prev) => prev + contentBufferRef.current);
            contentBufferRef.current = '';
          }
          if (reasoningBufferRef.current) {
            setStreamingReasoning((prev) => prev + reasoningBufferRef.current);
            reasoningBufferRef.current = '';
          }
          
          // Clear flush interval
          if (flushIntervalRef.current) {
            clearInterval(flushIntervalRef.current);
            flushIntervalRef.current = null;
          }
          
          // Mark streaming as complete but DON'T reset content/reasoning/tools immediately
          // This keeps the UI stable until the user sends another message
          // The streaming bubble will continue to show the completed response
          setIsStreaming(false);
          setIsReasoning(false);
          setIsToolRunning(false);
          setHasCompletedResponse(true); // Mark that we have a completed response to display
          // Store the completed message ID for accurate deduplication
          // This prevents race conditions where the wrong message gets filtered
          setCompletedMessageId(data.assistant_message_id);
          // NOTE: We intentionally do NOT reset streamingContent, streamingReasoning, 
          // streamingCitations, or streamingToolEvents here.
          // They will be reset when the user sends the next message.
          // This prevents the UI from "flashing" when the query invalidates.
          
          abortControllerRef.current = null;

          // Refresh messages to get the actual saved messages
          // The streaming bubble will remain visible until next message is sent
          queryClient.invalidateQueries({ queryKey: ['chat', 'messages', sessionId] });
          // Use partial match for infinite query to handle any pageSize
          queryClient.invalidateQueries({ queryKey: ['chat', 'messages', 'infinite', sessionId], exact: false });
          queryClient.invalidateQueries({ queryKey: ['chat', 'sessions'] });
          queryClient.invalidateQueries({ queryKey: ['chat', 'session', sessionId] });

          onComplete?.();
        },
        // Error handling
        onError: (error) => {
          // Clear buffers
          contentBufferRef.current = '';
          reasoningBufferRef.current = '';
          if (flushIntervalRef.current) {
            clearInterval(flushIntervalRef.current);
            flushIntervalRef.current = null;
          }
          
          setIsStreaming(false);
          setIsReasoning(false);
          setIsToolRunning(false);
          setHasCompletedResponse(false); // Don't show completed response on error
          setCompletedMessageId(null); // Clear completed message ID on error
          setStreamingContent('');
          setStreamingReasoning('');
          setStreamingCitations([]);
          setStreamingUsage(null);
          setStreamingToolEvents([]);
          abortControllerRef.current = null;

          // Remove optimistic message on error from legacy cache
          queryClient.setQueryData<ChatMessage[]>(
            ['chat', 'messages', sessionId],
            (old = []) => old.filter((msg) => !msg.id.startsWith('temp-'))
          );

          // Remove optimistic message from infinite query cache
          queryClient.setQueryData<InfiniteData<PaginatedMessagesResponse>>(
            ['chat', 'messages', 'infinite', sessionId, DEFAULT_CHAT_PAGE_SIZE],
            (old) => {
              if (!old) return old;
              const newPages = old.pages.map(page => ({
                ...page,
                messages: page.messages.filter((msg) => !msg.id.startsWith('temp-')),
              }));
              return { ...old, pages: newPages };
            }
          );

          showErrorToast(new Error(error), 'Failed to send message');
        },
      },
      aiSettings
    );
  }, [sessionId, isStreaming, queryClient, onComplete, aiSettings]);

  return {
    sendMessage,
    isStreaming,
    isReasoning,
    isToolRunning,
    hasCompletedResponse,
    completedMessageId,
    streamingContent,
    streamingReasoning,
    streamingCitations,
    streamingUsage,
    streamingToolEvents,
    cancelStream,
  };
}

// ==================== Non-Streaming Send Message Hook ====================

/**
 * Hook to send a message (non-streaming)
 */
export function useSendMessage(sessionId: string | null) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (content: string) => {
      if (!sessionId) throw new Error('No session selected');
      return chatService.sendMessage(sessionId, content);
    },
    onSuccess: (response) => {
      // Add messages to cache immediately for optimistic UI
      queryClient.setQueryData<ChatMessage[]>(
        ['chat', 'messages', sessionId],
        (old = []) => [...old, response.user_message, response.assistant_message]
      );
      
      // Invalidate to get fresh data
      queryClient.invalidateQueries({ queryKey: ['chat', 'messages', sessionId] });
      queryClient.invalidateQueries({ queryKey: ['chat', 'sessions'] });
      queryClient.invalidateQueries({ queryKey: ['chat', 'session', sessionId] });
    },
    onError: (error) => {
      showErrorToast(error, 'Failed to send message');
    },
  });
}

// ==================== Helper Hooks ====================

/**
 * Helper to filter context data for cascading dropdowns
 */
export function useChatContextFilters(context: ChatContextResponse | undefined) {
  const getCoursesForUniversity = useCallback((universityId: number) => {
    if (!context) return [];
    return context.courses.filter((c) => c.university_id === universityId);
  }, [context]);

  const getSemestersForCourse = useCallback((courseId: number) => {
    if (!context) return [];
    return context.semesters.filter((s) => s.course_id === courseId);
  }, [context]);

  const getSubjectsForSemester = useCallback((semesterId: number) => {
    if (!context) return [];
    return context.subjects.filter((s) => s.semester_id === semesterId);
  }, [context]);

  return {
    getCoursesForUniversity,
    getSemestersForCourse,
    getSubjectsForSemester,
  };
}

// ==================== Chat History Hooks ====================

/**
 * Infinite query hook for paginated chat sessions (for history page)
 * Supports infinite scroll with automatic page fetching
 */
export function useInfiniteChatHistory(pageSize: number = 20) {
  return useInfiniteQuery({
    queryKey: ['chat', 'history', 'sessions', pageSize],
    queryFn: async ({ pageParam = 1 }) => {
      return chatHistoryService.getAllSessions(pageParam, pageSize);
    },
    getNextPageParam: (lastPage: PaginatedSessionsResponse) => {
      if (lastPage.page < lastPage.total_pages) {
        return lastPage.page + 1;
      }
      return undefined;
    },
    initialPageParam: 1,
    staleTime: 30000, // 30 seconds
  });
}

/**
 * Hook to get session history with messages (for viewing a specific session)
 */
export function useSessionHistory(sessionId: number | null, page: number = 1, pageSize: number = 50) {
  return useQuery({
    queryKey: ['chat', 'history', 'session', sessionId, page, pageSize],
    queryFn: () => chatHistoryService.getSessionHistory(sessionId!, page, pageSize),
    enabled: !!sessionId,
    staleTime: 30000,
  });
}

/**
 * Infinite query hook for session messages (for infinite scroll in session view)
 */
export function useInfiniteSessionMessages(sessionId: number | null, pageSize: number = 50) {
  return useInfiniteQuery({
    queryKey: ['chat', 'history', 'messages', sessionId, pageSize],
    queryFn: async ({ pageParam = 1 }) => {
      return chatHistoryService.getSessionHistory(sessionId!, pageParam, pageSize);
    },
    getNextPageParam: (lastPage: SessionHistoryResponse) => {
      if (lastPage.page < lastPage.total_pages) {
        return lastPage.page + 1;
      }
      return undefined;
    },
    initialPageParam: 1,
    enabled: !!sessionId,
    staleTime: 30000,
  });
}

/**
 * Mutation hook to search conversation memory
 */
export function useSearchMemory(sessionId: number | null) {
  return useMutation({
    mutationFn: async ({ query, limit = 10 }: { query: string; limit?: number }) => {
      if (!sessionId) throw new Error('No session selected');
      return chatHistoryService.searchMemory(sessionId, query, limit);
    },
    onError: (error) => {
      showErrorToast(error, 'Failed to search memory');
    },
  });
}

/**
 * Hook to get compacted contexts for a session
 */
export function useCompactedContexts(sessionId: number | null) {
  return useQuery({
    queryKey: ['chat', 'history', 'contexts', sessionId],
    queryFn: () => chatHistoryService.getCompactedContexts(sessionId!),
    enabled: !!sessionId,
    staleTime: 60000, // 1 minute - contexts don't change often
  });
}

/**
 * Hook to get memory batches for a session
 */
export function useMemoryBatches(sessionId: number | null) {
  return useQuery({
    queryKey: ['chat', 'history', 'batches', sessionId],
    queryFn: () => chatHistoryService.getBatches(sessionId!),
    enabled: !!sessionId,
    staleTime: 60000,
  });
}
