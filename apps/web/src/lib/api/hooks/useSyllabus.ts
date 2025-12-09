import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { syllabusService } from '@/lib/api/syllabus';

// ============= Syllabus Queries =============

/**
 * Hook to get syllabus for a subject
 */
export function useSyllabus(subjectId: string | null) {
  return useQuery({
    queryKey: ['syllabus', 'subject', subjectId],
    queryFn: () => syllabusService.getSyllabusBySubject(subjectId!),
    enabled: !!subjectId,
    staleTime: 5 * 60 * 1000, // 5 minutes
    retry: false, // Don't retry on 404
  });
}

/**
 * Hook to get syllabus by ID
 */
export function useSyllabusById(syllabusId: string | null) {
  return useQuery({
    queryKey: ['syllabus', syllabusId],
    queryFn: () => syllabusService.getSyllabusById(syllabusId!),
    enabled: !!syllabusId,
    staleTime: 5 * 60 * 1000,
  });
}

/**
 * Hook to get extraction status with polling
 */
export function useExtractionStatus(syllabusId: string | null, shouldPoll: boolean = false) {
  return useQuery({
    queryKey: ['syllabus', syllabusId, 'status'],
    queryFn: () => syllabusService.getExtractionStatus(syllabusId!),
    enabled: !!syllabusId,
    refetchInterval: shouldPoll ? 3000 : false, // Poll every 3 seconds when processing
    staleTime: 1000,
  });
}

/**
 * Hook to search topics in a subject's syllabus
 */
export function useSearchTopics(subjectId: string | null, query: string) {
  return useQuery({
    queryKey: ['syllabus', 'search', subjectId, query],
    queryFn: () => syllabusService.searchTopics(subjectId!, query),
    enabled: !!subjectId && query.length >= 2,
    staleTime: 30 * 1000, // 30 seconds
  });
}

// ============= Syllabus Mutations =============

/**
 * Hook to trigger syllabus extraction
 */
export function useExtractSyllabus() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ documentId }: { documentId: string; subjectId: string }) =>
      syllabusService.extractSyllabus(documentId),
    onSuccess: (data, variables) => {
      // Invalidate syllabus query for this subject
      queryClient.invalidateQueries({ queryKey: ['syllabus', 'subject', variables.subjectId] });
      // Also set the data directly
      queryClient.setQueryData(['syllabus', 'subject', variables.subjectId], data);
    },
  });
}

/**
 * Hook to retry failed extraction
 */
export function useRetryExtraction() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ syllabusId }: { syllabusId: string; subjectId: string }) =>
      syllabusService.retryExtraction(syllabusId),
    onSuccess: (data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['syllabus', 'subject', variables.subjectId] });
      queryClient.setQueryData(['syllabus', 'subject', variables.subjectId], data);
    },
  });
}

/**
 * Hook to delete syllabus
 */
export function useDeleteSyllabus() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ syllabusId }: { syllabusId: string; subjectId: string }) =>
      syllabusService.deleteSyllabus(syllabusId),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['syllabus', 'subject', variables.subjectId] });
      queryClient.setQueryData(['syllabus', 'subject', variables.subjectId], null);
    },
  });
}
