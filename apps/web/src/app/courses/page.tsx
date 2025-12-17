'use client';

import { Suspense } from 'react';
import { CoursesTab } from '@/components/CoursesTab';

export default function CoursesPage() {
  return (
    <div className="h-full">
      <Suspense fallback={<div className="flex items-center justify-center h-full">Loading...</div>}>
        <CoursesTab />
      </Suspense>
    </div>
  );
}