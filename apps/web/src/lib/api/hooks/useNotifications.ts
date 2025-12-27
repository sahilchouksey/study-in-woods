import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  notificationService,
  batchIngestService,
  batchDocumentUploadService,
  aiSetupService,
  type ListNotificationsOptions,
  type BatchIngestPaper,
  type IndexingJobStatus,
  type DocumentType,
} from '@/lib/api/notifications';

// ============= Notification Queries =============

/**
 * Hook to get notifications for the current user
 */
export function useNotifications(options?: ListNotificationsOptions, enabled: boolean = true) {
  return useQuery({
    queryKey: ['notifications', options],
    queryFn: () => notificationService.getNotifications(options),
    enabled,
    staleTime: 30 * 1000, // 30 seconds
    refetchInterval: 60 * 1000, // Refetch every minute for updates
  });
}

/**
 * Hook to get unread notification count
 * Polls frequently for real-time badge updates
 */
export function useUnreadNotificationCount(enabled: boolean = true) {
  return useQuery({
    queryKey: ['notifications', 'unread-count'],
    queryFn: () => notificationService.getUnreadCount(),
    enabled,
    staleTime: 10 * 1000, // 10 seconds
    refetchInterval: 30 * 1000, // Poll every 30 seconds
  });
}

// ============= Notification Mutations =============

/**
 * Hook to mark a notification as read
 */
export function useMarkNotificationAsRead() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (notificationId: number) => notificationService.markAsRead(notificationId),
    onSuccess: () => {
      // Invalidate notifications and unread count
      queryClient.invalidateQueries({ queryKey: ['notifications'] });
    },
    // Optimistic update
    onMutate: async (notificationId) => {
      await queryClient.cancelQueries({ queryKey: ['notifications'] });

      // Snapshot previous value
      const previousNotifications = queryClient.getQueryData(['notifications']);

      // Optimistically update notification as read
      queryClient.setQueriesData({ queryKey: ['notifications'] }, (old: unknown) => {
        if (!old || typeof old !== 'object') return old;
        const data = old as { notifications: { id: number; is_read: boolean }[]; total: number };
        return {
          ...data,
          notifications: data.notifications?.map((n) =>
            n.id === notificationId ? { ...n, is_read: true } : n
          ),
        };
      });

      return { previousNotifications };
    },
    onError: (_err, _notificationId, context) => {
      // Rollback on error
      if (context?.previousNotifications) {
        queryClient.setQueryData(['notifications'], context.previousNotifications);
      }
    },
  });
}

/**
 * Hook to mark all notifications as read
 */
export function useMarkAllNotificationsAsRead() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: () => notificationService.markAllAsRead(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['notifications'] });
    },
    // Optimistic update
    onMutate: async () => {
      await queryClient.cancelQueries({ queryKey: ['notifications'] });

      const previousNotifications = queryClient.getQueryData(['notifications']);

      // Mark all as read optimistically
      queryClient.setQueriesData({ queryKey: ['notifications'] }, (old: unknown) => {
        if (!old || typeof old !== 'object') return old;
        const data = old as { notifications: { is_read: boolean }[]; total: number };
        return {
          ...data,
          notifications: data.notifications?.map((n) => ({ ...n, is_read: true })),
        };
      });

      // Set unread count to 0
      queryClient.setQueryData(['notifications', 'unread-count'], 0);

      return { previousNotifications };
    },
    onError: (_err, _vars, context) => {
      if (context?.previousNotifications) {
        queryClient.setQueryData(['notifications'], context.previousNotifications);
      }
    },
  });
}

/**
 * Hook to delete a notification
 */
export function useDeleteNotification() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (notificationId: number) => notificationService.deleteNotification(notificationId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['notifications'] });
    },
    // Optimistic update
    onMutate: async (notificationId) => {
      await queryClient.cancelQueries({ queryKey: ['notifications'] });

      const previousNotifications = queryClient.getQueryData(['notifications']);

      queryClient.setQueriesData({ queryKey: ['notifications'] }, (old: unknown) => {
        if (!old || typeof old !== 'object') return old;
        const data = old as { notifications: { id: number }[]; total: number };
        return {
          ...data,
          notifications: data.notifications?.filter((n) => n.id !== notificationId),
          total: Math.max(0, (data.total || 0) - 1),
        };
      });

      return { previousNotifications };
    },
    onError: (_err, _notificationId, context) => {
      if (context?.previousNotifications) {
        queryClient.setQueryData(['notifications'], context.previousNotifications);
      }
    },
  });
}

/**
 * Hook to delete all notifications
 */
