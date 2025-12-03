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
import { useCreateUniversity, useUpdateUniversity } from '@/lib/api/hooks/useAdminMutations';
import type { University } from '@/lib/api/courses';

interface UniversityFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  university?: University | null;
}

interface UniversityFormData {
  name: string;
  code: string;
  location: string;
  website?: string;
}

export function UniversityFormDialog({
  open,
  onOpenChange,
  university,
}: UniversityFormDialogProps) {
  const isEditing = !!university;
  const createMutation = useCreateUniversity();
  const updateMutation = useUpdateUniversity();

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useForm<UniversityFormData>({
    defaultValues: university
      ? {
          name: university.name,
          code: university.code,
          location: university.location,
          website: university.website,
        }
      : undefined,
  });

  useEffect(() => {
    if (university) {
      reset({
        name: university.name,
        code: university.code,
        location: university.location,
        website: university.website,
      });
    } else {
      reset({
        name: '',
        code: '',
        location: '',
        website: '',
      });
    }
  }, [university, reset]);

  const onSubmit = async (data: UniversityFormData) => {
    try {
      if (isEditing) {
        await updateMutation.mutateAsync({
          id: university.id,
          data,
        });
      } else {
        await createMutation.mutateAsync(data);
      }
      onOpenChange(false);
      reset();
    } catch (error) {
      console.error('Failed to save university:', error);
    }
  };

  const isPending = createMutation.isPending || updateMutation.isPending;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>{isEditing ? 'Edit University' : 'Create University'}</DialogTitle>
          <DialogDescription>
            {isEditing
              ? 'Update the university information below.'
              : 'Add a new university to the system.'}
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="name">University Name *</Label>
            <Input
              id="name"
              {...register('name', { required: 'Name is required' })}
              placeholder="e.g., Dr. A.P.J. Abdul Kalam Technical University"
            />
            {errors.name && (
              <p className="text-sm text-red-600">{errors.name.message}</p>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor="code">University Code *</Label>
            <Input
              id="code"
              {...register('code', { required: 'Code is required' })}
              placeholder="e.g., AKTU"
            />
            {errors.code && (
              <p className="text-sm text-red-600">{errors.code.message}</p>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor="location">Location *</Label>
            <Input
              id="location"
              {...register('location', { required: 'Location is required' })}
              placeholder="e.g., Lucknow, Uttar Pradesh"
            />
            {errors.location && (
              <p className="text-sm text-red-600">{errors.location.message}</p>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor="website">Website (Optional)</Label>
            <Input
              id="website"
              type="url"
              {...register('website')}
              placeholder="e.g., https://aktu.ac.in"
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
