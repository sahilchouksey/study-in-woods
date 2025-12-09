import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  universityService,
  courseService,
  semesterService,
  subjectService,
} from '@/lib/api/courses';

// ============= University Mutations =============

/**
 * Hook to create a new university (admin only)
 */
export function useCreateUniversity() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: {
      name: string;
      code: string;
      location: string;
      website?: string;
    }) => universityService.createUniversity(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['universities'] });
    },
  });
}

/**
 * Hook to update a university (admin only)
 */
export function useUpdateUniversity() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      id,
      data,
    }: {
      id: string;
      data: Partial<{
        name: string;
        code: string;
        location: string;
        website?: string;
        is_active: boolean;
      }>;
    }) => universityService.updateUniversity(id, data),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['universities'] });
      queryClient.invalidateQueries({ queryKey: ['university', variables.id] });
    },
  });
}

/**
 * Hook to delete a university (admin only)
 */
export function useDeleteUniversity() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) => universityService.deleteUniversity(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['universities'] });
    },
  });
}

// ============= Course Mutations =============

/**
 * Hook to create a new course (admin only)
 */
export function useCreateCourse() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: {
      university_id: string;
      name: string;
      code: string;
      description?: string;
      duration: number;
    }) => courseService.createCourse(data),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['courses'] });
      queryClient.invalidateQueries({ queryKey: ['courses', variables.university_id] });
    },
  });
}

/**
 * Hook to update a course (admin only)
 */
export function useUpdateCourse() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      id,
      data,
    }: {
      id: string;
      data: Partial<{
        name: string;
        code: string;
        description?: string;
        duration: number;
        is_active: boolean;
      }>;
    }) => courseService.updateCourse(id, data),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['courses'] });
      queryClient.invalidateQueries({ queryKey: ['course', variables.id] });
    },
  });
}

/**
 * Hook to delete a course (admin only)
 */
export function useDeleteCourse() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) => courseService.deleteCourse(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['courses'] });
    },
  });
}

// ============= Semester Mutations =============

/**
 * Hook to create a new semester (admin only)
 */
export function useCreateSemester() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: {
      course_id: string;
      number: number;
      name: string;
    }) => semesterService.createSemester({
      ...data,
      course_id: parseInt(data.course_id),
    }),
    onSuccess: (_, variables) => {
      // Invalidate semesters query - courseId should be string to match useSemesters hook
      queryClient.invalidateQueries({ queryKey: ['semesters', variables.course_id] });
    },
  });
}

/**
 * Hook to update a semester (admin only)
 */
export function useUpdateSemester() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      courseId,
      number,
      data,
    }: {
      courseId: string;
      number: number;
      data: Partial<{
        number: number;
        name: string;
      }>;
    }) => semesterService.updateSemester(courseId, number, data),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['semesters', variables.courseId] });
    },
  });
}

/**
 * Hook to delete a semester (admin only)
 */
export function useDeleteSemester() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ courseId, number }: { courseId: string; number: number }) =>
      semesterService.deleteSemester(courseId, number),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['semesters', variables.courseId] });
    },
  });
}

// ============= Subject Mutations =============

/**
 * Hook to create a new subject (admin only)
 */
export function useCreateSubject() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: {
      semester_id: string;
      name: string;
      code: string;
      description?: string;
      credits?: number;
    }) => subjectService.createSubject(data),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['subjects', variables.semester_id] });
    },
  });
}

/**
 * Hook to update a subject (admin only)
 */
export function useUpdateSubject() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      semesterId,
      subjectId,
      data,
    }: {
      semesterId: string;
      subjectId: string;
      data: Partial<{
        name: string;
        code: string;
        description?: string;
        credits?: number;
      }>;
    }) => subjectService.updateSubject(semesterId, subjectId, data),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['subjects', variables.semesterId] });
      queryClient.invalidateQueries({
        queryKey: ['subject', variables.semesterId, variables.subjectId],
      });
    },
  });
}

/**
 * Hook to delete a subject (admin only)
 */
export function useDeleteSubject() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ semesterId, subjectId }: { semesterId: string; subjectId: string }) =>
      subjectService.deleteSubject(semesterId, subjectId),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['subjects', variables.semesterId] });
    },
  });
}