export function useDeleteAllNotifications() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: () => notificationService.deleteAllNotifications(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['notifications'] });
    },
    onMutate: async () => {
      await queryClient.cancelQueries({ queryKey: ['notifications'] });

      const previousNotifications = queryClient.getQueryData(['notifications']);

      // Clear all notifications optimistically
      queryClient.setQueriesData({ queryKey: ['notifications'] }, () => ({
        notifications: [],
        total: 0,
      }));
      queryClient.setQueryData(['notifications', 'unread-count'], 0);

      return { previousNotifications };
    },
    onError: (_err, _vars, context) => {
      if (context?.previousNotifications) {
        queryClient.setQueryData(['notifications'], context.previousNotifications);
      }
    },
  });
}

// ============= Batch Ingest Queries =============

/**
 * Hook to get indexing job status with polling
 */
export function useIndexingJobStatus(jobId: number | null, shouldPoll: boolean = false) {
  const isEnabled = !!jobId && shouldPoll;
  
  console.log('[useIndexingJobStatus] Hook called:', { 
    jobId, 
    shouldPoll, 
    isEnabled,
    willPoll: isEnabled ? 'every 2s' : 'disabled'
  });
  
  return useQuery({
    queryKey: ['indexing-jobs', jobId],
    queryFn: async () => {
      console.log('[useIndexingJobStatus] Fetching job status for:', jobId);
      const result = await batchIngestService.getJobStatus(jobId!);
      console.log('[useIndexingJobStatus] Got result:', result);
      return result;
    },
    enabled: isEnabled,
    staleTime: 1000, // 1 second
    refetchInterval: isEnabled ? 2000 : false, // Poll every 2 seconds when active
    refetchIntervalInBackground: true, // Keep polling even when tab is not focused
  });
}

/**
 * Hook to get indexing jobs for a subject
 */
export function useIndexingJobsBySubject(
  subjectId: string | null,
  options?: { status?: IndexingJobStatus; limit?: number; offset?: number }
) {
  return useQuery({
    queryKey: ['indexing-jobs', 'subject', subjectId, options],
    queryFn: () => batchIngestService.getJobsBySubject(subjectId!, options),
    enabled: !!subjectId,
    staleTime: 30 * 1000, // 30 seconds
  });
}

// ============= Batch Ingest Mutations =============

/**
 * Hook to start a batch ingest job
 */
export function useBatchIngestPYQs() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ subjectId, papers }: { subjectId: string; papers: BatchIngestPaper[] }) =>
      batchIngestService.startBatchIngest(subjectId, papers),
    onSuccess: (data, variables) => {
      // Invalidate PYQ queries for this subject
      queryClient.invalidateQueries({ queryKey: ['pyqs', 'subject', variables.subjectId] });
      // Invalidate indexing jobs
      queryClient.invalidateQueries({ queryKey: ['indexing-jobs', 'subject', variables.subjectId] });
      // Invalidate notifications (new notification will be created)
      queryClient.invalidateQueries({ queryKey: ['notifications'] });

      // Return the job ID for tracking
      return data;
    },
  });
}

/**
 * Hook to cancel an indexing job
 */
export function useCancelIndexingJob() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (jobId: number) => batchIngestService.cancelJob(jobId),
    onSuccess: (_data, jobId) => {
      // Invalidate the specific job
      queryClient.invalidateQueries({ queryKey: ['indexing-jobs', jobId] });
      // Invalidate notifications
      queryClient.invalidateQueries({ queryKey: ['notifications'] });
    },
  });
}

// ============= Combined Hooks =============

/**
 * Hook that provides all notification-related functionality
 */
export function useNotificationManager() {
  const notifications = useNotifications();
  const unreadCount = useUnreadNotificationCount();
  const markAsRead = useMarkNotificationAsRead();
  const markAllAsRead = useMarkAllNotificationsAsRead();
  const deleteNotification = useDeleteNotification();
  const deleteAll = useDeleteAllNotifications();

  return {
    // Queries
    notifications: notifications.data?.notifications || [],
    total: notifications.data?.total || 0,
    unreadCount: unreadCount.data || 0,
    isLoading: notifications.isLoading,
    isError: notifications.isError,
    error: notifications.error,

    // Mutations
    markAsRead: markAsRead.mutate,
    markAllAsRead: markAllAsRead.mutate,
    deleteNotification: deleteNotification.mutate,
    deleteAll: deleteAll.mutate,

    // Mutation states
    isMarkingAsRead: markAsRead.isPending,
    isMarkingAllAsRead: markAllAsRead.isPending,
    isDeleting: deleteNotification.isPending,
    isDeletingAll: deleteAll.isPending,

    // Refetch
    refetch: notifications.refetch,
  };
}

