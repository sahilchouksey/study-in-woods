'use client';

import { useState, useCallback } from 'react';
import { FileText, Download, Trash2, RefreshCw, Upload, X, AlertCircle } from 'lucide-react';
import { LoadingSpinner, InlineSpinner } from '@/components/ui/loading-spinner';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent } from '@/components/ui/card';
import {
  Document,
  DocumentType,
  DOCUMENT_TYPE_LABELS,
  INDEXING_STATUS_CONFIG,
  formatFileSize,
  validateFile,
  ALLOWED_FILE_EXTENSIONS,
  MAX_FILE_SIZE,
} from '@/lib/api/documents';
import {
  useDocuments,
  useUploadDocument,
  useDeleteDocument,
  useDownloadDocument,
  useRefreshIndexingStatus,
} from '@/lib/api/hooks/useDocuments';
import { useExtractSyllabus } from '@/lib/api/hooks/useSyllabus';
import { useExtractPYQ } from '@/lib/api/hooks/usePYQ';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { DeleteConfirmationDialog } from '@/components/admin/DeleteConfirmationDialog';

// ============= Document Card Component =============

interface DocumentCardProps {
  document: Document;
  subjectId: string;
  canDelete: boolean;
  onRefreshStatus: () => void;
  isRefreshing: boolean;
}

function DocumentCard({
  document,
  subjectId,
  canDelete,
  onRefreshStatus,
  isRefreshing,
}: DocumentCardProps) {
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  
  const deleteDocument = useDeleteDocument();
  const downloadDocument = useDownloadDocument();

  const handleDownload = async () => {
    try {
      const result = await downloadDocument.mutateAsync({
        subjectId,
        documentId: String(document.id),
      });
      // Open download URL in new tab
      window.open(result.download_url, '_blank');
    } catch {
      toast.error('Failed to get download link');
    }
  };

  const handleDelete = async () => {
    try {
      await deleteDocument.mutateAsync({
        subjectId,
        documentId: String(document.id),
      });
      toast.success('Document deleted successfully');
      setDeleteDialogOpen(false);
    } catch {
      toast.error('Failed to delete document');
    }
  };

  const statusConfig = INDEXING_STATUS_CONFIG[document.indexing_status];

  return (
    <>
      <Card className="hover:shadow-md transition-shadow !py-0 !gap-0 w-full overflow-hidden">
        <CardContent className="p-4 w-full overflow-hidden">
          <div className="flex items-start gap-3 w-full">
            {/* File Icon */}
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10 shrink-0">
              <FileText className="h-5 w-5 text-primary" />
            </div>

            {/* File Info - use w-0 flex-1 trick to force truncation */}
            <div className="flex-1 w-0 min-w-0">
              <p className="font-medium text-sm truncate" title={document.filename}>
                {document.filename}
              </p>
              <div className="flex flex-wrap items-center gap-2 mt-1">
                <Badge variant="outline" className="text-xs">
                  {DOCUMENT_TYPE_LABELS[document.type]}
                </Badge>
                <span className="text-xs text-muted-foreground">
                  {formatFileSize(document.file_size)}
                </span>
                {document.page_count > 0 && (
                  <span className="text-xs text-muted-foreground">
                    {document.page_count} pages
                  </span>
                )}
              </div>
              
              {/* Indexing Status */}
              <div className="flex items-center gap-2 mt-2">
                <div className={`h-2 w-2 rounded-full ${statusConfig.color}`} />
                <span className="text-xs text-muted-foreground">
                  {statusConfig.label}
                </span>
                {document.indexing_status === 'in_progress' && (
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-6 px-2"
                    onClick={onRefreshStatus}
                    disabled={isRefreshing}
                  >
                    <RefreshCw className={`h-3 w-3 ${isRefreshing ? 'animate-spin' : ''}`} />
                  </Button>
                )}
                {document.indexing_status === 'failed' && document.indexing_error && (
                  <span className="text-xs text-destructive truncate max-w-32" title={document.indexing_error}>
                    {document.indexing_error}
                  </span>
                )}
              </div>
            </div>

            {/* Actions */}
            <div className="flex items-center gap-1 shrink-0">
              <Button
                variant="ghost"
                size="sm"
                className="h-8 w-8 p-0"
                onClick={handleDownload}
                disabled={downloadDocument.isPending}
              >
                {downloadDocument.isPending ? (
                  <InlineSpinner />
                ) : (
                  <Download className="h-4 w-4" />
                )}
              </Button>
              {canDelete && (
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-8 w-8 p-0 text-destructive hover:text-destructive"
                  onClick={() => setDeleteDialogOpen(true)}
                >
                  <Trash2 className="h-4 w-4" />
                </Button>
              )}
            </div>
          </div>
        </CardContent>
      </Card>

      <DeleteConfirmationDialog
        open={deleteDialogOpen}
        onOpenChange={setDeleteDialogOpen}
        onConfirm={handleDelete}
        title="Delete Document"
        description={`Are you sure you want to delete "${document.filename}"?`}
        cascadeWarning="This will remove the document from storage and the AI knowledge base."
        isDeleting={deleteDocument.isPending}
      />
    </>
  );
}

