"use client";

import { useControllableState } from "@radix-ui/react-use-controllable-state";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import { cn } from "@/lib/utils";
import { 
  ChevronDownIcon, 
  SearchIcon, 
  GlobeIcon, 
  CheckCircle2Icon, 
  LoaderIcon,
  AlertCircleIcon,
  WrenchIcon,
} from "lucide-react";
import type { ComponentProps, ReactNode } from "react";
import { createContext, memo, useContext, useState, useEffect } from "react";
import { Badge } from "@/components/ui/badge";
import { Shimmer } from "./shimmer";

// Tool state types matching our backend
export type ToolState = "pending" | "running" | "completed" | "error";

// Tool event from streaming API
export interface ToolEvent {
  type: "tool_start" | "tool_end" | "tool_error";
  tool_name: string;
  arguments?: Record<string, unknown>;
  result?: unknown;
  success?: boolean;
  error?: string;
}

// Aggregated tool call for display
export interface ToolCall {
  id: string;
  toolName: string;
  state: ToolState;
  arguments?: Record<string, unknown>;
  result?: unknown;
  error?: string;
  startTime?: number;
  endTime?: number;
}

type ToolContextValue = {
  state: ToolState;
  isOpen: boolean;
  setIsOpen: (open: boolean) => void;
  duration: number | undefined;
};

const ToolContext = createContext<ToolContextValue | null>(null);

export const useTool = () => {
  const context = useContext(ToolContext);
  if (!context) {
    throw new Error("Tool components must be used within Tool");
  }
  return context;
};

// Get icon based on tool name
function getToolIcon(toolName: string) {
  switch (toolName) {
    case "web_search":
      return SearchIcon;
    case "scrape_url":
    case "scrape_website":
      return GlobeIcon;
    default:
      return WrenchIcon;
  }
}

// Get human-readable tool name
function getToolDisplayName(toolName: string): string {
  switch (toolName) {
    case "web_search":
      return "Web Search";
    case "scrape_url":
    case "scrape_website":
      return "Web Scraper";
    default:
      return toolName.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
  }
}

// Get state icon
function getStateIcon(state: ToolState) {
  switch (state) {
    case "pending":
    case "running":
      return LoaderIcon;
    case "completed":
      return CheckCircle2Icon;
    case "error":
      return AlertCircleIcon;
  }
}

// Get state badge variant
function getStateBadgeVariant(state: ToolState): "default" | "secondary" | "destructive" | "outline" {
  switch (state) {
    case "pending":
    case "running":
      return "secondary";
    case "completed":
      return "default";
    case "error":
      return "destructive";
  }
}

// Get state label
function getStateLabel(state: ToolState): string {
  switch (state) {
    case "pending":
      return "Pending";
    case "running":
      return "Running";
    case "completed":
      return "Completed";
    case "error":
      return "Error";
  }
}

export type ToolProps = ComponentProps<typeof Collapsible> & {
  state?: ToolState;
  open?: boolean;
  defaultOpen?: boolean;
  onOpenChange?: (open: boolean) => void;
  duration?: number;
};

const AUTO_CLOSE_DELAY = 2000; // 2 seconds after completion
const MS_IN_S = 1000;

export const Tool = memo(
  ({
    className,
    state = "pending",
    open,
    defaultOpen,
    onOpenChange,
    duration: durationProp,
    children,
    ...props
  }: ToolProps) => {
    // Default to open when running, closed when completed (after delay)
    const initialOpen = defaultOpen ?? (state === "running" || state === "pending");
    
    const [isOpen, setIsOpen] = useControllableState({
      prop: open,
      defaultProp: initialOpen,
      onChange: onOpenChange,
    });
    const [duration, setDuration] = useControllableState({
      prop: durationProp,
      defaultProp: undefined,
    });

    const [hasAutoClosed, setHasAutoClosed] = useState(false);
    const [startTime, setStartTime] = useState<number | null>(null);

    // Track duration when tool starts and ends
    useEffect(() => {
      if (state === "running" || state === "pending") {
        if (startTime === null) {
          setStartTime(Date.now());
        }
      } else if (startTime !== null && (state === "completed" || state === "error")) {
        setDuration(Math.ceil((Date.now() - startTime) / MS_IN_S));
        setStartTime(null);
      }
    }, [state, startTime, setDuration]);

    // Auto-open when running, auto-close after completion
    useEffect(() => {
      // Open when running
      if (state === "running" || state === "pending") {
        setIsOpen(true);
        setHasAutoClosed(false);
      }
      // Auto-close after completion
      else if ((state === "completed" || state === "error") && isOpen && !hasAutoClosed) {
        const timer = setTimeout(() => {
          setIsOpen(false);
          setHasAutoClosed(true);
        }, AUTO_CLOSE_DELAY);

        return () => clearTimeout(timer);
      }
    }, [state, isOpen, setIsOpen, hasAutoClosed]);

    const handleOpenChange = (newOpen: boolean) => {
      setIsOpen(newOpen);
    };

    return (
      <ToolContext.Provider value={{ state, isOpen, setIsOpen, duration }}>
        <Collapsible
          className={cn("not-prose mb-3", className)}
          onOpenChange={handleOpenChange}
          open={isOpen}
          {...props}
        >
          {children}
        </Collapsible>
      </ToolContext.Provider>
    );
  }
);

export type ToolHeaderProps = ComponentProps<typeof CollapsibleTrigger> & {
  toolName: string;
  getStatusMessage?: (state: ToolState, duration?: number, toolName?: string) => ReactNode;
};

