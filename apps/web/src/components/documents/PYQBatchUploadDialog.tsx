'use client';

import React, { useState, useCallback, useEffect } from 'react';
import { 
  Upload, 
  FileText, 
  CheckCircle2, 
  AlertCircle,
  Trash2,
  ExternalLink,
  Plus,
  AlertTriangle,
  Loader2
} from 'lucide-react';
import { LoadingSpinner, InlineSpinner } from '@/components/ui/loading-spinner';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Card, CardContent } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { toast } from 'sonner';
import { useNotifications } from '@/providers/notification-provider';
import { useBatchIngestPYQs, useIndexingJobStatus } from '@/lib/api/hooks/useNotifications';
import type { AvailablePYQPaper } from '@/lib/api/pyq';
import { ALLOWED_FILE_EXTENSIONS, MAX_FILE_SIZE, validateFile } from '@/lib/api/documents';
import type { AISetupStatus } from '@/lib/api/notifications';

interface PYQBatchUploadDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  selectedPapers: AvailablePYQPaper[];
  subjectId: string;
  subjectName: string;
  onComplete?: () => void;
  /** AI setup status for the subject */
  aiSetupStatus?: AISetupStatus;
}

interface LocalFile {
  id: string;
  file: File;
  name: string;
  size: number;
  status: 'pending' | 'uploading' | 'done' | 'error';
  error?: string;
}

interface IngestStatus {
  paperId: string;
  status: 'pending' | 'ingesting' | 'done' | 'error';
  error?: string;
}

