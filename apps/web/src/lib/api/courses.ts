import { apiClient } from './client';
import type { ApiResponse } from '@/types/api';

/**
 * Pagination metadata type
 */
export interface PaginationMeta {
  current_page: number;
  per_page: number;
  total: number;
  total_pages: number;
}

/**
 * Paginated response type
 */
export interface PaginatedResponse<T> {
  data: T[];
  pagination: PaginationMeta;
}

/**
 * University type
 */
export interface University {
  id: string;
  name: string;
  code: string;
  location: string;
  website?: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

/**
 * Course type
 */
export interface Course {
  id: string;
  university_id: string;
  name: string;
  code: string;
  description?: string;
  duration: number; // in semesters
  is_active?: boolean;
  created_at: string;
  updated_at: string;
}

/**
 * Semester type
 */
export interface Semester {
  id: string;
  course_id: string;
  number: number;
  name: string;
  created_at: string;
  updated_at: string;
}

/**
 * Subject type
 */
export interface Subject {
  id: string;
  semester_id: string;
  name: string;
  code: string;
  description?: string;
  credits?: number;
  created_at: string;
  updated_at: string;
}

/**
 * University service
 */
export const universityService = {
  /**
   * Get all universities
   */
  async getUniversities(): Promise<University[]> {
    const response = await apiClient.get<ApiResponse<University[]>>(
      '/api/v1/universities'
    );
    return response.data.data || [];
  },

  /**
   * Get a specific university
   */
  async getUniversity(id: string): Promise<University> {
    const response = await apiClient.get<ApiResponse<University>>(
      `/api/v1/universities/${id}`
    );
    return response.data.data!;
  },

  /**
   * Create a new university (admin only)
   */
  async createUniversity(data: {
    name: string;
    code: string;
    location: string;
    website?: string;
  }): Promise<University> {
    const response = await apiClient.post<ApiResponse<University>>(
      '/api/v1/universities',
      data
    );
    return response.data.data!;
  },

  /**
   * Update a university (admin only)
   */
  async updateUniversity(
    id: string,
    data: Partial<{
      name: string;
      code: string;
      location: string;
      website?: string;
      is_active: boolean;
    }>
  ): Promise<University> {
    const response = await apiClient.put<ApiResponse<University>>(
      `/api/v1/universities/${id}`,
      data
    );
    return response.data.data!;
  },

  /**
   * Delete a university (admin only)
   */
  async deleteUniversity(id: string): Promise<void> {
    await apiClient.delete(`/api/v1/universities/${id}`);
  },
};

/**
 * Course service
 */
export const courseService = {
  /**
   * Get all courses (optionally filtered by university)
   */
  async getCourses(universityId?: string): Promise<Course[]> {
    const params = universityId ? { university_id: universityId } : {};
    const response = await apiClient.get<ApiResponse<Course[]>>(
      '/api/v1/courses',
      { params }
    );
    return response.data.data || [];
  },

  /**
   * Get a specific course
   */
  async getCourse(id: string): Promise<Course> {
    const response = await apiClient.get<ApiResponse<Course>>(
      `/api/v1/courses/${id}`
    );
    return response.data.data!;
  },

  /**
   * Get courses for a specific university
   */
  async getCoursesByUniversity(universityId: string): Promise<Course[]> {
    return this.getCourses(universityId);
  },

  /**
   * Create a new course (admin only)
   */
  async createCourse(data: {
    university_id: string;
    name: string;
    code: string;
    description?: string;
    duration: number;
  }): Promise<Course> {
    const response = await apiClient.post<ApiResponse<Course>>(
      '/api/v1/courses',
      data
    );
    return response.data.data!;
  },

  /**
   * Update a course (admin only)
   */
  async updateCourse(
    id: string,
    data: Partial<{
      name: string;
      code: string;
      description?: string;
      duration: number;
      is_active: boolean;
    }>
  ): Promise<Course> {
    const response = await apiClient.put<ApiResponse<Course>>(
      `/api/v1/courses/${id}`,
      data
    );
    return response.data.data!;
  },

  /**
   * Delete a course (admin only)
   */
  async deleteCourse(id: string): Promise<void> {
    await apiClient.delete(`/api/v1/courses/${id}`);
  },
};

/**
 * Semester service
 */
export const semesterService = {
  /**
   * Get semesters for a course
   */
  async getSemesters(courseId: string): Promise<Semester[]> {
    const response = await apiClient.get<ApiResponse<Semester[]>>(
      `/api/v1/courses/${courseId}/semesters`
    );
    return response.data.data || [];
  },

  /**
   * Get a specific semester
   */
  async getSemester(courseId: string, number: number): Promise<Semester> {
    const response = await apiClient.get<ApiResponse<Semester>>(
      `/api/v1/courses/${courseId}/semesters/${number}`
    );
    return response.data.data!;
  },

  /**
   * Create a new semester (admin only)
   */
  async createSemester(data: {
    course_id: number;
    number: number;
    name: string;
  }): Promise<Semester> {
    const response = await apiClient.post<ApiResponse<Semester>>(
      `/api/v1/courses/${data.course_id}/semesters`,
      data
    );
    return response.data.data!;
  },

  /**
   * Update a semester (admin only)
   */
  async updateSemester(
    courseId: string,
    number: number,
    data: Partial<{
      number: number;
      name: string;
    }>
  ): Promise<Semester> {
    const response = await apiClient.put<ApiResponse<Semester>>(
      `/api/v1/courses/${courseId}/semesters/${number}`,
      data
    );
    return response.data.data!;
  },

  /**
   * Delete a semester (admin only)
   */
  async deleteSemester(courseId: string, number: number): Promise<void> {
    await apiClient.delete(`/api/v1/courses/${courseId}/semesters/${number}`);
  },
};

/**
 * Subject query parameters
 */
export interface SubjectQueryParams {
  page?: number;
  per_page?: number;
  search?: string;
}

/**
 * Subject service
 */
export const subjectService = {
  /**
   * Get subjects for a semester with pagination and search
   */
  async getSubjects(
    semesterId: string,
    params?: SubjectQueryParams
  ): Promise<PaginatedResponse<Subject>> {
    const response = await apiClient.get<{
      success: boolean;
      data: Subject[];
      pagination: PaginationMeta;
    }>(`/api/v1/semesters/${semesterId}/subjects`, { params });
    return {
      data: response.data.data || [],
      pagination: response.data.pagination || {
        current_page: 1,
        per_page: 10,
        total: response.data.data?.length || 0,
        total_pages: 1,
      },
    };
  },

  /**
   * Get a specific subject
   */
  async getSubject(semesterId: string, subjectId: string): Promise<Subject> {
    const response = await apiClient.get<ApiResponse<Subject>>(
      `/api/v1/semesters/${semesterId}/subjects/${subjectId}`
    );
    return response.data.data!;
  },

  /**
   * Create a new subject (admin only)
   */
  async createSubject(data: {
    semester_id: string;
    name: string;
    code: string;
    description?: string;
    credits?: number;
  }): Promise<Subject> {
    const response = await apiClient.post<ApiResponse<Subject>>(
      `/api/v1/semesters/${data.semester_id}/subjects`,
      data
    );
    return response.data.data!;
  },

  /**
   * Update a subject (admin only)
   */
  async updateSubject(
    semesterId: string,
    subjectId: string,
    data: Partial<{
      name: string;
      code: string;
      description?: string;
      credits?: number;
    }>
  ): Promise<Subject> {
    const response = await apiClient.put<ApiResponse<Subject>>(
      `/api/v1/semesters/${semesterId}/subjects/${subjectId}`,
      data
    );
    return response.data.data!;
  },

  /**
   * Delete a subject (admin only)
   */
  async deleteSubject(semesterId: string, subjectId: string): Promise<void> {
    await apiClient.delete(`/api/v1/semesters/${semesterId}/subjects/${subjectId}`);
  },

  /**
   * Delete all subjects for a semester (admin only)
   */
  async deleteAllSubjects(semesterId: string): Promise<{
    deleted_count: number;
    failed_count: number;
    message: string;
  }> {
    const response = await apiClient.delete<ApiResponse<{
      deleted_count: number;
      failed_count: number;
      message: string;
    }>>(`/api/v1/semesters/${semesterId}/subjects`);
    return response.data.data!;
  },

  /**
   * Get subjects by course and semester
   */
  async getSubjectsByCourse(
    courseId: string,
    semester: string
  ): Promise<Subject[]> {
    // For now, we'll construct the semester ID
    // In real implementation, you might need a different approach
    const response = await apiClient.get<ApiResponse<Subject[]>>(
      `/api/v1/courses/${courseId}/subjects`,
      { params: { semester } }
    );
    return response.data.data || [];
  },
};
