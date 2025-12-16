'use client';

import { ChevronRight } from 'lucide-react';
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb';
import type { University, Course, Semester } from '@/lib/api/courses';

interface CoursesBreadcrumbProps {
  university?: University | null;
  course?: Course | null;
  semester?: Semester | null;
  onUniversityClick?: () => void;
  onCourseClick?: () => void;
}

export function CoursesBreadcrumb({
  university,
  course,
  semester,
  onUniversityClick,
  onCourseClick,
}: CoursesBreadcrumbProps) {
  if (!university && !course && !semester) {
    return null;
  }

  return (
    <Breadcrumb>
      <BreadcrumbList>
        {university && (
          <>
            <BreadcrumbItem>
              {course || semester ? (
                <BreadcrumbLink 
                  className="cursor-pointer hover:text-foreground"
                  onClick={onUniversityClick}
                >
                  {university.name}
                </BreadcrumbLink>
              ) : (
                <BreadcrumbPage>{university.name}</BreadcrumbPage>
              )}
            </BreadcrumbItem>
            {(course || semester) && (
              <BreadcrumbSeparator>
                <ChevronRight className="h-4 w-4" />
              </BreadcrumbSeparator>
            )}
          </>
        )}

        {course && (
          <>
            <BreadcrumbItem>
              {semester ? (
                <BreadcrumbLink 
                  className="cursor-pointer hover:text-foreground"
                  onClick={onCourseClick}
                >
                  {course.code}
                </BreadcrumbLink>
              ) : (
                <BreadcrumbPage>{course.code}</BreadcrumbPage>
              )}
            </BreadcrumbItem>
            {semester && (
              <BreadcrumbSeparator>
                <ChevronRight className="h-4 w-4" />
              </BreadcrumbSeparator>
            )}
          </>
        )}

        {semester && (
          <BreadcrumbItem>
            <BreadcrumbPage>{semester.name}</BreadcrumbPage>
          </BreadcrumbItem>
        )}
      </BreadcrumbList>
    </Breadcrumb>
  );
}
