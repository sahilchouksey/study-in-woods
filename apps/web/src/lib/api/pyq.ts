import { apiClient } from './client';
import type { ApiResponse } from '@/types/api';

/**
 * PYQ extraction status types
 */
export type PYQExtractionStatus = 'pending' | 'processing' | 'completed' | 'failed';

/**
 * Question choice interface (for OR-type questions)
 */
export interface PYQQuestionChoice {
  id: number;
  choice_label: string;
  choice_text: string;
  marks?: number;
}

/**
 * Question interface
 */
export interface PYQQuestion {
  id: number;
  question_number: string;
  section_name?: string;
  question_text: string;
  marks: number;
  is_compulsory: boolean;
  has_choices: boolean;
  choice_group?: string;
  unit_number?: number;
  topic_keywords?: string;
  choices?: PYQQuestionChoice[];
}

/**
 * Full PYQ paper interface
 */
export interface PYQPaper {
  id: number;
  subject_id: number;
  year: number;
  month?: string;
  exam_type?: string;
  total_marks: number;
  duration?: string;
  total_questions: number;
  instructions?: string;
  extraction_status: PYQExtractionStatus;
  extraction_error?: string;
  questions: PYQQuestion[];
  created_at: string;
  updated_at: string;
}

/**
 * PYQ paper summary (for listing)
 */
export interface PYQPaperSummary {
  id: number;
  year: number;
  month?: string;
  exam_type?: string;
  total_marks: number;
  total_questions: number;
  extraction_status: PYQExtractionStatus;
  created_at: string;
}

/**
 * PYQ papers list response
 */
export interface PYQPapersListResponse {
  papers: PYQPaperSummary[];
  total: number;
}

/**
 * Extraction status response
 */
export interface PYQExtractionStatusResponse {
  status: PYQExtractionStatus;
  error?: string;
}

/**
 * Search results response
 */
export interface PYQSearchResponse {
  query: string;
  count: number;
  results: PYQQuestion[];
}

// ============ CRAWLER INTERFACES ============

/**
 * Match confidence levels for PYQ papers
 */
export type MatchConfidence = 'exact' | 'partial' | 'none';

/**
 * Available PYQ paper from crawler
 */
export interface AvailablePYQPaper {
  title: string;
  source_url: string;
  pdf_url: string;
  file_type: string;
  subject_code: string;
  subject_name: string;
  year: number;
  month: string;
  exam_type: string;
  source_name: string;
  /** Whether the subject code matches the current subject */
  code_matched: boolean;
  /** Match confidence: "exact", "partial", or "none" */
  match_confidence: MatchConfidence;
}

/**
 * Search available PYQs response (categorized)
 */
export interface SearchAvailablePYQsResponse {
  subject_id: number;
  subject_name: string;
  subject_code: string;
  /** Papers matching current subject code (prioritized) */
  matched_papers: AvailablePYQPaper[];
  /** Papers with different codes (older syllabus) */
  unmatched_papers: AvailablePYQPaper[];
  matched_count: number;
  unmatched_count: number;
  total_available: number;
  ingested_count: number;
}

/**
 * Ingest PYQ request
 */
export interface IngestPYQRequest {
  pdf_url: string;
  title: string;
  year: number;
  month: string;
  exam_type?: string;
  source_name: string;
}

/**
 * Crawler source
 */
export interface CrawlerSource {
  name: string;
  display_name: string;
  base_url: string;
}

/**
 * Crawler sources response
 */
export interface CrawlerSourcesResponse {
  sources: CrawlerSource[];
  count: number;
}

/**
 * Status display configuration
 */
export const PYQ_EXTRACTION_STATUS_CONFIG: Record<PYQExtractionStatus, { 
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
    description: 'AI is extracting PYQ data'
  },
  completed: { 
    label: 'Completed', 
    color: 'bg-green-500',
    description: 'PYQ extracted successfully'
  },
  failed: { 
    label: 'Failed', 
    color: 'bg-red-500',
    description: 'Extraction failed'
  },
};

/**
 * Exam type display labels
 */
export const EXAM_TYPE_LABELS: Record<string, string> = {
  'End Semester': 'End Semester',
  'Mid Semester': 'Mid Semester',
  'Supplementary': 'Supplementary',
  'Regular': 'Regular',
};

/**
 * Format paper title
 */
export function formatPaperTitle(paper: PYQPaperSummary | PYQPaper): string {
  const parts: string[] = [];
  if (paper.month) parts.push(paper.month);
  parts.push(paper.year.toString());
  if (paper.exam_type) parts.push(`(${paper.exam_type})`);
  return parts.join(' ');
}