// ============= Document Upload Form Component =============

interface DocumentUploadFormProps {
  subjectId: string;
  onSuccess?: () => void;
  excludeTypes?: DocumentType[];
}

function DocumentUploadForm({ subjectId, onSuccess, excludeTypes = [] }: DocumentUploadFormProps) {
  const [file, setFile] = useState<File | null>(null);
  const [documentType, setDocumentType] = useState<DocumentType>('notes');
  const [dragActive, setDragActive] = useState(false);
  const [validationError, setValidationError] = useState<string | null>(null);

  const uploadDocument = useUploadDocument();
  const extractSyllabus = useExtractSyllabus();
  const extractPYQ = useExtractPYQ();

  const handleFile = useCallback((selectedFile: File) => {
    const validation = validateFile(selectedFile);
    if (!validation.valid) {
      setValidationError(validation.error || 'Invalid file');
      setFile(null);
      return;
    }
    setValidationError(null);
    setFile(selectedFile);
  }, []);

  const handleDrag = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (e.type === 'dragenter' || e.type === 'dragover') {
      setDragActive(true);
    } else if (e.type === 'dragleave') {
      setDragActive(false);
    }
  }, []);

  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setDragActive(false);
    
    if (e.dataTransfer.files && e.dataTransfer.files[0]) {
      handleFile(e.dataTransfer.files[0]);
    }
  }, [handleFile]);

  const handleFileInput = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files[0]) {
      handleFile(e.target.files[0]);
    }
  }, [handleFile]);

  const handleUpload = async () => {
    if (!file) return;

    try {
      const result = await uploadDocument.mutateAsync({
        subjectId,
        file,
        type: documentType,
      });
      
      // Auto-trigger syllabus extraction if document type is syllabus
      if (documentType === 'syllabus' && result.document) {
        toast.success('Document uploaded! Starting syllabus extraction...');
        try {
          await extractSyllabus.mutateAsync({
            documentId: String(result.document.id),
            subjectId,
          });
          toast.success('Syllabus extraction started. Check the Syllabus tab for results.');
        } catch {
          toast.info('Document uploaded but extraction could not start automatically. You can trigger it manually from the Syllabus tab.');
        }
      } else if (documentType === 'pyq' && result.document) {
        // Auto-trigger PYQ extraction if document type is pyq
        toast.success('Document uploaded! Starting PYQ extraction...');
        try {
          await extractPYQ.mutateAsync({
            documentId: String(result.document.id),
            subjectId,
            async: true,
          });
          toast.success('PYQ extraction started. Check the PYQs tab for results.');
        } catch {
          toast.info('Document uploaded but extraction could not start automatically. You can trigger it manually from the PYQs tab.');
        }
      } else {
        toast.success('Document uploaded successfully! It will be indexed shortly.');
      }
      
      setFile(null);
      onSuccess?.();
    } catch {
      toast.error('Failed to upload document');
    }
  };

  const clearFile = () => {
    setFile(null);
    setValidationError(null);
  };

  return (
    <div className="space-y-4">
      {/* Document Type Selection */}
      <div className="space-y-2">
        <label className="text-sm font-medium">Document Type</label>
        <Select value={documentType} onValueChange={(v) => setDocumentType(v as DocumentType)}>
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {(Object.entries(DOCUMENT_TYPE_LABELS) as [DocumentType, string][])
              .filter(([value]) => !excludeTypes.includes(value))
              .map(([value, label]) => (
                <SelectItem key={value} value={value}>
                  {label}
                </SelectItem>
              ))}
          </SelectContent>
        </Select>
      </div>

      {/* Drop Zone */}
      <div
        className={`relative border-2 border-dashed rounded-lg p-6 transition-colors ${
          dragActive
            ? 'border-primary bg-primary/5'
            : file
            ? 'border-green-500 bg-green-500/5'
            : 'border-muted-foreground/25 hover:border-muted-foreground/50'
        }`}
        onDragEnter={handleDrag}
        onDragLeave={handleDrag}
        onDragOver={handleDrag}
        onDrop={handleDrop}
      >
        <input
          type="file"
          className="absolute inset-0 w-full h-full opacity-0 cursor-pointer"
          onChange={handleFileInput}
          accept={ALLOWED_FILE_EXTENSIONS.join(',')}
        />

        {file ? (
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <FileText className="h-8 w-8 text-green-500" />
              <div>
                <p className="font-medium text-sm">{file.name}</p>
                <p className="text-xs text-muted-foreground">{formatFileSize(file.size)}</p>
              </div>
            </div>
            <Button variant="ghost" size="sm" onClick={clearFile}>
              <X className="h-4 w-4" />
            </Button>
          </div>
        ) : (
          <div className="text-center">
            <Upload className="h-10 w-10 mx-auto text-muted-foreground" />
            <p className="mt-2 text-sm font-medium">
              Drop your file here or click to browse
            </p>
            <p className="mt-1 text-xs text-muted-foreground">
              Max {MAX_FILE_SIZE / 1024 / 1024}MB - PDF, DOCX, TXT, MD, CSV, XLSX, PPTX, HTML, JSON
            </p>
          </div>
        )}
      </div>

      {/* Validation Error */}
      {validationError && (
        <div className="flex items-center gap-2 text-destructive text-sm">
          <AlertCircle className="h-4 w-4" />
          {validationError}
        </div>
      )}

      {/* Upload Button */}
      <Button
        className="w-full"
        onClick={handleUpload}
        disabled={!file || uploadDocument.isPending}
      >
        {uploadDocument.isPending ? (
          <>
            <InlineSpinner className="mr-2" />
            Uploading...
          </>
        ) : (
          <>
            <Upload className="mr-2 h-4 w-4" />
            Upload Document
          </>
        )}
      </Button>
    </div>
  );
}

