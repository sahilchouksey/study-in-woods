'use client';

import { useState, useEffect, useRef, useMemo, useCallback } from 'react';
import { Send, Sparkles, ArrowLeft, StopCircle, ChevronUp, ChevronDown, BookOpen, FileQuestion, Library, FileText, Quote, Settings2, Maximize2, RefreshCw } from 'lucide-react';
import { LoadingSpinner, InlineSpinner } from '@/components/ui/loading-spinner';
import { Streamdown } from 'streamdown';
import remarkMath from 'remark-math';
import remarkGfm from 'remark-gfm';
import rehypeKatex from 'rehype-katex';
import { Loader } from '@/components/ai-elements/loader';
import { Reasoning, ReasoningTrigger, ReasoningContent } from '@/components/ai-elements/reasoning';
import { Tool, ToolHeader, ToolContent, ToolInput, ToolOutput, ToolSearchResults, type ToolState, type SearchResult } from '@/components/ai-elements/tool';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog';
import { 
  useInfiniteChatMessages, 
  useStreamingChat,
  useChatSession,
} from '@/lib/api/hooks/useChat';
import { usePYQs, usePYQById } from '@/lib/api/hooks/usePYQ';
import type { SubjectOption, ChatMessage, Citation, AISettings, ToolEvent, RetrievalMethod } from '@/lib/api/chat';
import { DEFAULT_AI_SETTINGS } from '@/lib/api/chat';
import { cn } from '@/lib/utils';
import { ResourcesDrawer } from './ResourcesDrawer';
import { 
  getAISettings, 
  saveSubjectAISettings, 
  saveGlobalAISettings, 
  updateLastUsedSettings,
  hasSubjectCustomSettings,
  getSettingsMetadata 
} from '@/lib/ai-settings-storage';

/**
 * Escape pipe characters inside math expressions to prevent breaking markdown tables.
 * In math, | is used for conditional probability (P(A|B)) but conflicts with table syntax.
 * This replaces | with \mid inside math delimiters.
 */
function escapePipesInMath(content: string): string {
  if (!content) return content;
  
  // Process $$ ... $$ blocks (display math)
  let result = content.replace(/\$\$([\s\S]*?)\$\$/g, (match, inner) => {
    // Replace standalone | with \mid (but not \| which is already escaped)
    const escaped = inner.replace(/(?<!\\)\|/g, '\\mid ');
    return `$$${escaped}$$`;
  });
  
  // Process single $ ... $ inline math (be careful not to match $$)
  // Match $...$ where the content doesn't start or end with $
  result = result.replace(/\$(?!\$)((?:[^$]|\\\$)+?)\$(?!\$)/g, (match, inner) => {
    const escaped = inner.replace(/(?<!\\)\|/g, '\\mid ');
    return `$${escaped}$`;
  });
  
  return result;
}

/**
 * Process LaTeX math expressions for Streamdown compatibility
 * 
 * Streamdown requires $$ (double dollar) for math delimiters.
 * AI models often output single $ on separate lines. This function converts:
 * - Single $ delimiters → $$ (double dollar)
 * - \[...\] → $$...$$ (display math notation)
 * - \(...\) → $$...$$ (inline math notation)
 * - Escapes | inside math to prevent table breakage
 */
function processLatexForStreamdown(content: string): string {
  if (!content) return content;
  
  // Split content into lines for processing
  const lines = content.split('\n');
  const processedLines: string[] = [];
  
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    const trimmed = line.trim();
    
    // Check if line is JUST a single $ (not $$)
    if (trimmed === '$') {
      // Replace single $ with $$ (use callback to avoid $$ replacement issues)
      const leadingWhitespace = line.match(/^(\s*)/)?.[1] || '';
      processedLines.push(leadingWhitespace + '$$');
    } else {
      processedLines.push(line);
    }
  }
  
  let processed = processedLines.join('\n');
  
  // Convert \[...\] display math to $$...$$
  processed = processed.replace(/\\\[([\s\S]*?)\\\]/g, (_, inner) => `\n$$\n${inner}\n$$\n`);
  
  // Convert \(...\) inline math to $$...$$  
  processed = processed.replace(/\\\(([\s\S]*?)\\\)/g, (_, inner) => `$$${inner}$$`);
  
  // Convert bracketed LaTeX environments [ \begin{...} ... \end{...} ]
  processed = processed.replace(
    /\[\s*(\\begin\{[^}]+\}[\s\S]*?\\end\{[^}]+\})\s*\]/g, 
    (_, inner) => `\n$$\n${inner}\n$$\n`
  );
  
  // Escape pipe characters inside math expressions to prevent table breakage
  processed = escapePipesInMath(processed);
  
  return processed;
}

/**
 * Fix broken markdown tables that come from streaming
 * 
 * When markdown is streamed, table rows can get concatenated without proper newlines,
 * resulting in patterns like: "| A | B || C | D |" instead of proper table rows.
 * 
 * This function reconstructs proper table structure by:
 * 1. Detecting table patterns (consecutive ||)
 * 2. Adding newlines before each new row
 */
function fixMarkdownTables(content: string): string {
  if (!content) return content;
  
  // Pattern: || indicates a missing newline between table rows
  // But we need to be careful not to break valid markdown
  
  // First, fix the common pattern where rows are joined: "|| " at start of new row
  // This handles: "| col1 | col2 || row1 | row2 |" -> "| col1 | col2 |\n| row1 | row2 |"
  let fixed = content.replace(/\|\|(\s*)(?=\s*[^|])/g, '|\n|$1');
  
  // Fix separator rows that got joined: "|---|---||" -> "|---|---|\n|"
  fixed = fixed.replace(/\|(-+\|)+\|\|/g, (match) => {
    return match.slice(0, -2) + '\n|';
  });
  
  // Fix "||--" pattern (header separator joined to previous row)
  fixed = fixed.replace(/\|\|(\s*-)/g, '|\n|$1');
  
  // Fix "---||" pattern (end of separator joined to next row)  
  fixed = fixed.replace(/(-+\|)\|\|/g, '$1\n|');
  
  // General fix: "||" between cells that should be row breaks
  // Look for pattern: content | || | content (double pipe with spaces)
  fixed = fixed.replace(/\|\s*\|\|/g, '|\n|');
  
  return fixed;
}

/**
 * Process content for Streamdown rendering
 * Combines LaTeX conversion and table fixing
 */
function processContentForStreamdown(content: string): string {
  if (!content) return content;
  return fixMarkdownTables(processLatexForStreamdown(content));
}

/**
 * Convert citation references like [1], 【1】 to clickable markdown links
 * Uses URLs from search results when available
 */
function addCitationLinks(content: string, searchResults: SearchResult[]): string {
  if (!content || !searchResults || searchResults.length === 0) return content;
  
  // Pattern to match various citation formats: [1], 【1】, [1,2], etc.
  // Matches: [1], [2], 【1】, 【2】 (single citations)
  // Also matches ranges/lists but we'll process individual numbers
  const citationPattern = /[\[【](\d+)[\]】]/g;
  
  return content.replace(citationPattern, (match, numStr) => {
    const num = parseInt(numStr, 10);
    // Search results are 0-indexed, citation numbers are 1-indexed
    const index = num - 1;
    
    if (index >= 0 && index < searchResults.length && searchResults[index]?.url) {
      const url = searchResults[index].url;
      // Return markdown link format
      return `[${num}](${url})`;
    }
    // If no matching result, return original
    return match;
  });
}

export interface ChatInterfaceProps {
  sessionId: string;
  subject?: SubjectOption;
  onBack: () => void;
}

