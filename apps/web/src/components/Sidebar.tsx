'use client';

import { MessageSquare, History, BookOpen, Settings, LogOut } from 'lucide-react';
import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { Button } from '@/components/ui/button';
import { ThemeToggle } from '@/components/theme-toggle';
import { authService } from '@/lib/api/auth';
import { useQueryClient } from '@tanstack/react-query';

export function Sidebar() {
  const pathname = usePathname();
  const router = useRouter();
  const queryClient = useQueryClient();
  
  const tabs = [
    { href: '/chat', icon: MessageSquare, label: 'Chat' },
    { href: '/history', icon: History, label: 'History' },
    { href: '/courses', icon: BookOpen, label: 'Courses' },
    { href: '/settings', icon: Settings, label: 'Settings' },
  ];

  const handleLogout = async () => {
    try {
      await authService.logout();
      queryClient.clear(); // Clear all cached data
      router.push('/login');
    } catch (error) {
      console.error('Logout failed:', error);
    }
  };

  return (
    <aside className="w-64 border-r border-border bg-background flex flex-col">
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
            <Link key={tab.href} href={tab.href}>
              <Button
                variant={isActive ? 'default' : 'ghost'}
                className="w-full justify-start"
              >
                <Icon className="mr-3 h-5 w-5" />
                {tab.label}
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
    </aside>
  );
}