'use client';

import { useState } from 'react';
import {
  Bell,
  CheckCircle2,
  AlertCircle,
  Info,
  Loader2,
  AlertTriangle,
  Trash2,
  CheckCheck,
  X,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { ScrollArea } from '@/components/ui/scroll-area';
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover';
import { Progress } from '@/components/ui/progress';
import { useNotifications, type Notification, formatNotificationTime } from '@/providers/notification-provider';
import type { NotificationType } from '@/lib/api/notifications';
import { cn } from '@/lib/utils';

const typeIcons: Record<NotificationType, React.ElementType> = {
  info: Info,
  success: CheckCircle2,
  warning: AlertTriangle,
  error: AlertCircle,
  in_progress: Loader2,
};

const typeColors: Record<NotificationType, string> = {
  info: 'text-blue-500',
  success: 'text-emerald-500',
  warning: 'text-amber-500',
  error: 'text-red-500',
  in_progress: 'text-primary',
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
  const Icon = typeIcons[notification.type];
  const colorClass = typeColors[notification.type];

  return (
    <div
      className={cn(
        'p-3 border-b border-border last:border-b-0 hover:bg-muted/50 transition-colors cursor-pointer',
        !notification.read && 'bg-primary/5'
      )}
      onClick={onMarkAsRead}
    >
      <div className="flex items-start gap-3">
        {/* Icon */}
        <div className={cn('shrink-0 mt-0.5', colorClass)}>
          {notification.type === 'in_progress' ? (
            <Loader2 className="h-4 w-4 animate-spin" />
          ) : (
            <Icon className="h-4 w-4" />
          )}
        </div>

        {/* Content */}
        <div className="flex-1 min-w-0">
          <div className="flex items-start justify-between gap-2">
            <p className="text-sm font-medium line-clamp-1">{notification.title}</p>
            <Button
              variant="ghost"
              size="icon"
              className="h-5 w-5 shrink-0 opacity-0 group-hover:opacity-100 hover:bg-destructive/10"
              onClick={(e) => {
                e.stopPropagation();
                onDismiss();
              }}
            >
              <X className="h-3 w-3" />
            </Button>
          </div>
          <p className="text-xs text-muted-foreground mt-0.5 line-clamp-2">
            {notification.message}
          </p>

          {/* Progress bar for in-progress items */}
          {notification.type === 'in_progress' && notification.metadata?.progress !== undefined && (
            <div className="mt-2">
              <Progress value={notification.metadata.progress} className="h-1.5" />
              {notification.metadata.totalItems && notification.metadata.completedItems !== undefined && (
                <p className="text-xs text-muted-foreground mt-1">
                  {notification.metadata.completedItems} / {notification.metadata.totalItems} items
                </p>
              )}
            </div>
          )}

          {/* Timestamp */}
          <p className="text-xs text-muted-foreground mt-1">
            {formatNotificationTime(notification.timestamp)}
          </p>
        </div>

        {/* Unread indicator */}
        {!notification.read && (
          <div className="h-2 w-2 rounded-full bg-primary shrink-0 mt-1.5" />
        )}
      </div>
    </div>
  );
}

export function NotificationsPanel() {
  const [open, setOpen] = useState(false);
  const {
    notifications,
    unreadCount,
    isLoading,
    markAsRead,
    markAllAsRead,
    removeNotification,
    clearAll,
  } = useNotifications();

  const inProgressCount = notifications.filter((n) => n.type === 'in_progress').length;

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button variant="ghost" size="icon" className="relative">
          <Bell className="h-5 w-5" />
          {unreadCount > 0 && (
            <Badge
              variant="destructive"
              className="absolute -top-1 -right-1 h-5 min-w-5 px-1 text-xs flex items-center justify-center"
            >
              {unreadCount > 99 ? '99+' : unreadCount}
            </Badge>
          )}
          {inProgressCount > 0 && unreadCount === 0 && (
            <span className="absolute -top-1 -right-1 h-3 w-3 rounded-full bg-primary animate-pulse" />
          )}
        </Button>
      </PopoverTrigger>

      <PopoverContent className="w-80 p-0" align="end" sideOffset={8}>
        {/* Header */}
        <div className="flex items-center justify-between p-3 border-b border-border">
          <div className="flex items-center gap-2">
            <h4 className="font-semibold text-sm">Notifications</h4>
            {unreadCount > 0 && (
              <Badge variant="secondary" className="text-xs">
                {unreadCount} new
              </Badge>
            )}
          </div>
          <div className="flex items-center gap-1">
            {unreadCount > 0 && (
              <Button
                variant="ghost"
                size="sm"
                className="h-7 text-xs"
                onClick={markAllAsRead}
              >
                <CheckCheck className="h-3 w-3 mr-1" />
                Mark all read
              </Button>
            )}
          </div>
        </div>

        {/* Notifications List */}
        {isLoading ? (
          <div className="p-8 text-center">
            <Loader2 className="h-6 w-6 mx-auto mb-2 text-muted-foreground/50 animate-spin" />
            <p className="text-sm text-muted-foreground">Loading notifications...</p>
          </div>
        ) : notifications.length > 0 ? (
          <ScrollArea className="max-h-[400px]">
            <div className="group">
              {notifications.map((notification) => (
                <NotificationItem
                  key={notification.id}
                  notification={notification}
                  onMarkAsRead={() => markAsRead(notification.id)}
                  onDismiss={() => removeNotification(notification.id)}
                />
              ))}
            </div>
          </ScrollArea>
        ) : (
          <div className="p-8 text-center">
            <Bell className="h-8 w-8 mx-auto mb-2 text-muted-foreground/50" />
            <p className="text-sm text-muted-foreground">No notifications</p>
            <p className="text-xs text-muted-foreground mt-1">
              Upload activity will appear here
            </p>
          </div>
        )}

        {/* Footer */}
        {notifications.length > 0 && (
          <div className="p-2 border-t border-border">
            <Button
              variant="ghost"
              size="sm"
              className="w-full h-8 text-xs text-muted-foreground hover:text-destructive"
              onClick={clearAll}
            >
              <Trash2 className="h-3 w-3 mr-1" />
              Clear all notifications
            </Button>
          </div>
        )}
      </PopoverContent>
    </Popover>
  );
}
