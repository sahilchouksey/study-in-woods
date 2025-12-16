'use client';

import { useAuth } from '@/providers/auth-provider';
import { useRouter } from 'next/navigation';
import { useEffect } from 'react';
import { LoadingSpinner } from '@/components/ui/loading-spinner';

interface ProtectedRouteProps {
  children: React.ReactNode;
  requireAdmin?: boolean;
  fallbackPath?: string;
}

/**
 * Protected route wrapper that requires authentication
 * Optionally can require admin role
 */
export function ProtectedRoute({
  children,
  requireAdmin = false,
  fallbackPath = '/login',
}: ProtectedRouteProps) {
  const router = useRouter();
  const { isAuthenticated, isAdmin, isLoading } = useAuth();

  useEffect(() => {
    if (!isLoading) {
      // Not authenticated - redirect to login
      if (!isAuthenticated) {
        router.push(fallbackPath);
        return;
      }

      // Authenticated but not admin when admin is required
      if (requireAdmin && !isAdmin) {
        router.push('/dashboard'); // Redirect to dashboard instead of login
      }
    }
  }, [isAuthenticated, isAdmin, isLoading, requireAdmin, fallbackPath, router]);

  // Show loading state
  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <LoadingSpinner size="xl" />
      </div>
    );
  }

  // Not authenticated
  if (!isAuthenticated) {
    return null;
  }

  // Authenticated but not admin when admin is required
  if (requireAdmin && !isAdmin) {
    return (
      <div className="flex items-center justify-center min-h-screen p-8">
        <div className="text-center space-y-4">
          <h1 className="text-2xl font-bold">Access Denied</h1>
          <p className="text-muted-foreground">
            You don't have permission to access this page.
          </p>
        </div>
      </div>
    );
  }

  // Render children if authenticated (and admin if required)
  return <>{children}</>;
}
