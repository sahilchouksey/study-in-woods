import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  documentService,
  type DocumentType,
  type DocumentListParams,
  type UpdateDocumentData,
} from '@/lib/api/documents';

// ============= Document Queries =============

/**
 * Hook to get documents for a subject
 */
export function useDocuments(subjectId: string | null, params?: DocumentListParams) {
  return useQuery({
    queryKey: ['documents', subjectId, params],
    queryFn: () => documentService.getDocuments(subjectId!, params),
    enabled: !!subjectId,
    staleTime: 2 * 60 * 1000, // 2 minutes (shorter for documents as they change more)
  });
}

/**
 * Hook to get a specific document
 */
export function useDocument(subjectId: string | null, documentId: string | null) {
  return useQuery({
    queryKey: ['document', subjectId, documentId],
    queryFn: () => documentService.getDocument(subjectId!, documentId!),
    enabled: !!subjectId && !!documentId,
    staleTime: 2 * 60 * 1000,
  });
}

// ============= Document Mutations =============

/**
 * Hook to upload a document
 */
export function useUploadDocument() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      subjectId,
      file,
      type,
    }: {
      subjectId: string;
      file: File;
      type: DocumentType;
    }) => documentService.uploadDocument(subjectId, file, type),
    onSuccess: (_, variables) => {
      // Invalidate documents list for this subject
      queryClient.invalidateQueries({ queryKey: ['documents', variables.subjectId] });
    },
  });
}

/**
 * Hook to update a document
 */
export function useUpdateDocument() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      subjectId,
      documentId,
      data,
    }: {
      subjectId: string;
      documentId: string;
      data: UpdateDocumentData;
    }) => documentService.updateDocument(subjectId, documentId, data),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['documents', variables.subjectId] });
      queryClient.invalidateQueries({
        queryKey: ['document', variables.subjectId, variables.documentId],
      });
    },
  });
}

/**
 * Hook to delete a document
 */
export function useDeleteDocument() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      subjectId,
      documentId,
    }: {
      subjectId: string;
      documentId: string;
    }) => documentService.deleteDocument(subjectId, documentId),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['documents', variables.subjectId] });
    },
  });
}

/**
 * Hook to get download URL for a document
 */
export function useDownloadDocument() {
  return useMutation({
    mutationFn: ({
      subjectId,
      documentId,
      expirationMinutes,
    }: {
      subjectId: string;
      documentId: string;
      expirationMinutes?: number;
    }) => documentService.getDownloadURL(subjectId, documentId, expirationMinutes),
  });
}

/**
 * Hook to refresh indexing status of a document
 */
export function useRefreshIndexingStatus() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      subjectId,
      documentId,
    }: {
      subjectId: string;
      documentId: string;
    }) => documentService.refreshIndexingStatus(subjectId, documentId),
    onSuccess: (data, variables) => {
      // Update the document in cache
      queryClient.setQueryData(
        ['document', variables.subjectId, variables.documentId],
        data
      );
      // Also invalidate the list to reflect the new status
      queryClient.invalidateQueries({ queryKey: ['documents', variables.subjectId] });
    },
  });
}
