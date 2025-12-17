'use client';


import { ChatSetup } from '@/components/chat/ChatSetup';

export default function ChatPage() {
  // The /chat page shows the setup screen
  // When a session is created, it navigates to /chat/[sessionId]
  return <ChatSetup />;
}
