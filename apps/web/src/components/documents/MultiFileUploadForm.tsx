'use client';

import { useState, useCallback, useEffect } from 'react';
import { 
  FileText, 
  Upload, 
  AlertCircle, 
  CheckCircle2,
  Plus,
  Trash2,
  X,
  Clock,
  Loader2,
  AlertTriangle,
  Bot
} from 'lucide-react';
import { LoadingSpinner, InlineSpinner } from '@/components/ui/loading-spinner';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
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
import { useBatchUploadManager } from '@/lib/api/hooks/useNotifications';
import { INDEXING_JOB_STATUS_CONFIG, type IndexingJobItemStatus, type AISetupStatus } from '@/lib/api/notifications';

type LocalFileStatus = 'pending' | 'uploading' | 'done' | 'error';

interface LocalFile {
  id: string;
  file: File;
  name: string;
  size: number;
  status: LocalFileStatus;
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
  /** Use batch upload for multiple files (default: true) */
  useBatchUpload?: boolean;
  /** AI setup status for the subject */
  aiSetupStatus?: AISetupStatus;
}

// Threshold for using batch upload vs sequential upload
const BATCH_UPLOAD_THRESHOLD = 1; // Use batch upload for 2+ files

export function MultiFileUploadForm({
  subjectId,
  subjectName = 'Subject',
  onSuccess,
  excludeTypes = [],
  defaultType = 'notes',
  useBatchUpload = true,
  aiSetupStatus,
}: MultiFileUploadFormProps) {
  // Check if AI/KB is ready for uploads
  // If aiSetupStatus is undefined/not provided, assume AI is ready (backwards compatibility)
  const isAIReady = !aiSetupStatus || aiSetupStatus === 'completed';
  const isAIPending = aiSetupStatus === 'pending' || aiSetupStatus === 'in_progress';
  const isAIFailed = aiSetupStatus === 'failed';
  const [files, setFiles] = useState<LocalFile[]>([]);
  const [dragActive, setDragActive] = useState(false);
  const [isSequentialUploading, setIsSequentialUploading] = useState(false);
  const [sequentialProgress, setSequentialProgress] = useState(0);

  const uploadDocument = useUploadDocument();
  const extractSyllabus = useExtractSyllabus();
  const extractPYQ = useExtractPYQ();
  
  // Batch upload manager
  const batchManager = useBatchUploadManager(subjectId);

  // Available document types (excluding specified ones)
  const availableTypes = (Object.entries(DOCUMENT_TYPE_LABELS) as [DocumentType, string][])
    .filter(([value]) => !excludeTypes.includes(value));

  // Determine if we should use batch upload
  const shouldUseBatchUpload = useBatchUpload && files.length > BATCH_UPLOAD_THRESHOLD;
  
  // Track batch job completion
  useEffect(() => {
    if (batchManager.activeJob) {
      const status = batchManager.activeJob.status;
      if (status === 'completed' || status === 'partially_completed') {
        const successCount = batchManager.completedItems;
        const errorCount = batchManager.failedItems;
        
        if (errorCount > 0) {
          toast.warning(`Upload completed with errors`, {
            description: `${successCount} succeeded, ${errorCount} failed`,
          });
        } else {
          toast.success(`All ${successCount} documents uploaded`, {
            description: 'Documents are being indexed and will be available shortly',
          });
        }
        
        // Clear files and reset after a short delay
        setTimeout(() => {
          setFiles([]);
          batchManager.reset();
          if (errorCount === 0) {
            onSuccess?.();
          }
        }, 1500);
      } else if (status === 'failed') {
        toast.error('Batch upload failed', {
          description: batchManager.activeJob.error_message || 'An error occurred during upload',
        });
        batchManager.reset();
      }
    }
  }, [batchManager.activeJob?.status]);

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
    batchManager.reset();
  };

  // Sequential upload (for single file or when batch is disabled)
  const handleSequentialUpload = async () => {
    if (files.length === 0) return;

    setIsSequentialUploading(true);
    setSequentialProgress(0);

    const totalFiles = files.length;
    let completedFiles = 0;
    let successCount = 0;
    let errorCount = 0;

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
      setSequentialProgress((completedFiles / totalFiles) * 100);
    }

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

    setIsSequentialUploading(false);

    // Clear successful files after a delay
    setTimeout(() => {
      setFiles(prev => prev.filter(f => f.status !== 'done'));
      if (errorCount === 0) {
        onSuccess?.();
      }
    }, 1500);
  };

  // Batch upload (for multiple files)
  const handleBatchUpload = async () => {
    if (files.length === 0) return;

    try {
      const fileObjects = files.map(f => f.file);
      const types = files.map(f => f.documentType);
      
      await batchManager.startBatchUpload(fileObjects, types);
      
      // Update all files to uploading status
      setFiles(prev => prev.map(f => ({ ...f, status: 'uploading' as const })));
      
      toast.info('Batch upload started', {
        description: `Processing ${files.length} documents in background...`,
      });
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Failed to start batch upload';
      toast.error('Batch upload failed', { description: errorMessage });
    }
  };

  // Main upload handler - decides between batch and sequential
  const handleUploadAll = async () => {
    if (shouldUseBatchUpload) {
      await handleBatchUpload();
    } else {
      await handleSequentialUpload();
    }
  };

  const pendingCount = files.filter(f => f.status === 'pending').length;
  const isUploading = isSequentialUploading || batchManager.isProcessing;
  const progress = batchManager.isProcessing ? batchManager.progress : sequentialProgress;

  // Get item status from batch job
  const getItemStatus = (fileName: string): IndexingJobItemStatus | null => {
    if (!batchManager.activeJob?.items) return null;
    const item = batchManager.activeJob.items.find(
      i => i.title === fileName || i.source_url.includes(fileName)
    );
    return item?.status || null;
  };

  // Get status badge for a file during batch upload
  const getFileBatchStatus = (localFile: LocalFile) => {
    if (!batchManager.isProcessing && !batchManager.activeJob) return null;
    
    const itemStatus = getItemStatus(localFile.name);
    if (!itemStatus) return null;

    const statusConfig: Record<IndexingJobItemStatus, { label: string; variant: 'default' | 'secondary' | 'destructive' | 'outline' }> = {
      pending: { label: 'Queued', variant: 'outline' },
      downloading: { label: 'Processing', variant: 'secondary' },
      uploading: { label: 'Uploading', variant: 'secondary' },
      indexing: { label: 'Indexing', variant: 'default' },
      completed: { label: 'Done', variant: 'default' },
      failed: { label: 'Failed', variant: 'destructive' },
    };

    const config = statusConfig[itemStatus];
    return (
      <Badge variant={config.variant} className="text-xs">
        {config.label}
      </Badge>
    );
  };

  return (
    <div className="flex flex-col h-full">
      {/* Scrollable Content Area */}
      <div className="flex-1 min-h-0 overflow-y-auto space-y-4 pr-1">
        {/* AI Setup Status Warning */}
        {!isAIReady && aiSetupStatus && (
          <Alert variant={isAIFailed ? 'destructive' : 'default'} className={isAIPending ? 'border-blue-200 bg-blue-50' : ''}>
            {isAIPending ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : isAIFailed ? (
              <AlertCircle className="h-4 w-4" />
            ) : (
              <AlertTriangle className="h-4 w-4" />
            )}
            <AlertTitle className="flex items-center gap-2">
              {isAIPending ? 'AI Setup in Progress' : isAIFailed ? 'AI Setup Failed' : 'Knowledge Base Not Ready'}
            </AlertTitle>
            <AlertDescription>
              {isAIPending ? (
                <>Documents require a Knowledge Base for AI-powered search. Please wait for the AI setup to complete before uploading.</>
              ) : isAIFailed ? (
                <>AI setup failed for this subject. Please <a href="mailto:support@studyinwoods.app" className="underline hover:text-foreground">contact support</a> or try again later.</>
              ) : (
                <>This subject does not have a Knowledge Base configured. Documents cannot be uploaded until AI setup is complete.</>
              )}
            </AlertDescription>
          </Alert>
        )}

        {/* Batch Job Status Banner */}
        {batchManager.isProcessing && batchManager.activeJob && (
          <Card className="border-primary/50 bg-primary/5">
            <CardContent className="py-3 px-4">
              <div className="flex items-center justify-between mb-2">
                <div className="flex items-center gap-2">
                  <Loader2 className="h-4 w-4 animate-spin text-primary" />
                  <span className="text-sm font-medium">
                    {INDEXING_JOB_STATUS_CONFIG[batchManager.activeJob.status]?.label || 'Processing'}
                  </span>
                </div>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => batchManager.cancelJob(batchManager.activeJobId!)}
                  disabled={batchManager.isCancelling}
                >
                  <X className="h-4 w-4 mr-1" />
                  Cancel
                </Button>
              </div>
              <Progress value={batchManager.progress} className="h-2 mb-2" />
              <div className="flex items-center justify-between text-xs text-muted-foreground">
                <span>
                  {batchManager.completedItems} of {batchManager.totalItems} completed
                </span>
                {batchManager.failedItems > 0 && (
                  <span className="text-destructive">
                    {batchManager.failedItems} failed
                  </span>
                )}
              </div>
            </CardContent>
          </Card>
        )}

        {/* Drop Zone */}
        <div
          className={`border-2 border-dashed rounded-lg p-6 text-center transition-colors ${
            dragActive
              ? 'border-primary bg-primary/5'
              : 'border-muted-foreground/25 hover:border-muted-foreground/50'
          } ${isUploading || !isAIReady ? 'opacity-50 pointer-events-none' : ''}`}
          onDrop={isAIReady ? handleDrop : undefined}
          onDragOver={isAIReady ? handleDragOver : undefined}
          onDragLeave={isAIReady ? handleDragLeave : undefined}
        >
          <input
            type="file"
            id="multi-file-upload"
            className="hidden"
            multiple
            accept={ALLOWED_FILE_EXTENSIONS.join(',')}
            onChange={handleFileInput}
            disabled={isUploading || !isAIReady}
          />
          <label htmlFor="multi-file-upload" className={isAIReady ? 'cursor-pointer' : 'cursor-not-allowed'}>
            <Plus className="h-10 w-10 mx-auto mb-2 text-muted-foreground" />
            <p className="text-sm font-medium">
              {isAIReady ? 'Drop files here or click to browse' : 'Upload disabled - AI setup required'}
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
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium">
                  {files.length} file{files.length !== 1 ? 's' : ''} selected
                </span>
                {shouldUseBatchUpload && !isUploading && (
                  <Badge variant="outline" className="text-xs">
                    <Clock className="h-3 w-3 mr-1" />
                    Batch upload
                  </Badge>
                )}
              </div>
              {!isUploading && (
                <Button variant="ghost" size="sm" onClick={clearAllFiles}>
                  Clear All
                </Button>
              )}
            </div>

            <div className="space-y-2">
              {files.map((localFile) => (
                <Card key={localFile.id} className="overflow-hidden !py-0 !gap-0">
                  <CardContent className="py-2 px-3">
                    <div className="flex items-center gap-2 w-full overflow-hidden">
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
                        {localFile.status === 'pending' && !batchManager.isProcessing && (
                          <FileText className="h-4 w-4 text-muted-foreground" />
                        )}
                        {localFile.status === 'pending' && batchManager.isProcessing && (
                          <LoadingSpinner size="sm" className="text-primary" />
                        )}
                      </div>

                      {/* File Info - use w-0 flex-1 trick for truncation */}
                      <div className="flex-1 w-0 min-w-0">
                        <p className="text-sm font-medium truncate">{localFile.name}</p>
                        <div className="flex items-center gap-2 mt-0.5">
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

                      {/* Type Selector (only for pending, non-batch mode) */}
                      {localFile.status === 'pending' && !isUploading && (
                        <Select
                          value={localFile.documentType}
                          onValueChange={(v) => updateFileType(localFile.id, v as DocumentType)}
                        >
                          <SelectTrigger className="w-[100px] h-7 text-xs shrink-0">
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

                      {/* Batch Status Badge */}
                      {batchManager.isProcessing && getFileBatchStatus(localFile)}

                      {/* Status Badge (for non-pending, non-batch) */}
                      {localFile.status !== 'pending' && !batchManager.isProcessing && (
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
          </div>
        )}
      </div>

      {/* Sticky Footer - Progress and Upload Button */}
      {files.length > 0 && (
        <div className="flex-shrink-0 pt-4 mt-4 border-t bg-background sticky bottom-0 space-y-3">
          {/* Progress (for sequential upload) */}
          {isSequentialUploading && (
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
                {batchManager.isProcessing ? 'Processing...' : 'Uploading...'}
              </>
            ) : (
              <>
                <Upload className="mr-2 h-4 w-4" />
                Upload {pendingCount} Document{pendingCount !== 1 ? 's' : ''}
                {shouldUseBatchUpload && ' (Batch)'}
              </>
            )}
          </Button>
          
          {/* Batch upload info */}
          {shouldUseBatchUpload && !isUploading && (
            <p className="text-xs text-center text-muted-foreground">
              Multiple files will be processed in background with OCR support
            </p>
          )}
        </div>
      )}
    </div>
  );
}
