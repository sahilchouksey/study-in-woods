'use client';

import { usePathname } from 'next/navigation';
import { Sidebar } from './Sidebar';

// Routes where sidebar should be shown (authenticated routes)
const SIDEBAR_ROUTES = ['/dashboard', '/chat', '/history', '/courses', '/notifications', '/settings'];

export function ConditionalSidebar({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  
  // Check if current route should show sidebar
  const showSidebar = SIDEBAR_ROUTES.some(route => pathname.startsWith(route));

  if (!showSidebar) {
    // No sidebar - full width layout
    return <>{children}</>;
  }

  // With sidebar - split layout
  return (
    <div className="flex h-screen bg-background">
      <Sidebar />
      <main className="flex-1 h-full overflow-hidden">
        {children}
      </main>
    </div>
  );
}
