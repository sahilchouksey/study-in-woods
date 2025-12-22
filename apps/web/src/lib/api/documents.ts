import { apiClient } from './client';
import type { ApiResponse } from '@/types/api';

/**
 * Document types
 */
export type DocumentType = 'pyq' | 'book' | 'reference' | 'syllabus' | 'notes';

export type IndexingStatus = 
  | 'pending' 
  | 'in_progress' 
  | 'completed' 
  | 'failed' 
  | 'partially_completed';

/**
 * Document interface
 */
export interface Document {
  id: number;
  subject_id: number;
  type: DocumentType;
  filename: string;
  original_url: string;
  spaces_url: string;
  spaces_key: string;
  data_source_id: string;
  indexing_job_id: string;
  indexing_status: IndexingStatus;
  indexing_error?: string;
  file_size: number;
  page_count: number;
  uploaded_by_user_id: number;
  created_at: string;
  updated_at: string;
  subject?: {
    id: number;
    name: string;
    code: string;
  };
  uploaded_by?: {
    id: number;
    email: string;
    name: string;
  };
}

/**
 * Upload document response
 */
export interface UploadDocumentResponse {
  document: Document;
  uploaded_to_spaces: boolean;
  indexed_in_kb: boolean;
}

/**
 * Download URL response
 */
export interface DownloadURLResponse {
  download_url: string;
  expires_in: number;
}

/**
 * Document list query parameters
 */
export interface DocumentListParams {
  page?: number;
  limit?: number;
  type?: DocumentType;
  status?: IndexingStatus;
}

/**
 * Update document data
 */
export interface UpdateDocumentData {
  type?: DocumentType;
  original_url?: string;
}

/**
 * Allowed file extensions for upload
 */
export const ALLOWED_FILE_EXTENSIONS = [
  '.pdf', '.docx', '.doc', '.txt', '.md', 
  '.csv', '.xlsx', '.xls', '.pptx', '.ppt', 
  '.html', '.htm', '.json'
];

/**
 * Maximum file size (50MB)
 */
export const MAX_FILE_SIZE = 50 * 1024 * 1024;

/**
 * Document type display labels
 */
export const DOCUMENT_TYPE_LABELS: Record<DocumentType, string> = {
  pyq: 'Previous Year Questions',
  book: 'Textbook',
  reference: 'Reference Material',
  syllabus: 'Syllabus',
  notes: 'Notes',
};

/**
 * Indexing status display labels and colors
 */
export const INDEXING_STATUS_CONFIG: Record<IndexingStatus, { label: string; color: string }> = {
  pending: { label: 'Pending', color: 'bg-yellow-500' },
  in_progress: { label: 'Processing', color: 'bg-blue-500' },
  completed: { label: 'Indexed', color: 'bg-green-500' },
  failed: { label: 'Failed', color: 'bg-red-500' },
  partially_completed: { label: 'Partial', color: 'bg-orange-500' },
};

/**
 * Paginated documents response (custom type for documents API)
 */
export interface DocumentsListResponse {
  data: Document[];
  total: number;
  page: number;
  limit: number;
  totalPages: number;
}

/**
 * Document service
 */