export function ChatInterface({ sessionId, subject: propSubject, onBack }: ChatInterfaceProps) {
  const [input, setInput] = useState('');
  const scrollRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const [shouldAutoScroll, setShouldAutoScroll] = useState(true);
  
  // Resources drawer state
  const [resourcesOpen, setResourcesOpen] = useState(false);
  const [resourceType, setResourceType] = useState<'syllabus' | 'pyqs' | 'settings'>('syllabus');
  
  // AI Settings state with persistence
  const [aiSettings, setAISettings] = useState<AISettings>({ ...DEFAULT_AI_SETTINGS });
  const [settingsLoaded, setSettingsLoaded] = useState(false);
  const loadedSubjectIdRef = useRef<string | null>(null);
  
  // Retrieval method state - 'none' means don't send the field
  const [retrievalMethod, setRetrievalMethod] = useState<RetrievalMethod | 'none'>('none');
  
  // Scroll-to-bottom button visibility
  const [showScrollButton, setShowScrollButton] = useState(false);
  
  // Track expanded citation - only one can be expanded at a time
  // Format: "messageId-citationIndex" or null if none expanded
  const [expandedCitation, setExpandedCitation] = useState<string | null>(null);

  // Fetch session and messages with pagination
  const { data: session } = useChatSession(sessionId);
  const { 
    data: messagesData,
    isLoading: messagesLoading,
    hasNextPage,
    isFetchingNextPage,
    fetchNextPage,
  } = useInfiniteChatMessages(sessionId);

  // Flatten paginated messages and sort by created_at
  const allMessages = useMemo(() => {
    if (!messagesData?.pages) return [];
    // Pages are in order: page 1 (newest), page 2 (older), etc.
    // We need to reverse pages order and flatten to get chronological order
    const flatMessages = messagesData.pages.flatMap(page => page.messages);
    // Sort by created_at to ensure proper order
    return flatMessages.sort((a, b) => 
      new Date(a.created_at).getTime() - new Date(b.created_at).getTime()
    );
  }, [messagesData]);

  // Load more messages handler
  const handleLoadMore = useCallback(() => {
    if (hasNextPage && !isFetchingNextPage) {
      // Store current scroll position before loading
      const scrollContainer = scrollRef.current;
      const previousScrollHeight = scrollContainer?.scrollHeight || 0;
      
      fetchNextPage().then(() => {
        // After loading, adjust scroll to maintain position
        if (scrollContainer) {
          const newScrollHeight = scrollContainer.scrollHeight;
          scrollContainer.scrollTop = newScrollHeight - previousScrollHeight;
        }
      });
    }
  }, [hasNextPage, isFetchingNextPage, fetchNextPage]);

  // Use prop subject if available, otherwise derive from session
  const subject = propSubject || (session?.subject ? {
    id: session.subject.id,
    name: session.subject.name,
    code: session.subject.code,
    semester_id: session.subject.semester_id,
    credits: 0,
    knowledge_base_uuid: '',
    agent_uuid: '',
    has_syllabus: false,
  } : null);

  // Load AI settings from localStorage when component mounts or subject changes
  useEffect(() => {
    // Skip if running on server
    if (typeof window === 'undefined') return;
    
    // Get subject ID - use prop subject first, then session subject
    const subjectId = subject?.id?.toString() || null;
    
    // Skip if we've already loaded settings for this subject
    if (loadedSubjectIdRef.current === subjectId && settingsLoaded) {
      console.log('[AI Settings] Skipping load - already loaded for subject:', subjectId);
      return;
    }
    
    // Debug: Log what we're loading
    console.log('[AI Settings] Loading settings...', { 
      subjectId, 
      previousSubjectId: loadedSubjectIdRef.current,
      hasSubject: !!subject,
      hasPropSubject: !!propSubject,
      hasSession: !!session 
    });
    
    try {
      const loadedSettings = getAISettings(subjectId || undefined);
      const metadata = getSettingsMetadata(subjectId || undefined);
      
      console.log('[AI Settings] Loaded:', {
        subjectId,
        settings: loadedSettings,
        source: metadata.source,
        hasCustom: subjectId ? hasSubjectCustomSettings(subjectId) : false,
      });
      
      setAISettings(loadedSettings);
      setSettingsLoaded(true);
      loadedSubjectIdRef.current = subjectId;
    } catch (error) {
      console.error('[AI Settings] Failed to load settings:', error);
      setAISettings({ ...DEFAULT_AI_SETTINGS });
      setSettingsLoaded(true);
      loadedSubjectIdRef.current = subjectId;
    }
  }, [subject?.id, propSubject, session, settingsLoaded]);

  // Handle AI settings changes with persistence
  const handleAISettingsChange = useCallback((newSettings: AISettings) => {
    setAISettings(newSettings);
    
    // Get subject ID
    const subjectId = subject?.id?.toString();
    
    // Debug: Log what we're saving
    console.log('[AI Settings] Saving...', { subjectId, newSettings });
    
    try {
      // Update last used settings immediately for UI responsiveness
      updateLastUsedSettings(newSettings);
      
      // Save to appropriate storage based on subject
      if (subjectId) {
        saveSubjectAISettings(subjectId, newSettings);
        console.log(`[AI Settings] Saved for subject ${subjectId}`);
      } else {
        saveGlobalAISettings(newSettings);
        console.log('[AI Settings] Saved globally');
      }
      
      // Verify save worked
      const verifySettings = getAISettings(subjectId);
      console.log('[AI Settings] Verified save:', verifySettings);
    } catch (error) {
      console.error('[AI Settings] Failed to save settings:', error);
    }
  }, [subject?.id]);

  // Merge retrieval method into AI settings for the hook
  const effectiveAISettings = useMemo((): AISettings => {
    if (retrievalMethod === 'none') {
      // Don't include retrieval_method when set to 'none'
      // eslint-disable-next-line @typescript-eslint/no-unused-vars
      const { retrieval_method, ...rest } = aiSettings;
      return rest;
    }
    return { ...aiSettings, retrieval_method: retrievalMethod };
  }, [aiSettings, retrievalMethod]);

  // Streaming chat hook with enhanced reasoning, citations, and tool support
  const { 
    sendMessage, 
    isStreaming, 
    isReasoning,
    isToolRunning,
    hasCompletedResponse, // Keep showing streaming bubble after completion until next message
    streamingContent, 
    streamingReasoning,
    streamingCitations,
    streamingToolEvents,
    // streamingUsage - available for future use (e.g., token count display)
    cancelStream 
  } = useStreamingChat({ 
    sessionId,
    onComplete: () => {
      // Focus input after response completes
      inputRef.current?.focus();
    },
    aiSettings: effectiveAISettings,
  });

  // Filter messages to avoid duplicate display when streaming completes
  // When hasCompletedResponse is true, the StreamingMessageBubble shows the AI response,
  // but the query invalidation also fetches the persisted assistant message.
  // We filter out the last assistant message to prevent showing it twice.
  const messages = useMemo(() => {
    if (!allMessages.length) return [];
    
    // Check if we have any streaming data that would cause StreamingMessageBubble to render
    const hasStreamingData = streamingContent || streamingReasoning || streamingToolEvents.length > 0;
    
    // If we have a completed streaming response with any data, filter out the last assistant message
    // to avoid showing it twice (once in StreamingMessageBubble, once in MessageBubble)
    if (hasCompletedResponse && hasStreamingData) {
      // Find the index of the last assistant message
      const lastAssistantIndex = allMessages.findLastIndex(m => m.role === 'assistant');
      if (lastAssistantIndex !== -1) {
        return allMessages.filter((_, index) => index !== lastAssistantIndex);
      }
    }
    
    return allMessages;
  }, [allMessages, hasCompletedResponse, streamingContent, streamingReasoning, streamingToolEvents]);

  // Auto-scroll to bottom when new messages arrive or streaming content updates
  // Only auto-scroll if user hasn't scrolled up to load more
  useEffect(() => {
    if (scrollRef.current && shouldAutoScroll) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages, streamingContent, shouldAutoScroll]);

  // Reset auto-scroll when streaming starts
  useEffect(() => {
    if (isStreaming) {
      setShouldAutoScroll(true);
    }
  }, [isStreaming]);

  // Scroll to bottom when messages first load (page refresh/initial load)
  const initialScrollDone = useRef(false);
  useEffect(() => {
    if (!messagesLoading && messages.length > 0 && !initialScrollDone.current) {
      initialScrollDone.current = true;
      // Small delay to ensure DOM is updated
      setTimeout(() => {
        if (scrollRef.current) {
          scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
        }
      }, 100);
    }
  }, [messagesLoading, messages.length]);

  // Focus input on mount
  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  const handleSend = () => {
    if (!input.trim() || isStreaming) return;
    
    const messageText = input.trim();
    setInput('');
    
    // Force scroll to bottom when sending a message
    setShouldAutoScroll(true);
    if (scrollRef.current) {
      // Use setTimeout to ensure scroll happens after optimistic message is rendered
      setTimeout(() => {
        scrollRef.current?.scrollTo({
          top: scrollRef.current.scrollHeight,
          behavior: 'smooth'
        });
      }, 50);
    }
    
    sendMessage(messageText);
  };

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  // Handle regenerating the last response
  const handleRegenerate = useCallback(() => {
    if (isStreaming) return;
    
    // Find the last user message in the conversation
    const lastUserMessage = [...allMessages].reverse().find(m => m.role === 'user');
    if (lastUserMessage) {
      // Force scroll to bottom
      setShouldAutoScroll(true);
      // Resend the last user message to regenerate response
      sendMessage(lastUserMessage.content);
    }
  }, [isStreaming, allMessages, sendMessage]);

  // Handle selecting a question from resources drawer
  const handleSelectQuestion = (question: string) => {
    setInput(question);
    inputRef.current?.focus();
  };

  // Open resources drawer
  const openResources = (type: 'syllabus' | 'pyqs' | 'settings') => {
    setResourceType(type);
    setResourcesOpen(true);
  };

  // Handle scroll to show/hide scroll-to-bottom button
  const handleScroll = useCallback((e: React.UIEvent<HTMLDivElement>) => {
    const { scrollTop, scrollHeight, clientHeight } = e.currentTarget;
    const distanceFromBottom = scrollHeight - scrollTop - clientHeight;
    // Show button when scrolled up more than 200px from bottom
    setShowScrollButton(distanceFromBottom > 200);
    // Update shouldAutoScroll based on position
    setShouldAutoScroll(distanceFromBottom < 50);
  }, []);

  // Scroll to bottom handler
  const scrollToBottom = useCallback(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTo({
        top: scrollRef.current.scrollHeight,
        behavior: 'smooth'
      });
      setShouldAutoScroll(true);
    }
  }, []);

  // Handle citation click - scroll to citation and expand it
  const handleCitationClick = useCallback((messageId: string | number, citationIndex: number) => {
    const key = `${messageId}-${citationIndex}`;
    
    // Toggle the citation expansion - only one can be expanded at a time
    setExpandedCitation(prev => prev === key ? null : key);
    
    // Scroll to the citation element after a brief delay for DOM update
    setTimeout(() => {
      const citationElement = document.getElementById(`citation-${messageId}-${citationIndex + 1}`);
      if (citationElement) {
        // Check if element is already visible in viewport
        const rect = citationElement.getBoundingClientRect();
        const viewportHeight = window.innerHeight;
        const isVisible = rect.top >= 0 && rect.bottom <= viewportHeight;
        
        // Only scroll if citation is not fully visible
        if (!isVisible) {
          // Scroll to TOP of citation, not center
          citationElement.scrollIntoView({ 
            behavior: 'smooth', 
            block: 'start' 
          });
        }
        
        // Add a brief highlight effect regardless of scroll
        citationElement.classList.add('ring-2', 'ring-primary');
        setTimeout(() => {
          citationElement.classList.remove('ring-2', 'ring-primary');
        }, 2000);
      }
    }, 100);
  }, []);

  return (
    <div className="flex flex-col h-full overflow-hidden relative">
      {/* Header */}
      <div className="flex-shrink-0 flex items-center gap-4 border-b bg-card px-4 py-3">
        <Button variant="ghost" size="icon" onClick={onBack}>
          <ArrowLeft className="h-5 w-5" />
        </Button>
        <div className="flex-1 min-w-0">
          <h2 className="font-semibold truncate flex items-center gap-2">
            <Sparkles className="h-4 w-4 text-primary flex-shrink-0" />
            {subject?.name || 'Chat Session'}
          </h2>
          {(subject?.code || subject?.has_syllabus || (settingsLoaded && subject?.id && hasSubjectCustomSettings(subject.id.toString()))) && (
            <p className="text-xs text-muted-foreground truncate">
              {subject?.code}{subject?.code && subject?.has_syllabus && ' • '}{subject?.has_syllabus && 'Syllabus context enabled'}
              {settingsLoaded && subject?.id && hasSubjectCustomSettings(subject.id.toString()) && (
                <span className="ml-2 text-primary">• Custom AI settings</span>
              )}
            </p>
          )}
        </div>
        <div className="flex items-center gap-2">
          {session && (
            <Badge variant="outline" className="text-xs h-8 px-3 flex items-center">
              {session.message_count} messages
            </Badge>
          )}
          
          {/* Resources Dropdown */}
          {subject && (
            <>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="outline" size="sm" className="gap-2">
                    <Library className="h-4 w-4" />
                    Resources
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem onClick={() => openResources('syllabus')}>
                    <BookOpen className="h-4 w-4 mr-2" />
                    Syllabus
                  </DropdownMenuItem>
                  <DropdownMenuItem onClick={() => openResources('pyqs')}>
                    <FileQuestion className="h-4 w-4 mr-2" />
                    Previous Year Questions
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
              
              {/* AI Settings Button */}
              <Button 
                variant="outline" 
                size="sm" 
                className="gap-2"
                onClick={() => openResources('settings')}
              >
                <Settings2 className="h-4 w-4" />
                <span className="hidden sm:inline">Settings</span>
                {settingsLoaded && subject?.id && hasSubjectCustomSettings(subject.id.toString()) && (
                  <span className="h-2 w-2 rounded-full bg-primary" />
                )}
              </Button>
            </>
          )}
        </div>
      </div>

      {/* Messages Area */}
      <div className="flex-1 overflow-y-auto relative" ref={scrollRef} onScroll={handleScroll}>
        <div className="p-4 space-y-4 max-w-4xl mx-auto min-h-full">
          {messagesLoading ? (
            <LoadingSpinner size="lg" centered withPadding />
          ) : messages.length === 0 && !isStreaming ? (
            <EmptyState 
              subjectId={String(subject?.id || '')} 
              subjectName={subject?.name || 'this subject'} 
              onSelectQuestion={handleSelectQuestion}
            />
          ) : (
            <>
              {/* Load earlier messages button */}
              {hasNextPage && (
                <div className="flex justify-center pb-2">
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={handleLoadMore}
                    disabled={isFetchingNextPage}
                    className="text-muted-foreground hover:text-foreground"
                  >
                    {isFetchingNextPage ? (
                      <>
                        <InlineSpinner className="mr-2" />
                        Loading...
                      </>
                    ) : (
                      <>
                        <ChevronUp className="h-4 w-4 mr-2" />
                        Load earlier messages
                      </>
                    )}
                  </Button>
                </div>
              )}

              {messages
                .filter((message: ChatMessage) => {
                  // Filter out empty assistant messages (failed/empty responses)
                  if (message.role === 'assistant' && !message.content?.trim()) {
                    return false;
                  }
                  return true;
                })
                .map((message: ChatMessage) => (
                <MessageBubble 
                  key={message.id} 
                  message={message}
                  expandedCitation={expandedCitation}
                  onCitationClick={handleCitationClick}
                />
              ))}
              
              {/* Streaming response with reasoning, tools, and content */}
              {/* Show while streaming OR after completion until user sends next message */}
              {(isStreaming || hasCompletedResponse) && (streamingContent || streamingReasoning || streamingToolEvents.length > 0) && (
                <StreamingMessageBubble
                  content={streamingContent}
                  reasoning={streamingReasoning}
                  citations={streamingCitations}
                  toolEvents={streamingToolEvents}
                  isReasoning={isReasoning}
                  isToolRunning={isToolRunning}
                  isActivelyStreaming={isStreaming}
                  sessionId={sessionId}
                  onCitationClick={handleCitationClick}
                  onRegenerate={!isStreaming ? handleRegenerate : undefined}
                />
              )}
              
              {/* Streaming indicator when waiting for first chunk */}
              {isStreaming && !streamingContent && !streamingReasoning && (
                <div className="flex justify-start">
                  <div className="bg-muted rounded-2xl px-4 py-3">
                    <div className="flex items-center gap-2">
                      <Loader className="text-primary" size={16} />
                      <span className="text-sm text-muted-foreground">
                        AI is thinking...
                      </span>
                    </div>
                  </div>
                </div>
              )}
            </>
          )}
        </div>
      </div>

      {/* Scroll to Bottom Button - Floating above input area */}
      {showScrollButton && (
        <div className="absolute bottom-20 right-6 z-20">
          <Button
            className="rounded-full shadow-lg h-10 w-10"
            size="icon"
            variant="secondary"
            onClick={scrollToBottom}
            title="Scroll to bottom"
          >
            <ChevronDown className="h-5 w-5" />
          </Button>
        </div>
      )}

      {/* Input Area */}
      <div className="border-t bg-card p-4">
        <div className="max-w-4xl mx-auto">
          <div className="flex gap-2">
            <Input
              ref={inputRef}
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={handleKeyPress}
              placeholder={`Ask about ${subject?.name || 'this subject'}...`}
              disabled={isStreaming}
              className="flex-1"
            />
            {isStreaming ? (
              <Button 
                onClick={cancelStream} 
                variant="destructive" 
                size="icon"
                title="Stop generating"
              >
                <StopCircle className="h-4 w-4" />
              </Button>
            ) : (
              /* Send button group with retrieval method dropdown */
              <div className="flex items-center">
                <Button
                  onClick={handleSend}
                  disabled={!input.trim()}
                  size="icon"
                  className="rounded-r-none"
                >
                  <Send className="h-4 w-4" />
                </Button>
                {/* Retrieval Method Dropdown - attached to send button */}
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button 
                      variant="default"
                      size="icon"
                      className={cn(
                        "rounded-l-none border-l border-primary-foreground/20 w-6 px-0",
                        retrievalMethod !== 'none' && "bg-primary/80"
                      )}
                      disabled={!input.trim()}
                      title={`Retrieval: ${retrievalMethod === 'none' ? 'Default' : retrievalMethod.replace('_', ' ')}`}
                    >
                      <ChevronDown className="h-3.5 w-3.5" />
                    </Button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end" className="w-48">
                    <DropdownMenuItem 
                      onClick={() => setRetrievalMethod('none')}
                      className={cn(retrievalMethod === 'none' && "bg-accent")}
                    >
                      <span className="flex-1">Default</span>
                      {retrievalMethod === 'none' && <span className="text-xs text-muted-foreground">✓</span>}
                    </DropdownMenuItem>
                    <DropdownMenuItem 
                      onClick={() => setRetrievalMethod('rewrite')}
                      className={cn(retrievalMethod === 'rewrite' && "bg-accent")}
                    >
                      <span className="flex-1">Rewrite</span>
                      {retrievalMethod === 'rewrite' && <span className="text-xs text-muted-foreground">✓</span>}
                    </DropdownMenuItem>
                    <DropdownMenuItem 
                      onClick={() => setRetrievalMethod('step_back')}
                      className={cn(retrievalMethod === 'step_back' && "bg-accent")}
                    >
                      <span className="flex-1">Step Back</span>
                      {retrievalMethod === 'step_back' && <span className="text-xs text-muted-foreground">✓</span>}
                    </DropdownMenuItem>
                    <DropdownMenuItem 
                      onClick={() => setRetrievalMethod('sub_queries')}
                      className={cn(retrievalMethod === 'sub_queries' && "bg-accent")}
                    >
                      <span className="flex-1">Sub Queries</span>
                      {retrievalMethod === 'sub_queries' && <span className="text-xs text-muted-foreground">✓</span>}
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
            )}
          </div>
          <p className="text-xs text-muted-foreground mt-2 text-center">
            AI responses include citations from course materials
          </p>
        </div>
      </div>

      {/* Resources Drawer */}
      {subject && settingsLoaded && (
        <ResourcesDrawer
          open={resourcesOpen}
          onOpenChange={setResourcesOpen}
          resourceType={resourceType}
          subjectId={String(subject.id)}
          subjectName={subject.name}
          onSelectQuestion={handleSelectQuestion}
          aiSettings={aiSettings}
          onAISettingsChange={handleAISettingsChange}
        />
      )}
    </div>
  );
}

