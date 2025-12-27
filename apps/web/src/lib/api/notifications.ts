import { apiClient } from './client';
import type { ApiResponse } from '@/types/api';

/**
 * Notification type
 */
export type NotificationType = 'info' | 'success' | 'warning' | 'error' | 'in_progress';

/**
 * Notification category
 */
export type NotificationCategory = 'pyq_ingest' | 'document_upload' | 'syllabus_extraction' | 'ai_setup' | 'general';

/**
 * Notification metadata
 */
export interface NotificationMetadata {
  subject_id?: number;
  subject_name?: string;
  document_id?: number;
  document_name?: string;
  paper_count?: number;
  completed_count?: number;
  failed_count?: number;
  progress?: number;
  error_details?: string;
  [key: string]: unknown;
}

/**
 * User notification interface
 */
export interface UserNotification {
  id: number;
  user_id: number;
  type: NotificationType;
  category: NotificationCategory;
  title: string;
  message: string;
  is_read: boolean;
  indexing_job_id?: number;
  metadata?: NotificationMetadata;
  created_at: string;
  updated_at: string;
}

/**
 * Notifications list response
 */
export interface NotificationsListResponse {
  notifications: UserNotification[];
  total: number;
}

/**
 * Unread count response
 */
export interface UnreadCountResponse {
  unread_count: number;
}

/**
 * List notifications options
 */
export interface ListNotificationsOptions {
  unread_only?: boolean;
  category?: NotificationCategory;
  limit?: number;
  offset?: number;
}

// ============ INDEXING JOB INTERFACES ============

/**
 * Indexing job status
 */
export type IndexingJobStatus = 'pending' | 'processing' | 'kb_indexing' | 'completed' | 'failed' | 'partially_completed' | 'cancelled';

/**
 * Indexing job item status
 */
export type IndexingJobItemStatus = 'pending' | 'downloading' | 'uploading' | 'indexing' | 'completed' | 'failed';

/**
 * Indexing job item interface
 */
export interface IndexingJobItem {
  id: number;
  job_id: number;
  source_url: string;
  title: string;
  status: IndexingJobItemStatus;
  document_id?: number;
  error_message?: string;
  started_at?: string;
  completed_at?: string;
  created_at: string;
}

/**
 * Indexing job interface
 */
export interface IndexingJob {
  id: number;
  user_id: number;
  subject_id: number;
  job_type: string;
  status: IndexingJobStatus;
  total_items: number;
  completed_items: number;
  failed_items: number;
  progress: number;
  error_message?: string;
  started_at?: string;
  completed_at?: string;
  items?: IndexingJobItem[];
  created_at: string;
  updated_at: string;
}

/**
 * Batch ingest paper request
 */
export interface BatchIngestPaper {
  pdf_url: string;
  title: string;
  year: number;
  month?: string;
  exam_type?: string;
  source_name: string;
}

/**
 * Batch ingest request
 */
export interface BatchIngestRequest {
  papers: BatchIngestPaper[];
}

/**
 * Batch ingest response
 */
export interface BatchIngestResponse {
  job_id: number;
  status: IndexingJobStatus;
  total_items: number;
  message: string;
}

/**
 * Indexing jobs list response
 */
export interface IndexingJobsListResponse {
  jobs: IndexingJob[];
  total: number;
}

/**
 * Status display configuration for notifications
 */
export const NOTIFICATION_TYPE_CONFIG: Record<NotificationType, {
  label: string;
  color: string;
  bgColor: string;
}> = {
  info: {
    label: 'Info',
    color: 'text-blue-600',
    bgColor: 'bg-blue-100',
  },
  success: {
    label: 'Success',
    color: 'text-green-600',
    bgColor: 'bg-green-100',
  },
  warning: {
    label: 'Warning',
    color: 'text-yellow-600',
    bgColor: 'bg-yellow-100',
  },
  error: {
    label: 'Error',
    color: 'text-red-600',
    bgColor: 'bg-red-100',
  },
  in_progress: {
    label: 'In Progress',
    color: 'text-indigo-600',
    bgColor: 'bg-indigo-100',
  },
};

/**
 * Status display configuration for indexing jobs
 */
