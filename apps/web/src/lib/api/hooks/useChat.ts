import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  chatService,
  type ChatSession,
  type ChatMessage,
  type SendMessageRequest,
} from '@/lib/api/chat';
import { showErrorToast, showSuccessToast } from '@/lib/utils/errors';

/**
 * Hook to get all chat sessions
 */
export function useChatSessions() {
  return useQuery({
    queryKey: ['chat', 'sessions'],
    queryFn: () => chatService.getSessions(),
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
 * Hook to get messages for a session
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
 * Hook to create a new chat session
 */
export function useCreateChatSession() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      title,
      context,
    }: {
      title?: string;
      context?: ChatSession['context'];
    }) => chatService.createSession(title, context),
    onSuccess: () => {
      // Invalidate sessions list
      queryClient.invalidateQueries({ queryKey: ['chat', 'sessions'] });
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

/**
 * Hook to send a message
 */
export function useSendMessage() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: SendMessageRequest) => chatService.sendMessage(data),
    onSuccess: (response) => {
      const sessionId = response.session_id;
      
      // Add messages to cache immediately for optimistic UI
      queryClient.setQueryData<ChatMessage[]>(
        ['chat', 'messages', sessionId],
        (old = []) => [...old, response.message, response.response]
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

/**
 * Hook to manage optimistic message updates
 */
export function useOptimisticMessage(sessionId: string | null) {
  const queryClient = useQueryClient();

  const addOptimisticMessage = (content: string) => {
    if (!sessionId) return;

    const optimisticMessage: ChatMessage = {
      id: `temp-${Date.now()}`,
      session_id: sessionId,
      user_id: 'current-user',
      role: 'user',
      content,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    };

    queryClient.setQueryData<ChatMessage[]>(
      ['chat', 'messages', sessionId],
      (old = []) => [...old, optimisticMessage]
    );
  };

  const removeOptimisticMessage = () => {
    if (!sessionId) return;

    queryClient.setQueryData<ChatMessage[]>(
      ['chat', 'messages', sessionId],
      (old = []) => old.filter((msg) => !msg.id.startsWith('temp-'))
    );
  };

  return { addOptimisticMessage, removeOptimisticMessage };
}