// Message Bubble Component
interface MessageBubbleProps {
  message: ChatMessage;
  isStreaming?: boolean;
}

// Extract inline citation markers like [[C1]], [[C2]] from content
function extractInlineCitations(content: string): string[] {
  const matches = content.match(/\[\[C\d+\]\]/g);
  if (!matches) return [];
  // Return unique citations in order
  return [...new Set(matches)];
}

/**
 * Extract citation indices that are actually referenced in the content.
 * Returns an array of 0-based indices for citations that appear as [[C1]], [[C2]], etc.
 */
function extractCitedIndices(content: string): number[] {
  // Match both [[C#]] and 【C#】 formats (AI may use either)
  const matches = content.match(/(?:\[\[C|\u3010C)(\d+)(?:\]\]|\u3011)/g);
  if (!matches) return [];
  
  // Extract unique indices (convert from 1-based to 0-based)
  const indices = new Set<number>();
  for (const match of matches) {
    // Extract number from either format
    const num = parseInt(match.replace(/[\[\]【】C]/g, ''), 10);
    if (!isNaN(num) && num > 0) {
      indices.add(num - 1); // Convert to 0-based index
    }
  }
  
  // Return sorted indices
  return Array.from(indices).sort((a, b) => a - b);
}

/**
 * Filter citations to include:
 * 1. Those actually referenced in the content with [[C#]] markers
 * 2. Those with high confidence score (>= 0.85) even without markers
 * 
 * Also returns a mapping from original index to new index for proper display.
 */
