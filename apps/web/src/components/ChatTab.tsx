'use client';

import { useState } from 'react';
import { Send, Sparkles } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Badge } from '@/components/ui/badge';
import { ScrollArea } from '@/components/ui/scroll-area';

interface Message {
  id: string;
  type: 'user' | 'ai';
  content: string;
  unit?: string;
  topics?: string[];
}

export function ChatTab() {
  const [selectedCourse, setSelectedCourse] = useState('mca');
  const [messages, setMessages] = useState<Message[]>([
    {
      id: '1',
      type: 'ai',
      content: "Hello! I'm your AI study assistant for MCA. Ask me anything about your syllabus or previous year questions. I can help you understand concepts, solve problems, and prepare for exams.",
    },
  ]);
  const [input, setInput] = useState('');

  const handleSend = () => {
    if (!input.trim()) return;

    const userMessage: Message = {
      id: Date.now().toString(),
      type: 'user',
      content: input,
    };

    // Mock AI response
    const aiMessage: Message = {
      id: (Date.now() + 1).toString(),
      type: 'ai',
      content: `Based on your MCA syllabus and previous year questions, here's what I found:\n\n${getMockResponse(input)}`,
      unit: 'Unit 2',
      topics: ['Data Structures', 'Algorithms'],
    };

    setMessages([...messages, userMessage, aiMessage]);
    setInput('');
  };

  const getMockResponse = (query: string) => {
    if (query.toLowerCase().includes('algorithm')) {
      return "Algorithms are step-by-step procedures for solving problems. In your MCA curriculum, this topic appears frequently in PYQs. Key concepts include:\n\n1. Time Complexity (O-notation)\n2. Sorting Algorithms (QuickSort, MergeSort)\n3. Searching Algorithms (Binary Search)\n4. Dynamic Programming\n\nWould you like me to explain any specific algorithm?";
    }
    return "I understand you're asking about this topic. Let me help you with detailed explanations and examples from your syllabus and previous year questions.";
  };

  return (
    <div className="flex flex-col h-full">
      <div className="border-b border-border p-6">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-foreground">Chat with AI Assistant</h2>
            <p className="text-muted-foreground mt-1">Ask questions about your syllabus and PYQs</p>
          </div>
          <Select value={selectedCourse} onValueChange={setSelectedCourse}>
            <SelectTrigger className="w-48">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="mca">MCA</SelectItem>
              <SelectItem value="bca">BCA</SelectItem>
              <SelectItem value="btech">B.Tech</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      <ScrollArea className="flex-1 p-6">
        <div className="space-y-4">
          {messages.map((message) => (
            <div
              key={message.id}
              className={`flex ${message.type === 'user' ? 'justify-end' : 'justify-start'}`}
            >
              <div
                className={`max-w-[80%] rounded-lg p-4 ${
                  message.type === 'user'
                    ? 'bg-primary text-primary-foreground'
                    : 'bg-muted text-foreground'
                }`}
              >
                {message.type === 'ai' && (
                  <div className="flex items-center gap-2 mb-2">
                    <Sparkles className="h-4 w-4 text-muted-foreground" />
                    <span className="text-sm font-medium text-muted-foreground">AI Assistant</span>
                  </div>
                )}
                <div className="whitespace-pre-wrap">{message.content}</div>
                {message.unit && message.topics && (
                  <div className="flex flex-wrap gap-2 mt-3">
                    <Badge variant="outline" className="text-xs">
                      {message.unit}
                    </Badge>
                    {message.topics.map((topic) => (
                      <Badge key={topic} variant="secondary" className="text-xs">
                        {topic}
                      </Badge>
                    ))}
                  </div>
                )}
              </div>
            </div>
          ))}
        </div>
      </ScrollArea>

      <div className="border-t border-border p-6">
        <div className="flex gap-3">
          <Input
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder="Ask about algorithms, data structures, or any topic..."
            className="flex-1"
            onKeyPress={(e) => e.key === 'Enter' && handleSend()}
          />
          <Button onClick={handleSend} disabled={!input.trim()}>
            <Send className="h-4 w-4" />
          </Button>
        </div>
      </div>
    </div>
  );
}