/**
 * Hook that provides batch ingest functionality with job tracking
 */
export function useBatchIngestManager(subjectId: string | null) {
  const queryClient = useQueryClient();
  const batchIngest = useBatchIngestPYQs();
  const cancelJob = useCancelIndexingJob();

  // Track active job ID
  const activeJobId = batchIngest.data?.job_id || null;

  // Get job status with polling when there's an active job
  const jobStatus = useIndexingJobStatus(
    activeJobId,
    activeJobId !== null && batchIngest.data?.status === 'processing'
  );

  // Get all jobs for subject
  const subjectJobs = useIndexingJobsBySubject(subjectId);

  return {
    // Start batch ingest
    startBatchIngest: (papers: BatchIngestPaper[]) => {
      if (!subjectId) return;
      return batchIngest.mutateAsync({ subjectId, papers });
    },

    // Cancel active job
    cancelJob: (jobId: number) => cancelJob.mutate(jobId),

    // Active job info
    activeJobId,
    activeJob: jobStatus.data,
    isProcessing: batchIngest.isPending || jobStatus.data?.status === 'processing',

    // Job status
    progress: jobStatus.data?.progress || 0,
    completedItems: jobStatus.data?.completed_items || 0,
    failedItems: jobStatus.data?.failed_items || 0,
    totalItems: jobStatus.data?.total_items || 0,

    // All jobs for subject
    jobs: subjectJobs.data?.jobs || [],
    totalJobs: subjectJobs.data?.total || 0,

    // States
    isStarting: batchIngest.isPending,
    isCancelling: cancelJob.isPending,
    isLoadingJobs: subjectJobs.isLoading,
    error: batchIngest.error || jobStatus.error,

    // Reset
    reset: () => {
      batchIngest.reset();
      queryClient.invalidateQueries({ queryKey: ['indexing-jobs'] });
    },
  };
}

// ============= Batch Document Upload Hooks =============

/**
 * Hook to start a batch document upload job
 */
export function useBatchUploadDocuments() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ 
      subjectId, 
      files, 
      types 
    }: { 
      subjectId: string; 
      files: File[]; 
      types?: DocumentType[] 
    }) => batchDocumentUploadService.startBatchUpload(subjectId, files, types),
    onSuccess: (data, variables) => {
      // Invalidate documents queries for this subject
      queryClient.invalidateQueries({ queryKey: ['documents', 'subject', variables.subjectId] });
      // Invalidate indexing jobs
      queryClient.invalidateQueries({ queryKey: ['indexing-jobs', 'subject', variables.subjectId] });
      queryClient.invalidateQueries({ queryKey: ['document-upload-jobs', 'subject', variables.subjectId] });
      // Invalidate notifications (new notification will be created)
      queryClient.invalidateQueries({ queryKey: ['notifications'] });

      // Return the job ID for tracking
      return data;
    },
  });
}

/**
 * Hook to get document upload jobs for a subject
 */
export function useDocumentUploadJobsBySubject(
  subjectId: string | null,
  options?: { limit?: number; offset?: number }
) {
  return useQuery({
    queryKey: ['document-upload-jobs', 'subject', subjectId, options],
    queryFn: () => batchDocumentUploadService.getUploadJobsBySubject(subjectId!, options),
    enabled: !!subjectId,
    staleTime: 30 * 1000, // 30 seconds
  });
}

/**
 * Hook that provides batch document upload functionality with job tracking
 */