// ============= Document List Component =============

interface DocumentListProps {
  subjectId: string;
  userId?: string;
  isAdmin: boolean;
}

function DocumentList({ subjectId, userId, isAdmin }: DocumentListProps) {
  const { data, isLoading, error } = useDocuments(subjectId);
  const refreshStatus = useRefreshIndexingStatus();
  const [refreshingId, setRefreshingId] = useState<number | null>(null);

  const handleRefreshStatus = async (documentId: number) => {
    setRefreshingId(documentId);
    try {
      await refreshStatus.mutateAsync({
        subjectId,
        documentId: String(documentId),
      });
    } catch {
      toast.error('Failed to refresh status');
    } finally {
      setRefreshingId(null);
    }
  };

  if (isLoading) {
    return (
      <div className="flex justify-center py-8">
        <LoadingSpinner size="md" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="text-center py-8 text-destructive">
        <AlertCircle className="h-8 w-8 mx-auto mb-2" />
        <p>Failed to load documents</p>
      </div>
    );
  }

  const documents = data?.data || [];

  if (documents.length === 0) {
    return (
      <div className="text-center py-8 text-muted-foreground">
        <FileText className="h-12 w-12 mx-auto mb-3 opacity-50" />
        <p className="font-medium">No documents yet</p>
        <p className="text-sm mt-1">Upload your first document to get started</p>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {documents.map((doc) => (
        <DocumentCard
          key={doc.id}
          document={doc}
          subjectId={subjectId}
          canDelete={isAdmin || String(doc.uploaded_by_user_id) === userId}
          onRefreshStatus={() => handleRefreshStatus(doc.id)}
          isRefreshing={refreshingId === doc.id}
        />
      ))}
    </div>
  );
}

// ============= Exported Components =============

export { DocumentCard, DocumentUploadForm, DocumentList };
