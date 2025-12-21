'use client';

import React, { createContext, useContext, useMemo, useState, useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import { authService, type User } from '@/lib/api/auth';

interface AuthContextType {
  user: User | null;
  isLoading: boolean;
  isAuthenticated: boolean;
  isAdmin: boolean;
  refetchUser: () => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  // Track if we're on the client and if we have a token
  // This prevents hydration mismatches by not enabling the query until after mount
  const [isClientAuthenticated, setIsClientAuthenticated] = useState(false);
  
  useEffect(() => {
    // Only check auth after hydration to prevent SSR/client mismatch
    setIsClientAuthenticated(authService.isAuthenticated());
  }, []);

  const { data: user, isLoading, refetch } = useQuery({
    queryKey: ['user'],
    queryFn: () => authService.getProfile(),
    enabled: isClientAuthenticated, // Only enable after client-side auth check
    retry: false,
    staleTime: 5 * 60 * 1000, // 5 minutes
  });

  // Memoize the context value to prevent unnecessary re-renders
  const value = useMemo<AuthContextType>(() => ({
    user: user || null,
    // Show loading while we're checking auth status or fetching user
    isLoading: !isClientAuthenticated ? false : isLoading,
    isAuthenticated: !!user,
    isAdmin: user?.role === 'admin',
    refetchUser: refetch,
  }), [user, isLoading, isClientAuthenticated, refetch]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}
