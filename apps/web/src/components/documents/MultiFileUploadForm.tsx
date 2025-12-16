'use client';

import { useState, useCallback } from 'react';
import { 
  FileText, 
  Upload, 
  AlertCircle, 
  CheckCircle2,
  Plus,
  Trash2
} from 'lucide-react';
import { LoadingSpinner, InlineSpinner } from '@/components/ui/loading-spinner';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import { ScrollArea } from '@/components/ui/scroll-area';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  DocumentType,
  DOCUMENT_TYPE_LABELS,
  formatFileSize,
  validateFile,
  ALLOWED_FILE_EXTENSIONS,
  MAX_FILE_SIZE,
} from '@/lib/api/documents';
import { useUploadDocument } from '@/lib/api/hooks/useDocuments';
import { useExtractSyllabus } from '@/lib/api/hooks/useSyllabus';
import { useExtractPYQ } from '@/lib/api/hooks/usePYQ';
import { useNotifications } from '@/providers/notification-provider';

interface LocalFile {
  id: string;
  file: File;
  name: string;
  size: number;
  status: 'pending' | 'uploading' | 'done' | 'error';
  error?: string;
  documentType: DocumentType;
}

interface MultiFileUploadFormProps {
  subjectId: string;
  subjectName?: string;
  onSuccess?: () => void;
  excludeTypes?: DocumentType[];
  /** Default document type for new files */
  defaultType?: DocumentType;
}

