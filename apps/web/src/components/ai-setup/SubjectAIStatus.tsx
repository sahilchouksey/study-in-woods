'use client';

import { Badge } from '@/components/ui/badge';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { Bot, Loader2, AlertCircle, CheckCircle2, Clock } from 'lucide-react';
import { AI_SETUP_STATUS_CONFIG, type AISetupStatus } from '@/lib/api/notifications';
import { cn } from '@/lib/utils';

interface SubjectAIStatusProps {
  /**
   * The AI setup status of the subject
   */
  status: AISetupStatus;
  /**
   * Whether to show as a compact badge (default) or expanded
   */
  compact?: boolean;
  /**
   * Additional class names
   */
  className?: string;
  /**
   * Whether to show tooltip
   */
  showTooltip?: boolean;
}

/**
 * Component to display the AI setup status of a subject
 */
export function SubjectAIStatus({ 
  status, 
  compact = true, 
  className,
  showTooltip = true 
}: SubjectAIStatusProps) {
  const config = AI_SETUP_STATUS_CONFIG[status] || AI_SETUP_STATUS_CONFIG.none;
  
  // Get the appropriate icon
  const Icon = getStatusIcon(status);
  
  // For "none" status, we might want to hide the badge entirely in some contexts
  if (status === 'none' && compact) {
    return null;
  }
  
  const badge = (
    <Badge 
      variant="outline" 
      className={cn(
        'gap-1 font-normal',
        config.bgColor,
        config.color,
        className
      )}
    >
      <Icon className={cn(
        'h-3 w-3',
        status === 'in_progress' && 'animate-spin'
      )} />
      {!compact && <span>{config.label}</span>}
    </Badge>
  );
  
  if (!showTooltip) {
    return badge;
  }
  
  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger asChild>
          {badge}
        </TooltipTrigger>
        <TooltipContent>
          <p className="font-medium">{config.label}</p>
          <p className="text-xs text-muted-foreground">{config.description}</p>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}

/**
 * Get the appropriate icon for the status
 */
function getStatusIcon(status: AISetupStatus) {
  switch (status) {
    case 'completed':
      return CheckCircle2;
    case 'in_progress':
      return Loader2;
    case 'pending':
      return Clock;
    case 'failed':
      return AlertCircle;
    case 'none':
    default:
      return Bot;
  }
}

/**
 * Inline AI status indicator for use in lists/cards
 */
export function SubjectAIStatusInline({ 
  status,
  className 
}: { 
  status: AISetupStatus;
  className?: string;
}) {
  const config = AI_SETUP_STATUS_CONFIG[status] || AI_SETUP_STATUS_CONFIG.none;
  const Icon = getStatusIcon(status);
  
  return (
    <div className={cn('flex items-center gap-1.5 text-xs', config.color, className)}>
      <Icon className={cn(
        'h-3.5 w-3.5',
        status === 'in_progress' && 'animate-spin'
      )} />
      <span>{config.label}</span>
    </div>
  );
}

/**
 * AI Ready badge - shows only when AI is ready (completed status)
 */
export function AIReadyBadge({ 
  status,
  className 
}: { 
  status: AISetupStatus;
  className?: string;
}) {
  if (status !== 'completed') {
    return null;
  }
  
  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger asChild>
          <Badge 
            variant="outline" 
            className={cn(
              'gap-1 bg-green-100 text-green-700 border-green-200',
              className
            )}
          >
            <Bot className="h-3 w-3" />
            <span>AI</span>
          </Badge>
        </TooltipTrigger>
        <TooltipContent>
          <p>AI assistant is ready for this subject</p>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}

/**
 * AI Setup In Progress indicator
 */
export function AISetupInProgress({ 
  subjectName,
  className 
}: { 
  subjectName?: string;
  className?: string;
}) {
  return (
    <div className={cn('flex items-center gap-2 text-sm text-blue-600', className)}>
      <Loader2 className="h-4 w-4 animate-spin" />
      <span>
        Setting up AI{subjectName ? ` for ${subjectName}` : ''}...
      </span>
    </div>
  );
}