function filterCitedCitations(
  citations: Citation[], 
  content: string
): { citations: Citation[]; indexMap: Map<number, number> } {
  const citedIndices = extractCitedIndices(content);
  
  // Filter citations and create index mapping
  const filteredCitations: Citation[] = [];
  const indexMap = new Map<number, number>(); // original index -> new index
  const addedIndices = new Set<number>();
  
  // First, add citations that are explicitly referenced with [[C#]] markers
  for (const originalIndex of citedIndices) {
    if (originalIndex < citations.length && !addedIndices.has(originalIndex)) {
      indexMap.set(originalIndex, filteredCitations.length);
      filteredCitations.push(citations[originalIndex]);
      addedIndices.add(originalIndex);
    }
  }
  
  // Then, add citations with valid scores (>= 0.50) that weren't already added
  // This ensures sources are shown even when AI doesn't use [[C#]] markers
  // PYQ sources typically have scores around 0.5-0.6 which are still relevant
  const CONFIDENCE_THRESHOLD = 0.50;
  
  for (let i = 0; i < citations.length; i++) {
    if (addedIndices.has(i)) continue;
    
    const citation = citations[i];
    const score = citation.score;
    const filename = citation.filename || '';
    
    // Check if this is a PYQ/syllabus source (always show these as they're course materials)
    const isPYQSource = filename.includes('/pyqs/') || filename.includes('/syllabus/');
    
    // Check if score is a valid confidence value (between 0 and 1)
    const hasValidScore = typeof score === 'number' && score >= CONFIDENCE_THRESHOLD && score <= 1;
    
    // Show citation if: has valid score OR is a PYQ/syllabus source
    if (hasValidScore || isPYQSource) {
      indexMap.set(i, filteredCitations.length);
      filteredCitations.push(citation);
      addedIndices.add(i);
    }
  }
  
  return { citations: filteredCitations, indexMap };
}

