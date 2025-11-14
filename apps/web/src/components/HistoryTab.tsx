'use client';

import { useState } from 'react';
import { MessageSquare, Calendar, Tag } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Input } from '@/components/ui/input';

interface ChatHistory {
  id: string;
  title: string;
  date: string;
  messageCount: number;
  tags: {
    semester: string;
    units: string[];
    topics: string[];
  };
}

const mockHistory: ChatHistory[] = [
  {
    id: '1',
    title: 'Data Structures and Algorithms Discussion',
    date: '2025-10-19',
    messageCount: 12,
    tags: {
      semester: 'Semester 1',
      units: ['Unit 2', 'Unit 3'],
      topics: ['Arrays', 'Linked Lists', 'Binary Trees'],
    },
  },
  {
    id: '2',
    title: 'Database Management Systems',
    date: '2025-10-18',
    messageCount: 8,
    tags: {
      semester: 'Semester 1',
      units: ['Unit 4'],
      topics: ['SQL', 'Normalization', 'Transactions'],
    },
  },
  {
    id: '3',
    title: 'Operating Systems Concepts',
    date: '2025-10-17',
    messageCount: 15,
    tags: {
      semester: 'Semester 3',
      units: ['Unit 1', 'Unit 2'],
      topics: ['Process Scheduling', 'Deadlocks', 'Memory Management'],
    },
  },
  {
    id: '4',
    title: 'Computer Networks PYQ Solutions',
    date: '2025-10-16',
    messageCount: 20,
    tags: {
      semester: 'Semester 3',
      units: ['Unit 5'],
      topics: ['TCP/IP', 'Routing', 'Network Security'],
    },
  },
];

export function HistoryTab() {
  const [searchQuery, setSearchQuery] = useState('');
  
  const filteredHistory = mockHistory.filter((chat) =>
    chat.title.toLowerCase().includes(searchQuery.toLowerCase()) ||
    chat.tags.topics.some((topic) =>
      topic.toLowerCase().includes(searchQuery.toLowerCase())
    )
  );

  return (
    <div className="flex flex-col h-full">
      <div className="border-b border-border p-6">
        <h2 className="text-foreground">Chat History</h2>
        <p className="text-muted-foreground mt-1">Review your previous conversations</p>
      </div>

      <div className="p-6 border-b border-border">
        <Input
          placeholder="Search conversations..."
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          className="w-full"
        />
      </div>

      <ScrollArea className="flex-1">
        <div className="p-6 space-y-4">
          {filteredHistory.map((chat) => (
            <div
              key={chat.id}
              className="border border-border rounded-lg p-4 hover:bg-muted cursor-pointer transition-colors"
            >
              <div className="flex items-start justify-between mb-3">
                <h3 className="font-medium text-foreground">{chat.title}</h3>
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                  <Calendar className="h-4 w-4" />
                  {new Date(chat.date).toLocaleDateString()}
                </div>
              </div>

              <div className="flex items-center gap-4 mb-3 text-sm text-muted-foreground">
                <div className="flex items-center gap-1">
                  <MessageSquare className="h-4 w-4" />
                  {chat.messageCount} messages
                </div>
                <Badge variant="outline" className="text-xs">
                  {chat.tags.semester}
                </Badge>
              </div>

              <div className="space-y-2">
                <div className="flex flex-wrap gap-1">
                  <span className="text-sm font-medium text-muted-foreground">Units:</span>
                  {chat.tags.units.map((unit) => (
                    <Badge key={unit} variant="secondary" className="text-xs">
                      {unit}
                    </Badge>
                  ))}
                </div>

                <div className="flex flex-wrap gap-1">
                  <span className="text-sm font-medium text-muted-foreground">Topics:</span>
                  {chat.tags.topics.map((topic) => (
                    <Badge key={topic} variant="outline" className="text-xs">
                      <Tag className="h-3 w-3 mr-1" />
                      {topic}
                    </Badge>
                  ))}
                </div>
              </div>
            </div>
          ))}
        </div>
      </ScrollArea>
    </div>
  );
}