export const INDEXING_JOB_STATUS_CONFIG: Record<IndexingJobStatus, {
  label: string;
  color: string;
  bgColor: string;
  description: string;
}> = {
  pending: {
    label: 'Pending',
    color: 'text-gray-600',
    bgColor: 'bg-gray-100',
    description: 'Waiting to start',
  },
  processing: {
    label: 'Processing',
    color: 'text-blue-600',
    bgColor: 'bg-blue-100',
    description: 'Currently processing items',
  },
  kb_indexing: {
    label: 'Indexing for AI',
    color: 'text-purple-600',
    bgColor: 'bg-purple-100',
    description: 'Files uploaded, AI indexing in progress',
  },
  completed: {
    label: 'Completed',
    color: 'text-green-600',
    bgColor: 'bg-green-100',
    description: 'All items processed successfully',
  },
  failed: {
    label: 'Failed',
    color: 'text-red-600',
    bgColor: 'bg-red-100',
    description: 'Job failed to complete',
  },
  partially_completed: {
    label: 'Partially Completed',
    color: 'text-yellow-600',
    bgColor: 'bg-yellow-100',
    description: 'Some items failed to process',
  },
  cancelled: {
    label: 'Cancelled',
    color: 'text-gray-500',
    bgColor: 'bg-gray-100',
    description: 'Job was cancelled',
  },
};

/**
 * Notification service
 */
export const notificationService = {
  /**
   * Get notifications for the current user
   */
  async getNotifications(options?: ListNotificationsOptions): Promise<NotificationsListResponse> {
    const params: Record<string, string> = {};
    if (options?.unread_only) params.unread_only = 'true';
    if (options?.category) params.category = options.category;
    if (options?.limit) params.limit = options.limit.toString();
    if (options?.offset) params.offset = options.offset.toString();

    const response = await apiClient.get<ApiResponse<NotificationsListResponse>>(
      '/api/v1/notifications',
      { params }
    );
    return response.data.data || { notifications: [], total: 0 };
  },

  /**
   * Get unread notification count
   */
  async getUnreadCount(): Promise<number> {
    const response = await apiClient.get<ApiResponse<UnreadCountResponse>>(
      '/api/v1/notifications/unread-count'
    );
    return response.data.data?.unread_count || 0;
  },

  /**
   * Mark a notification as read
   */
  async markAsRead(notificationId: number): Promise<void> {
    await apiClient.post(`/api/v1/notifications/${notificationId}/read`);
  },

  /**
   * Mark all notifications as read
   */
  async markAllAsRead(): Promise<void> {
    await apiClient.post('/api/v1/notifications/read-all');
  },

  /**
   * Delete a notification
   */
  async deleteNotification(notificationId: number): Promise<void> {
    await apiClient.delete(`/api/v1/notifications/${notificationId}`);
  },

  /**
   * Delete all notifications
   */
  async deleteAllNotifications(): Promise<void> {
    await apiClient.delete('/api/v1/notifications');
  },
};

/**
 * Batch ingest service
 */
export const batchIngestService = {
  /**
   * Start a batch ingest job for multiple PYQ papers
   */
  async startBatchIngest(subjectId: string, papers: BatchIngestPaper[]): Promise<BatchIngestResponse> {
    console.log('[batchIngestService] startBatchIngest called:', { subjectId, paperCount: papers.length });
    const response = await apiClient.post<ApiResponse<BatchIngestResponse>>(
      `/api/v1/subjects/${subjectId}/pyqs/batch-ingest`,
      { papers }
    );
    console.log('[batchIngestService] startBatchIngest response:', response.data);
    return response.data.data!;
  },

  /**
   * Get indexing job status
   */
  async getJobStatus(jobId: number): Promise<IndexingJob> {
    console.log('[batchIngestService] getJobStatus called for jobId:', jobId);
    const response = await apiClient.get<ApiResponse<IndexingJob>>(
      `/api/v1/indexing-jobs/${jobId}`
    );
    console.log('[batchIngestService] getJobStatus response:', response.data);
    return response.data.data!;
  },

  /**
   * Get indexing jobs for a subject
   */
  async getJobsBySubject(
    subjectId: string,
    options?: { status?: IndexingJobStatus; limit?: number; offset?: number }
  ): Promise<IndexingJobsListResponse> {
    const params: Record<string, string> = {};
    if (options?.status) params.status = options.status;
    if (options?.limit) params.limit = options.limit.toString();
    if (options?.offset) params.offset = options.offset.toString();

    const response = await apiClient.get<ApiResponse<IndexingJobsListResponse>>(
      `/api/v1/subjects/${subjectId}/pyqs/indexing-jobs`,
      { params }
    );
    return response.data.data || { jobs: [], total: 0 };
  },

  /**
   * Cancel an active indexing job
   */
  async cancelJob(jobId: number): Promise<{ message: string }> {
    const response = await apiClient.post<ApiResponse<{ message: string }>>(
      `/api/v1/indexing-jobs/${jobId}/cancel`
    );
    return response.data.data || { message: 'Job cancelled' };
  },
};

// ============ BATCH DOCUMENT UPLOAD INTERFACES ============

/**
 * Document type for batch upload
 */
export type DocumentType = 'pyq' | 'book' | 'reference' | 'syllabus' | 'notes';

