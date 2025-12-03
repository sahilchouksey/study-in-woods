'use client';

import { useEffect } from 'react';
import { useForm } from 'react-hook-form';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { useCreateCourse, useUpdateCourse } from '@/lib/api/hooks/useAdminMutations';
import type { Course } from '@/lib/api/courses';

interface CourseFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  course?: Course | null;
  universityId: string;
}

interface CourseFormData {
  name: string;
  code: string;
  description?: string;
  duration: number;
}

export function CourseFormDialog({
  open,
  onOpenChange,
  course,
  universityId,
}: CourseFormDialogProps) {
  const isEditing = !!course;
  const createMutation = useCreateCourse();
  const updateMutation = useUpdateCourse();

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<CourseFormData>({
    defaultValues: course
      ? {
          name: course.name,
          code: course.code,
          description: course.description,
          duration: course.duration,
        }
      : {
          duration: 8,
        },
  });

  useEffect(() => {
    if (course) {
      reset({
        name: course.name,
        code: course.code,
        description: course.description,
        duration: course.duration,
      });
    } else {
      reset({
        name: '',
        code: '',
        description: '',
        duration: 8,
      });
    }
  }, [course, reset]);

  const onSubmit = async (data: CourseFormData) => {
    try {
      if (isEditing) {
        await updateMutation.mutateAsync({
          id: course.id,
          data,
        });
      } else {
        await createMutation.mutateAsync({
          ...data,
          university_id: universityId,
        });
      }
      onOpenChange(false);
      reset();
    } catch (error) {
      console.error('Failed to save course:', error);
    }
  };

  const isPending = createMutation.isPending || updateMutation.isPending;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>{isEditing ? 'Edit Course' : 'Create Course'}</DialogTitle>
          <DialogDescription>
            {isEditing
              ? 'Update the course information below.'
              : 'Add a new course to the university.'}
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="name">Course Name *</Label>
            <Input
              id="name"
              {...register('name', { required: 'Name is required' })}
              placeholder="e.g., Computer Science and Engineering"
            />
            {errors.name && (
              <p className="text-sm text-red-600">{errors.name.message}</p>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor="code">Course Code *</Label>
            <Input
              id="code"
              {...register('code', { required: 'Code is required' })}
              placeholder="e.g., CSE"
            />
            {errors.code && (
              <p className="text-sm text-red-600">{errors.code.message}</p>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor="duration">Duration (Semesters) *</Label>
            <Input
              id="duration"
              type="number"
              min="1"
              max="20"
              {...register('duration', {
                required: 'Duration is required',
                min: { value: 1, message: 'Minimum 1 semester' },
                max: { value: 20, message: 'Maximum 20 semesters' },
                valueAsNumber: true,
              })}
              placeholder="e.g., 8"
            />
            {errors.duration && (
              <p className="text-sm text-red-600">{errors.duration.message}</p>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor="description">Description (Optional)</Label>
            <textarea
              id="description"
              {...register('description')}
              placeholder="Brief description of the course"
              rows={3}
              className="flex min-h-[80px] w-full rounded-md border border-neutral-200 bg-white px-3 py-2 text-sm ring-offset-white placeholder:text-neutral-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-neutral-950 focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 dark:border-neutral-800 dark:bg-neutral-950 dark:ring-offset-neutral-950 dark:placeholder:text-neutral-400 dark:focus-visible:ring-neutral-300"
            />
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={isPending}
            >
              Cancel
            </Button>
            <Button
              type="submit"
              disabled={isPending}
              className="bg-black hover:bg-neutral-800 dark:bg-white dark:hover:bg-neutral-200 dark:text-black"
            >
              {isPending ? 'Saving...' : isEditing ? 'Update' : 'Create'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