export function PYQBatchUploadDialog({
  open,
  onOpenChange,
  selectedPapers,
  subjectId,
  subjectName: _subjectName, // Reserved for future use (e.g., notification messages)
  onComplete,
  aiSetupStatus,
}: PYQBatchUploadDialogProps) {
  const { refetch: refetchNotifications } = useNotifications();
  const batchIngestMutation = useBatchIngestPYQs();
  
  // Check if AI/KB is ready for ingestion
  const isAIReady = aiSetupStatus === 'completed';
  const isAIPending = aiSetupStatus === 'pending' || aiSetupStatus === 'in_progress';
  const isAIFailed = aiSetupStatus === 'failed';
  
  // Track active job for polling
  const [activeJobId, setActiveJobId] = useState<number | null>(null);
  // Separate flag to control polling - prevents race conditions
  const [shouldPollJob, setShouldPollJob] = useState(false);
  
  // Poll job status when we have an active job AND polling is enabled
  const { data: jobStatus, isLoading: isPollingJob, error: pollingError, refetch: refetchJobStatus } = useIndexingJobStatus(
    activeJobId,
    shouldPollJob && activeJobId !== null // Enable polling when both conditions are true
  );
  
  // Debug: Log polling state changes
  useEffect(() => {
    console.log('[BatchUpload] Polling state:', {
      activeJobId,
      shouldPollJob,
      isEnabled: shouldPollJob && activeJobId !== null,
      jobStatus: jobStatus ? {
        id: jobStatus.id,
        status: jobStatus.status,
        progress: jobStatus.progress,
        completed_items: jobStatus.completed_items,
        total_items: jobStatus.total_items,
      } : null,
      isPollingJob,
      pollingError: pollingError?.message,
    });
  }, [activeJobId, shouldPollJob, jobStatus, isPollingJob, pollingError]);
  
  // When activeJobId is set, enable polling after a small delay to ensure React state is settled
  useEffect(() => {
    if (activeJobId !== null && !shouldPollJob) {
      console.log('[BatchUpload] activeJobId set, enabling polling after delay:', activeJobId);
      // Small delay to ensure state is settled, then enable polling
      const timer = setTimeout(() => {
        console.log('[BatchUpload] Enabling shouldPollJob');
        setShouldPollJob(true);
        // Also do an immediate refetch
        refetchJobStatus();
      }, 100);
      return () => clearTimeout(timer);
    }
  }, [activeJobId, shouldPollJob, refetchJobStatus]);
  
  // Local file uploads
  const [localFiles, setLocalFiles] = useState<LocalFile[]>([]);
  const [dragActive, setDragActive] = useState(false);
  
  // Ingest status tracking
  const [ingestStatuses, setIngestStatuses] = useState<Map<string, IngestStatus>>(new Map());
  const [isIngesting, setIsIngesting] = useState(false);
  const [progress, setProgress] = useState(0);
  
  // Update progress from job status
  useEffect(() => {
    if (jobStatus) {
      setProgress(jobStatus.progress || 0);
      
      // Update individual item statuses from job items
      if (jobStatus.items) {
        const newStatuses = new Map<string, IngestStatus>();
        jobStatus.items.forEach((item, index) => {
          const paper = selectedPapers[index];
          if (paper) {
            const paperId = `${paper.pdf_url}-${paper.year}-${paper.month}`;
            newStatuses.set(paperId, {
              paperId,
              status: item.status === 'completed' ? 'done' 
                    : item.status === 'failed' ? 'error'
                    : ['downloading', 'uploading', 'indexing'].includes(item.status) ? 'ingesting'
                    : 'pending',
              error: item.error_message,
            });
          }
        });
        setIngestStatuses(newStatuses);
      }
      
      // Check if job is complete
      // kb_indexing means all files uploaded successfully, now waiting for AI indexing (handled by cron)
      if (['completed', 'failed', 'partially_completed', 'cancelled', 'kb_indexing'].includes(jobStatus.status)) {
        console.log('[BatchUpload] Job completed with status:', jobStatus.status);
        setIsIngesting(false);
        setShouldPollJob(false); // Stop polling
        setActiveJobId(null);
        
        // Refetch notifications to get the final status
        refetchNotifications();
        
        // Show toast based on status
        if (jobStatus.status === 'completed') {
          toast.success(`All ${jobStatus.total_items} papers ingested`, {
            description: 'Papers are ready for AI search',
          });
        } else if (jobStatus.status === 'kb_indexing') {
          // All items uploaded, waiting for AI indexing (background process)
          toast.success(`All ${jobStatus.total_items} papers uploaded`, {
            description: 'AI indexing in progress. You\'ll be notified when complete.',
          });
        } else if (jobStatus.status === 'partially_completed') {
          toast.warning(`Ingestion completed with errors`, {
            description: `${jobStatus.completed_items} succeeded, ${jobStatus.failed_items} failed`,
          });
        } else if (jobStatus.status === 'failed') {
          toast.error('Batch ingestion failed', {
            description: jobStatus.error_message || 'An error occurred',
          });
        }
        
        if (onComplete) {
          onComplete();
        }
      }
    }
  }, [jobStatus, selectedPapers, refetchNotifications, onComplete]);

  // Handle file drop/select
  const handleFiles = useCallback((files: FileList | File[]) => {
    const fileArray = Array.from(files);
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
      });
    });
    
    setLocalFiles(prev => [...prev, ...newFiles]);
  }, []);

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
  }, [handleFiles]);

  const removeLocalFile = (id: string) => {
    setLocalFiles(prev => prev.filter(f => f.id !== id));
  };

  // Start batch ingest using the new batch API
  const handleStartIngest = async () => {
    if (selectedPapers.length === 0 && localFiles.length === 0) return;
    
    // Reset polling state for new job
    setShouldPollJob(false);
    setActiveJobId(null);
    
    setIsIngesting(true);
    setProgress(0);
    
    // Initialize all papers as pending
    const initialStatuses = new Map<string, IngestStatus>();
    selectedPapers.forEach(paper => {
      const paperId = `${paper.pdf_url}-${paper.year}-${paper.month}`;
      initialStatuses.set(paperId, { paperId, status: 'pending' });
    });
    setIngestStatuses(initialStatuses);
    
    // Prepare papers for batch API
    const papersToIngest = selectedPapers.map(paper => ({
      pdf_url: paper.pdf_url,
      title: paper.title,
      year: paper.year,
      month: paper.month,
      exam_type: paper.exam_type,
      source_name: paper.source_name,
    }));

    try {
      console.log('[BatchUpload] Starting batch ingest with papers:', papersToIngest.length);
      
      // Call the batch ingest API
      const result = await batchIngestMutation.mutateAsync({
        subjectId,
        papers: papersToIngest,
      });
      
      console.log('[BatchUpload] Batch ingest API response:', result);
      console.log('[BatchUpload] Setting activeJobId to:', result.job_id);
      
      // Start polling the job status
      setActiveJobId(result.job_id);
      
      toast.info(`Started batch ingestion`, {
        description: `Processing ${result.total_items} papers...`,
      });
      
    } catch (error: unknown) {
      const errorMessage = error instanceof Error ? error.message : 'Failed to start batch ingest';
      
      toast.error('Batch ingestion failed', {
        description: errorMessage,
      });
      
      // Mark all as error
      const errorStatuses = new Map<string, IngestStatus>();
      selectedPapers.forEach(paper => {
        const paperId = `${paper.pdf_url}-${paper.year}-${paper.month}`;
        errorStatuses.set(paperId, { paperId, status: 'error', error: errorMessage });
      });
      setIngestStatuses(errorStatuses);
      setIsIngesting(false);
    }

    // TODO: Process local files (upload to storage, then create document)
    // For now, we'll mark them as needing the upload tab
    for (const localFile of localFiles) {
      setLocalFiles(prev => 
        prev.map(f => f.id === localFile.id ? { ...f, status: 'done' as const } : f)
      );
    }
  };

  const formatFileSize = (bytes: number) => {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  };

  const canClose = !isIngesting;
  const hasItems = selectedPapers.length > 0 || localFiles.length > 0;

  return (
    <Dialog open={open} onOpenChange={canClose ? onOpenChange : undefined}>
      <DialogContent className="sm:max-w-2xl max-h-[85vh] flex flex-col overflow-hidden">
        <DialogHeader className="shrink-0">
          <DialogTitle className="flex items-center gap-2">
            <Upload className="h-5 w-5" />
            Batch Upload PYQ Papers
          </DialogTitle>
          <DialogDescription>
            Review selected papers and add any local PDFs before ingesting
          </DialogDescription>
        </DialogHeader>

        <div className="flex-1 min-h-0 overflow-y-auto">
          <ScrollArea className="max-h-[50vh]">
            <div className="space-y-6 pr-4">
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
                <AlertTitle>
                  {isAIPending ? 'AI Setup in Progress' : isAIFailed ? 'AI Setup Failed' : 'Knowledge Base Not Ready'}
                </AlertTitle>
                <AlertDescription>
                  {isAIPending ? (
                    <>PYQ papers require a Knowledge Base for AI-powered search. Please wait for the AI setup to complete before ingesting.</>
                  ) : isAIFailed ? (
                    <>AI setup failed for this subject. Please contact support or try again later.</>
                  ) : (
                    <>This subject does not have a Knowledge Base configured. PYQ papers cannot be ingested until AI setup is complete.</>
                  )}
                </AlertDescription>
              </Alert>
            )}
            
            {/* Selected External Papers */}
            {selectedPapers.length > 0 && (
              <div className="space-y-3">
                <div className="flex items-center justify-between">
                  <h4 className="text-sm font-medium flex items-center gap-2">
                    <ExternalLink className="h-4 w-4 text-muted-foreground" />
                    External Papers
                    <Badge variant="secondary">{selectedPapers.length}</Badge>
                  </h4>
                </div>
                <div className="space-y-2">
                  {selectedPapers.map((paper) => {
                    const paperId = `${paper.pdf_url}-${paper.year}-${paper.month}`;
                    const status = ingestStatuses.get(paperId);
                    
                    return (
                      <Card key={paperId} className="overflow-hidden">
                        <CardContent className="py-2 px-3">
                          <div className="flex items-center gap-3">
                            <div className="shrink-0">
                              {status?.status === 'done' ? (
                                <CheckCircle2 className="h-4 w-4 text-emerald-500" />
                              ) : status?.status === 'error' ? (
                                <AlertCircle className="h-4 w-4 text-destructive" />
                              ) : isIngesting || status?.status === 'ingesting' ? (
                                <LoadingSpinner size="sm" className="text-primary" />
                              ) : (
                                <FileText className="h-4 w-4 text-muted-foreground" />
                              )}
                            </div>
                            <div className="flex-1 min-w-0">
                              <p className="text-sm font-medium truncate">{paper.title}</p>
                              <div className="flex items-center gap-2 mt-1">
                                <Badge variant="outline" className="text-xs">{paper.year}</Badge>
                                {paper.month && (
                                  <Badge variant="outline" className="text-xs">{paper.month}</Badge>
                                )}
                                <span className="text-xs text-muted-foreground">{paper.source_name}</span>
                              </div>
                              {status?.error && (
                                <p className="text-xs text-destructive mt-1">{status.error}</p>
                              )}
                            </div>
                          </div>
                        </CardContent>
                      </Card>
                    );
                  })}
                </div>
              </div>
            )}

            {/* Local File Uploads */}
            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <h4 className="text-sm font-medium flex items-center gap-2">
                  <Upload className="h-4 w-4 text-muted-foreground" />
                  Local PDFs
                  {localFiles.length > 0 && (
                    <Badge variant="secondary">{localFiles.length}</Badge>
                  )}
                </h4>
              </div>

              {/* Drop zone */}
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
                  id="pyq-file-upload"
                  className="hidden"
                  multiple
                  accept={ALLOWED_FILE_EXTENSIONS.join(',')}
                  onChange={handleFileInput}
                />
                <label htmlFor="pyq-file-upload" className="cursor-pointer">
                  <Plus className="h-8 w-8 mx-auto mb-2 text-muted-foreground" />
                  <p className="text-sm font-medium">
                    Drop PDF files here or click to browse
                  </p>
                  <p className="text-xs text-muted-foreground mt-1">
                    Max {MAX_FILE_SIZE / (1024 * 1024)}MB per file
                  </p>
                </label>
              </div>

              {/* Local files list */}
              {localFiles.length > 0 && (
                <div className="space-y-2">
                  {localFiles.map((localFile) => (
                    <Card key={localFile.id} className="overflow-hidden">
                      <CardContent className="py-2 px-3">
                        <div className="flex items-center gap-3">
                          <div className="shrink-0">
                            {localFile.status === 'done' ? (
                              <CheckCircle2 className="h-4 w-4 text-emerald-500" />
                            ) : localFile.status === 'error' ? (
                              <AlertCircle className="h-4 w-4 text-destructive" />
                            ) : isIngesting || localFile.status === 'uploading' ? (
                              <LoadingSpinner size="sm" className="text-primary" />
                            ) : (
                              <FileText className="h-4 w-4 text-muted-foreground" />
                            )}
                          </div>
                          <div className="flex-1 min-w-0">
                            <p className="text-sm font-medium truncate">{localFile.name}</p>
                            <p className="text-xs text-muted-foreground">
                              {formatFileSize(localFile.size)}
                            </p>
                            {localFile.error && (
                              <p className="text-xs text-destructive mt-1">{localFile.error}</p>
                            )}
                          </div>
                          {localFile.status === 'pending' && !isIngesting && (
                            <Button
                              variant="ghost"
                              size="icon"
                              className="h-8 w-8 shrink-0"
                              onClick={() => removeLocalFile(localFile.id)}
                            >
                              <Trash2 className="h-4 w-4" />
                            </Button>
                          )}
                        </div>
                      </CardContent>
                    </Card>
                  ))}
                </div>
              )}
            </div>

            {/* Progress */}
            {isIngesting && (
              <div className="space-y-2">
                <div className="flex items-center justify-between text-sm">
                  <span>Progress</span>
                  <span>{Math.round(progress)}%</span>
                </div>
                <Progress value={progress} className="h-2" />
              </div>
            )}
            </div>
          </ScrollArea>
        </div>

        <DialogFooter className="shrink-0 gap-2 sm:gap-2">
          <Button
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={!canClose}
          >
            {isIngesting ? 'Processing...' : 'Cancel'}
          </Button>
          <Button
            onClick={handleStartIngest}
            disabled={!hasItems || isIngesting || !isAIReady}
            className="gap-2"
            title={!isAIReady ? 'Knowledge Base is required for PYQ ingestion' : undefined}
          >
            {isIngesting ? (
              <>
                <InlineSpinner />
                Ingesting...
              </>
            ) : !isAIReady ? (
              <>
                <AlertTriangle className="h-4 w-4" />
                AI Setup Required
              </>
            ) : (
              <>
                <Upload className="h-4 w-4" />
                Start Ingest ({selectedPapers.length + localFiles.length})
              </>
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
