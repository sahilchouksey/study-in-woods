/**
 * Notification system for tracking async jobs (ingest, upload, etc.)
 */

export type NotificationType = 'info' | 'success' | 'warning' | 'error' | 'in_progress';

export type NotificationCategory = 'pyq_ingest' | 'document_upload' | 'syllabus_extraction' | 'general';

export interface Notification {
  id: string;
  type: NotificationType;
  category: NotificationCategory;
  title: string;
  message: string;
  timestamp: number;
  read: boolean;
  /** Optional metadata for tracking job progress */
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

export interface NotificationStore {
  notifications: Notification[];
  unreadCount: number;
}

const STORAGE_KEY = 'study-in-woods-notifications';
const MAX_NOTIFICATIONS = 50;

/**
 * Load notifications from localStorage
 */
export function loadNotifications(): Notification[] {
  if (typeof window === 'undefined') return [];
  
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (!stored) return [];
    
    const notifications = JSON.parse(stored) as Notification[];
    // Filter out notifications older than 7 days
    const sevenDaysAgo = Date.now() - 7 * 24 * 60 * 60 * 1000;
    return notifications.filter(n => n.timestamp > sevenDaysAgo);
  } catch {
    return [];
  }
}

/**
 * Save notifications to localStorage
 */
export function saveNotifications(notifications: Notification[]): void {
  if (typeof window === 'undefined') return;
  
  try {
    // Keep only the most recent notifications
    const toSave = notifications.slice(0, MAX_NOTIFICATIONS);
    localStorage.setItem(STORAGE_KEY, JSON.stringify(toSave));
  } catch {
    // Ignore storage errors
  }
}

/**
 * Create a new notification
 */
export function createNotification(
  type: NotificationType,
  category: NotificationCategory,
  title: string,
  message: string,
  metadata?: Notification['metadata']
): Notification {
  return {
    id: `${Date.now()}-${Math.random().toString(36).slice(2, 11)}`,
    type,
    category,
    title,
    message,
    timestamp: Date.now(),
    read: false,
    metadata,
  };
}

/**
 * Format timestamp for display
 */
export function formatNotificationTime(timestamp: number): string {
  const now = Date.now();
  const diff = now - timestamp;
  
  const seconds = Math.floor(diff / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);
  const days = Math.floor(hours / 24);
  
  if (seconds < 60) return 'Just now';
  if (minutes < 60) return `${minutes}m ago`;
  if (hours < 24) return `${hours}h ago`;
  if (days === 1) return 'Yesterday';
  if (days < 7) return `${days}d ago`;
  
  return new Date(timestamp).toLocaleDateString();
}
