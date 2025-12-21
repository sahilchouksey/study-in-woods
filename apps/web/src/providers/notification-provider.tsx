'use client';

import React, { createContext, useContext, useCallback, useMemo, useState, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  notificationService,
  type UserNotification,
  type NotificationType,
  type NotificationCategory,
} from '@/lib/api/notifications';
import { formatNotificationTime as formatApiTime } from '@/lib/notifications';
import { authService } from '@/lib/api/auth';

// Re-export the Notification type for backward compatibility
// The API notification structure is similar but with numeric IDs
export interface Notification {
  id: string;
  type: NotificationType;
  category: NotificationCategory;
  title: string;
  message: string;
  timestamp: number;
  read: boolean;
  metadata?: {
    subjectId?: string;
    subjectName?: string;
    documentId?: string;
    jobId?: string;
    progress?: number;
    totalItems?: number;
    completedItems?: number;
  };
}

// Convert API notification to local format
function toLocalNotification(apiNotification: UserNotification): Notification {
  return {
    id: apiNotification.id.toString(),
    type: apiNotification.type,
    category: apiNotification.category,
    title: apiNotification.title,
    message: apiNotification.message,
    timestamp: new Date(apiNotification.created_at).getTime(),
    read: apiNotification.is_read,
    metadata: apiNotification.metadata ? {
      subjectId: apiNotification.metadata.subject_id?.toString(),
      subjectName: apiNotification.metadata.subject_name,
      documentId: apiNotification.metadata.document_id?.toString(),
      jobId: apiNotification.indexing_job_id?.toString(),
      progress: apiNotification.metadata.progress,
      totalItems: apiNotification.metadata.paper_count,
      completedItems: apiNotification.metadata.completed_count,
    } : undefined,
  };
}

interface NotificationContextType {
  notifications: Notification[];
  unreadCount: number;
  isLoading: boolean;
  isError: boolean;
  addNotification: (
    type: NotificationType,
    category: NotificationCategory,
    title: string,
    message: string,
    metadata?: Notification['metadata']
  ) => Notification;
  updateNotification: (id: string, updates: Partial<Notification>) => void;
  markAsRead: (id: string) => void;
  markAllAsRead: () => void;
  removeNotification: (id: string) => void;
  clearAll: () => void;
  refetch: () => void;
}

const NotificationContext = createContext<NotificationContextType | undefined>(undefined);