const defaultGetStatusMessage = (state: ToolState, duration?: number, toolName?: string) => {
  const displayName = toolName ? getToolDisplayName(toolName) : "Tool";
  
  if (state === "running" || state === "pending") {
    return <Shimmer duration={1}>{`Using ${displayName}...`}</Shimmer>;
  }
  if (state === "error") {
    return <span className="text-destructive">{displayName} failed</span>;
  }
  if (duration === undefined || duration === 0) {
    return <span>{displayName} completed</span>;
  }
  return <span>{displayName} completed in {duration}s</span>;
};

export const ToolHeader = memo(
  ({ className, toolName, getStatusMessage = defaultGetStatusMessage, ...props }: ToolHeaderProps) => {
    const { state, isOpen, duration } = useTool();
    const ToolIcon = getToolIcon(toolName);
    const StateIcon = getStateIcon(state);
    const isRunning = state === "running" || state === "pending";

    return (
      <CollapsibleTrigger
        className={cn(
          "flex w-full items-center gap-2 text-muted-foreground text-sm transition-colors hover:text-foreground rounded-lg px-3 py-2 bg-accent/50 hover:bg-accent",
          className
        )}
        {...props}
      >
        <ToolIcon className="size-4 text-primary" />
        <span className="flex-1 text-left">
          {getStatusMessage(state, duration, toolName)}
        </span>
        <Badge variant={getStateBadgeVariant(state)} className="gap-1 text-xs">
          <StateIcon className={cn("size-3", isRunning && "animate-spin")} />
          {getStateLabel(state)}
        </Badge>
        <ChevronDownIcon
          className={cn(
            "size-4 transition-transform",
            isOpen ? "rotate-180" : "rotate-0"
          )}
        />
      </CollapsibleTrigger>
    );
  }
);

export type ToolContentProps = ComponentProps<typeof CollapsibleContent>;

export const ToolContent = memo(
  ({ className, children, ...props }: ToolContentProps) => (
    <CollapsibleContent
      className={cn(
        "mt-2 text-sm",
        "data-[state=closed]:fade-out-0 data-[state=closed]:slide-out-to-top-2 data-[state=open]:slide-in-from-top-2 text-muted-foreground outline-none data-[state=closed]:animate-out data-[state=open]:animate-in",
        className
      )}
      {...props}
    >
      <div className="rounded-lg border bg-card p-3 space-y-3">
        {children}
      </div>
    </CollapsibleContent>
  )
);

export type ToolInputProps = ComponentProps<"div"> & {
  input?: Record<string, unknown>;
};

export const ToolInput = memo(
  ({ className, input, ...props }: ToolInputProps) => {
    if (!input || Object.keys(input).length === 0) return null;
    
    return (
      <div className={cn("space-y-1", className)} {...props}>
        <p className="text-xs font-medium text-muted-foreground">Input:</p>
        <pre className="text-xs bg-muted/50 rounded p-2 overflow-x-auto">
          {JSON.stringify(input, null, 2)}
        </pre>
      </div>
    );
  }
);

export type ToolOutputProps = ComponentProps<"div"> & {
  output?: ReactNode;
  errorText?: string;
};

export const ToolOutput = memo(
  ({ className, output, errorText, ...props }: ToolOutputProps) => {
    if (errorText) {
      return (
        <div className={cn("space-y-1", className)} {...props}>
          <p className="text-xs font-medium text-destructive">Error:</p>
          <p className="text-xs text-destructive bg-destructive/10 rounded p-2">
            {errorText}
          </p>
        </div>
      );
    }
    
    if (!output) return null;
    
    return (
      <div className={cn("space-y-1", className)} {...props}>
        <p className="text-xs font-medium text-muted-foreground">Result:</p>
        <div className="text-xs">
          {typeof output === "string" ? (
            <pre className="bg-muted/50 rounded p-2 overflow-x-auto whitespace-pre-wrap">
              {output}
            </pre>
          ) : (
            output
          )}
        </div>
      </div>
    );
  }
);

// Convenience component for rendering search results
export interface SearchResult {
  title: string;
  url: string;
  content: string;
  score?: number;
}

export type ToolSearchResultsProps = Omit<ComponentProps<"div">, "results"> & {
  results?: SearchResult[];
  query?: string;
};

export const ToolSearchResults = memo(
  ({ className, results, query, ...props }: ToolSearchResultsProps) => {
    if (!results || results.length === 0) return null;
    
    return (
      <div className={cn("space-y-2", className)} {...props}>
        {query && (
          <p className="text-xs text-muted-foreground">
            Found {results.length} results for "{query}"
          </p>
        )}
        <div className="space-y-2 max-h-48 overflow-y-auto">
          {results.slice(0, 5).map((result, idx) => (
            <div key={idx} className="bg-muted/30 rounded p-2 space-y-1">
              <a 
                href={result.url} 
                target="_blank" 
                rel="noopener noreferrer"
                className="text-xs font-medium text-primary hover:underline line-clamp-1"
              >
                {result.title}
              </a>
              <p className="text-xs text-muted-foreground line-clamp-2">
                {result.content}
              </p>
            </div>
          ))}
        </div>
      </div>
    );
  }
);

Tool.displayName = "Tool";
ToolHeader.displayName = "ToolHeader";
ToolContent.displayName = "ToolContent";
ToolInput.displayName = "ToolInput";
ToolOutput.displayName = "ToolOutput";
ToolSearchResults.displayName = "ToolSearchResults";
