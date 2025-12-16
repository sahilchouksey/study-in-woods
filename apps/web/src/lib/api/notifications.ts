import { apiClient } from './client';
import type { ApiResponse } from '@/types/api';

/**
 * Notification type
 */
export type NotificationType = 'info' | 'success' | 'warning' | 'error' | 'in_progress';

/**
 * Notification category
 */
export type NotificationCategory = 'pyq_ingest' | 'document_upload' | 'syllabus_extraction' | 'general';

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