export function NotificationProvider({ children }: { children: React.ReactNode }) {
  const queryClient = useQueryClient();

  // Use state to track authentication status after hydration
  // This prevents hydration mismatches and ensures stable query behavior
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  
  useEffect(() => {
    // Check auth status after component mounts (client-side only)
    setIsAuthenticated(authService.isAuthenticated());
  }, []);

  // Fetch notifications from API
  const {
    data: notificationsData,
    isLoading,
    isError,
    refetch,
  } = useQuery({
    queryKey: ['notifications'],
    queryFn: () => notificationService.getNotifications({ limit: 50 }),
    staleTime: 30 * 1000, // 30 seconds
    refetchInterval: isAuthenticated ? 60 * 1000 : false, // Poll every minute only when authenticated
    // Don't throw on error - we'll handle it gracefully
    retry: 2,
    enabled: isAuthenticated, // Only run query when authenticated
  });

  // Fetch unread count separately for more frequent updates
  const { data: unreadCountData } = useQuery({
    queryKey: ['notifications', 'unread-count'],
    queryFn: () => notificationService.getUnreadCount(),
    staleTime: 10 * 1000, // 10 seconds
    refetchInterval: isAuthenticated ? 30 * 1000 : false, // Poll every 30 seconds only when authenticated
    retry: 2,
    enabled: isAuthenticated, // Only run query when authenticated
  });

  // Convert API notifications to local format
  const notifications = useMemo(() => {
    return (notificationsData?.notifications || []).map(toLocalNotification);
  }, [notificationsData?.notifications]);

  const unreadCount = unreadCountData ?? notifications.filter(n => !n.read).length;

  // Mark as read mutation
  const markAsReadMutation = useMutation({
    mutationFn: (id: string) => notificationService.markAsRead(parseInt(id, 10)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['notifications'] });
    },
    // Optimistic update
    onMutate: async (id: string) => {
      await queryClient.cancelQueries({ queryKey: ['notifications'] });
      const previousData = queryClient.getQueryData(['notifications']);
      
      queryClient.setQueryData(['notifications'], (old: typeof notificationsData) => {
        if (!old) return old;
        return {
          ...old,
          notifications: old.notifications.map((n: UserNotification) =>
            n.id.toString() === id ? { ...n, is_read: true } : n
          ),
        };
      });
      
      return { previousData };
    },
    onError: (_err, _id, context) => {
      if (context?.previousData) {
        queryClient.setQueryData(['notifications'], context.previousData);
      }
    },
  });

  // Mark all as read mutation
  const markAllAsReadMutation = useMutation({
    mutationFn: () => notificationService.markAllAsRead(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['notifications'] });
    },
    onMutate: async () => {
      await queryClient.cancelQueries({ queryKey: ['notifications'] });
      const previousData = queryClient.getQueryData(['notifications']);
      
      queryClient.setQueryData(['notifications'], (old: typeof notificationsData) => {
        if (!old) return old;
        return {
          ...old,
          notifications: old.notifications.map((n: UserNotification) => ({ ...n, is_read: true })),
        };
      });
      queryClient.setQueryData(['notifications', 'unread-count'], 0);
      
      return { previousData };
    },
    onError: (_err, _vars, context) => {
      if (context?.previousData) {
        queryClient.setQueryData(['notifications'], context.previousData);
      }
    },
  });

  // Delete notification mutation
  const deleteNotificationMutation = useMutation({
    mutationFn: (id: string) => notificationService.deleteNotification(parseInt(id, 10)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['notifications'] });
    },
    onMutate: async (id: string) => {
      await queryClient.cancelQueries({ queryKey: ['notifications'] });
      const previousData = queryClient.getQueryData(['notifications']);
      
      queryClient.setQueryData(['notifications'], (old: typeof notificationsData) => {
        if (!old) return old;
        return {
          ...old,
          notifications: old.notifications.filter((n: UserNotification) => n.id.toString() !== id),
          total: Math.max(0, old.total - 1),
        };
      });
      
      return { previousData };
    },
    onError: (_err, _id, context) => {
      if (context?.previousData) {
        queryClient.setQueryData(['notifications'], context.previousData);
      }
    },
  });

  // Delete all notifications mutation
  const deleteAllMutation = useMutation({
    mutationFn: () => notificationService.deleteAllNotifications(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['notifications'] });
    },
    onMutate: async () => {
      await queryClient.cancelQueries({ queryKey: ['notifications'] });
      const previousData = queryClient.getQueryData(['notifications']);
      
      queryClient.setQueryData(['notifications'], { notifications: [], total: 0 });
      queryClient.setQueryData(['notifications', 'unread-count'], 0);
      
      return { previousData };
    },
    onError: (_err, _vars, context) => {
      if (context?.previousData) {
        queryClient.setQueryData(['notifications'], context.previousData);
      }
    },
  });

  // Add notification - this is a local-only operation for immediate feedback
  // The backend creates notifications automatically for batch jobs
  const addNotification = useCallback((
    type: NotificationType,
    category: NotificationCategory,
    title: string,
    message: string,
    metadata?: Notification['metadata']
  ): Notification => {
    const notification: Notification = {
      id: `local-${Date.now()}-${Math.random().toString(36).slice(2, 11)}`,
      type,
      category,
      title,
      message,
      timestamp: Date.now(),
      read: false,
      metadata,
    };

    // Add to local cache optimistically
    queryClient.setQueryData(['notifications'], (old: typeof notificationsData) => {
      if (!old) {
        return {
          notifications: [{
            id: 0, // Temporary ID
            user_id: 0,
            type,
            category,
            title,
            message,
            is_read: false,
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
            metadata: metadata ? {
              subject_id: metadata.subjectId ? parseInt(metadata.subjectId, 10) : undefined,
              subject_name: metadata.subjectName,
              document_id: metadata.documentId ? parseInt(metadata.documentId, 10) : undefined,
              progress: metadata.progress,
              paper_count: metadata.totalItems,
              completed_count: metadata.completedItems,
            } : undefined,
          }],
          total: 1,
        };
      }
      return old;
    });

    // Refetch to get the actual notification from backend
    setTimeout(() => refetch(), 1000);

    return notification;
  }, [queryClient, refetch]);

  // Update notification - primarily for local progress updates
  const updateNotification = useCallback((id: string, updates: Partial<Notification>) => {
    queryClient.setQueryData(['notifications'], (old: typeof notificationsData) => {
      if (!old) return old;
      return {
        ...old,
        notifications: old.notifications.map((n: UserNotification) => {
          if (n.id.toString() === id) {
            return {
              ...n,
              ...(updates.type && { type: updates.type }),
              ...(updates.title && { title: updates.title }),
              ...(updates.message && { message: updates.message }),
              ...(updates.read !== undefined && { is_read: updates.read }),
              ...(updates.metadata && {
                metadata: {
                  ...n.metadata,
                  progress: updates.metadata.progress,
                  completed_count: updates.metadata.completedItems,
                },
              }),
            };
          }
          return n;
        }),
      };
    });
  }, [queryClient]);

  const markAsRead = useCallback((id: string) => {
    markAsReadMutation.mutate(id);
  }, [markAsReadMutation]);

  const markAllAsRead = useCallback(() => {
    markAllAsReadMutation.mutate();
  }, [markAllAsReadMutation]);

  const removeNotification = useCallback((id: string) => {
    deleteNotificationMutation.mutate(id);
  }, [deleteNotificationMutation]);

  const clearAll = useCallback(() => {
    deleteAllMutation.mutate();
  }, [deleteAllMutation]);

  const contextValue = useMemo(() => ({
    notifications,
    unreadCount,
    isLoading,
    isError,
    addNotification,
    updateNotification,
    markAsRead,
    markAllAsRead,
    removeNotification,
    clearAll,
    refetch,
  }), [
    notifications,
    unreadCount,
    isLoading,
    isError,
    addNotification,
    updateNotification,
    markAsRead,
    markAllAsRead,
    removeNotification,
    clearAll,
    refetch,
  ]);

  return (
    <NotificationContext.Provider value={contextValue}>
      {children}
    </NotificationContext.Provider>
  );
}

export function useNotifications() {
  const context = useContext(NotificationContext);
  if (!context) {
    throw new Error('useNotifications must be used within NotificationProvider');
  }
  return context;
}

// Re-export the time formatter for use in components
export { formatApiTime as formatNotificationTime };
