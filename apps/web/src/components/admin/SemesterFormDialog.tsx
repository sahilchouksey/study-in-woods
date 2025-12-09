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
import { useCreateSemester, useUpdateSemester } from '@/lib/api/hooks/useAdminMutations';
import type { Semester } from '@/lib/api/courses';

interface SemesterFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  semester?: Semester | null;
  courseId: string;
}

interface SemesterFormData {
  number: number;
  name: string;
}

export function SemesterFormDialog({
  open,
  onOpenChange,
  semester,
  courseId,
}: SemesterFormDialogProps) {
  const isEditing = !!semester;
  const createMutation = useCreateSemester();
  const updateMutation = useUpdateSemester();

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<SemesterFormData>({
    defaultValues: semester
      ? {
          number: semester.number,
          name: semester.name,
        }
      : {
          number: 1,
          name: '',
        },
  });

  useEffect(() => {
    if (semester) {
      reset({
        number: semester.number,
        name: semester.name,
      });
    } else {
      reset({
        number: 1,
        name: '',
      });
    }
  }, [semester, reset]);

  const onSubmit = async (data: SemesterFormData) => {
    try {
      if (isEditing) {
        await updateMutation.mutateAsync({
          courseId,
          number: semester.number,
          data,
        });
      } else {
        await createMutation.mutateAsync({
          course_id: courseId,
          number: data.number,
          name: data.name,
        });
      }
      onOpenChange(false);
      reset();
    } catch (error) {
      console.error('Failed to save semester:', error);
    }
  };

  const isPending = createMutation.isPending || updateMutation.isPending;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>{isEditing ? 'Edit Semester' : 'Create Semester'}</DialogTitle>
          <DialogDescription>
            {isEditing
              ? 'Update the semester information below.'
              : 'Add a new semester to the course.'}
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="number">Semester Number *</Label>
            <Input
              id="number"
              type="number"
              min="1"
              max="20"
              {...register('number', {
                required: 'Semester number is required',
                min: { value: 1, message: 'Minimum 1' },
                max: { value: 20, message: 'Maximum 20' },
                valueAsNumber: true,
              })}
              placeholder="e.g., 1"
            />
            {errors.number && (
              <p className="text-sm text-red-600">{errors.number.message}</p>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor="name">Semester Name *</Label>
            <Input
              id="name"
              {...register('name', { required: 'Name is required' })}
              placeholder="e.g., 1st Semester"
            />
            {errors.name && (
              <p className="text-sm text-red-600">{errors.name.message}</p>
            )}
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
