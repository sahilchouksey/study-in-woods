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
import { useCreateSubject, useUpdateSubject } from '@/lib/api/hooks/useAdminMutations';
import type { Subject } from '@/lib/api/courses';

interface SubjectFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  subject?: Subject | null;
  semesterId: string;
}

interface SubjectFormData {
  name: string;
  code: string;
  description?: string;
  credits?: number;
}

export function SubjectFormDialog({
  open,
  onOpenChange,
  subject,
  semesterId,
}: SubjectFormDialogProps) {
  const isEditing = !!subject;
  const createMutation = useCreateSubject();
  const updateMutation = useUpdateSubject();

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<SubjectFormData>({
    defaultValues: subject
      ? {
          name: subject.name,
          code: subject.code,
          description: subject.description,
          credits: subject.credits,
        }
      : {
          credits: 4,
        },
  });

  useEffect(() => {
    if (subject) {
      reset({
        name: subject.name,
        code: subject.code,
        description: subject.description,
        credits: subject.credits,
      });
    } else {
      reset({
        name: '',
        code: '',
        description: '',
        credits: 4,
      });
    }
  }, [subject, reset]);

  const onSubmit = async (data: SubjectFormData) => {
    try {
      if (isEditing) {
        await updateMutation.mutateAsync({
          semesterId,
          subjectId: subject.id,
          data,
        });
      } else {
        await createMutation.mutateAsync({
          semester_id: semesterId,
          name: data.name,
          code: data.code,
          description: data.description,
          credits: data.credits,
        });
      }
      onOpenChange(false);
      reset();
    } catch (error) {
      console.error('Failed to save subject:', error);
    }
  };

  const isPending = createMutation.isPending || updateMutation.isPending;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>{isEditing ? 'Edit Subject' : 'Create Subject'}</DialogTitle>
          <DialogDescription>
            {isEditing
              ? 'Update the subject information below.'
              : 'Add a new subject to the semester.'}
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="name">Subject Name *</Label>
            <Input
              id="name"
              {...register('name', { required: 'Name is required' })}
              placeholder="e.g., Data Structures and Algorithms"
            />
            {errors.name && (
              <p className="text-sm text-red-600">{errors.name.message}</p>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor="code">Subject Code *</Label>
            <Input
              id="code"
              {...register('code', { required: 'Code is required' })}
              placeholder="e.g., CS201"
            />
            {errors.code && (
              <p className="text-sm text-red-600">{errors.code.message}</p>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor="credits">Credits (Optional)</Label>
            <Input
              id="credits"
              type="number"
              min="1"
              max="10"
              {...register('credits', {
                min: { value: 1, message: 'Minimum 1 credit' },
                max: { value: 10, message: 'Maximum 10 credits' },
                valueAsNumber: true,
              })}
              placeholder="e.g., 4"
            />
            {errors.credits && (
              <p className="text-sm text-red-600">{errors.credits.message}</p>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor="description">Description (Optional)</Label>
            <textarea
              id="description"
              {...register('description')}
              placeholder="Brief description of the subject"
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