export function useBatchUploadManager(subjectId: string | null) {
  const queryClient = useQueryClient();
  const batchUpload = useBatchUploadDocuments();
  const cancelJob = useCancelIndexingJob();

  // Track active job ID
  const activeJobId = batchUpload.data?.job_id || null;

  // Determine if we should poll based on job status
  const shouldPoll = activeJobId !== null && 
    batchUpload.data?.status !== 'completed' && 
    batchUpload.data?.status !== 'failed' &&
    batchUpload.data?.status !== 'cancelled';

  // Get job status with polling when there's an active job
  const jobStatus = useIndexingJobStatus(activeJobId, shouldPoll);

  // Get all upload jobs for subject
  const subjectJobs = useDocumentUploadJobsBySubject(subjectId);

  // Check if job is still in a processing state
  const isJobActive = jobStatus.data?.status === 'pending' || 
    jobStatus.data?.status === 'processing' || 
    jobStatus.data?.status === 'kb_indexing';

  return {
    // Start batch upload
    startBatchUpload: (files: File[], types?: DocumentType[]) => {
      if (!subjectId) return;
      return batchUpload.mutateAsync({ subjectId, files, types });
    },

    // Cancel active job
    cancelJob: (jobId: number) => cancelJob.mutate(jobId),

    // Active job info
    activeJobId,
    activeJob: jobStatus.data,
    isProcessing: batchUpload.isPending || isJobActive,

    // Job status
    status: jobStatus.data?.status,
    progress: jobStatus.data?.progress || 0,
    completedItems: jobStatus.data?.completed_items || 0,
    failedItems: jobStatus.data?.failed_items || 0,
    totalItems: jobStatus.data?.total_items || 0,
    items: jobStatus.data?.items || [],

    // All jobs for subject
    jobs: subjectJobs.data?.jobs || [],
    totalJobs: subjectJobs.data?.total || 0,

    // States
    isStarting: batchUpload.isPending,
    isCancelling: cancelJob.isPending,
    isLoadingJobs: subjectJobs.isLoading,
    error: batchUpload.error || jobStatus.error,

    // Reset
    reset: () => {
      batchUpload.reset();
      queryClient.invalidateQueries({ queryKey: ['indexing-jobs'] });
      queryClient.invalidateQueries({ queryKey: ['document-upload-jobs'] });
    },
  };
}

// ============= AI Setup Job Hooks =============

/**
 * Hook to get AI setup job status with polling
 */
export function useAISetupJobStatus(jobId: number | null, shouldPoll: boolean = false) {
  const isEnabled = !!jobId && shouldPoll;
  
  console.log('[useAISetupJobStatus] Hook called:', { 
    jobId, 
    shouldPoll, 
    isEnabled,
    willPoll: isEnabled ? 'every 3s' : 'disabled'
  });
  
  return useQuery({
    queryKey: ['ai-setup-jobs', jobId],
    queryFn: async () => {
      console.log('[useAISetupJobStatus] Fetching job status for:', jobId);
      const result = await aiSetupService.getJobStatus(jobId!);
      console.log('[useAISetupJobStatus] Got result:', result);
      return result;
    },
    enabled: isEnabled,
    staleTime: 2000, // 2 seconds
    refetchInterval: isEnabled ? 3000 : false, // Poll every 3 seconds when active
    refetchIntervalInBackground: true,
  });
}

/**
 * Hook to get user's active AI setup job
 */
export function useActiveAISetupJob(enabled: boolean = true) {
  return useQuery({
    queryKey: ['ai-setup-jobs', 'active'],
    queryFn: () => aiSetupService.getActiveJob(),
    enabled,
    staleTime: 10 * 1000, // 10 seconds
    refetchInterval: 30 * 1000, // Poll every 30 seconds
  });
}

/**
 * Hook that provides AI setup job tracking functionality
 */
export function useAISetupJobManager() {
  // Get active job
  const activeJobQuery = useActiveAISetupJob();
  const activeJobId = activeJobQuery.data?.id || null;
  
  // Determine if we should poll based on job status
  const shouldPoll = activeJobId !== null && 
    activeJobQuery.data?.status !== 'completed' && 
    activeJobQuery.data?.status !== 'failed' &&
    activeJobQuery.data?.status !== 'cancelled';
  
  // Get detailed job status with polling when there's an active job
  const jobStatus = useAISetupJobStatus(activeJobId, shouldPoll);
  
  // Check if job is still in a processing state
  const isJobActive = jobStatus.data?.status === 'pending' || 
    jobStatus.data?.status === 'processing' || 
    jobStatus.data?.status === 'kb_indexing';
  
  return {
    // Active job info
    activeJobId,
    activeJob: jobStatus.data || activeJobQuery.data,
    hasActiveJob: activeJobId !== null,
    isProcessing: isJobActive,
    
    // Job status
    status: jobStatus.data?.status || activeJobQuery.data?.status,
    completedItems: jobStatus.data?.completed_items || 0,
    failedItems: jobStatus.data?.failed_items || 0,
    totalItems: jobStatus.data?.total_items || 0,
    items: jobStatus.data?.items || [],
    
    // Loading states
    isLoading: activeJobQuery.isLoading,
    isPolling: shouldPoll && jobStatus.isFetching,
    error: activeJobQuery.error || jobStatus.error,
    
    // Refetch
    refetch: () => {
      activeJobQuery.refetch();
      if (activeJobId) {
        jobStatus.refetch();
      }
    },
  };
}