/**
 * Batch upload response
 */
export interface BatchUploadResponse {
  job_id: number;
  status: IndexingJobStatus;
  total_items: number;
  message: string;
}

/**
 * Batch document upload service
 */
export const batchDocumentUploadService = {
  /**
   * Start a batch upload job for multiple documents
   * @param subjectId - The subject ID to upload documents to
   * @param files - Array of files to upload
   * @param types - Optional array of document types corresponding to each file (defaults to 'notes')
   */
  async startBatchUpload(
    subjectId: string,
    files: File[],
    types?: DocumentType[]
  ): Promise<BatchUploadResponse> {
    console.log('[batchDocumentUploadService] startBatchUpload called:', { 
      subjectId, 
      fileCount: files.length,
      files: files.map(f => ({ name: f.name, size: f.size, type: f.type }))
    });
    
    // Build FormData immediately - don't do any async operations before this
    const formData = new FormData();
    
    // Add files to form data
    files.forEach((file, index) => {
      console.log(`[batchDocumentUploadService] Appending file ${index}:`, file.name, file.size);
      formData.append('files', file, file.name);
    });
    
    // Add types if provided
    if (types && types.length > 0) {
      types.forEach((type) => {
        formData.append('types', type);
      });
    }
    
    const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';
    const accessToken = typeof window !== 'undefined' ? localStorage.getItem('access_token') : null;
    const url = `${API_URL}/api/v1/subjects/${subjectId}/documents/batch-upload`;
    
    console.log('[batchDocumentUploadService] Using XMLHttpRequest to upload');
    console.log('[batchDocumentUploadService] API URL:', url);
    
    // Use XMLHttpRequest for better control over file uploads
    // This avoids issues with fetch() and large file uploads being cancelled
    return new Promise<BatchUploadResponse>((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      
      xhr.open('POST', url, true);
      
      // Set auth header
      if (accessToken) {
        xhr.setRequestHeader('Authorization', `Bearer ${accessToken}`);
      }
      // Don't set Content-Type - browser will set it with boundary for FormData
      
      // Track upload progress
      xhr.upload.onprogress = (event) => {
        if (event.lengthComputable) {
          const percentComplete = Math.round((event.loaded / event.total) * 100);
          console.log(`[batchDocumentUploadService] Upload progress: ${percentComplete}% (${event.loaded}/${event.total} bytes)`);
        }
      };
      
      xhr.upload.onloadstart = () => {
        console.log('[batchDocumentUploadService] Upload started');
      };
      
      xhr.upload.onloadend = () => {
        console.log('[batchDocumentUploadService] Upload ended');
      };
      
      xhr.upload.onerror = (event) => {
        console.error('[batchDocumentUploadService] Upload error:', event);
        reject(new Error('Upload failed: Network error'));
      };
      
      xhr.upload.onabort = () => {
        console.error('[batchDocumentUploadService] Upload aborted!');
        reject(new Error('Upload was aborted'));
      };
      
      xhr.onreadystatechange = () => {
        console.log(`[batchDocumentUploadService] XHR readyState: ${xhr.readyState}, status: ${xhr.status}`);
        
        if (xhr.readyState === XMLHttpRequest.DONE) {
          console.log('[batchDocumentUploadService] XHR completed, status:', xhr.status);
          console.log('[batchDocumentUploadService] Response:', xhr.responseText);
          
          if (xhr.status >= 200 && xhr.status < 300) {
            try {
              const responseData = JSON.parse(xhr.responseText) as ApiResponse<BatchUploadResponse>;
              console.log('[batchDocumentUploadService] Parsed response:', responseData);
              resolve(responseData.data!);
            } catch (e) {
              reject(new Error(`Failed to parse response: ${xhr.responseText}`));
            }
          } else if (xhr.status === 0) {
            // Status 0 means request was cancelled/aborted
            reject(new Error('Request was cancelled or blocked (status 0). Check CORS or network.'));
          } else {
            reject(new Error(`Upload failed: ${xhr.status} - ${xhr.responseText}`));
          }
        }
      };
      
      xhr.onerror = () => {
        console.error('[batchDocumentUploadService] XHR error event');
        reject(new Error('Network error during upload'));
      };
      
      xhr.onabort = () => {
        console.error('[batchDocumentUploadService] XHR abort event');
        reject(new Error('Upload was aborted'));
      };
      
      xhr.ontimeout = () => {
        console.error('[batchDocumentUploadService] XHR timeout');
        reject(new Error('Upload timed out'));
      };
      
      // Set a long timeout for large files (10 minutes)
      xhr.timeout = 600000;
      
      console.log('[batchDocumentUploadService] Sending XHR request...');
      xhr.send(formData);
      console.log('[batchDocumentUploadService] XHR.send() called');
    });
  },

  /**
   * Get document upload jobs for a subject
   */
  async getUploadJobsBySubject(
    subjectId: string,
    options?: { limit?: number; offset?: number }
  ): Promise<IndexingJobsListResponse> {
    const params: Record<string, string> = {};
    if (options?.limit) params.limit = options.limit.toString();
    if (options?.offset) params.offset = options.offset.toString();

    const response = await apiClient.get<ApiResponse<IndexingJobsListResponse>>(
      `/api/v1/subjects/${subjectId}/documents/upload-jobs`,
      { params }
    );
    return response.data.data || { jobs: [], total: 0 };
  },

  /**
   * Get indexing job status (reuses the same endpoint as batch ingest)
   */
  async getJobStatus(jobId: number): Promise<IndexingJob> {
    console.log('[batchDocumentUploadService] getJobStatus called for jobId:', jobId);
    const response = await apiClient.get<ApiResponse<IndexingJob>>(
      `/api/v1/indexing-jobs/${jobId}`
    );
    console.log('[batchDocumentUploadService] getJobStatus response:', response.data);
    return response.data.data!;
  },

  /**
   * Cancel an active upload job (reuses the same endpoint as batch ingest)
   */
  async cancelJob(jobId: number): Promise<{ message: string }> {
    const response = await apiClient.post<ApiResponse<{ message: string }>>(
      `/api/v1/indexing-jobs/${jobId}/cancel`
    );
    return response.data.data || { message: 'Job cancelled' };
  },
};

