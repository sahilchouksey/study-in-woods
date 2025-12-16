'use client';

import { useState, useRef, useCallback, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { MessageSquare, Calendar, AlertCircle, BookOpen, Trash2 } from 'lucide-react';
import { LoadingSpinner, InlineSpinner } from '@/components/ui/loading-spinner';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog';
import { useInfiniteChatHistory } from '@/lib/api/hooks/useChat';
import { chatService } from '@/lib/api/chat';
import { useQueryClient } from '@tanstack/react-query';
import { showSuccessToast, showErrorToast } from '@/lib/utils/errors';
import type { ChatSession } from '@/lib/api/chat';

/**
 * Format a date string to a readable format
 */
function formatDate(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const diffDays = Math.floor((now.getTime() - date.getTime()) / (1000 * 60 * 60 * 24));
  
  if (diffDays === 0) {
    return 'Today';
  } else if (diffDays === 1) {
    return 'Yesterday';
  } else if (diffDays < 7) {
    return `${diffDays} days ago`;
  } else {
    return date.toLocaleDateString('en-US', { 
      month: 'short', 
      day: 'numeric',
      year: date.getFullYear() !== now.getFullYear() ? 'numeric' : undefined 
    });
  }
}

/**
 * Get status badge variant
 */
function getStatusVariant(status: string): 'default' | 'secondary' | 'outline' {
  switch (status) {
    case 'active':
      return 'default';
    case 'archived':
      return 'secondary';
    default:
      return 'outline';
  }
}

interface SessionCardProps {
  session: ChatSession;
  onClick: () => void;
  onDelete: () => void;
  isDeleting: boolean;
}

function SessionCard({ session, onClick, onDelete, isDeleting }: SessionCardProps) {
  return (
    <div className="border border-border rounded-lg p-4 hover:bg-muted/50 transition-colors group">
      <div className="flex items-start justify-between mb-3">
        <div 
          className="flex-1 cursor-pointer"
          onClick={onClick}
        >
          <h3 className="font-medium text-foreground line-clamp-2">
            {session.title || 'Untitled Chat'}
          </h3>
        </div>
        <div className="flex items-center gap-2 shrink-0 ml-2">
          <div className="flex items-center gap-1 text-sm text-muted-foreground">
            <Calendar className="h-4 w-4" />
            {formatDate(session.last_message_at || session.created_at)}
          </div>
          <AlertDialog>
            <AlertDialogTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="h-8 w-8 opacity-0 group-hover:opacity-100 transition-opacity text-muted-foreground hover:text-destructive"
                onClick={(e) => e.stopPropagation()}
                disabled={isDeleting}
              >
                {isDeleting ? (
                  <InlineSpinner />
                ) : (
                  <Trash2 className="h-4 w-4" />
                )}
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent onClick={(e: React.MouseEvent) => e.stopPropagation()}>
              <AlertDialogHeader>
                <AlertDialogTitle>Delete conversation?</AlertDialogTitle>
                <AlertDialogDescription>
                  This will permanently delete this chat session and all its messages. 
                  This action cannot be undone.
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>Cancel</AlertDialogCancel>
                <AlertDialogAction
                  onClick={onDelete}
                  className="bg-destructive text-white hover:bg-destructive/90"
                >
                  Delete
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </div>
      </div>

      <div 
        className="cursor-pointer"
        onClick={onClick}
      >
        <div className="flex items-center gap-4 mb-3 text-sm text-muted-foreground">
          <div className="flex items-center gap-1">
            <MessageSquare className="h-4 w-4" />
            {session.message_count} messages
          </div>
          <Badge variant={getStatusVariant(session.status)} className="text-xs">
            {session.status}
          </Badge>
        </div>

        {session.subject && (
          <div className="flex items-center gap-2">
            <BookOpen className="h-4 w-4 text-muted-foreground" />
            <Badge variant="outline" className="text-xs">
              {session.subject.code}
            </Badge>
            <span className="text-sm text-muted-foreground truncate">
              {session.subject.name}
            </span>
          </div>
        )}

        {session.description && (
          <p className="text-sm text-muted-foreground mt-2 line-clamp-2">
            {session.description}
          </p>
        )}
      </div>
    </div>
  );
}

export function HistoryTab() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const [searchQuery, setSearchQuery] = useState('');
  const [deletingId, setDeletingId] = useState<number | null>(null);
  const loadMoreRef = useRef<HTMLDivElement>(null);
  const scrollContainerRef = useRef<HTMLDivElement>(null);
  
  const {
    data,
    isLoading,
    isError,
    error,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useInfiniteChatHistory(20);

  // Intersection Observer for infinite scroll
  const handleObserver = useCallback(
    (entries: IntersectionObserverEntry[]) => {
      const [target] = entries;
      if (target.isIntersecting && hasNextPage && !isFetchingNextPage) {
        fetchNextPage();
      }
    },
    [fetchNextPage, hasNextPage, isFetchingNextPage]
  );

  useEffect(() => {
    const element = loadMoreRef.current;
    const container = scrollContainerRef.current;
    if (!element || !container) return;

    const observer = new IntersectionObserver(handleObserver, {
      root: container,
      rootMargin: '100px',
      threshold: 0,
    });

    observer.observe(element);

    return () => {
      observer.disconnect();
    };
  }, [handleObserver]);

  // Flatten pages into single array
  const allSessions = data?.pages.flatMap((page) => page.sessions) ?? [];

  // Filter sessions by search query (client-side for now)
  const filteredSessions = searchQuery
    ? allSessions.filter((session) =>
        session.title?.toLowerCase().includes(searchQuery.toLowerCase()) ||
        session.subject?.name?.toLowerCase().includes(searchQuery.toLowerCase()) ||
        session.subject?.code?.toLowerCase().includes(searchQuery.toLowerCase()) ||
        session.description?.toLowerCase().includes(searchQuery.toLowerCase())
      )
    : allSessions;

  // Handle session click - navigate to chat
  const handleSessionClick = (session: ChatSession) => {
    router.push(`/chat/${session.id}`);
  };

  // Handle delete session
  const handleDeleteSession = async (sessionId: number) => {
    setDeletingId(sessionId);
    try {
      await chatService.deleteSession(String(sessionId));
      // Invalidate queries
      queryClient.invalidateQueries({ queryKey: ['chat', 'history'] });
      queryClient.invalidateQueries({ queryKey: ['chat', 'sessions'] });
      queryClient.removeQueries({ queryKey: ['chat', 'session', String(sessionId)] });
      queryClient.removeQueries({ queryKey: ['chat', 'messages', String(sessionId)] });
      showSuccessToast('Chat session deleted');
    } catch (err) {
      showErrorToast(err, 'Failed to delete session');
    } finally {
      setDeletingId(null);
    }
  };

  // Get total count from first page
  const totalCount = data?.pages[0]?.total_count ?? 0;

  return (
    <div className="flex flex-col h-full min-h-0">
      <div className="border-b border-border p-6 shrink-0">
        <h2 className="text-foreground text-lg font-semibold">Chat History</h2>
        <p className="text-muted-foreground mt-1">
          Review your previous conversations
          {totalCount > 0 && (
            <span className="text-muted-foreground/70"> ({totalCount} total)</span>
          )}
        </p>
      </div>

      <div className="p-6 border-b border-border shrink-0">
        <Input
          placeholder="Search conversations..."
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          className="w-full"
        />
      </div>

      <div 
        ref={scrollContainerRef}
        className="flex-1 overflow-y-auto min-h-0"
      >
        <div className="p-6 space-y-4">
          {/* Loading state */}
          {isLoading && (
            <LoadingSpinner size="lg" centered withPadding />
          )}

          {/* Error state */}
          {isError && (
            <div className="flex flex-col items-center justify-center py-12 text-center">
              <AlertCircle className="h-12 w-12 text-destructive mb-4" />
              <h3 className="text-lg font-medium text-foreground mb-2">
                Failed to load history
              </h3>
              <p className="text-muted-foreground">
                {error instanceof Error ? error.message : 'An error occurred'}
              </p>
            </div>
          )}

          {/* Empty state */}
          {!isLoading && !isError && filteredSessions.length === 0 && (
            <div className="flex flex-col items-center justify-center py-12 text-center">
              <MessageSquare className="h-12 w-12 text-muted-foreground/50 mb-4" />
              <h3 className="text-lg font-medium text-foreground mb-2">
                {searchQuery ? 'No matching conversations' : 'No conversations yet'}
              </h3>
              <p className="text-muted-foreground">
                {searchQuery
                  ? 'Try adjusting your search query'
                  : 'Start a new chat to begin your learning journey'}
              </p>
            </div>
          )}

          {/* Session list */}
          {filteredSessions.map((session) => (
            <SessionCard
              key={session.id}
              session={session}
              onClick={() => handleSessionClick(session)}
              onDelete={() => handleDeleteSession(session.id)}
              isDeleting={deletingId === session.id}
            />
          ))}

          {/* Load more trigger */}
          {hasNextPage && (
            <div ref={loadMoreRef} className="flex justify-center py-4">
              {isFetchingNextPage ? (
                <div className="flex items-center gap-2 text-muted-foreground">
                  <InlineSpinner />
                  Loading more...
                </div>
              ) : (
                <span className="text-muted-foreground text-sm">
                  Scroll for more
                </span>
              )}
            </div>
          )}

          {/* End of list indicator */}
          {!hasNextPage && filteredSessions.length > 0 && (
            <div className="flex justify-center py-4">
              <span className="text-muted-foreground/50 text-sm">
                End of history
              </span>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