// Format content to highlight citation markers (unused for now, keeping for future use)
// eslint-disable-next-line @typescript-eslint/no-unused-vars
function formatContentWithCitations(content: string): string {
  // Replace [[C#]] with styled superscript-like markers
  return content.replace(/\[\[C(\d+)\]\]/g, '<sup class="text-primary font-medium">[C$1]</sup>');
}

// Citation Item Component with expandable content
interface CitationItemProps {
  citation: Citation;
  index: number;
  messageId: string | number;
  isExpanded?: boolean;
  onToggle?: () => void;
}

function CitationItem({ citation, index, messageId, isExpanded, onToggle }: CitationItemProps) {
  const [localExpanded, setLocalExpanded] = useState(false);
  const [dialogOpen, setDialogOpen] = useState(false);
  const content = citation.page_content || citation.content;
  const hasContent = content && content.trim().length > 0;
  
  // Use controlled expansion if provided, otherwise use local state
  const expanded = isExpanded !== undefined ? isExpanded : localExpanded;
  const handleToggle = onToggle || (() => setLocalExpanded(!localExpanded));
  
  // Truncate long content for preview
  const previewLength = 150;
  const shouldTruncate = hasContent && content.length > previewLength;
  const previewContent = shouldTruncate && !expanded 
    ? content.slice(0, previewLength) + '...' 
    : content;

  // Create a unique ID for scroll targeting
  const citationId = `citation-${messageId}-${index + 1}`;

  return (
    <>
      <div 
        id={citationId}
        className={cn(
          "border rounded-lg overflow-hidden transition-all duration-300",
          // Use card background (white/dark) to contrast with muted message bubble
          "bg-card border-border shadow-sm",
          expanded && "ring-2 ring-primary/40 shadow-md"
        )}
      >
        {/* Citation Header - Always visible */}
        <button
          onClick={() => hasContent && handleToggle()}
          className={cn(
            "w-full flex items-center gap-2 px-3 py-2 text-left",
            hasContent && "hover:bg-accent/50 cursor-pointer",
            !hasContent && "cursor-default"
          )}
        >
          <span className="flex-shrink-0 flex items-center justify-center w-6 h-6 rounded-full bg-primary/15 text-primary text-xs font-semibold">
            {index + 1}
          </span>
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2">
              <FileText className="h-3 w-3 text-muted-foreground flex-shrink-0" />
              <span className="text-xs font-medium truncate text-foreground">
                {citation.filename || citation.source || 'Knowledge Base'}
              </span>
              {citation.page && (
                <Badge variant="outline" className="text-[10px] px-1.5 py-0">
                  p. {citation.page}
                </Badge>
              )}
              {citation.score != null && citation.score > 0 && citation.score <= 1 && (
                <Badge variant="secondary" className="text-[10px] px-1.5 py-0">
                  {(citation.score * 100).toFixed(0)}% match
                </Badge>
              )}
            </div>
          </div>
          {/* Expand to dialog button */}
          {hasContent && (
            <button
              onClick={(e) => {
                e.stopPropagation();
                setDialogOpen(true);
              }}
              className="p-1 rounded hover:bg-accent text-muted-foreground hover:text-foreground transition-colors flex-shrink-0"
              title="View full content"
            >
              <Maximize2 className="h-3.5 w-3.5" />
            </button>
          )}
          {hasContent && (
            <ChevronDown className={cn(
              "h-4 w-4 text-muted-foreground transition-transform flex-shrink-0",
              expanded && "rotate-180"
            )} />
          )}
        </button>
        
        {/* Citation Content - Expandable */}
        {hasContent && (
          <div className={cn(
            "border-t border-border px-3 py-2 bg-accent/30",
            !expanded && "hidden"
          )}>
            <div className="flex items-start gap-2">
              <Quote className="h-3 w-3 text-primary/60 mt-1 flex-shrink-0" />
              <p className="text-xs text-foreground/80 leading-relaxed whitespace-pre-wrap">
                {previewContent}
              </p>
            </div>
            {shouldTruncate && (
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  handleToggle();
                }}
                className="text-xs text-primary hover:underline mt-1 ml-5"
              >
                {expanded ? 'Show less' : 'Show more'}
              </button>
            )}
          </div>
        )}
      </div>

      {/* Full Content Dialog */}
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="max-w-2xl max-h-[80vh] flex flex-col">
          <DialogHeader className="flex-shrink-0">
            <DialogTitle className="flex items-center gap-2">
              <span className="flex items-center justify-center w-6 h-6 rounded-full bg-primary/15 text-primary text-xs font-semibold">
                {index + 1}
              </span>
              <FileText className="h-4 w-4 text-muted-foreground" />
              <span className="truncate">
                {citation.filename || citation.source || 'Knowledge Base'}
              </span>
            </DialogTitle>
            <DialogDescription className="flex items-center gap-2 flex-wrap">
              {citation.page && (
                <Badge variant="outline" className="text-xs">
                  Page {citation.page}
                </Badge>
              )}
              {citation.score != null && citation.score > 0 && citation.score <= 1 && (
                <Badge variant="secondary" className="text-xs">
                  {(citation.score * 100).toFixed(0)}% relevance match
                </Badge>
              )}
              <span className="text-muted-foreground">
                Source content from course materials
              </span>
            </DialogDescription>
          </DialogHeader>
          
          {/* Scrollable Content Area */}
          <div className="flex-1 overflow-y-auto min-h-0 mt-4 pr-2">
            <div className="bg-accent/30 rounded-lg p-4 border border-border">
              <div className="flex items-start gap-3">
                <Quote className="h-4 w-4 text-primary/60 mt-1 flex-shrink-0" />
                <p className="text-sm text-foreground leading-relaxed whitespace-pre-wrap">
                  {content}
                </p>
              </div>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}

// Content with clickable citations - uses DOM manipulation to preserve markdown structure
interface ContentWithCitationsProps {
  content: string;
  messageId: string | number;
  citations?: Citation[];
  onCitationClick: (messageId: string | number, citationIndex: number) => void;
  isStreaming?: boolean;
}

