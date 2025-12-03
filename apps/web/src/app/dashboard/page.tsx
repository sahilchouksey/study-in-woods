'use client';

import { ProtectedRoute } from '@/components/ProtectedRoute';
import { useState, useEffect } from 'react';
import { retrievePendingQuery, clearPendingQuery } from '@/lib/utils/sessionStorage';
import { useAuth } from '@/providers/auth-provider';
import { ChatTab } from '@/components/ChatTab';
import { CoursesTab } from '@/components/CoursesTab';
import { HistoryTab } from '@/components/HistoryTab';
import { SettingsTab } from '@/components/SettingsTab';

export default function DashboardPage() {
  return (
    <ProtectedRoute>
      <DashboardContent />
    </ProtectedRoute>
  );
}

function DashboardContent() {
  const { user } = useAuth();
  const [activeTab, setActiveTab] = useState('chat');

  // Check for pending question from landing page and clear it
  useEffect(() => {
    const pendingQuery = retrievePendingQuery();
    if (pendingQuery) {
      // TODO: Pass question to ChatTab when we implement chat functionality
      clearPendingQuery();
      setActiveTab('chat');
    }
  }, []);

  return (
    <div className="h-full flex flex-col">
      {/* Welcome Header */}
      <div className="border-b bg-card p-4">
        <h1 className="text-2xl font-bold">
          Welcome back, {user?.name?.split(' ')[0] || 'User'}!
        </h1>
        <p className="text-sm text-muted-foreground">
          {user?.university_id 
            ? 'Your personalized AI learning assistant is ready'
            : 'Complete your profile to get personalized assistance'}
        </p>
      </div>

      {/* Tab Content */}
      <div className="flex-1 overflow-hidden">
        {activeTab === 'chat' && <ChatTab />}
        {activeTab === 'courses' && <CoursesTab />}
        {activeTab === 'history' && <HistoryTab />}
        {activeTab === 'settings' && <SettingsTab />}
      </div>
    </div>
  );
}
