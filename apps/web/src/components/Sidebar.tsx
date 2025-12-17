'use client';

import { MessageSquare, History, BookOpen, Settings, LogOut, Bell } from 'lucide-react';
import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { ThemeToggle } from '@/components/theme-toggle';
import { authService } from '@/lib/api/auth';
import { useQueryClient } from '@tanstack/react-query';
import { useNotifications } from '@/providers/notification-provider';

interface SidebarContentProps {
  onNavigate?: () => void;
}

// Shared sidebar content - used by both desktop sidebar and mobile drawer
export function SidebarContent({ onNavigate }: SidebarContentProps) {
  const pathname = usePathname();
  const router = useRouter();
  const queryClient = useQueryClient();
  const { unreadCount } = useNotifications();
  
  const tabs = [
    { href: '/chat', icon: MessageSquare, label: 'Chat' },
    { href: '/history', icon: History, label: 'History' },
    { href: '/courses', icon: BookOpen, label: 'Courses' },
    { href: '/notifications', icon: Bell, label: 'Notifications', badge: unreadCount },
    { href: '/settings', icon: Settings, label: 'Settings' },
  ];

  const handleLogout = async () => {
    try {
      await authService.logout();
      queryClient.clear();
      router.push('/');
    } catch (error) {
      console.error('Logout failed:', error);
    }
  };

  const handleNavClick = () => {
    onNavigate?.();
  };

  return (
    <>
      <div className="p-6 border-b border-border">
        <div className="flex items-center justify-between">
          <h1 className="text-foreground flex items-center gap-2">
            Study in Woods 🪵
          </h1>
          <ThemeToggle />
        </div>
      </div>

      <nav className="flex-1 p-4 space-y-2">
        {tabs.map((tab) => {
          const Icon = tab.icon;
          const isActive = pathname === tab.href;
          return (
            <Link key={tab.href} href={tab.href} onClick={handleNavClick}>
              <Button
                variant={isActive ? 'default' : 'ghost'}
                className="w-full justify-start"
              >
                <Icon className="mr-3 h-5 w-5" />
                {tab.label}
                {tab.badge !== undefined && tab.badge > 0 && (
                  <Badge 
                    variant="destructive" 
                    className="ml-auto h-5 min-w-5 px-1.5 text-xs"
                  >
                    {tab.badge > 99 ? '99+' : tab.badge}
                  </Badge>
                )}
              </Button>
            </Link>
          );
        })}
      </nav>

      {/* Logout Button */}
      <div className="p-4 border-t border-border">
        <Button
          variant="ghost"
          className="w-full justify-start text-destructive hover:text-destructive hover:bg-destructive/10"
          onClick={handleLogout}
        >
          <LogOut className="mr-3 h-5 w-5" />
          Logout
        </Button>
      </div>
    </>
  );
}

// Desktop sidebar - hidden on mobile
export function Sidebar() {
  return (
    <aside className="hidden md:flex w-64 border-r border-border bg-background flex-col">
      <SidebarContent />
    </aside>
  );
}