function ContentWithCitations({ content, messageId, citations = [], onCitationClick, isStreaming = false }: ContentWithCitationsProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  
  // After render, find and replace citation markers in the DOM
  useEffect(() => {
    if (!containerRef.current) return;
    
    // Detect dark mode
    const isDarkMode = document.documentElement.classList.contains('dark');
    
    // Theme-aware colors: black bg in light mode, white bg in dark mode
    const bgColor = isDarkMode ? '#ffffff' : '#000000';
    const textColor = isDarkMode ? '#000000' : '#ffffff';
    const hoverBgColor = isDarkMode ? '#e5e5e5' : '#333333';
    
    const replaceCitations = (node: Node) => {
      if (node.nodeType === Node.TEXT_NODE) {
        const text = node.textContent || '';
        // Match both [[C#]] and 【C#】 formats (AI may use either)
        const citationRegex = /(?:\[\[C|【C)(\d+)(?:\]\]|】)/g;
        
        if (citationRegex.test(text)) {
          // Reset regex
          citationRegex.lastIndex = 0;
          
          const fragment = document.createDocumentFragment();
          let lastIndex = 0;
          let match;
          
          while ((match = citationRegex.exec(text)) !== null) {
            // Add text before the match
            if (match.index > lastIndex) {
              fragment.appendChild(document.createTextNode(text.slice(lastIndex, match.index)));
            }
            
            // Create citation button with tooltip wrapper
            const citationNum = parseInt(match[1], 10);
            const citationIndex = citationNum - 1;
            
            // Check if citation exists (AI may hallucinate non-existent citations)
            const isValidCitation = citationIndex >= 0 && citationIndex < citations.length;
            
            // If invalid citation, just show as plain text and skip
            if (!isValidCitation) {
              fragment.appendChild(document.createTextNode(match[0]));
              lastIndex = match.index + match[0].length;
              continue;
            }
            
            // Get citation info for tooltip
            const citation = citations[citationIndex];
            const citationTitle = citation?.filename || citation?.source || `Citation ${citationNum}`;
            
            // Create wrapper for positioning tooltip
            const wrapper = document.createElement('span');
            wrapper.className = 'citation-wrapper';
            wrapper.style.cssText = `
              position: relative;
              display: inline-flex;
              align-items: center;
            `;
            
            const button = document.createElement('button');
            button.className = 'citation-link';
            button.style.cssText = `
              display: inline-flex;
              align-items: center;
              justify-content: center;
              font-weight: 700;
              font-size: 10px;
              line-height: 1;
              background-color: ${bgColor};
              color: ${textColor};
              border-radius: 4px;
              padding: 3px 7px;
              margin: 0 3px;
              transition: all 0.15s ease;
              cursor: pointer;
              vertical-align: middle;
              border: none;
              font-family: inherit;
              box-shadow: 0 1px 3px rgba(0,0,0,0.2);
              text-decoration: none;
              letter-spacing: 0.02em;
            `;
            button.textContent = `C${citationNum}`;
            button.setAttribute('data-citation', String(citationNum));
            button.setAttribute('data-message-id', String(messageId));
            
            // Create tooltip element
            const tooltip = document.createElement('span');
            tooltip.className = 'citation-tooltip';
            tooltip.textContent = citationTitle;
            tooltip.style.cssText = `
              position: absolute;
              bottom: calc(100% + 6px);
              left: 50%;
              transform: translateX(-50%);
              background-color: ${isDarkMode ? '#1f1f1f' : '#333333'};
              color: #ffffff;
              padding: 6px 10px;
              border-radius: 6px;
              font-size: 11px;
              font-weight: 500;
              white-space: nowrap;
              max-width: 250px;
              overflow: hidden;
              text-overflow: ellipsis;
              opacity: 0;
              visibility: hidden;
              transition: opacity 0.15s ease, visibility 0.15s ease;
              z-index: 1000;
              pointer-events: none;
              box-shadow: 0 2px 8px rgba(0,0,0,0.3);
            `;
            
            // Add hover effects
            button.onmouseenter = () => {
              button.style.backgroundColor = hoverBgColor;
              button.style.transform = 'scale(1.08)';
              button.style.boxShadow = '0 2px 4px rgba(0,0,0,0.25)';
              tooltip.style.opacity = '1';
              tooltip.style.visibility = 'visible';
            };
            button.onmouseleave = () => {
              button.style.backgroundColor = bgColor;
              button.style.transform = 'scale(1)';
              button.style.boxShadow = '0 1px 3px rgba(0,0,0,0.2)';
              tooltip.style.opacity = '0';
              tooltip.style.visibility = 'hidden';
            };
            
            // Add click handler
            button.onclick = (e) => {
              e.preventDefault();
              e.stopPropagation();
              onCitationClick(messageId, citationIndex);
            };
            
            wrapper.appendChild(button);
            wrapper.appendChild(tooltip);
            fragment.appendChild(wrapper);
            lastIndex = match.index + match[0].length;
          }
          
          // Add remaining text
          if (lastIndex < text.length) {
            fragment.appendChild(document.createTextNode(text.slice(lastIndex)));
          }
          
          // Replace the text node with the fragment
          node.parentNode?.replaceChild(fragment, node);
        }
      } else if (node.nodeType === Node.ELEMENT_NODE) {
        // Skip code blocks and pre elements to preserve their content
        const tagName = (node as Element).tagName.toLowerCase();
        if (tagName === 'code' || tagName === 'pre') {
          return;
        }
        
        // Process child nodes (make a copy since we're modifying)
        const children = Array.from(node.childNodes);
        children.forEach(child => replaceCitations(child));
      }
    };
    
    // Small delay to ensure Streamdown has finished rendering
    const timeoutId = setTimeout(() => {
      if (containerRef.current) {
        replaceCitations(containerRef.current);
      }
    }, 50);
    
    return () => clearTimeout(timeoutId);
  }, [content, messageId, citations, onCitationClick]);
  
  return (
    <div ref={containerRef} className="overflow-hidden max-w-full">
      <Streamdown 
        mode={isStreaming ? "streaming" : "static"}
        parseIncompleteMarkdown
        controls={{ table: true, code: true }}
        className="w-full max-w-full overflow-hidden text-sm [&>*:first-child]:mt-0 [&>*:last-child]:mb-0"
        remarkPlugins={[
          remarkGfm,
          [remarkMath, { singleDollarTextMath: false }]
        ]}
        rehypePlugins={[rehypeKatex]}
      >
        {processContentForStreamdown(content)}
      </Streamdown>
    </div>
  );
}

// Extended MessageBubble props to support citation click handling
interface ExtendedMessageBubbleProps extends MessageBubbleProps {
  expandedCitation: string | null; // Only one citation can be expanded at a time
  onCitationClick: (messageId: string | number, citationIndex: number) => void;
}

