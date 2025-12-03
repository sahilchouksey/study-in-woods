/**
 * Session storage utilities for preserving user questions through auth flow
 */

const PENDING_QUERY_KEY = 'study_woods_pending_query';

export interface PendingQuery {
  question: string;
  timestamp: number;
}

/**
 * Store a pending query in sessionStorage
 */
export function storePendingQuery(question: string): void {
  if (typeof window === 'undefined') return;
  
  const query: PendingQuery = {
    question,
    timestamp: Date.now(),
  };
  
  sessionStorage.setItem(PENDING_QUERY_KEY, JSON.stringify(query));
}

/**
 * Retrieve pending query from sessionStorage
 * Returns null if no query or if expired (>30 minutes)
 */
export function retrievePendingQuery(): PendingQuery | null {
  if (typeof window === 'undefined') return null;
  
  const stored = sessionStorage.getItem(PENDING_QUERY_KEY);
  if (!stored) return null;
  
  try {
    const query: PendingQuery = JSON.parse(stored);
    
    // Check if expired (30 minutes)
    const thirtyMinutes = 30 * 60 * 1000;
    if (Date.now() - query.timestamp > thirtyMinutes) {
      clearPendingQuery();
      return null;
    }
    
    return query;
  } catch (error) {
    console.error('Failed to parse pending query:', error);
    clearPendingQuery();
    return null;
  }
}

/**
 * Clear pending query from sessionStorage
 */
export function clearPendingQuery(): void {
  if (typeof window === 'undefined') return;
  sessionStorage.removeItem(PENDING_QUERY_KEY);
}
