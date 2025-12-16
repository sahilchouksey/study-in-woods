import { cn } from '@/lib/utils';
import { Loader2 } from 'lucide-react';

interface LoadingSpinnerProps {
  /** Size of the spinner */
  size?: 'xs' | 'sm' | 'md' | 'lg' | 'xl';
  /** Optional className for additional styling */
  className?: string;
  /** Optional text to display below the spinner */
  text?: string;
  /** Whether to center the spinner in its container */
  centered?: boolean;
  /** Add padding when centered */
  withPadding?: boolean;
}

const sizeClasses = {
  xs: 'h-3 w-3',
  sm: 'h-4 w-4',
  md: 'h-6 w-6',
  lg: 'h-8 w-8',
  xl: 'h-12 w-12',
};

/**
 * Consistent loading spinner component for the app.
 * Uses Lucide Loader2 icon with proper dark/light mode colors.
 */
export function LoadingSpinner({
  size = 'md',
  className,
  text,
  centered = false,
  withPadding = false,
}: LoadingSpinnerProps) {
  const spinner = (
    <Loader2
      className={cn(
        'animate-spin text-muted-foreground',
        sizeClasses[size],
        className
      )}
    />
  );

  if (!centered && !text) {
    return spinner;
  }

  return (
    <div
      className={cn(
        'flex flex-col items-center justify-center gap-3',
        centered && 'w-full',
        withPadding && 'py-12'
      )}
    >
      {spinner}
      {text && (
        <p className="text-sm text-muted-foreground">{text}</p>
      )}
    </div>
  );
}

/**
 * Full page loading state
 */
export function PageLoader({ text = 'Loading...' }: { text?: string }) {
  return (
    <div className="flex items-center justify-center min-h-[400px] w-full">
      <LoadingSpinner size="xl" text={text} centered />
    </div>
  );
}

/**
 * Inline loading spinner for buttons and small areas
 */
export function InlineSpinner({ className }: { className?: string }) {
  return <LoadingSpinner size="sm" className={className} />;
}

/**
 * Loading skeleton for content areas
 */
export function LoadingSkeleton({
  className,
  lines = 3,
}: {
  className?: string;
  lines?: number;
}) {
  return (
    <div className={cn('space-y-3 animate-pulse', className)}>
      {Array.from({ length: lines }).map((_, i) => (
        <div
          key={i}
          className={cn(
            'h-4 bg-muted rounded',
            i === lines - 1 ? 'w-3/4' : 'w-full'
          )}
        />
      ))}
    </div>
  );
}
