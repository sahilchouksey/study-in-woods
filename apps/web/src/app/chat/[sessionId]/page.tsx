
'use client';

import { useParams, useRouter } from 'next/navigation';
import { ChatInterface } from '@/components/chat/ChatInterface';
import { useChatSession } from '@/lib/api/hooks/useChat';
import { AlertCircle } from 'lucide-react';
import { LoadingSpinner } from '@/components/ui/loading-spinner';
import { Button } from '@/components/ui/button';
import type { SubjectOption } from '@/lib/api/chat';

export default function ChatSessionPage() {
  const params = useParams();
  const router = useRouter();
  const sessionId = params.sessionId as string;

  const { data: session, isLoading, error } = useChatSession(sessionId);

  const handleBack = () => {
    router.push('/chat');
  };

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-full">
        <LoadingSpinner size="xl" text="Loading chat session..." centered />
      </div>
    );
  }

  if (error || !session) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-center space-y-4 max-w-md">
          <AlertCircle className="h-12 w-12 text-destructive mx-auto" />
          <h3 className="text-lg font-semibold">Session not found</h3>
          <p className="text-muted-foreground">
            This chat session doesn&apos;t exist or you don&apos;t have access to it.
          </p>
          <Button onClick={handleBack}>Start New Chat</Button>
        </div>
      </div>
    );
  }

  // Convert session.subject to SubjectOption format (with defaults for missing fields)
  const subject: SubjectOption | undefined = session.subject ? {
    id: session.subject.id,
    name: session.subject.name,
    code: session.subject.code,
    semester_id: session.subject.semester_id,
    credits: 0, // Not available from session API
    knowledge_base_uuid: '', // Not available from session API
    agent_uuid: '', // Not available from session API
    has_syllabus: false, // Not available from session API
  } : undefined;

  return (
    <ChatInterface
      sessionId={sessionId}
      subject={subject}
      onBack={handleBack}
    />
  );
}
