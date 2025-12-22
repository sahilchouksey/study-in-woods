import { apiClient } from './client';
import type { ApiResponse } from '@/types/api';
import { getApiKey } from '@/lib/api-keys';
import type { ApiProvider } from '@/types/api-keys';

/**
 * Chat message type
 */
export interface ChatMessage {
  id: string;
  session_id: string;
  subject_id: string;
  user_id: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
  citations?: Citation[];
  citation_count?: number; // Total citations count (citations may be truncated)
  tokens_used?: number;
  model_used?: string;
  response_time?: number;
  is_streamed?: boolean;
  metadata?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

/**
 * Citation from knowledge base
 */
export interface Citation {
  id?: string;
  filename?: string;
  page_content?: string;
  score?: number;
  data_source_id?: string;
  metadata?: Record<string, unknown>;
  // Legacy/alternative field names (for compatibility)
  source?: string;
  content?: string;
  page?: number;
}

/**
 * Chat session type
 */
export interface ChatSession {
  id: number;  // Backend returns uint
  user_id: number;
  subject_id: number;
  title: string;
  description?: string;
  status: 'active' | 'archived';
  agent_uuid: string;
  message_count: number;
  total_tokens: number;
  subject?: {
    id: number;
    name: string;
    code: string;
    semester_id: number;
  };
  created_at: string;
  updated_at: string;
  last_message_at?: string;
}

/**
 * Create session request
 */
export interface CreateSessionRequest {
  subject_id: number;
  title?: string;
  description?: string;
}

/**
 * AI Settings that can be configured by the user
 */
export interface AISettings {
  /** Custom system prompt - if empty, uses default */
  system_prompt?: string;
  /** Whether to include citations in responses (default: true) */
  include_citations?: boolean;
  /** Maximum tokens for the response (256-8192) */
  max_tokens?: number;
}

/**
 * Default AI settings
 */
export const DEFAULT_AI_SETTINGS: AISettings = {
  system_prompt: '',
  include_citations: true,
  max_tokens: 2048,
};

/**
 * Default system prompt template (shown in UI for reference)
 */
export const DEFAULT_SYSTEM_PROMPT = `You are an expert AI tutor for the subject "{subject_name}". Your role is to help students understand concepts, answer questions, and provide explanations related to this subject.

CRITICAL INSTRUCTION - RESPONSE PRIORITIES:
1. FIRST PRIORITY: Always directly answer what the user is asking. If they ask to list, share, or show something specific (like previous year questions), DO THAT FIRST.
2. SECOND PRIORITY: Use the conversation context to understand what the user needs.
3. THIRD PRIORITY: Use knowledge base materials to support your answer.

IMPORTANT: When a user explicitly asks to "share", "list", "show", or "give me" specific items (like PYQs, questions, topics), your response should directly provide those items in a clear format - NOT solve or explain them unless asked.

Guidelines:
- Provide accurate, educational responses based on the subject matter
- Include relevant examples and explanations
- When citing information from course materials, clearly indicate the source
- Be encouraging and supportive while maintaining academic rigor
- If you're unsure about something, acknowledge it honestly
- Structure your responses clearly with appropriate formatting`;

/**
 * Send message request (for non-streaming)
 */
export interface SendMessageRequest {
  content: string;
  stream?: boolean;
  /** AI settings for this message */
  settings?: AISettings;
}

/**
 * Send message response (for non-streaming)
 */
export interface SendMessageResponse {
  user_message: ChatMessage;
  assistant_message: ChatMessage;
}

// ==================== Chat Context Types ====================

/**
 * University option for dropdown
 */
export interface UniversityOption {
  id: number;
  name: string;
  code: string;
}

/**
 * Course option for dropdown
 */
export interface CourseOption {
  id: number;
  university_id: number;
  name: string;
  code: string;
  duration: number;
}

/**
 * Semester option for dropdown
 */
export interface SemesterOption {
  id: number;
  course_id: number;
  number: number;
  name: string;
}

/**
 * Subject option for dropdown (only subjects with KB and Agent)
 */
export interface SubjectOption {
  id: number;
  semester_id: number;
  name: string;
  code: string;
  credits: number;
  description?: string;
  knowledge_base_uuid: string;
  agent_uuid: string;
  has_syllabus: boolean;
  is_starred: boolean;
}

/**
 * Complete chat context response with all dropdown data
 */
export interface ChatContextResponse {
  universities: UniversityOption[];
  courses: CourseOption[];
  semesters: SemesterOption[];
  subjects: SubjectOption[];
}

/**
 * Syllabus context for chat
 */
export interface SyllabusContext {
  subject_name: string;
  subject_code: string;
  total_credits: number;
  units: SyllabusUnit[];
  books: SyllabusBook[];
  formatted_text: string;
}

export interface SyllabusUnit {
  unit_number: number;
  title: string;
  description?: string;
  topics?: string[];
  hours?: number;
}

export interface SyllabusBook {
  title: string;
  authors: string;
  is_textbook: boolean;
}

export interface SubjectSyllabusResponse {
  has_syllabus: boolean;
  syllabus?: SyllabusContext;
  message?: string;
}

// ==================== SSE Stream Types ====================

export type StreamEventType = 'start' | 'chunk' | 'reasoning' | 'citations' | 'usage' | 'tool' | 'done' | 'error';

export interface StreamStartEvent {
  status: 'streaming';
}

export interface StreamDoneEvent {
  user_message_id: number;
  assistant_message_id: number;
  tokens_used?: number;
}

export interface StreamErrorEvent {
  error: string;
}

export interface StreamUsage {
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
}

/**
 * Tool event from streaming (tool_start, tool_end)
 */
export interface ToolEvent {
  type: 'tool_start' | 'tool_end' | 'tool_error';
  tool_name: string;
  arguments?: Record<string, unknown>;
  result?: unknown;
  success?: boolean;
  error?: string;
}

// Callback types for streaming
export type StreamCallback = (chunk: string) => void;
export type StreamReasoningCallback = (chunk: string) => void;
export type StreamCitationsCallback = (citations: Citation[]) => void;
export type StreamUsageCallback = (usage: StreamUsage) => void;
export type StreamToolCallback = (event: ToolEvent) => void;
export type StreamCompleteCallback = (data: StreamDoneEvent) => void;
export type StreamErrorCallback = (error: string) => void;

// Enhanced stream callbacks with reasoning, citations, and tool support
export interface EnhancedStreamCallbacks {
  onChunk: StreamCallback;           // Content chunks
  onReasoning?: StreamReasoningCallback;  // Reasoning/thinking chunks
  onCitations?: StreamCitationsCallback;  // Citations when available
  onUsage?: StreamUsageCallback;     // Token usage
  onTool?: StreamToolCallback;       // Tool events (tool_start, tool_end)
  onComplete: StreamCompleteCallback;
  onError: StreamErrorCallback;
}

/**
 * Paginated messages response
 */
export interface PaginatedMessagesResponse {
  messages: ChatMessage[];
  pagination: {
    current_page: number;
    per_page: number;
    total: number;
    total_pages: number;
  };
}

/**
 * Chat service
 */
export const chatService = {
  /**
   * Get all chat sessions for current user
   */
  async getSessions(options?: { status?: string; subject_id?: string }): Promise<ChatSession[]> {
    const params = new URLSearchParams();
    if (options?.status) params.append('status', options.status);
    if (options?.subject_id) params.append('subject_id', options.subject_id);
    
    const queryString = params.toString();
    const url = `/api/v1/chat/sessions${queryString ? `?${queryString}` : ''}`;
    
    const response = await apiClient.get<ApiResponse<ChatSession[]>>(url);
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
   * Create a new chat session for a subject
   */
  async createSession(data: CreateSessionRequest): Promise<ChatSession> {
    const response = await apiClient.post<ApiResponse<ChatSession>>(
      '/api/v1/chat/sessions',
      data
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
   * Get messages for a chat session (all messages - legacy)
   */
  async getMessages(sessionId: string, options?: { page?: number; limit?: number }): Promise<ChatMessage[]> {
    const params = new URLSearchParams();
    if (options?.page) params.append('page', options.page.toString());
    if (options?.limit) params.append('limit', options.limit.toString());
    
    const queryString = params.toString();
    const url = `/api/v1/chat/sessions/${sessionId}/messages${queryString ? `?${queryString}` : ''}`;
    
    // Backend returns paginated response, extract data
    const response = await apiClient.get<{ success: boolean; data: ChatMessage[]; pagination: PaginatedMessagesResponse['pagination'] }>(url);
    return response.data.data || [];
  },

  /**
   * Get messages for a chat session with pagination info
   */
  async getMessagesPaginated(sessionId: string, page: number = 1, limit: number = 50): Promise<PaginatedMessagesResponse> {
    const url = `/api/v1/chat/sessions/${sessionId}/messages?page=${page}&limit=${limit}`;
    
    const response = await apiClient.get<{ success: boolean; data: ChatMessage[]; pagination: PaginatedMessagesResponse['pagination'] }>(url);
    return {
      messages: response.data.data || [],
      pagination: response.data.pagination,
    };
  },

  /**
   * Get full citations for a specific message
   * Use this when user wants to see full citation content
   */
  async getMessageCitations(sessionId: string, messageId: string): Promise<Citation[]> {
    const url = `/api/v1/chat/sessions/${sessionId}/messages/${messageId}/citations`;
    const response = await apiClient.get<ApiResponse<{ message_id: number; citations: Citation[] }>>(url);
    return response.data.data?.citations || [];
  },

  /**
   * Send a message (non-streaming)
   */
  async sendMessage(sessionId: string, content: string): Promise<SendMessageResponse> {
    const response = await apiClient.post<ApiResponse<SendMessageResponse>>(
      `/api/v1/chat/sessions/${sessionId}/messages`,
      { content, stream: false }
    );
    return response.data.data!;
  },

  /**
   * Send a message with SSE streaming (enhanced with reasoning and citations)
   * Returns an abort controller to cancel the stream
   */
  sendMessageStream(
    sessionId: string,
    content: string,
    callbacks: EnhancedStreamCallbacks,
    settings?: AISettings
  ): AbortController {
    const abortController = new AbortController();
    
    // Get the auth token from storage (same key as apiClient uses)
    const token = typeof window !== 'undefined' 
      ? localStorage.getItem('access_token') 
      : null;
    
    // Get the API base URL
    const baseURL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';
    const url = `${baseURL}/api/v1/chat/sessions/${sessionId}/messages`;

    // Fetch API keys asynchronously and then make the request
    this._getApiKeysForRequest().then(apiKeys => {
      fetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Accept': 'text/event-stream',
          ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
          // Include API keys in headers if available
          ...(apiKeys.tavily ? { 'X-Tavily-Api-Key': apiKeys.tavily } : {}),
          ...(apiKeys.exa ? { 'X-Exa-Api-Key': apiKeys.exa } : {}),
          ...(apiKeys.firecrawl ? { 'X-Firecrawl-Api-Key': apiKeys.firecrawl } : {}),
        },
        body: JSON.stringify({ content, stream: true, settings }),
        signal: abortController.signal,
      })
        .then(async (response) => {
          if (!response.ok) {
            const errorText = await response.text();
            throw new Error(errorText || `HTTP ${response.status}`);
          }

          const reader = response.body?.getReader();
          if (!reader) {
            throw new Error('No response body');
          }

          const decoder = new TextDecoder();
          let buffer = '';

          while (true) {
            const { done, value } = await reader.read();
            
            if (done) break;

            const rawChunk = decoder.decode(value, { stream: true });
            buffer += rawChunk;
            
            // SSE events are separated by double newlines (\n\n)
            // Split on \n\n to get complete events
            const events = buffer.split('\n\n');
            buffer = events.pop() || ''; // Keep incomplete event in buffer

            for (const event of events) {
              if (!event.trim()) continue;
              
              // Parse each line within the event
              const lines = event.split('\n');
              let eventType = '';
              let eventData = '';
              
              for (const line of lines) {
                if (line.startsWith('event: ')) {
                  eventType = line.slice(7);
                } else if (line.startsWith('data: ')) {
                  // Accumulate data (in case it spans multiple lines)
                  eventData += (eventData ? '\n' : '') + line.slice(6);
                }
              }
              
              if (!eventData) continue;
              
              // Handle events based on event type from backend
              switch (eventType) {
                case 'start':
                  // Stream started, nothing to do
                  continue;
                
                case 'reasoning':
                  // AI thinking/reasoning chunk - unescape newlines from SSE transport
                  if (callbacks.onReasoning) {
                    const unescapedReasoning = eventData
                      .replace(/\\n/g, '\n')
                      .replace(/\\r/g, '\r')
                      .replace(/\\\\/g, '\\');
                    callbacks.onReasoning(unescapedReasoning);
                  }
                  continue;
                
                case 'chunk':
                  // Content chunk - unescape newlines from SSE transport
                  const unescapedChunk = eventData
                    .replace(/\\n/g, '\n')
                    .replace(/\\r/g, '\r')
                    .replace(/\\\\/g, '\\');
                  callbacks.onChunk(unescapedChunk);
                  continue;
                
                case 'citations':
                  // Citations arrived - parse JSON array
                  if (callbacks.onCitations) {
                    try {
                      const citations = JSON.parse(eventData) as Citation[];
                      callbacks.onCitations(citations);
                    } catch {
                      console.warn('[SSE] Failed to parse citations:', eventData.substring(0, 200));
                    }
                  }
                  continue;
                
                case 'usage':
                  // Token usage info
                  if (callbacks.onUsage) {
                    try {
                      const usage = JSON.parse(eventData) as StreamUsage;
                      callbacks.onUsage(usage);
                    } catch {
                      console.warn('[SSE] Failed to parse usage:', eventData);
                    }
                  }
                  continue;
                
                case 'tool':
                  // Tool events (tool_start, tool_end)
                  if (callbacks.onTool) {
                    try {
                      const toolEvent = JSON.parse(eventData) as ToolEvent;
                      callbacks.onTool(toolEvent);
                    } catch {
                      console.warn('[SSE] Failed to parse tool event:', eventData);
                    }
                  }
                  continue;
                
                case 'done':
                  // Stream complete
                  try {
                    const doneData = JSON.parse(eventData) as StreamDoneEvent;
                    callbacks.onComplete(doneData);
                  } catch {
                    console.warn('[SSE] Failed to parse done event:', eventData);
                    callbacks.onComplete({ user_message_id: 0, assistant_message_id: 0 });
                  }
                  continue;
                
                case 'error':
                  // Error event
                  try {
                    const errorData = JSON.parse(eventData) as StreamErrorEvent;
                    callbacks.onError(errorData.error);
                  } catch {
                    callbacks.onError(eventData);
                  }
                  continue;
                
                default:
                  // Fallback: try to parse as JSON for legacy support
                  try {
                    const parsed = JSON.parse(eventData);
                    
                    if (parsed.status === 'streaming') {
                      continue;
                    } else if (parsed.error) {
                      callbacks.onError(parsed.error);
                    } else if (parsed.user_message_id !== undefined) {
                      callbacks.onComplete(parsed as StreamDoneEvent);
                    }
                  } catch {
                    // Plain text chunk - treat as content
                    callbacks.onChunk(eventData);
                  }
              }
            }
          }
        })
        .catch((error) => {
          if (error.name === 'AbortError') {
            return; // Intentionally aborted
          }
          callbacks.onError(error.message || 'Stream failed');
        });
    });

    return abortController;
  },

  /**
   * Helper to get API keys for requests
   */
  async _getApiKeysForRequest(): Promise<{ tavily: string | null; exa: string | null; firecrawl: string | null }> {
    const providers: ApiProvider[] = ['tavily', 'exa', 'firecrawl'];
    const keys: { tavily: string | null; exa: string | null; firecrawl: string | null } = {
      tavily: null,
      exa: null,
      firecrawl: null,
    };

    for (const provider of providers) {
      try {
        keys[provider] = await getApiKey(provider);
      } catch {
        // Ignore errors, key will remain null
      }
    }

    return keys;
  },
};

// ==================== Chat Context Service ====================

// ==================== Chat History Types ====================

/**
 * Paginated sessions response from history API
 */
export interface PaginatedSessionsResponse {
  sessions: ChatSession[];
  total_count: number;
  page: number;
  page_size: number;
  total_pages: number;
}

/**
 * Memory batch info
 */
export interface ChatMemoryBatch {
  id: number;
  batch_number: number;
  status: 'active' | 'complete' | 'compacted';
  message_count: number;
  start_msg_id: number;
  end_msg_id: number;
  compacted_at?: string;
  context_id?: number;
  created_at: string;
}

/**
 * Compacted context from memory
 */
export interface ChatCompactedContext {
  id: number;
  batch_number: number;
  summary: string;
  key_topics: string[];
  key_entities: string[];
  user_intents: string[];
  message_range: string;
  original_message_count: number;
  created_at: string;
}

/**
 * Session history response with messages and memory info
 */
export interface SessionHistoryResponse {
  messages: ChatMessage[];
  total_count: number;
  page: number;
  page_size: number;
  total_pages: number;
  batches: ChatMemoryBatch[];
  compacted_contexts: ChatCompactedContext[];
}

/**
 * Memory search result
 */
export interface MemorySearchResult {
  type: 'message' | 'context';
  content: string;
  role?: string;
  timestamp: string;
  relevance: number;
  batch_num: number;
  message_id?: number;
  context_id?: number;
}

/**
 * Memory search response
 */
export interface MemorySearchResponse {
  query: string;
  results: MemorySearchResult[];
  count: number;
}

// ==================== Chat History Service ====================

export const chatHistoryService = {
  /**
   * Get all chat sessions with pagination (for history page)
   */
  async getAllSessions(page: number = 1, pageSize: number = 20): Promise<PaginatedSessionsResponse> {
    const response = await apiClient.get<ApiResponse<PaginatedSessionsResponse>>(
      `/api/v1/chat/history?page=${page}&page_size=${pageSize}`
    );
    return response.data.data!;
  },

  /**
   * Get full session history with messages and memory info
   */
  async getSessionHistory(sessionId: number, page: number = 1, pageSize: number = 50): Promise<SessionHistoryResponse> {
    const response = await apiClient.get<ApiResponse<SessionHistoryResponse>>(
      `/api/v1/chat/history/${sessionId}?page=${page}&page_size=${pageSize}`
    );
    return response.data.data!;
  },

  /**
   * Search through conversation memory
   */
  async searchMemory(sessionId: number, query: string, limit: number = 10): Promise<MemorySearchResponse> {
    const response = await apiClient.post<ApiResponse<MemorySearchResponse>>(
      `/api/v1/chat/history/${sessionId}/search`,
      { query, limit }
    );
    return response.data.data!;
  },

  /**
   * Get compacted contexts for a session
   */
  async getCompactedContexts(sessionId: number): Promise<ChatCompactedContext[]> {
    const response = await apiClient.get<ApiResponse<ChatCompactedContext[]>>(
      `/api/v1/chat/history/${sessionId}/contexts`
    );
    return response.data.data || [];
  },

  /**
   * Get memory batches for a session
   */
  async getBatches(sessionId: number): Promise<ChatMemoryBatch[]> {
    const response = await apiClient.get<ApiResponse<ChatMemoryBatch[]>>(
      `/api/v1/chat/history/${sessionId}/batches`
    );
    return response.data.data || [];
  },
};

// ==================== Chat Context Service ====================

export const chatContextService = {
  /**
   * Get all dropdown data for chat setup in a single call
   */
  async getChatContext(): Promise<ChatContextResponse> {
    const response = await apiClient.get<ApiResponse<ChatContextResponse>>(
      '/api/v1/chat/context'
    );
    return response.data.data!;
  },

  /**
   * Get universities list
   */
  async getUniversities(): Promise<UniversityOption[]> {
    const response = await apiClient.get<ApiResponse<UniversityOption[]>>(
      '/api/v1/chat/context/universities'
    );
    return response.data.data || [];
  },

  /**
   * Get courses for a university
   */
  async getCourses(universityId: number): Promise<CourseOption[]> {
    const response = await apiClient.get<ApiResponse<CourseOption[]>>(
      `/api/v1/chat/context/universities/${universityId}/courses`
    );
    return response.data.data || [];
  },

  /**
   * Get semesters for a course
   */
  async getSemesters(courseId: number): Promise<SemesterOption[]> {
    const response = await apiClient.get<ApiResponse<SemesterOption[]>>(
      `/api/v1/chat/context/courses/${courseId}/semesters`
    );
    return response.data.data || [];
  },

  /**
   * Get subjects for a semester (only those with KB and Agent)
   */
  async getSubjects(semesterId: number): Promise<SubjectOption[]> {
    const response = await apiClient.get<ApiResponse<SubjectOption[]>>(
      `/api/v1/chat/context/semesters/${semesterId}/subjects`
    );
    return response.data.data || [];
  },

  /**
   * Get syllabus context for a subject
   */
  async getSubjectSyllabus(subjectId: number): Promise<SubjectSyllabusResponse> {
    const response = await apiClient.get<ApiResponse<SubjectSyllabusResponse>>(
      `/api/v1/chat/context/subjects/${subjectId}/syllabus`
    );
    return response.data.data!;
  },
};
