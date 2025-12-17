'use client';

import { useState } from 'react';
import { usePathname } from 'next/navigation';
import { Menu } from 'lucide-react';
import { Sidebar, SidebarContent } from './Sidebar';
import { Button } from '@/components/ui/button';
import {
  Sheet,
  SheetContent,
  SheetTitle,
} from '@/components/ui/sheet';

// Routes where sidebar should be shown (authenticated routes)
const SIDEBAR_ROUTES = ['/dashboard', '/chat', '/history', '/courses', '/notifications', '/settings'];

export function ConditionalSidebar({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  
  // Check if current route should show sidebar
  const showSidebar = SIDEBAR_ROUTES.some(route => pathname.startsWith(route));

  if (!showSidebar) {
    // No sidebar - full width layout
    return <>{children}</>;
  }

  // With sidebar - split layout
  return (
    <div className="flex h-screen bg-background">
      {/* Desktop Sidebar - hidden on mobile */}
      <Sidebar />
      
      {/* Mobile Sheet/Drawer */}
      <Sheet open={mobileMenuOpen} onOpenChange={setMobileMenuOpen}>
        <SheetContent side="left" className="w-64 p-0 flex flex-col">
          <SheetTitle className="sr-only">Navigation Menu</SheetTitle>
          <SidebarContent onNavigate={() => setMobileMenuOpen(false)} />
        </SheetContent>
      </Sheet>
      
      {/* Main Content Area */}
      <main className="flex-1 h-full overflow-hidden flex flex-col">
        {/* Mobile Header with Menu Button - visible only on mobile */}
        <div className="md:hidden flex items-center gap-3 p-4 border-b border-border bg-background">
          <Button
            variant="ghost"
            size="icon"
            onClick={() => setMobileMenuOpen(true)}
            aria-label="Open menu"
          >
            <Menu className="h-5 w-5" />
          </Button>
          <span className="font-medium text-foreground">Study in Woods 🪵</span>
        </div>
        
        {/* Page Content */}
        <div className="flex-1 overflow-hidden">
          {children}
        </div>
      </main>
    </div>
  );
}