export function MultiFileUploadForm({
  subjectId,
  subjectName = 'Subject',
  onSuccess,
  excludeTypes = [],
  defaultType = 'notes',
}: MultiFileUploadFormProps) {
  const { addNotification, updateNotification } = useNotifications();
  
  const [files, setFiles] = useState<LocalFile[]>([]);
  const [dragActive, setDragActive] = useState(false);
  const [isUploading, setIsUploading] = useState(false);
  const [progress, setProgress] = useState(0);

  const uploadDocument = useUploadDocument();
  const extractSyllabus = useExtractSyllabus();
  const extractPYQ = useExtractPYQ();

  // Available document types (excluding specified ones)
  const availableTypes = (Object.entries(DOCUMENT_TYPE_LABELS) as [DocumentType, string][])
    .filter(([value]) => !excludeTypes.includes(value));

  const handleFiles = useCallback((fileList: FileList | File[]) => {
    const fileArray = Array.from(fileList);
    const newFiles: LocalFile[] = [];

    fileArray.forEach(file => {
      const validation = validateFile(file);
      if (!validation.valid) {
        toast.error(`Invalid file: ${file.name}`, {
          description: validation.error,
        });
        return;
      }

      newFiles.push({
        id: `${Date.now()}-${Math.random().toString(36).slice(2, 9)}`,
        file,
        name: file.name,
        size: file.size,
        status: 'pending',
        documentType: defaultType,
      });
    });

    setFiles(prev => [...prev, ...newFiles]);
  }, [defaultType]);

  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setDragActive(false);

    if (e.dataTransfer.files?.length) {
      handleFiles(e.dataTransfer.files);
    }
  }, [handleFiles]);

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setDragActive(true);
  }, []);

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setDragActive(false);
  }, []);

  const handleFileInput = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files?.length) {
      handleFiles(e.target.files);
    }
    // Reset input so same file can be selected again
    e.target.value = '';
  }, [handleFiles]);

  const removeFile = (id: string) => {
    setFiles(prev => prev.filter(f => f.id !== id));
  };

  const updateFileType = (id: string, type: DocumentType) => {
    setFiles(prev => prev.map(f => f.id === id ? { ...f, documentType: type } : f));
  };

  const clearAllFiles = () => {
    setFiles([]);
  };

  const handleUploadAll = async () => {
    if (files.length === 0) return;

    setIsUploading(true);
    setProgress(0);

    const totalFiles = files.length;
    let completedFiles = 0;
    let successCount = 0;
    let errorCount = 0;

    // Create notification for tracking
    const notification = addNotification(
      'in_progress',
      'document_upload',
      `Uploading ${totalFiles} document${totalFiles > 1 ? 's' : ''}`,
      `Processing documents for ${subjectName}...`,
      {
        subjectId,
        subjectName,
        totalItems: totalFiles,
        completedItems: 0,
      }
    );

    for (const localFile of files) {
      // Update status to uploading
      setFiles(prev => prev.map(f => 
        f.id === localFile.id ? { ...f, status: 'uploading' } : f
      ));

      try {
        const result = await uploadDocument.mutateAsync({
          subjectId,
          file: localFile.file,
          type: localFile.documentType,
        });

        // Auto-trigger extraction for syllabus/pyq
        if (localFile.documentType === 'syllabus' && result.document) {
          try {
            await extractSyllabus.mutateAsync({
              documentId: String(result.document.id),
              subjectId,
            });
          } catch {
            // Extraction failed, but upload succeeded
          }
        } else if (localFile.documentType === 'pyq' && result.document) {
          try {
            await extractPYQ.mutateAsync({
              documentId: String(result.document.id),
              subjectId,
              async: true,
            });
          } catch {
            // Extraction failed, but upload succeeded
          }
        }

        setFiles(prev => prev.map(f =>
          f.id === localFile.id ? { ...f, status: 'done' } : f
        ));
        successCount++;
      } catch (error: unknown) {
        const errorMessage = error instanceof Error ? error.message : 'Upload failed';
        setFiles(prev => prev.map(f =>
          f.id === localFile.id ? { ...f, status: 'error', error: errorMessage } : f
        ));
        errorCount++;
      }

      completedFiles++;
      setProgress((completedFiles / totalFiles) * 100);

      updateNotification(notification.id, {
        metadata: {
          ...notification.metadata,
          completedItems: completedFiles,
          progress: (completedFiles / totalFiles) * 100,
        },
      });
    }

    // Update notification to complete
    updateNotification(notification.id, {
      type: errorCount > 0 ? 'warning' : 'success',
      title: `Upload ${errorCount > 0 ? 'Partially Complete' : 'Complete'}`,
      message: `${successCount} document${successCount !== 1 ? 's' : ''} uploaded${errorCount > 0 ? `, ${errorCount} failed` : ''}`,
    });

    // Show toast
    if (errorCount > 0) {
      toast.warning(`Upload completed with errors`, {
        description: `${successCount} succeeded, ${errorCount} failed`,
      });
    } else {
      toast.success(`All ${successCount} documents uploaded`, {
        description: 'Documents are being indexed and will be available shortly',
      });
    }

    setIsUploading(false);

    // Clear successful files after a delay
    setTimeout(() => {
      setFiles(prev => prev.filter(f => f.status !== 'done'));
      if (errorCount === 0) {
        onSuccess?.();
      }
    }, 1500);
  };

  const pendingCount = files.filter(f => f.status === 'pending').length;

  return (
    <div className="space-y-4">
      {/* Drop Zone */}
      <div
        className={`border-2 border-dashed rounded-lg p-6 text-center transition-colors ${
          dragActive
            ? 'border-primary bg-primary/5'
            : 'border-muted-foreground/25 hover:border-muted-foreground/50'
        }`}
        onDrop={handleDrop}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
      >
        <input
          type="file"
          id="multi-file-upload"
          className="hidden"
          multiple
          accept={ALLOWED_FILE_EXTENSIONS.join(',')}
          onChange={handleFileInput}
        />
        <label htmlFor="multi-file-upload" className="cursor-pointer">
          <Plus className="h-10 w-10 mx-auto mb-2 text-muted-foreground" />
          <p className="text-sm font-medium">
            Drop files here or click to browse
          </p>
          <p className="text-xs text-muted-foreground mt-1">
            Max {MAX_FILE_SIZE / (1024 * 1024)}MB per file - PDF, DOCX, TXT, and more
          </p>
          <p className="text-xs text-muted-foreground mt-1">
            You can select multiple files at once
          </p>
        </label>
      </div>

      {/* Files List */}
      {files.length > 0 && (
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium">
              {files.length} file{files.length !== 1 ? 's' : ''} selected
            </span>
            {!isUploading && (
              <Button variant="ghost" size="sm" onClick={clearAllFiles}>
                Clear All
              </Button>
            )}
          </div>

          <ScrollArea className="max-h-[300px]">
            <div className="space-y-2 pr-4">
              {files.map((localFile) => (
                <Card key={localFile.id} className="overflow-hidden">
                  <CardContent className="py-2 px-3">
                    <div className="flex items-center gap-3">
                      {/* Status Icon */}
                      <div className="shrink-0">
                        {localFile.status === 'uploading' && (
                          <LoadingSpinner size="sm" className="text-primary" />
                        )}
                        {localFile.status === 'done' && (
                          <CheckCircle2 className="h-4 w-4 text-emerald-500" />
                        )}
                        {localFile.status === 'error' && (
                          <AlertCircle className="h-4 w-4 text-destructive" />
                        )}
                        {localFile.status === 'pending' && (
                          <FileText className="h-4 w-4 text-muted-foreground" />
                        )}
                      </div>

                      {/* File Info */}
                      <div className="flex-1 min-w-0">
                        <p className="text-sm font-medium truncate">{localFile.name}</p>
                        <div className="flex items-center gap-2 mt-1">
                          <span className="text-xs text-muted-foreground">
                            {formatFileSize(localFile.size)}
                          </span>
                          {localFile.error && (
                            <span className="text-xs text-destructive truncate">
                              {localFile.error}
                            </span>
                          )}
                        </div>
                      </div>

                      {/* Type Selector */}
                      {localFile.status === 'pending' && !isUploading && (
                        <Select
                          value={localFile.documentType}
                          onValueChange={(v) => updateFileType(localFile.id, v as DocumentType)}
                        >
                          <SelectTrigger className="w-[130px] h-8 text-xs">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {availableTypes.map(([value, label]) => (
                              <SelectItem key={value} value={value} className="text-xs">
                                {label}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      )}

                      {/* Status Badge (for non-pending) */}
                      {localFile.status !== 'pending' && (
                        <Badge
                          variant={
                            localFile.status === 'done'
                              ? 'default'
                              : localFile.status === 'error'
                              ? 'destructive'
                              : 'secondary'
                          }
                          className="text-xs shrink-0"
                        >
                          {DOCUMENT_TYPE_LABELS[localFile.documentType]}
                        </Badge>
                      )}

                      {/* Remove Button */}
                      {localFile.status === 'pending' && !isUploading && (
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-8 w-8 shrink-0"
                          onClick={() => removeFile(localFile.id)}
                        >
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      )}
                    </div>
                  </CardContent>
                </Card>
              ))}
            </div>
          </ScrollArea>

          {/* Progress */}
          {isUploading && (
            <div className="space-y-2">
              <div className="flex items-center justify-between text-sm">
                <span>Uploading...</span>
                <span>{Math.round(progress)}%</span>
              </div>
              <Progress value={progress} className="h-2" />
            </div>
          )}

          {/* Upload Button */}
          <Button
            className="w-full"
            onClick={handleUploadAll}
            disabled={pendingCount === 0 || isUploading}
          >
            {isUploading ? (
              <>
                <InlineSpinner className="mr-2" />
                Uploading...
              </>
            ) : (
              <>
                <Upload className="mr-2 h-4 w-4" />
                Upload {pendingCount} Document{pendingCount !== 1 ? 's' : ''}
              </>
            )}
          </Button>
        </div>
      )}
    </div>
  );
}