/**
 * PYQ service
 */
export const pyqService = {
  /**
   * Get PYQ papers for a subject
   */
  async getPYQsBySubject(subjectId: string): Promise<PYQPapersListResponse> {
    const response = await apiClient.get<ApiResponse<PYQPapersListResponse>>(
      `/api/v1/subjects/${subjectId}/pyqs`
    );
    return response.data.data || { papers: [], total: 0 };
  },

  /**
   * Get PYQ paper by ID (with questions and choices)
   */
  async getPYQById(pyqId: string): Promise<PYQPaper> {
    const response = await apiClient.get<ApiResponse<PYQPaper>>(
      `/api/v1/pyqs/${pyqId}`
    );
    return response.data.data!;
  },

  /**
   * Trigger PYQ extraction from a document
   */
  async extractPYQ(documentId: string, async: boolean = true): Promise<{ message: string; pyq?: PYQPaper; status?: string }> {
    const response = await apiClient.post<ApiResponse<{ message: string; pyq?: PYQPaper; status?: string }>>(
      `/api/v1/documents/${documentId}/extract-pyq`,
      null,
      { params: { async: async.toString() } }
    );
    return response.data.data!;
  },

  /**
   * Get extraction status
   */
  async getExtractionStatus(pyqId: string): Promise<PYQExtractionStatusResponse> {
    const response = await apiClient.get<ApiResponse<PYQExtractionStatusResponse>>(
      `/api/v1/pyqs/${pyqId}/status`
    );
    return response.data.data!;
  },

  /**
   * Retry failed extraction
   */
  async retryExtraction(pyqId: string, async: boolean = false): Promise<{ message: string; pyq?: PYQPaper; status?: string }> {
    const response = await apiClient.post<ApiResponse<{ message: string; pyq?: PYQPaper; status?: string }>>(
      `/api/v1/pyqs/${pyqId}/retry`,
      null,
      { params: { async: async.toString() } }
    );
    return response.data.data!;
  },

  /**
   * Delete PYQ paper (admin only)
   */
  async deletePYQ(pyqId: string): Promise<void> {
    await apiClient.delete(`/api/v1/pyqs/${pyqId}`);
  },

  /**
   * Get questions for a PYQ paper
   */
  async getQuestions(pyqId: string): Promise<PYQQuestion[]> {
    const response = await apiClient.get<ApiResponse<PYQQuestion[]>>(
      `/api/v1/pyqs/${pyqId}/questions`
    );
    return response.data.data || [];
  },

  /**
   * Search questions across all PYQs for a subject
   */
  async searchQuestions(subjectId: string, query: string): Promise<PYQSearchResponse> {
    const response = await apiClient.get<ApiResponse<PYQSearchResponse>>(
      `/api/v1/subjects/${subjectId}/pyqs/search`,
      { params: { q: query } }
    );
    return response.data.data || { query, count: 0, results: [] };
  },

  // ============ CRAWLER FUNCTIONS ============

  /**
   * Search available PYQs from crawlers
   */
  async searchAvailablePYQs(
    subjectId: string,
    params?: {
      course?: string;
      semester?: number;
      year?: number;
      month?: string;
      source?: string;
      limit?: number;
      /** Optional fuzzy search query to filter by name within results */
      search?: string;
    }
  ): Promise<SearchAvailablePYQsResponse> {
    const response = await apiClient.get<ApiResponse<SearchAvailablePYQsResponse>>(
      `/api/v1/subjects/${subjectId}/pyqs/search-available`,
      { params }
    );
    return response.data.data || {
      subject_id: 0,
      subject_name: '',
      subject_code: '',
      matched_papers: [],
      unmatched_papers: [],
      matched_count: 0,
      unmatched_count: 0,
      total_available: 0,
      ingested_count: 0,
    };
  },

  /**
   * Ingest a crawled PYQ paper
   */
  async ingestCrawledPYQ(subjectId: string, data: IngestPYQRequest): Promise<{ message: string; status: string }> {
    const response = await apiClient.post<ApiResponse<{ message: string; status: string }>>(
      `/api/v1/subjects/${subjectId}/pyqs/ingest`,
      data
    );
    return response.data.data || { message: 'Ingestion initiated', status: 'pending' };
  },

  /**
   * Get available crawler sources
   */
  async getCrawlerSources(): Promise<CrawlerSourcesResponse> {
    const response = await apiClient.get<ApiResponse<CrawlerSourcesResponse>>(
      `/api/v1/pyqs/crawler-sources`
    );
    return response.data.data || { sources: [], count: 0 };
  },
};