export const documentService = {
  /**
   * Get documents for a subject
   */
  async getDocuments(
    subjectId: string,
    params?: DocumentListParams
  ): Promise<DocumentsListResponse> {
    const response = await apiClient.get<DocumentsListResponse>(
      `/api/v1/subjects/${subjectId}/documents`,
      { params }
    );
    return response.data;
  },

  /**
   * Get a specific document
   */
  async getDocument(subjectId: string, documentId: string): Promise<Document> {
    const response = await apiClient.get<ApiResponse<Document>>(
      `/api/v1/subjects/${subjectId}/documents/${documentId}`
    );
    return response.data.data!;
  },

  /**
   * Upload a document
   */
  async uploadDocument(
    subjectId: string,
    file: File,
    type: DocumentType
  ): Promise<UploadDocumentResponse> {
    const formData = new FormData();
    formData.append('file', file);
    formData.append('type', type);

    const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';
    const accessToken = typeof window !== 'undefined' ? localStorage.getItem('access_token') : null;
    const url = `${API_URL}/api/v1/subjects/${subjectId}/documents`;

    console.log('[documentService] uploadDocument called:', { subjectId, fileName: file.name, fileSize: file.size, type });

    // Use XMLHttpRequest for reliable file uploads
    return new Promise<UploadDocumentResponse>((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      xhr.open('POST', url, true);
      
      if (accessToken) {
        xhr.setRequestHeader('Authorization', `Bearer ${accessToken}`);
      }
      
      xhr.upload.onprogress = (event) => {
        if (event.lengthComputable) {
          const percent = Math.round((event.loaded / event.total) * 100);
          console.log(`[documentService] Upload progress: ${percent}%`);
        }
      };
      
      xhr.onreadystatechange = () => {
        if (xhr.readyState === XMLHttpRequest.DONE) {
          console.log('[documentService] XHR done, status:', xhr.status);
          if (xhr.status >= 200 && xhr.status < 300) {
            try {
              const data = JSON.parse(xhr.responseText) as ApiResponse<UploadDocumentResponse>;
              resolve(data.data!);
            } catch (e) {
              reject(new Error(`Failed to parse response: ${xhr.responseText}`));
            }
          } else if (xhr.status === 0) {
            reject(new Error('Request cancelled or network error'));
          } else {
            reject(new Error(`Upload failed: ${xhr.status} - ${xhr.responseText}`));
          }
        }
      };
      
      xhr.onerror = () => reject(new Error('Network error'));
      xhr.onabort = () => reject(new Error('Upload aborted'));
      xhr.ontimeout = () => reject(new Error('Upload timed out'));
      xhr.timeout = 300000; // 5 minutes
      
      xhr.send(formData);
    });
  },

  /**
   * Update a document
   */
  async updateDocument(
    subjectId: string,
    documentId: string,
    data: UpdateDocumentData
  ): Promise<Document> {
    const response = await apiClient.put<ApiResponse<Document>>(
      `/api/v1/subjects/${subjectId}/documents/${documentId}`,
      data
    );
    return response.data.data!;
  },

  /**
   * Delete a document
   */
  async deleteDocument(subjectId: string, documentId: string): Promise<void> {
    await apiClient.delete(`/api/v1/subjects/${subjectId}/documents/${documentId}`);
  },

  /**
   * Get download URL for a document
   */
  async getDownloadURL(
    subjectId: string,
    documentId: string,
    expirationMinutes?: number
  ): Promise<DownloadURLResponse> {
    const response = await apiClient.get<ApiResponse<DownloadURLResponse>>(
      `/api/v1/subjects/${subjectId}/documents/${documentId}/download`,
      { params: { expiration: expirationMinutes } }
    );
    return response.data.data!;
  },

  /**
   * Refresh indexing status of a document
   */
  async refreshIndexingStatus(subjectId: string, documentId: string): Promise<Document> {
    const response = await apiClient.post<ApiResponse<Document>>(
      `/api/v1/subjects/${subjectId}/documents/${documentId}/refresh-status`
    );
    return response.data.data!;
  },
};

/**
 * Validate file for upload
 */
export function validateFile(file: File): { valid: boolean; error?: string } {
  // Check file size
  if (file.size > MAX_FILE_SIZE) {
    return {
      valid: false,
      error: `File size exceeds maximum of ${MAX_FILE_SIZE / 1024 / 1024}MB`,
    };
  }

  // Check file extension
  const extension = '.' + file.name.split('.').pop()?.toLowerCase();
  if (!ALLOWED_FILE_EXTENSIONS.includes(extension)) {
    return {
      valid: false,
      error: `File type not allowed. Allowed types: ${ALLOWED_FILE_EXTENSIONS.join(', ')}`,
    };
  }

  return { valid: true };
}

/**
 * Format file size for display
 */
export function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 Bytes';
  const k = 1024;
  const sizes = ['Bytes', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}