// ============ AI SETUP JOB INTERFACES ============

/**
 * AI setup status for subjects
 */
export type AISetupStatus = 'none' | 'pending' | 'in_progress' | 'completed' | 'failed';

/**
 * AI setup item metadata
 */
export interface AISetupItemMetadata {
  subject_name?: string;
  subject_code?: string;
  phase?: string;
  knowledge_base_uuid?: string;
  agent_uuid?: string;
  agent_deployment_url?: string;
  has_api_key?: boolean;
}

/**
 * AI setup job item interface
 */
export interface AISetupJobItem {
  id: number;
  job_id?: number;
  item_type: string;
  subject_id?: number;
  status: IndexingJobItemStatus | 'kb_creating' | 'kb_indexing' | 'agent_creating' | 'apikey_creating';
  error_message?: string;
  metadata?: AISetupItemMetadata;
  created_at: string;
  updated_at: string;
}

/**
 * AI setup job interface
 */
export interface AISetupJob {
  id: number;
  job_type: string;
  status: IndexingJobStatus;
  total_items: number;
  completed_items: number;
  failed_items: number;
  started_at?: string;
  completed_at?: string;
  items?: AISetupJobItem[];
}

/**
 * AI setup job status display configuration
 */
export const AI_SETUP_STATUS_CONFIG: Record<AISetupStatus, {
  label: string;
  color: string;
  bgColor: string;
  description: string;
}> = {
  none: {
    label: 'No AI',
    color: 'text-gray-500',
    bgColor: 'bg-gray-100',
    description: 'AI assistant not set up for this subject',
  },
  pending: {
    label: 'Pending',
    color: 'text-gray-600',
    bgColor: 'bg-gray-100',
    description: 'AI setup is queued',
  },
  in_progress: {
    label: 'Setting Up...',
    color: 'text-blue-600',
    bgColor: 'bg-blue-100',
    description: 'AI resources are being created',
  },
  completed: {
    label: 'AI Ready',
    color: 'text-green-600',
    bgColor: 'bg-green-100',
    description: 'AI assistant is ready to use',
  },
  failed: {
    label: 'AI Setup Failed',
    color: 'text-red-600',
    bgColor: 'bg-red-100',
    description: 'Failed to set up AI resources',
  },
};

/**
 * AI setup service
 */
export const aiSetupService = {
  /**
   * Get status of an AI setup job
   */
  async getJobStatus(jobId: number): Promise<AISetupJob> {
    console.log('[aiSetupService] getJobStatus called for jobId:', jobId);
    const response = await apiClient.get<ApiResponse<AISetupJob>>(
      `/api/v1/ai-setup-jobs/${jobId}`
    );
    console.log('[aiSetupService] getJobStatus response:', response.data);
    return response.data.data!;
  },

  /**
   * Get user's active AI setup job
   */
  async getActiveJob(): Promise<AISetupJob | null> {
    console.log('[aiSetupService] getActiveJob called');
    const response = await apiClient.get<ApiResponse<{ active_job: AISetupJob | null }>>(
      '/api/v1/ai-setup-jobs/active'
    );
    console.log('[aiSetupService] getActiveJob response:', response.data);
    return response.data.data?.active_job || null;
  },
};
