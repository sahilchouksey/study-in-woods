import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { pyqService } from '@/lib/api/pyq';

// ============= PYQ Queries =============

/**
 * Hook to get PYQ papers for a subject
 */
export function usePYQs(subjectId: string | null) {
  return useQuery({
    queryKey: ['pyqs', 'subject', subjectId],
    queryFn: () => pyqService.getPYQsBySubject(subjectId!),
    enabled: !!subjectId,
    staleTime: 5 * 60 * 1000, // 5 minutes
  });
}

/**
 * Hook to get PYQ paper by ID (with questions and choices)
 */
export function usePYQById(pyqId: string | null) {
  return useQuery({
    queryKey: ['pyqs', pyqId],
    queryFn: () => pyqService.getPYQById(pyqId!),
    enabled: !!pyqId,
    staleTime: 5 * 60 * 1000,
  });
}

/**
 * Hook to get questions for a PYQ paper
 */
export function usePYQQuestions(pyqId: string | null) {
  return useQuery({
    queryKey: ['pyqs', pyqId, 'questions'],
    queryFn: () => pyqService.getQuestions(pyqId!),
    enabled: !!pyqId,
    staleTime: 5 * 60 * 1000,
  });
}

/**
 * Hook to get extraction status with polling
 */
export function usePYQExtractionStatus(pyqId: string | null, shouldPoll: boolean = false) {
  return useQuery({
    queryKey: ['pyqs', pyqId, 'status'],
    queryFn: () => pyqService.getExtractionStatus(pyqId!),
    enabled: !!pyqId,
    refetchInterval: shouldPoll ? 3000 : false, // Poll every 3 seconds when processing
    staleTime: 1000,
  });
}

/**
 * Hook to search questions in a subject's PYQs
 */
export function useSearchPYQQuestions(subjectId: string | null, query: string) {
  return useQuery({
    queryKey: ['pyqs', 'search', subjectId, query],
    queryFn: () => pyqService.searchQuestions(subjectId!, query),
    enabled: !!subjectId && query.length >= 2,
    staleTime: 30 * 1000, // 30 seconds
  });
}

// ============= PYQ Mutations =============

/**
 * Hook to trigger PYQ extraction
 */
export function useExtractPYQ() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ documentId, async = true }: { documentId: string; subjectId: string; async?: boolean }) =>
      pyqService.extractPYQ(documentId, async),
    onSuccess: (_, variables) => {
      // Invalidate PYQ query for this subject
      queryClient.invalidateQueries({ queryKey: ['pyqs', 'subject', variables.subjectId] });
    },
  });
}

/**
 * Hook to retry failed extraction
 */
export function useRetryPYQExtraction() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ pyqId, async = false }: { pyqId: string; subjectId: string; async?: boolean }) =>
      pyqService.retryExtraction(pyqId, async),
    onSuccess: (data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['pyqs', 'subject', variables.subjectId] });
      if (data.pyq) {
        queryClient.setQueryData(['pyqs', variables.pyqId], data.pyq);
      }
    },
  });
}

/**
 * Hook to delete PYQ paper
 */
export function useDeletePYQ() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ pyqId }: { pyqId: string; subjectId: string }) =>
      pyqService.deletePYQ(pyqId),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['pyqs', 'subject', variables.subjectId] });
      queryClient.removeQueries({ queryKey: ['pyqs', variables.pyqId] });
    },
  });
}
