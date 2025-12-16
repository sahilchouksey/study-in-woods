'use client';

import {
  Bell,
  CheckCircle2,
  AlertCircle,
  Info,
  AlertTriangle,
  Trash2,
  CheckCheck,
  X,
} from 'lucide-react';
import { LoadingSpinner, InlineSpinner } from '@/components/ui/loading-spinner';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import { useNotifications, type Notification, formatNotificationTime } from '@/providers/notification-provider';
import type { NotificationType } from '@/lib/api/notifications';
import { cn } from '@/lib/utils';

const typeConfig: Record<NotificationType, { 
  icon: React.ElementType; 
  borderColor: string;
}> = {
  info: {
    icon: Info,
    borderColor: 'border-l-border',
  },
  success: {
    icon: CheckCircle2,
    borderColor: 'border-l-emerald-500',
  },
  warning: {
    icon: AlertTriangle,
    borderColor: 'border-l-amber-500',
  },
  error: {
    icon: AlertCircle,
    borderColor: 'border-l-red-500',
  },
  in_progress: {
    icon: Info,
    borderColor: 'border-l-border',
  },
};

function NotificationItem({
  notification,
  onMarkAsRead,
  onDismiss,
}: {
  notification: Notification;
  onMarkAsRead: () => void;
  onDismiss: () => void;
}) {
  const config = typeConfig[notification.type];
  const Icon = config.icon;

  return (
    <div
      className={cn(
        'relative border-l-4 border bg-card rounded-lg transition-colors cursor-pointer group',
        config.borderColor,
        !notification.read 
          ? 'bg-card' 
          : 'bg-muted/30'
      )}
      onClick={onMarkAsRead}
    >
      <div className="p-4">
        <div className="flex items-start gap-3">
          {/* Icon */}
          <div className="shrink-0 mt-0.5 text-muted-foreground">
            {notification.type === 'in_progress' ? (
              <InlineSpinner />
            ) : (
              <Icon className="h-5 w-5" />
            )}
          </div>

          {/* Content */}
          <div className="flex-1 min-w-0">
            <div className="flex items-start justify-between gap-3">
              <div className="flex-1 min-w-0">
                <p className={cn(
                  'text-sm font-medium',
                  !notification.read ? 'text-foreground' : 'text-muted-foreground'
                )}>
                  {notification.title}
                </p>
                <p className="text-sm text-muted-foreground mt-1">
                  {notification.message}
                </p>
              </div>
              
              {/* Dismiss button */}
              <Button
                variant="ghost"
                size="icon"
                className={cn(
                  'h-8 w-8 shrink-0',
                  'opacity-0 group-hover:opacity-100 transition-opacity',
                  'hover:bg-muted'
                )}
                onClick={(e) => {
                  e.stopPropagation();
                  onDismiss();
                }}
              >
                <X className="h-4 w-4" />
              </Button>
            </div>

            {/* Progress bar for in-progress items */}
            {notification.type === 'in_progress' && notification.metadata?.progress !== undefined && (
              <div className="mt-3 space-y-1">
                <Progress value={notification.metadata.progress} className="h-1.5" />
                <div className="flex items-center justify-between text-xs text-muted-foreground">
                  {notification.metadata.totalItems && notification.metadata.completedItems !== undefined && (
                    <span>
                      {notification.metadata.completedItems} / {notification.metadata.totalItems} items
                    </span>
                  )}
                  <span>{notification.metadata.progress}%</span>
                </div>
              </div>
            )}

            {/* Timestamp */}
            <p className="text-xs text-muted-foreground mt-2">
              {formatNotificationTime(notification.timestamp)}
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}

export function NotificationsTab() {
  const {
    notifications,
    unreadCount,
    isLoading,
    markAsRead,
    markAllAsRead,
    removeNotification,
    clearAll,
  } = useNotifications();

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="border-b border-border bg-card p-6">
        <div className="max-w-3xl mx-auto">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="text-foreground text-xl font-semibold">Notifications</h2>
              <p className="text-muted-foreground text-sm mt-1">
                {unreadCount > 0 
                  ? `${unreadCount} unread`
                  : 'All caught up'}
              </p>
            </div>
            
            {/* Actions */}
            <div className="flex items-center gap-2">
              {unreadCount > 0 && (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={markAllAsRead}
                >
                  <CheckCheck className="h-4 w-4 mr-2" />
                  Mark all read
                </Button>
              )}
              {notifications.length > 0 && (
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={clearAll}
                >
                  <Trash2 className="h-4 w-4 mr-2" />
                  Clear all
                </Button>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* Notifications List */}
      <div className="flex-1 overflow-auto">
        {isLoading ? (
          <div className="p-8">
            <LoadingSpinner size="lg" text="Loading notifications..." centered />
          </div>
        ) : notifications.length > 0 ? (
          <div className="max-w-3xl mx-auto p-6 space-y-2">
            {notifications.map((notification) => (
              <NotificationItem
                key={notification.id}
                notification={notification}
                onMarkAsRead={() => markAsRead(notification.id)}
                onDismiss={() => removeNotification(notification.id)}
              />
            ))}
          </div>
        ) : (
          <div className="flex flex-col items-center justify-center h-full p-8 text-center">
            <Bell className="h-12 w-12 mb-4 text-muted-foreground/30" />
            <h3 className="text-lg font-medium text-foreground">No notifications</h3>
            <p className="text-sm text-muted-foreground mt-1 max-w-sm">
              When you upload files or perform actions, notifications will appear here
            </p>
          </div>
        )}
      </div>
    </div>
  );
}
