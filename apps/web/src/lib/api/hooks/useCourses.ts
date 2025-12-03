import { useQuery } from '@tanstack/react-query';
import {
  universityService,
  courseService,
  semesterService,
  subjectService,
} from '@/lib/api/courses';

/**
 * Hook to get all universities
 */
export function useUniversities() {
  return useQuery({
    queryKey: ['universities'],
    queryFn: () => universityService.getUniversities(),
    staleTime: 5 * 60 * 1000, // 5 minutes
  });
}

/**
 * Hook to get a specific university
 */
export function useUniversity(id: string | null) {
  return useQuery({
    queryKey: ['university', id],
    queryFn: () => universityService.getUniversity(id!),
    enabled: !!id,
    staleTime: 5 * 60 * 1000,
  });
}

/**
 * Hook to get all courses (optionally filtered by university)
 */
export function useCourses(universityId?: string) {
  return useQuery({
    queryKey: ['courses', universityId],
    queryFn: () => courseService.getCourses(universityId),
    staleTime: 5 * 60 * 1000,
  });
}

/**
 * Hook to get a specific course
 */
export function useCourse(id: string | null) {
  return useQuery({
    queryKey: ['course', id],
    queryFn: () => courseService.getCourse(id!),
    enabled: !!id,
    staleTime: 5 * 60 * 1000,
  });
}

/**
 * Hook to get courses by university
 */
export function useCoursesByUniversity(universityId: string | null) {
  return useQuery({
    queryKey: ['courses', 'university', universityId],
    queryFn: () => courseService.getCoursesByUniversity(universityId!),
    enabled: !!universityId,
    staleTime: 5 * 60 * 1000,
  });
}

/**
 * Hook to get semesters for a course
 */
export function useSemesters(courseId: string | null) {
  return useQuery({
    queryKey: ['semesters', courseId],
    queryFn: () => semesterService.getSemesters(courseId!),
    enabled: !!courseId,
    staleTime: 5 * 60 * 1000,
  });
}

/**
 * Hook to get a specific semester
 */
export function useSemester(courseId: string | null, number: number | null) {
  return useQuery({
    queryKey: ['semester', courseId, number],
    queryFn: () => semesterService.getSemester(courseId!, number!),
    enabled: !!courseId && number !== null,
    staleTime: 5 * 60 * 1000,
  });
}

/**
 * Hook to get subjects for a semester
 */
export function useSubjects(semesterId: string | null) {
  return useQuery({
    queryKey: ['subjects', semesterId],
    queryFn: () => subjectService.getSubjects(semesterId!),
    enabled: !!semesterId,
    staleTime: 5 * 60 * 1000,
  });
}

/**
 * Hook to get a specific subject
 */
export function useSubject(semesterId: string | null, subjectId: string | null) {
  return useQuery({
    queryKey: ['subject', semesterId, subjectId],
    queryFn: () => subjectService.getSubject(semesterId!, subjectId!),
    enabled: !!semesterId && !!subjectId,
    staleTime: 5 * 60 * 1000,
  });
}

/**
 * Hook to get subjects by course and semester
 */
export function useSubjectsByCourse(courseId: string | null, semester: string | null) {
  return useQuery({
    queryKey: ['subjects', 'course', courseId, semester],
    queryFn: () => subjectService.getSubjectsByCourse(courseId!, semester!),
    enabled: !!courseId && !!semester,
    staleTime: 5 * 60 * 1000,
  });
}
