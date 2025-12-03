import { apiClient } from './client';
import type { ApiResponse } from '@/types/api';

/**
 * Chat message type
 */
export interface ChatMessage {
  id: string;
  session_id: string;
  user_id: string;
  role: 'user' | 'assistant';
  content: string;
  metadata?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

/**
 * Chat session type
 */
export interface ChatSession {
  id: string;
  user_id: string;
  title: string;
  context?: {
    university_id?: string;
    course_id?: string;
    semester?: string;
    subject_name?: string;
  };
  is_archived: boolean;
  created_at: string;
  updated_at: string;
  last_message_at?: string;
  message_count?: number;
}

/**
 * Send message request
 */
export interface SendMessageRequest {
  session_id?: string;
  message: string;
  context?: {
    university_id?: string;
    course_id?: string;
    semester?: string;
    subject_name?: string;
  };
}

/**
 * Send message response
 */
export interface SendMessageResponse {
  session_id: string;
  message: ChatMessage;
  response: ChatMessage;
}

/**
 * Chat service
 */
export const chatService = {
  /**
   * Get all chat sessions for current user
   */
  async getSessions(): Promise<ChatSession[]> {
    const response = await apiClient.get<ApiResponse<ChatSession[]>>(
      '/api/v1/chat/sessions'
    );
    return response.data.data || [];
  },

  /**
   * Get a specific chat session
   */
  async getSession(sessionId: string): Promise<ChatSession> {
    const response = await apiClient.get<ApiResponse<ChatSession>>(
      `/api/v1/chat/sessions/${sessionId}`
    );
    return response.data.data!;
  },

  /**
   * Create a new chat session
   */
  async createSession(
    title?: string,
    context?: ChatSession['context']
  ): Promise<ChatSession> {
    const response = await apiClient.post<ApiResponse<ChatSession>>(
      '/api/v1/chat/sessions',
      { title, context }
    );
    return response.data.data!;
  },

  /**
   * Delete a chat session
   */
  async deleteSession(sessionId: string): Promise<void> {
    await apiClient.delete(`/api/v1/chat/sessions/${sessionId}`);
  },

  /**
   * Archive a chat session
   */
  async archiveSession(sessionId: string): Promise<void> {
    await apiClient.post(`/api/v1/chat/sessions/${sessionId}/archive`);
  },

  /**
   * Get messages for a chat session
   */
  async getMessages(sessionId: string): Promise<ChatMessage[]> {
    const response = await apiClient.get<ApiResponse<ChatMessage[]>>(
      `/api/v1/chat/sessions/${sessionId}/messages`
    );
    return response.data.data || [];
  },

  /**
   * Send a message in a chat session
   */
  async sendMessage(data: SendMessageRequest): Promise<SendMessageResponse> {
    const sessionId = data.session_id;
    
    if (!sessionId) {
      // Create new session first
      const newSession = await this.createSession('New Chat', data.context);
      
      // Send message to new session
      const response = await apiClient.post<ApiResponse<SendMessageResponse>>(
        `/api/v1/chat/sessions/${newSession.id}/messages`,
        {
          message: data.message,
          context: data.context,
        }
      );
      
      return response.data.data!;
    }

    // Send message to existing session
    const response = await apiClient.post<ApiResponse<SendMessageResponse>>(
      `/api/v1/chat/sessions/${sessionId}/messages`,
      {
        message: data.message,
        context: data.context,
      }
    );
    
    return response.data.data!;
  },
};
