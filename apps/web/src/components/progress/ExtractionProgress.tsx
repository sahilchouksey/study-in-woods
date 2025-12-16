'use client';

import { useState, useRef, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { 
  Download, 
  FileSearch, 
  Brain, 
  Merge, 
  Database, 
  CheckCircle2, 
  Loader2,
  XCircle,
  RefreshCw,
  ChevronDown,
  ChevronUp,
  Terminal,
  Clock,
  Zap,
  AlertTriangle
} from 'lucide-react';
import { Progress } from '@/components/ui/progress';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { ScrollArea } from '@/components/ui/scroll-area';
import { cn } from '@/lib/utils';
import type { ProgressEvent } from '@/lib/hooks/useSSE';

interface ExtractionProgressProps {
  progress: number;
  phase: string;
  message: string;
  events: ProgressEvent[];
  latestEvent: ProgressEvent | null;
  isComplete: boolean;
  error: string | null;
  totalChunks?: number;
  completedChunks?: number;
}

// Phase configuration with icons and colors
const phaseConfig: Record<string, { 
  icon: React.ElementType; 
  label: string; 
  color: string;
  bgColor: string;
}> = {
  initializing: { 
    icon: Loader2, 
    label: 'Initializing', 
    color: 'text-blue-500',
    bgColor: 'bg-blue-500/10'
  },
  download: { 
    icon: Download, 
    label: 'Downloading', 
    color: 'text-cyan-500',
    bgColor: 'bg-cyan-500/10'
  },
  chunking: { 
    icon: FileSearch, 
    label: 'Analyzing', 
    color: 'text-purple-500',
    bgColor: 'bg-purple-500/10'
  },
  extraction: { 
    icon: Brain, 
    label: 'AI Processing', 
    color: 'text-amber-500',
    bgColor: 'bg-amber-500/10'
  },
  merge: { 
    icon: Merge, 
    label: 'Merging', 
    color: 'text-indigo-500',
    bgColor: 'bg-indigo-500/10'
  },
  save: { 
    icon: Database, 
    label: 'Saving', 
    color: 'text-green-500',
    bgColor: 'bg-green-500/10'
  },
  complete: { 
    icon: CheckCircle2, 
    label: 'Complete', 
    color: 'text-emerald-500',
    bgColor: 'bg-emerald-500/10'
  },
};

// Step indicator component - improved with circular loading indicator
function StepIndicator({ 
  phase, 
  currentPhase,
  progress 
}: { 
  phase: string; 
  currentPhase: string;
  progress: number;
}) {
  const config = phaseConfig[phase];
  if (!config) return null;

  const phases = Object.keys(phaseConfig);
  const currentIndex = phases.indexOf(currentPhase);
  const thisIndex = phases.indexOf(phase);
  
  // If no current phase or progress is 0, treat all as pending
  const hasStarted = currentPhase && currentIndex >= 0 && progress > 0;
  
  const isActive = hasStarted && phase === currentPhase;
  const isComplete = hasStarted && (thisIndex < currentIndex || (phase === 'complete' && progress === 100));
  const isPending = !hasStarted || thisIndex > currentIndex;

  const Icon = config.icon;

  return (
    <div
      className={cn(
        "flex flex-col items-center gap-1 relative z-10",
        isPending && "opacity-40"
      )}
    >
      <div className="relative w-10 h-10 flex items-center justify-center overflow-visible">
        {/* Circular loading indicator for active step - rotates around the circle */}
        {isActive && (
          <motion.svg 
            className="absolute inset-0 w-10 h-10"
            viewBox="0 0 40 40"
            animate={{ rotate: 360 }}
            transition={{ duration: 2, repeat: Infinity, ease: "linear" }}
          >
            <circle
              cx="20"
              cy="20"
              r="16"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              className={config.color}
              strokeDasharray="15 85"
              style={{ transform: 'rotate(-90deg)', transformOrigin: '20px 20px' }}
            />
          </motion.svg>
        )}
        
        {/* Step circle - always solid background */}
        <div
          className={cn(
            "w-8 h-8 rounded-full flex items-center justify-center transition-all duration-300 relative z-10",
            "border-2",
            isComplete && "!bg-emerald-500 !border-emerald-500 text-white shadow-lg shadow-emerald-500/50",
            isActive && cn("shadow-md border-current bg-background", config.color),
            isPending && "!bg-muted/80 dark:!bg-muted border-border"
          )}
        >
          {isComplete ? (
            <CheckCircle2 className="w-4 h-4" />
          ) : (
            <Icon className={cn("w-4 h-4", isActive ? config.color : "text-muted-foreground")} />
          )}
        </div>
      </div>
      
      <span className={cn(
        "text-[10px] font-medium whitespace-nowrap transition-colors duration-300",
        isActive && config.color,
        isComplete && "text-emerald-500",
        isPending && "text-muted-foreground"
      )}>
        {config.label}
      </span>
    </div>
  );
}

// Chunk progress visualization - compact
function ChunkProgress({ 
  totalChunks, 
  completedChunks 
}: { 
  totalChunks: number; 
  completedChunks: number;
}) {
  return (
    <div className="flex gap-0.5 flex-wrap justify-center">
      {Array.from({ length: totalChunks }).map((_, i) => (
        <motion.div
          key={i}
          className={cn(
            "w-2.5 h-2.5 rounded-sm",
            i < completedChunks 
              ? "bg-amber-500" 
              : "bg-muted border border-border"
          )}
          initial={{ scale: 0 }}
          animate={{ scale: 1 }}
          transition={{ delay: i * 0.03 }}
        />
      ))}
    </div>
  );
}

// Animated dots for loading states
function LoadingDots() {
  return (
    <span className="inline-flex gap-1 ml-1">
      {[0, 1, 2].map((i) => (
        <motion.span
          key={i}
          className="w-1 h-1 bg-current rounded-full"
          animate={{ opacity: [0.3, 1, 0.3] }}
          transition={{
            duration: 1,
            repeat: Infinity,
            delay: i * 0.2,
          }}
        />
      ))}
    </span>
  );
}

// Event type configuration for logs - theme-compatible colors
const eventTypeConfig: Record<string, {
  label: string;
  // Using semantic colors that work in both light and dark themes
  colorClass: string;
  bgClass: string;
  icon: React.ElementType;
}> = {
  started: {
    label: 'START',
    colorClass: 'text-blue-600 dark:text-blue-400',
    bgClass: 'bg-blue-100 dark:bg-blue-500/20',
    icon: Zap,
  },
  progress: {
    label: 'PROG',
    colorClass: 'text-sky-600 dark:text-sky-400',
    bgClass: 'bg-sky-100 dark:bg-sky-500/20',
    icon: Loader2,
  },
  info: {
    label: 'INFO',
    colorClass: 'text-emerald-600 dark:text-emerald-400',
    bgClass: 'bg-emerald-100 dark:bg-emerald-500/20',
    icon: CheckCircle2,
  },
  debug: {
    label: 'DEBUG',
    colorClass: 'text-slate-500 dark:text-slate-400',
    bgClass: 'bg-slate-100 dark:bg-slate-500/20',
    icon: Terminal,
  },
  warning: {
    label: 'WARN',
    colorClass: 'text-amber-600 dark:text-amber-400',
    bgClass: 'bg-amber-100 dark:bg-amber-500/20',
    icon: AlertTriangle,
  },
  error: {
    label: 'ERROR',
    colorClass: 'text-red-600 dark:text-red-400',
    bgClass: 'bg-red-100 dark:bg-red-500/20',
    icon: XCircle,
  },
  complete: {
    label: 'DONE',
    colorClass: 'text-green-600 dark:text-green-400',
    bgClass: 'bg-green-100 dark:bg-green-500/20',
    icon: CheckCircle2,
  },
};

// Format timestamp for logs - shorter format
function formatLogTime(timestamp: string): string {
  try {
    const date = new Date(timestamp);
    return date.toLocaleTimeString('en-US', {
      hour12: false,
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    });
  } catch {
    return '--:--:--';
  }
}

// Single log entry component - compact design with proper formatting
function LogEntry({ event }: { event: ProgressEvent }) {
  const config = eventTypeConfig[event.type] || eventTypeConfig.info;

  return (
    <div className="flex items-start gap-1.5 py-0.5 px-2 hover:bg-muted/50 text-[11px] font-mono leading-relaxed whitespace-nowrap">
      {/* Timestamp */}
      <span className="text-muted-foreground/70 shrink-0 tabular-nums text-[10px]">
        {formatLogTime(event.timestamp)}
      </span>

      {/* Event type badge */}
      <span className={cn(
        "px-1 py-px rounded text-[9px] font-semibold shrink-0 uppercase min-w-[38px] text-center",
        config.bgClass,
        config.colorClass
      )}>
        {config.label}
      </span>

      {/* Message and details - nowrap for horizontal scroll */}
      <div className="shrink-0">
        <span className="text-foreground/90">{event.message}</span>
        {event.detail && (
          <span className="text-muted-foreground/70 ml-1 text-[10px]">
            — {event.detail}
          </span>
        )}
        {event.page_range && !event.detail && (
          <span className="text-muted-foreground/70 ml-1 text-[10px]">
            (pages {event.page_range})
          </span>
        )}
        {event.duration && !event.detail && (
          <span className="text-muted-foreground/70 ml-1 text-[10px]">
            [{event.duration}]
          </span>
        )}
        {event.subjects_found !== undefined && event.subjects_found > 0 && (
          <span className="text-emerald-600 dark:text-emerald-400 ml-1 text-[10px]">
            (+{event.subjects_found} subjects)
          </span>
        )}
        {event.error_message && (
          <span className="text-red-600 dark:text-red-400 block mt-0.5 text-[10px]">
            Error: {event.error_message}
          </span>
        )}
      </div>
    </div>
  );
}

// Event logs panel component - theme-compatible design
function EventLogs({ 
  events, 
  showDebug = false,
}: { 
  events: ProgressEvent[];
  showDebug?: boolean;
}) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const [isAutoScroll, setIsAutoScroll] = useState(true);

  // Filter events based on showDebug
  const filteredEvents = showDebug 
    ? events 
    : events.filter(e => e.type !== 'debug');

  // Auto-scroll to bottom when new events arrive
  useEffect(() => {
    if (isAutoScroll && scrollRef.current) {
      const scrollElement = scrollRef.current;
      scrollElement.scrollTop = scrollElement.scrollHeight;
    }
  }, [filteredEvents.length, isAutoScroll]);

  // Handle scroll to detect user scrolling up
  const handleScroll = () => {
    if (!scrollRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = scrollRef.current;
    const isAtBottom = scrollHeight - scrollTop - clientHeight < 30;
    setIsAutoScroll(isAtBottom);
  };

  return (
    <div className="rounded-md border bg-muted/30 dark:bg-muted/10 overflow-hidden">
      {/* Log content - fixed height with scroll (both axes) */}
      <div 
        ref={scrollRef}
        onScroll={handleScroll}
        className="h-[160px] overflow-auto"
      >
        {filteredEvents.length === 0 ? (
          <div className="flex items-center justify-center h-full text-muted-foreground text-xs">
            <Loader2 className="w-3 h-3 animate-spin mr-1.5" />
            Waiting for events...
          </div>
        ) : (
          <div className="py-1">
            {filteredEvents.map((event, i) => (
              <LogEntry key={`${event.timestamp}-${i}`} event={event} />
            ))}
          </div>
        )}
      </div>

      {/* Minimal status bar */}
      <div className="flex items-center justify-between px-2 py-1 border-t bg-muted/50 dark:bg-muted/20 text-[10px] text-muted-foreground">
        <span className="flex items-center gap-1">
          <span className={cn(
            "w-1.5 h-1.5 rounded-full",
            isAutoScroll ? "bg-emerald-500 animate-pulse" : "bg-amber-500"
          )} />
          {isAutoScroll ? 'Live' : 'Scrolled'}
        </span>
        <span>{filteredEvents.length} events</span>
      </div>
    </div>
  );
}

// Main progress component
export function ExtractionProgress({
  progress,
  phase,
  message,
  events,
  latestEvent,
  isComplete,
  error,
  totalChunks = 0,
  completedChunks = 0,
}: ExtractionProgressProps) {
  // Show logs expanded by default, and include debug events by default for full detail
  const [showLogs, setShowLogs] = useState(true);
  const [showDebugLogs, setShowDebugLogs] = useState(true);
  
  // Preserve chunk info in state to prevent disappearing when phase changes
  const [chunkInfo, setChunkInfo] = useState({ total: 0, completed: 0 });
  
  // Update chunk info when we receive chunk data from events
  useEffect(() => {
    if (totalChunks > 0) {
      setChunkInfo({
        total: totalChunks,
        completed: completedChunks || 0
      });
    }
  }, [totalChunks, completedChunks]);
  
  const currentConfig = phaseConfig[phase] || phaseConfig.initializing;
  const Icon = currentConfig.icon;

  // Get warning events for display
  const warningEvents = events.filter(e => e.type === 'warning').slice(-3);

  return (
    <div className="w-full space-y-4 overflow-hidden">
      {/* Main progress bar */}
      <div className="space-y-2 overflow-hidden">
        <div className="flex items-center justify-between gap-2">
          <div className="flex items-center gap-2 min-w-0 flex-1 overflow-hidden">
            <div className="shrink-0">
              <Icon className={cn("w-4 h-4", currentConfig.color)} />
            </div>
            <span className="font-medium text-sm truncate block max-w-full" title={message}>
              {message}
              {!isComplete && !error && <LoadingDots />}
            </span>
          </div>
          <Badge variant="outline" className={cn("font-mono text-xs shrink-0", currentConfig.color)}>
            {progress}%
          </Badge>
        </div>

        {/* Animated progress bar */}
        <div className="relative overflow-hidden rounded-full">
          <Progress value={progress} className="h-1.5" />
          <motion.div
            className="absolute top-0 left-0 h-1.5 bg-gradient-to-r from-transparent via-white/30 to-transparent rounded-full"
            style={{ width: '30%' }}
            animate={!isComplete && !error ? { x: ['-100%', '300%'] } : {}}
            transition={{ duration: 1.5, repeat: Infinity, ease: "linear" }}
          />
        </div>
      </div>

      {/* Phase steps indicator - improved with edge-to-edge connecting lines */}
      <div className="relative py-2">
        {/* Connecting lines - positioned absolutely behind circles */}
        <div className="absolute left-0 right-0 flex items-center justify-between px-9 -z-10" style={{ top: '25px' }}>
          {['download', 'chunking', 'extraction', 'merge'].map((p, i) => {
            const phases = ['download', 'chunking', 'extraction', 'merge', 'save'];
            const currentIndex = phases.indexOf(phase);
            const hasStarted = phase && currentIndex >= 0 && progress > 0;
            // Line is completed if we've reached or passed the step AFTER this line
            // e.g., line 0 (download->chunking) is green when currentIndex >= 1
            const lineCompleted = hasStarted && currentIndex > i;
            
            return (
              <div key={`line-${p}`} className="flex-1 h-0.5 mx-px first:ml-0 last:mr-0">
                <motion.div 
                  className={cn(
                    "h-full transition-colors duration-500",
                    lineCompleted ? "bg-emerald-500" : "bg-border"
                  )}
                  initial={{ scaleX: 0 }}
                  animate={{ scaleX: 1 }}
                  transition={{ duration: 0.3, delay: i * 0.1 }}
                  style={{ transformOrigin: 'left' }}
                />
              </div>
            );
          })}
        </div>
        
        {/* Step indicators - on top of lines */}
        <div className="relative flex items-center justify-between px-4">
          {['download', 'chunking', 'extraction', 'merge', 'save'].map((p, i) => (
            <StepIndicator key={p} phase={p} currentPhase={phase} progress={progress} />
          ))}
        </div>
      </div>

      {/* Chunk progress (when in extraction phase) - more compact */}
      <AnimatePresence>
        {phase === 'extraction' && chunkInfo.total > 0 && progress < 70 && (
          <motion.div
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: 'auto' }}
            exit={{ opacity: 0, height: 0 }}
            className="bg-muted/30 rounded-md p-3"
          >
            <div className="flex items-center justify-between mb-1.5">
              <span className="text-xs font-medium flex items-center gap-1.5">
                <Brain className="w-3.5 h-3.5 text-amber-500" />
                Processing Chunks
              </span>
              <span className="text-xs text-muted-foreground">
                {chunkInfo.completed}/{chunkInfo.total}
              </span>
            </div>
            <ChunkProgress totalChunks={chunkInfo.total} completedChunks={chunkInfo.completed} />
          </motion.div>
        )}
      </AnimatePresence>

      {/* Warning events - compact */}
      <AnimatePresence>
        {warningEvents.length > 0 && (
          <motion.div
            initial={{ opacity: 0, y: 5 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -5 }}
            className="space-y-1.5"
          >
            {warningEvents.map((event, i) => (
              <motion.div
                key={i}
                initial={{ opacity: 0, x: -10 }}
                animate={{ opacity: 1, x: 0 }}
                className="flex items-start gap-2 text-xs bg-amber-500/10 border border-amber-500/20 rounded-md p-2 overflow-hidden"
              >
                <RefreshCw className="w-3.5 h-3.5 text-amber-500 mt-0.5 shrink-0 animate-spin" />
                <div className="flex-1 min-w-0 overflow-hidden">
                  <p className="text-amber-600 dark:text-amber-400 font-medium truncate" title={event.message}>
                    {event.message}
                  </p>
                  {event.retry_count && event.max_retries && (
                    <p className="text-[10px] text-muted-foreground mt-0.5">
                      Retry {event.retry_count}/{event.max_retries}
                    </p>
                  )}
                </div>
              </motion.div>
            ))}
          </motion.div>
        )}
      </AnimatePresence>

      {/* Error state - compact */}
      <AnimatePresence>
        {error && (
          <motion.div
            initial={{ opacity: 0, scale: 0.98 }}
            animate={{ opacity: 1, scale: 1 }}
            exit={{ opacity: 0, scale: 0.98 }}
            className="flex items-start gap-2 bg-destructive/10 border border-destructive/20 rounded-md p-2.5"
          >
            <XCircle className="w-4 h-4 text-destructive shrink-0 mt-0.5" />
            <div className="min-w-0">
              <p className="text-sm font-medium text-destructive">Extraction Failed</p>
              <p className="text-xs text-muted-foreground mt-0.5 break-words">{error}</p>
            </div>
          </motion.div>
        )}
      </AnimatePresence>

      {/* Success state - compact */}
      <AnimatePresence>
        {isComplete && !error && (
          <motion.div
            initial={{ opacity: 0, scale: 0.98 }}
            animate={{ opacity: 1, scale: 1 }}
            exit={{ opacity: 0, scale: 0.98 }}
            className="flex items-center gap-2 bg-emerald-500/10 border border-emerald-500/20 rounded-md p-2.5"
          >
            <motion.div
              initial={{ scale: 0 }}
              animate={{ scale: 1 }}
              transition={{ type: "spring", stiffness: 200, delay: 0.1 }}
            >
              <CheckCircle2 className="w-5 h-5 text-emerald-500" />
            </motion.div>
            <div>
              <p className="text-sm font-medium text-emerald-600 dark:text-emerald-400">
                Extraction Complete!
              </p>
              <p className="text-xs text-muted-foreground">
                {latestEvent?.result_syllabus_ids?.length || 0} subjects extracted
              </p>
            </div>
          </motion.div>
        )}
      </AnimatePresence>

      {/* Collapsible Logs Section */}
      <div className="space-y-1.5">
        <button
          type="button"
          className="w-full flex items-center justify-between py-1.5 px-2 rounded-md text-xs text-muted-foreground hover:text-foreground hover:bg-muted/50 transition-colors"
          onClick={() => setShowLogs(!showLogs)}
        >
          <span className="flex items-center gap-1.5">
            <Terminal className="w-3.5 h-3.5" />
            <span>{showLogs ? 'Hide' : 'Show'} Logs</span>
            <span className="text-[10px] text-muted-foreground/70">
              ({events.length})
            </span>
          </span>
          {showLogs ? (
            <ChevronUp className="w-3.5 h-3.5" />
          ) : (
            <ChevronDown className="w-3.5 h-3.5" />
          )}
        </button>

        <AnimatePresence>
          {showLogs && (
            <motion.div
              initial={{ opacity: 0, height: 0 }}
              animate={{ opacity: 1, height: 'auto' }}
              exit={{ opacity: 0, height: 0 }}
              transition={{ duration: 0.15 }}
              className="overflow-hidden"
            >
              {/* Header with debug toggle */}
              <div className="flex items-center justify-between mb-1.5">
                <span className="text-[10px] text-muted-foreground">
                  Extraction activity
                </span>
                <button
                  type="button"
                  className={cn(
                    "text-[10px] px-1.5 py-0.5 rounded transition-colors",
                    showDebugLogs 
                      ? "bg-muted text-foreground" 
                      : "text-muted-foreground hover:text-foreground"
                  )}
                  onClick={() => setShowDebugLogs(!showDebugLogs)}
                >
                  {showDebugLogs ? 'Hide Debug' : 'Show Debug'}
                </button>
              </div>
              
              <EventLogs 
                events={events} 
                showDebug={showDebugLogs}
              />
            </motion.div>
          )}
        </AnimatePresence>
      </div>
    </div>
  );
}

// Compact version for smaller spaces
export function ExtractionProgressCompact({
  progress,
  phase,
  message,
  isComplete,
  error,
}: Pick<ExtractionProgressProps, 'progress' | 'phase' | 'message' | 'isComplete' | 'error'>) {
  const currentConfig = phaseConfig[phase] || phaseConfig.initializing;
  const Icon = currentConfig.icon;

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-2">
        {error ? (
          <XCircle className="w-4 h-4 text-destructive" />
        ) : isComplete ? (
          <CheckCircle2 className="w-4 h-4 text-emerald-500" />
        ) : (
          <motion.div
            animate={{ rotate: 360 }}
            transition={{ duration: 2, repeat: Infinity, ease: "linear" }}
          >
            <Icon className={cn("w-4 h-4", currentConfig.color)} />
          </motion.div>
        )}
        <span className="text-sm flex-1 truncate">{message}</span>
        <span className="text-xs font-mono text-muted-foreground">{progress}%</span>
      </div>
      <Progress value={progress} className="h-1" />
    </div>
  );
}