function MessageBubble({ 
  message, 
  isStreaming = false,
  expandedCitation,
  onCitationClick
}: ExtendedMessageBubbleProps) {
  const isUser = message.role === 'user';
  
  // Extract inline citations if no structured citations provided
  const inlineCitations = !message.citations?.length 
    ? extractInlineCitations(message.content) 
    : [];

  // Filter citations to only show those actually referenced in the content
  const { citations: citedCitations, indexMap: citationIndexMap } = useMemo(() => {
    if (!message.citations || message.citations.length === 0) {
      return { citations: [], indexMap: new Map<number, number>() };
    }
    return filterCitedCitations(message.citations, message.content);
  }, [message.citations, message.content]);

  // Get the original citation indices for display (1-based)
  const originalIndices = useMemo(() => {
    const indices: number[] = [];
    citationIndexMap.forEach((_, originalIndex) => {
      indices.push(originalIndex + 1); // Convert to 1-based for display
    });
    return indices;
  }, [citationIndexMap]);

  // Check if a citation should be expanded (using original index)
  const isCitationExpanded = (originalIndex: number) => {
    return expandedCitation === `${message.id}-${originalIndex}`;
  };

  // Handle toggling a citation (using original index)
  const handleCitationToggle = (originalIndex: number) => {
    onCitationClick(message.id, originalIndex);
  };

  return (
    <div className={cn('flex', isUser ? 'justify-end' : 'justify-start')}>
      <div
        className={cn(
          'max-w-[85%] rounded-2xl px-4 py-3',
          isUser
            ? 'bg-primary text-primary-foreground'
            : 'bg-muted'
        )}
      >
        {!isUser && (
          <div className="flex items-center gap-2 mb-2">
            <Sparkles className="h-4 w-4 text-primary" />
            <span className="text-xs font-semibold text-primary">
              AI Assistant
            </span>
            {isStreaming && (
              <Badge variant="secondary" className="text-xs animate-pulse">
                Streaming
              </Badge>
            )}
          </div>
        )}
        
        {isUser ? (
          <p className="whitespace-pre-wrap">{message.content}</p>
        ) : message.content ? (
          // Use ContentWithCitations for AI messages to make [[C#]] clickable
          message.citations && message.citations.length > 0 ? (
            <ContentWithCitations
              content={message.content}
              messageId={message.id}
              citations={message.citations}
              onCitationClick={onCitationClick}
              isStreaming={isStreaming}
            />
          ) : (
            <Streamdown 
              mode={isStreaming ? "streaming" : "static"}
              parseIncompleteMarkdown
              controls={{ table: true, code: true }}
              className="w-full max-w-full overflow-hidden text-sm [&>*:first-child]:mt-0 [&>*:last-child]:mb-0"
              remarkPlugins={[
                remarkGfm,
                [remarkMath, { singleDollarTextMath: false }]
              ]}
              rehypePlugins={[rehypeKatex]}
            >
              {processContentForStreamdown(message.content)}
            </Streamdown>
          )
        ) : (
          <p className="text-muted-foreground italic">No content</p>
        )}
        
        {!isStreaming && (
          <p className="text-xs opacity-60 mt-2">
            {new Date(message.created_at).toLocaleTimeString([], { 
              hour: '2-digit', 
              minute: '2-digit' 
            })}
          </p>
        )}

        {/* Citations - Only show citations that are actually referenced in the content */}
        {citedCitations.length > 0 && (
          <div className="mt-3 pt-2 border-t border-border/50">
            <div className="flex items-center gap-2 mb-2">
              <Library className="h-3.5 w-3.5 text-primary" />
              <p className="text-xs font-medium">
                Sources ({citedCitations.length})
              </p>
            </div>
            <div className={cn(
              "space-y-2",
              citedCitations.length > 2 && "max-h-[120px] overflow-y-auto pr-1 scrollbar-thin scrollbar-thumb-border scrollbar-track-transparent"
            )}>
              {citedCitations.map((citation, idx) => {
                const originalIndex = originalIndices[idx] - 1; // Convert back to 0-based
                return (
                  <CitationItem 
                    key={originalIndex} 
                    citation={citation} 
                    index={originalIndex}
                    messageId={message.id}
                    isExpanded={isCitationExpanded(originalIndex)}
                    onToggle={() => handleCitationToggle(originalIndex)}
                  />
                );
              })}
            </div>
          </div>
        )}
        
        {/* Inline Citations - When no structured citations but markers exist in content */}
        {!isStreaming && (!message.citations || message.citations.length === 0) && inlineCitations.length > 0 && (
          <div className="mt-3 pt-2 border-t border-border/50">
            <p className="text-xs font-medium mb-1">
              Referenced Sources: {inlineCitations.length}
            </p>
            <p className="text-xs text-muted-foreground">
              {inlineCitations.join(', ')} - From course knowledge base
            </p>
          </div>
        )}
      </div>
    </div>
  );
}

// Streaming Message Bubble with Reasoning and Tool Support
interface StreamingMessageBubbleProps {
  content: string;
  reasoning: string;
  citations: Citation[];
  toolEvents: ToolEvent[];
  isReasoning: boolean;
  isToolRunning: boolean;
  isActivelyStreaming: boolean; // True only while streaming, false after completion
  sessionId: string;
  onCitationClick: (messageId: string | number, citationIndex: number) => void;
  onRegenerate?: () => void; // Called when user clicks regenerate button
}



