import { apiClient } from './client';
import type { ApiResponse } from '@/types/api';

/**
 * Syllabus extraction status types
 */
export type SyllabusExtractionStatus = 'pending' | 'processing' | 'completed' | 'failed';

/**
 * Syllabus topic interface
 */
export interface SyllabusTopic {
  id: number;
  topic_number: number;
  title: string;
  description?: string;
  keywords?: string;
}

/**
 * Syllabus unit interface
 */
export interface SyllabusUnit {
  id: number;
  unit_number: number;
  title: string;
  description?: string;
  hours?: number;
  topics: SyllabusTopic[];
}

/**
 * Book reference interface
 */
export interface BookReference {
  id: number;
  title: string;
  authors: string;
  publisher?: string;
  edition?: string;
  year?: number;
  isbn?: string;
  is_textbook: boolean;
  book_type: 'textbook' | 'reference' | 'recommended';
}

/**
 * Full syllabus interface
 */
export interface Syllabus {
  id: number;
  subject_id: number;
  subject_name: string;
  subject_code: string;
  total_credits: number;
  extraction_status: SyllabusExtractionStatus;
  extraction_error?: string;
  units: SyllabusUnit[];
  books: BookReference[];
  created_at: string;
  updated_at: string;
}

/**
 * Extraction status response
 */
export interface ExtractionStatusResponse {
  id: number;
  extraction_status: SyllabusExtractionStatus;
  extraction_error?: string;
}

/**
 * Status display configuration
 */
export const EXTRACTION_STATUS_CONFIG: Record<SyllabusExtractionStatus, { 
  label: string; 
  color: string;
  description: string;
}> = {
  pending: { 
    label: 'Pending', 
    color: 'bg-yellow-500',
    description: 'Waiting to start extraction'
  },
  processing: { 
    label: 'Processing', 
    color: 'bg-blue-500',
    description: 'AI is extracting syllabus data'
  },
  completed: { 
    label: 'Completed', 
    color: 'bg-green-500',
    description: 'Syllabus extracted successfully'
  },
  failed: { 
    label: 'Failed', 
    color: 'bg-red-500',
    description: 'Extraction failed'
  },
};

/**
 * Book type display labels
 */
export const BOOK_TYPE_LABELS: Record<string, string> = {
  textbook: 'Textbook',
  reference: 'Reference',
  recommended: 'Recommended',
};

/**
 * Syllabus service
 */
export const syllabusService = {
  /**
   * Get syllabus for a subject
   */
  async getSyllabusBySubject(subjectId: string): Promise<Syllabus | null> {
    try {
      const response = await apiClient.get<ApiResponse<Syllabus>>(
        `/api/v1/subjects/${subjectId}/syllabus`
      );
      return response.data.data || null;
    } catch (error: unknown) {
      // Return null if not found (404)
      const axiosError = error as { response?: { status?: number } };
      if (axiosError.response?.status === 404) {
        return null;
      }
      throw error;
    }
  },

  /**
   * Get syllabus by ID
   */
  async getSyllabusById(syllabusId: string): Promise<Syllabus> {
    const response = await apiClient.get<ApiResponse<Syllabus>>(
      `/api/v1/syllabus/${syllabusId}`
    );
    return response.data.data!;
  },

  /**
   * Trigger syllabus extraction from a document
   */
  async extractSyllabus(documentId: string): Promise<Syllabus> {
    const response = await apiClient.post<ApiResponse<Syllabus>>(
      `/api/v1/documents/${documentId}/extract-syllabus`
    );
    return response.data.data!;
  },

  /**
   * Get extraction status
   */
  async getExtractionStatus(syllabusId: string): Promise<ExtractionStatusResponse> {
    const response = await apiClient.get<ApiResponse<ExtractionStatusResponse>>(
      `/api/v1/syllabus/${syllabusId}/status`
    );
    return response.data.data!;
  },

  /**
   * Retry failed extraction
   */
  async retryExtraction(syllabusId: string): Promise<Syllabus> {
    const response = await apiClient.post<ApiResponse<Syllabus>>(
      `/api/v1/syllabus/${syllabusId}/retry`
    );
    return response.data.data!;
  },

  /**
   * Delete syllabus (admin only)
   */
  async deleteSyllabus(syllabusId: string): Promise<void> {
    await apiClient.delete(`/api/v1/syllabus/${syllabusId}`);
  },

  /**
   * Search topics across syllabus
   */
  async searchTopics(subjectId: string, query: string): Promise<SyllabusTopic[]> {
    const response = await apiClient.get<ApiResponse<SyllabusTopic[]>>(
      `/api/v1/subjects/${subjectId}/syllabus/search`,
      { params: { q: query } }
    );
    return response.data.data || [];
  },
};
