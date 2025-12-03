'use client';

import { useState, useEffect, useRef } from 'react';
import { Send, Sparkles } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import {
  useChatMessages,
  useSendMessage,
} from '@/lib/api/hooks/useChat';
import { retrievePendingQuery, clearPendingQuery } from '@/lib/utils/sessionStorage';
import { useAuth } from '@/providers/auth-provider';
import ReactMarkdown from 'react-markdown';

export function ChatTab() {
  const { user } = useAuth();
  const [currentSessionId, setCurrentSessionId] = useState<string | null>(null);
  const [input, setInput] = useState('');
  const scrollRef = useRef<HTMLDivElement>(null);

  // Hooks
  const { data: messages = [], isLoading: messagesLoading } = useChatMessages(currentSessionId);
  const sendMessageMutation = useSendMessage();

  // Check for pending question from landing page
  useEffect(() => {
    const pendingQuery = retrievePendingQuery();
    if (pendingQuery) {
      setInput(pendingQuery.question);
      clearPendingQuery();
    }
  }, []);

  // Auto-scroll to bottom when new messages arrive
  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages]);



  const handleSend = async () => {
    if (!input.trim()) return;

    const messageText = input.trim();
    setInput('');

    try {
      const response = await sendMessageMutation.mutateAsync({
        session_id: currentSessionId || undefined,
        message: messageText,
        context: user
          ? {
              university_id: user.university_id,
              course_id: user.course_id,
              semester: user.semester,
            }
          : undefined,
      });

      // Set current session to the new or existing session
      if (!currentSessionId) {
        setCurrentSessionId(response.session_id);
      }
    } catch (error) {
      console.error('Failed to send message:', error);
      // Error toast is handled by the hook
    }
  };



  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  return (
    <div className="flex h-full">
      {/* Main Chat Area */}
      <div className="flex-1 flex flex-col">
        {/* Header */}
        <div className="border-b bg-card p-4">
          <div>
            <h2 className="text-lg font-semibold flex items-center gap-2">
              <Sparkles className="h-5 w-5 text-primary" />
              AI Study Assistant
            </h2>
            <p className="text-sm text-muted-foreground">
              Ask anything about your courses and syllabus
            </p>
          </div>
        </div>

        {/* Messages */}
        <ScrollArea className="flex-1 p-6" ref={scrollRef}>
          {!currentSessionId ? (
            <div className="flex items-center justify-center h-full">
              <div className="text-center space-y-4 max-w-md">
                <Sparkles className="h-16 w-16 text-primary mx-auto" />
                <h3 className="text-2xl font-bold">Welcome to Study in Woods</h3>
                <p className="text-muted-foreground">
                  Ask a question below to begin learning with AI
                </p>
              </div>
            </div>
          ) : messagesLoading ? (
            <div className="flex items-center justify-center h-full">
              <div className="animate-spin rounded-full h-12 w-12 border-t-2 border-b-2 border-primary" />
            </div>
          ) : (
            <div className="space-y-6 max-w-4xl mx-auto">
              {messages.map((message) => (
                <div
                  key={message.id}
                  className={`flex ${message.role === 'user' ? 'justify-end' : 'justify-start'}`}
                >
                  <div
                    className={`max-w-[85%] rounded-2xl p-4 ${
                      message.role === 'user'
                        ? 'bg-primary text-primary-foreground'
                        : 'bg-muted'
                    }`}
                  >
                    {message.role === 'assistant' && (
                      <div className="flex items-center gap-2 mb-2">
                        <Sparkles className="h-4 w-4 text-primary" />
                        <span className="text-xs font-semibold text-primary">
                          AI Assistant
                        </span>
                      </div>
                    )}
                    <div className="prose prose-sm dark:prose-invert max-w-none">
                      <ReactMarkdown>{message.content}</ReactMarkdown>
                    </div>
                    <p className="text-xs opacity-70 mt-2">
                      {new Date(message.created_at).toLocaleTimeString()}
                    </p>
                  </div>
                </div>
              ))}
              
              {sendMessageMutation.isPending && (
                <div className="flex justify-start">
                  <div className="bg-muted rounded-2xl p-4 max-w-[85%]">
                    <div className="flex items-center gap-2">
                      <div className="animate-spin rounded-full h-4 w-4 border-t-2 border-b-2 border-primary" />
                      <span className="text-sm text-muted-foreground">
                        AI is thinking...
                      </span>
                    </div>
                  </div>
                </div>
              )}
            </div>
          )}
        </ScrollArea>

        {/* Input */}
        <div className="border-t bg-card p-4">
          <div className="max-w-4xl mx-auto">
            <div className="flex gap-2">
              <Input
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={handleKeyPress}
                placeholder="Ask a question about your course..."
                disabled={sendMessageMutation.isPending}
                className="flex-1"
              />
              <Button
                onClick={handleSend}
                disabled={!input.trim() || sendMessageMutation.isPending}
                size="icon"
              >
                <Send className="h-4 w-4" />
              </Button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