function StreamingMessageBubble({ 
  content, 
  reasoning, 
  citations,
  toolEvents,
  isReasoning,
  isToolRunning,
  isActivelyStreaming,
  sessionId,
  onCitationClick,
  onRegenerate
}: StreamingMessageBubbleProps) {
  // Use a temporary ID for streaming messages
  const streamingMessageId = `streaming-${sessionId}`;
  
  // Local state for expanded citation during streaming - only one at a time
  const [expandedCitation, setExpandedCitation] = useState<string | null>(null);

  // Extract search results from tool events at component level for citation linking
  const searchResults = useMemo<SearchResult[]>(() => {
    const results: SearchResult[] = [];
    const endEvent = toolEvents.find(e => e.type === 'tool_end');
    const startEvent = toolEvents.find(e => e.type === 'tool_start');
    
    if (endEvent?.result && startEvent?.tool_name === 'web_search') {
      const result = endEvent.result as { results?: Array<{ title?: string; url?: string; content?: string; score?: number }> };
      if (result.results && Array.isArray(result.results)) {
        for (const r of result.results) {
          results.push({
            title: r.title || 'Untitled',
            url: r.url || '',
            content: r.content || '',
            score: r.score,
          });
        }
      }
    }
    return results;
  }, [toolEvents]);

  // Filter citations to only show those actually referenced in the content
  const { citations: citedCitations, indexMap: citationIndexMap } = useMemo(() => {
    if (!citations || citations.length === 0 || !content) {
      return { citations: [], indexMap: new Map<number, number>() };
    }
    return filterCitedCitations(citations, content);
  }, [citations, content]);

  // Get the original citation indices for display (1-based)
  const originalIndices = useMemo(() => {
    const indices: number[] = [];
    citationIndexMap.forEach((_, originalIndex) => {
      indices.push(originalIndex + 1); // Convert to 1-based for display
    });
    return indices;
  }, [citationIndexMap]);
  
  const handleLocalCitationClick = (messageId: string | number, citationIndex: number) => {
    const key = `${messageId}-${citationIndex}`;
    // Toggle - if same citation clicked, collapse it; otherwise expand the new one
    setExpandedCitation(prev => prev === key ? null : key);
    onCitationClick(messageId, citationIndex);
  };
  
  return (
    <div className="flex justify-start">
      <div className="max-w-[85%] rounded-2xl px-4 py-3 bg-muted">
        {/* Header */}
        <div className="flex items-center gap-2 mb-2">
          <Sparkles className="h-4 w-4 text-primary" />
          <span className="text-xs font-semibold text-primary">
            AI Assistant
          </span>
          {isActivelyStreaming ? (
            <Badge variant="secondary" className="text-xs animate-pulse">
              {isReasoning ? 'Thinking' : 'Streaming'}
            </Badge>
          ) : onRegenerate ? (
            <button
              onClick={onRegenerate}
              className="ml-auto p-1 rounded hover:bg-accent transition-colors"
              title="Regenerate response"
            >
              <RefreshCw className="h-3.5 w-3.5 text-muted-foreground hover:text-foreground" />
            </button>
          ) : null}
        </div>
        
        {/* Reasoning Section - Collapsible */}
        {reasoning && (
          <Reasoning isStreaming={isReasoning} defaultOpen={true}>
            <ReasoningTrigger />
            <ReasoningContent>{reasoning}</ReasoningContent>
          </Reasoning>
        )}
        
        {/* Tool Execution Section */}
        {toolEvents.length > 0 && (() => {
          // Get tool info from events
          const startEvent = toolEvents.find(e => e.type === 'tool_start');
          const endEvent = toolEvents.find(e => e.type === 'tool_end' || e.type === 'tool_error');
          const toolName = startEvent?.tool_name || 'tool';
          
          // Determine tool state
          let toolState: ToolState = 'pending';
          if (isToolRunning) {
            toolState = 'running';
          } else if (endEvent) {
            toolState = endEvent.type === 'tool_error' || !endEvent.success ? 'error' : 'completed';
          }
          
          // Note: searchResults are extracted at component level via useMemo
          
          return (
            <Tool state={toolState} defaultOpen={isToolRunning}>
              <ToolHeader toolName={toolName} />
              <ToolContent>
                <ToolInput input={startEvent?.arguments} />
                {toolState === 'error' && endEvent?.error && (
                  <ToolOutput errorText={endEvent.error} />
                )}
                {toolState === 'completed' && searchResults.length > 0 && (
                  <ToolSearchResults 
                    results={searchResults} 
                    query={startEvent?.arguments?.query as string}
                  />
                )}
                {toolState === 'completed' && searchResults.length === 0 && endEvent?.result != null && (
                  <ToolOutput output={JSON.stringify(endEvent.result as object, null, 2)} />
                )}
              </ToolContent>
            </Tool>
          );
        })()}
        
        {/* Content Section - show cursor only while actively streaming */}
        {content ? (
          <div className={isActivelyStreaming ? "streaming-cursor" : ""}>
            {citations && citations.length > 0 ? (
              <ContentWithCitations
                content={content}
                messageId={streamingMessageId}
                citations={citations}
                onCitationClick={handleLocalCitationClick}
                isStreaming={true}
              />
            ) : (
              <Streamdown 
                mode="streaming"
                parseIncompleteMarkdown
                controls={{ table: true, code: true }}
                className="w-full max-w-full overflow-hidden text-sm [&>*:first-child]:mt-0 [&>*:last-child]:mb-0"
                remarkPlugins={[
                  remarkGfm,
                  [remarkMath, { singleDollarTextMath: false }]
                ]}
                rehypePlugins={[rehypeKatex]}
              >
                {addCitationLinks(processContentForStreamdown(content), searchResults)}
              </Streamdown>
            )}
          </div>
        ) : isReasoning && reasoning ? null : isActivelyStreaming ? (
          <div className="flex items-center gap-2">
            <Loader className="text-primary" size={16} />
            <span className="text-sm text-muted-foreground">
              Generating response...
            </span>
          </div>
        ) : null}
        
        {/* Citations - Only show citations that are actually referenced in the content */}
        {citedCitations.length > 0 && (
          <div className="mt-3 pt-2 border-t border-border/50">
            <div className="flex items-center gap-2 mb-2">
              <Library className="h-3.5 w-3.5 text-primary" />
              <p className="text-xs font-medium">
                Sources ({citedCitations.length})
              </p>
            </div>
            <div className={cn(
              "space-y-2",
              citedCitations.length > 2 && "max-h-[120px] overflow-y-auto pr-1 scrollbar-thin scrollbar-thumb-border scrollbar-track-transparent"
            )}>
              {citedCitations.map((citation, idx) => {
                const originalIndex = originalIndices[idx] - 1; // Convert back to 0-based
                return (
                  <CitationItem 
                    key={originalIndex} 
                    citation={citation} 
                    index={originalIndex}
                    messageId={streamingMessageId}
                    isExpanded={expandedCitation === `${streamingMessageId}-${originalIndex}`}
                    onToggle={() => handleLocalCitationClick(streamingMessageId, originalIndex)}
                  />
                );
              })}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

// Empty State Component
interface EmptyStateProps {
  subjectId: string;
  subjectName: string;
  onSelectQuestion: (question: string) => void;
}

function EmptyState({ subjectId, subjectName, onSelectQuestion }: EmptyStateProps) {
  // Fetch PYQs to get recent questions for suggestions
  const { data: pyqsData } = usePYQs(subjectId || null);
  
  // Get the most recent paper ID to fetch its questions
  const papers = pyqsData?.papers || [];
  const recentPapers = papers
    .filter((p) => p.extraction_status === 'completed')
    .sort((a, b) => b.year - a.year)
    .slice(0, 2); // Get top 2 most recent papers
  
  // Fetch questions from the most recent paper
  const { data: recentPaper1 } = usePYQById(recentPapers[0]?.id?.toString() || null);
  const { data: recentPaper2 } = usePYQById(recentPapers[1]?.id?.toString() || null);
  
  // Collect up to 3 questions from recent papers
  const recentQuestions = useMemo(() => {
    const questions: { text: string; year: number; month?: string }[] = [];
    
    if (recentPaper1?.questions) {
      for (const q of recentPaper1.questions.slice(0, 2)) {
        questions.push({
          text: q.question_text.length > 80 
            ? q.question_text.slice(0, 80) + '...' 
            : q.question_text,
          year: recentPaper1.year,
          month: recentPaper1.month,
        });
      }
    }
    
    if (recentPaper2?.questions && questions.length < 3) {
      for (const q of recentPaper2.questions.slice(0, 3 - questions.length)) {
        questions.push({
          text: q.question_text.length > 80 
            ? q.question_text.slice(0, 80) + '...' 
            : q.question_text,
          year: recentPaper2.year,
          month: recentPaper2.month,
        });
      }
    }
    
    return questions.slice(0, 3);
  }, [recentPaper1, recentPaper2]);
  
  // Default suggestions if no PYQ questions available
  const defaultSuggestions = [
    'Explain the key concepts',
    'What are the main topics?',
    'Help me understand Unit 1',
  ];
  
  const hasPYQQuestions = recentQuestions.length > 0;

  return (
    <div className="flex flex-col items-center justify-center py-12 text-center">
      <div className="mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-primary/10">
        <Sparkles className="h-8 w-8 text-primary" />
      </div>
      <h3 className="text-xl font-semibold mb-2">
        Start learning about {subjectName}
      </h3>
      <p className="text-muted-foreground max-w-md">
        Ask any question about the subject. The AI will use course materials and syllabus to provide accurate, contextual answers with citations.
      </p>
      
      {/* Suggestion chips */}
      <div className="mt-6 flex flex-col items-center gap-3 w-full max-w-lg">
        {hasPYQQuestions && (
          <p className="text-xs text-muted-foreground">
            <FileQuestion className="h-3 w-3 inline mr-1" />
            Recent exam questions:
          </p>
        )}
        <div className="flex flex-wrap justify-center gap-2">
          {hasPYQQuestions ? (
            recentQuestions.map((q, idx) => (
              <SuggestionChip 
                key={idx} 
                onClick={() => onSelectQuestion(`Explain: "${q.text}"`)}
              >
                <span className="line-clamp-1">{q.text}</span>
                <Badge variant="secondary" className="ml-2 text-[10px] shrink-0">
                  {q.year}
                </Badge>
              </SuggestionChip>
            ))
          ) : (
            defaultSuggestions.map((suggestion, idx) => (
              <SuggestionChip 
                key={idx}
                onClick={() => onSelectQuestion(suggestion)}
              >
                {suggestion}
              </SuggestionChip>
            ))
          )}
        </div>
      </div>
    </div>
  );
}

function SuggestionChip({ 
  children, 
  onClick 
}: { 
  children: React.ReactNode;
  onClick?: () => void;
}) {
  return (
    <Badge 
      variant="outline" 
      className="cursor-pointer hover:bg-muted transition-colors px-3 py-1.5 max-w-full"
      onClick={onClick}
    >
      {children}
    </Badge>
  );
}
