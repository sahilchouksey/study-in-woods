'use client';

import { Pencil, Trash2, Plus } from 'lucide-react';
import { Button } from '@/components/ui/button';

interface AdminActionButtonsProps {
  onEdit?: () => void;
  onDelete?: () => void;
  onCreate?: () => void;
  createLabel?: string;
  isDeleting?: boolean;
  showEdit?: boolean;
  showDelete?: boolean;
  showCreate?: boolean;
}

export function AdminActionButtons({
  onEdit,
  onDelete,
  onCreate,
  createLabel = 'Create',
  isDeleting = false,
  showEdit = true,
  showDelete = true,
  showCreate = false,
}: AdminActionButtonsProps) {
  return (
    <div className="flex items-center gap-2">
      {showCreate && onCreate && (
        <Button
          onClick={onCreate}
          size="sm"
          className="bg-black hover:bg-neutral-800 dark:bg-white dark:hover:bg-neutral-200 dark:text-black"
        >
          <Plus className="h-4 w-4 mr-1" />
          {createLabel}
        </Button>
      )}
      
      {showEdit && onEdit && (
        <Button
          onClick={onEdit}
          size="sm"
          variant="outline"
          className="border-neutral-300 dark:border-neutral-700"
        >
          <Pencil className="h-4 w-4" />
        </Button>
      )}
      
      {showDelete && onDelete && (
        <Button
          onClick={onDelete}
          size="sm"
          variant="outline"
          disabled={isDeleting}
          className="border-red-300 text-red-600 hover:bg-red-50 hover:text-red-700 dark:border-red-800 dark:text-red-400 dark:hover:bg-red-950"
        >
          <Trash2 className="h-4 w-4" />
        </Button>
      )}
    </div>
  );
}
