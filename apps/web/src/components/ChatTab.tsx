'use client';

import { ChatSetup } from '@/components/chat/ChatSetup';

/**
 * ChatTab is now a simple wrapper around ChatSetup.
 * The actual chat interface is rendered at /chat/[sessionId]
 * 
 * @deprecated Use ChatSetup directly or navigate to /chat
 */
export function ChatTab() {
  return <ChatSetup />;
}
