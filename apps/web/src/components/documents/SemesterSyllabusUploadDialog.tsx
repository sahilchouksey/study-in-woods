'use client';

import { useState, useCallback, useEffect, useRef } from 'react';
import { Upload, FileText, X, AlertCircle, FolderOpen, CheckCircle, Sparkles } from 'lucide-react';
import { motion, AnimatePresence } from 'framer-motion';
import { toast } from 'sonner';
import { useQueryState, parseAsInteger, parseAsString } from 'nuqs';
import { Button } from '@/components/ui/button';
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Badge } from '@/components/ui/badge';
import {
    validateFile,
    formatFileSize,
    MAX_FILE_SIZE,
} from '@/lib/api/documents';
import { apiClient } from '@/lib/api/client';
import { useSSE } from '@/lib/hooks/useSSE';
import { ExtractionProgress } from '@/components/progress/ExtractionProgress';

interface Semester {
    id: string;
    name: string;
    number: number;
}

interface Subject {
    id: string;
    name: string;
    code: string;
    credits: number;
}

interface SemesterSyllabusUploadDialogProps {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    semester: Semester | null;
    isAuthenticated: boolean;
    onSubjectsCreated?: () => void;
    semesterId?: string;
}

type UploadState = 'idle' | 'uploading-file' | 'extracting' | 'complete' | 'error' | 'reconnecting';

export function SemesterSyllabusUploadDialog({
    open,
    onOpenChange,
    semester,
    isAuthenticated,
    onSubjectsCreated,
    semesterId,
}: SemesterSyllabusUploadDialogProps) {
    const [file, setFile] = useState<File | null>(null);
    const [dragActive, setDragActive] = useState(false);
    const [validationError, setValidationError] = useState<string | null>(null);
    const [uploadState, setUploadState] = useState<UploadState>('idle');
    const [uploadResult, setUploadResult] = useState<{
        syllabusesCount: number;
        subjectsCreated: Subject[];
    } | null>(null);
    console.info(uploadResult)

    // URL state persistence with nuqs - survives page refresh
    const [urlDocumentId, setUrlDocumentId] = useQueryState('extractDocId', parseAsInteger);
    const [urlJobId, setUrlJobId] = useQueryState('extractJobId', parseAsString);
    const [urlExtracting, setUrlExtracting] = useQueryState('extracting', parseAsString);

    // Get auth token for SSE
    const getAuthToken = useCallback(() => {
        if (typeof window !== 'undefined') {
            return localStorage.getItem('access_token') || '';
        }
        return '';
    }, []);

    // SSE hook for streaming progress
    const targetSemesterId = semester?.id || semesterId;
    const [documentId, setDocumentId] = useState<number | null>(urlDocumentId);
    const [jobId, setJobId] = useState<string | null>(urlJobId);
    const [sseUrl, setSseUrl] = useState<string>('');
    const [sseConnected, setSseConnected] = useState(false);
    const [fallbackAttempted, setFallbackAttempted] = useState(false);
    const [isReconnecting, setIsReconnecting] = useState(false);

    const sse = useSSE({
        url: sseUrl,
        token: getAuthToken(),
        onMessage: (event) => {
            // Store job ID from events - this is critical for reconnection!
            if (event.job_id && !jobId) {
                setJobId(event.job_id);
                // Immediately persist to URL for page refresh recovery
                setUrlJobId(event.job_id);
            }

            if (event.type === 'complete') {
                // Use actual subject data from backend if available, otherwise fall back to IDs
                const subjectsCreated = event.result_subjects
                    ? event.result_subjects.map((subject) => ({
                        id: String(subject.id),
                        name: subject.name,
                        code: subject.code,
                        credits: subject.credits,
                    }))
                    : (event.result_syllabus_ids || []).map((id, index) => ({
                        id: String(id),
                        name: `Subject ${index + 1}`,
                        code: `SUB${index + 1}`,
                        credits: 3,
                    }));

                setUploadResult({
                    syllabusesCount: subjectsCreated.length,
                    subjectsCreated,
                });
                setUploadState('complete');
                setIsReconnecting(false);
                // Don't clear URL state yet - keep it until user closes dialog
                toast.success(`Syllabus processed! ${subjectsCreated.length} subjects created.`);
                onSubjectsCreated?.();
            }

            if (event.type === 'error') {
                setUploadState('error');
                setIsReconnecting(false);
                setValidationError(event.error_message || event.message || 'Extraction failed');
                // Don't clear URL state yet - keep it until user closes dialog
            }
        },
        onError: () => {
            console.error('SSE connection error');
            // If reconnecting and SSE fails, try to get job status
            if (isReconnecting) {
                checkJobStatus();
            }
        },
    });

    // Clear URL extraction state
    const clearUrlState = useCallback(() => {
        setUrlDocumentId(null);
        setUrlJobId(null);
        setUrlExtracting(null);
    }, [setUrlDocumentId, setUrlJobId, setUrlExtracting]);

    // Check job status (used for reconnection after page refresh)
    const checkJobStatus = useCallback(async () => {
        if (!urlJobId) return;

        try {
            const response = await apiClient.get(`/api/v2/extraction-jobs/${urlJobId}`);
            const job = response.data.data;

            if (job.status === 'completed') {
                // Use result_subjects from job if available (contains actual subject data)
                const subjectsCreated = job.result_subjects
                    ? job.result_subjects.map((subject: any) => ({
                        id: String(subject.id),
                        name: subject.name,
                        code: subject.code,
                        credits: subject.credits,
                    }))
                    : [];

                setUploadResult({
                    syllabusesCount: subjectsCreated.length || (job.result_syllabus_ids || []).length,
                    subjectsCreated,
                });

                setUploadState('complete');
                setIsReconnecting(false);
                clearUrlState();
                const createdCount = job.result_syllabus_ids?.length || 0;
                toast.success(`Syllabus processed! ${createdCount} subjects created.`);
                onSubjectsCreated?.();
            } else if (job.status === 'failed' || job.status === 'cancelled') {
                setUploadState('error');
                setIsReconnecting(false);
                setValidationError(job.error || job.message || 'Extraction failed');
                clearUrlState();
            } else {
                // Job still in progress - try to reconnect to stream
                reconnectToJob();
            }
        } catch (error) {
            console.error('Failed to check job status:', error);
            setUploadState('error');
            setIsReconnecting(false);
            setValidationError('Failed to reconnect to extraction job');
            clearUrlState();
        }
    }, [urlJobId, clearUrlState, onSubjectsCreated]);

    // Reconnect to an existing job stream
    const reconnectToJob = useCallback(() => {
        if (!urlJobId) return;

        const reconnectUrl = `${process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'}/api/v2/extraction-jobs/${urlJobId}/stream`;
        console.log('Reconnecting to job stream:', reconnectUrl);
        setSseUrl(reconnectUrl);
    }, [urlJobId]);

    // Fallback to non-streaming extraction if SSE fails
    const handleFallbackExtraction = async () => {
        if (!documentId) return;

        try {
            const response = await apiClient.post(
                `/api/v1/documents/${documentId}/extract-syllabus`,
                {},
                { timeout: 180000 }
            );

            const result = response.data.data;
            setUploadResult({
                syllabusesCount: result.syllabuses_count || 0,
                subjectsCreated: result.subjects_created || [],
            });
            setUploadState('complete');
            isFreshUpload.current = false; // Reset fresh upload flag
            clearUrlState();
            toast.success(`Syllabus processed! ${result.subjects_created?.length || 0} subjects created.`);
            onSubjectsCreated?.();
        } catch (error: unknown) {
            const err = error as { response?: { data?: { message?: string } }; message?: string };
            const errorMessage = err?.response?.data?.message || err?.message || 'Extraction failed';
            setValidationError(errorMessage);
            setUploadState('error');
            isFreshUpload.current = false; // Reset fresh upload flag on error too
            clearUrlState();
        }
    };

    // On mount, check if we have an extraction in progress (from URL state)
    const hasCheckedReconnection = useRef(false);
    const isFreshUpload = useRef(false); // Track if we're in the middle of a fresh upload

    useEffect(() => {
        if (hasCheckedReconnection.current) return;

        // If URL has extraction state AND dialog is open, try to reconnect
        // Skip if we're in the middle of a fresh upload
        if (open && urlExtracting === 'true' && urlDocumentId && !isFreshUpload.current) {
            hasCheckedReconnection.current = true;
            console.log('Found extraction in progress from URL state, reconnecting...');
            setDocumentId(urlDocumentId);
            setJobId(urlJobId || null);
            setUploadState('reconnecting');
            setIsReconnecting(true);

            // If we have a job ID, check its status first
            if (urlJobId) {
                checkJobStatus();
            } else {
                // No job ID means we can't reconnect safely (would trigger new extraction)
                // Clear the reconnection state
                console.warn('Cannot reconnect without job ID - clearing extraction state');
                setIsReconnecting(false);
                setUploadState('idle');
                clearUrlState();
            }
        }
    }, [open, urlExtracting, urlDocumentId, urlJobId, checkJobStatus]);

    const handleFile = useCallback((selectedFile: File) => {
        const validation = validateFile(selectedFile);
        if (!validation.valid) {
            setValidationError(validation.error || 'Invalid file');
            setFile(null);
            return;
        }

        // Additional validation for PDF only
        const isPDF = selectedFile.name.toLowerCase().endsWith('.pdf');
        if (!isPDF) {
            setValidationError('Only PDF files are supported for syllabus upload');
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

    // Two-step SSE streaming upload
    const handleUploadWithStreaming = async () => {
        if (!file || !targetSemesterId) return;

        // Cancel any existing operations first
        sse.disconnect();
        clearUrlState();
        connectionAttemptedRef.current = false;

        setUploadState('uploading-file');
        setValidationError(null);
        setDocumentId(null);
        setJobId(null);
        setSseUrl('');
        setFallbackAttempted(false);

        try {
            // Step 1: Upload the file to get document_id
            const formData = new FormData();
            formData.append('file', file);

            const uploadResponse = await apiClient.post(
                `/api/v2/semesters/${targetSemesterId}/syllabus/upload`,
                formData,
                {
                    // IMPORTANT: Don't set Content-Type manually for FormData!
                    headers: {
                        'Content-Type': undefined as unknown as string,
                    },
                    timeout: 300000, // 5 minutes for upload + OCR processing
                }
            );

            console.log('Upload response:', uploadResponse.data);
            const data = uploadResponse.data.data;
            const docId = data?.document_id;

            console.log('Document ID:', docId);

            if (!docId) {
                throw new Error('No document ID returned from upload');
            }

            // Step 2: Set document ID and SSE URL, then start extraction
            isFreshUpload.current = true; // Mark as fresh upload to prevent reconnection logic
            setDocumentId(docId);
            setUploadState('extracting');

            // Persist to URL for page refresh recovery
            await setUrlDocumentId(docId);
            await setUrlExtracting('true');

            // Use the SSE URL from response or construct it
            const extractionUrl = data?.sse_url
                ? `${process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'}${data.sse_url}`
                : `${process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'}/api/v2/documents/${docId}/extract-syllabus?stream=true`;

            console.log('SSE URL:', extractionUrl);
            console.log('Setting sseUrl and uploadState to extracting');
            setSseUrl(extractionUrl);
            // Note: Connection will be triggered by useEffect watching sseUrl

        } catch (error: unknown) {
            const err = error as { response?: { data?: { message?: string } }; message?: string };
            const errorMessage = err?.response?.data?.message || err?.message || 'Failed to upload syllabus';
            toast.error(errorMessage);
            setValidationError(errorMessage);
            setUploadState('error');
            clearUrlState();
        }
    };

    // Non-streaming upload (fallback)
    const handleUploadNonStreaming = async () => {
        if (!file || !targetSemesterId) return;

        setUploadState('uploading-file');
        setValidationError(null);

        try {
            const formData = new FormData();
            formData.append('file', file);

            const response = await apiClient.post(
                `/api/v1/semesters/${targetSemesterId}/syllabus/upload`,
                formData,
                {
                    // IMPORTANT: Don't set Content-Type manually for FormData!
                    headers: {
                        'Content-Type': undefined as unknown as string,
                    },
                    timeout: 180000, // 3 minutes for extraction
                }
            );

            const result = response.data.data;

            setUploadResult({
                syllabusesCount: result.syllabuses_count || 0,
                subjectsCreated: result.subjects_created || [],
            });
            setUploadState('complete');

            toast.success(`Syllabus uploaded and ${result.subjects_created?.length || 0} subject(s) created!`);

            setFile(null);
            onSubjectsCreated?.();
        } catch (error: unknown) {
            const err = error as { response?: { data?: { message?: string } }; message?: string };
            const errorMessage = err?.response?.data?.message || err?.message || 'Failed to upload syllabus';
            toast.error(errorMessage);
            setValidationError(errorMessage);
            setUploadState('error');
        }
    };

    const handleUpload = async () => {
        // Try SSE streaming first, fall back to non-streaming if it fails
        // SSE streaming provides real-time progress updates
        await handleUploadWithStreaming();
    };

    const clearFile = () => {
        setFile(null);
        setValidationError(null);
    };

    // Check if dialog can be closed
    const isProcessing = uploadState === 'uploading-file' || uploadState === 'extracting' || uploadState === 'reconnecting';
    const canClose = !isProcessing;

    const handleClose = () => {
        // NEVER allow closing during extraction - this is critical
        if (isProcessing) {
            toast.error('Cannot close while extraction is in progress. Please wait for it to complete.');
            return;
        }
        setFile(null);
        setValidationError(null);
        setUploadResult(null);
        setUploadState('idle');
        setDocumentId(null);
        setJobId(null);
        clearUrlState();
        sse.disconnect();
        onOpenChange(false);
    };

    // Prevent closing via escape key or clicking outside during extraction
    const handleOpenChange = (newOpen: boolean) => {
        if (!newOpen && isProcessing) {
            // Prevent closing
            toast.error('Cannot close while extraction is in progress. Please wait for it to complete.');
            return;
        }
        if (!newOpen) {
            handleClose();
        }
    };

    // Reset state when dialog opens
    const sseResetRef = useRef(sse.reset);
    sseResetRef.current = sse.reset;

    useEffect(() => {
        if (open) {
            // Only reset if ALL of these conditions are true:
            // 1. Not reconnecting from URL state
            // 2. Not in complete state (preserve completion UI)
            // 3. Not in error state (preserve error UI)
            // 4. Not actively extracting (preserve extraction state)
            // 5. Not uploading file (preserve upload state)
            const shouldReset =
                urlExtracting !== 'true' &&
                uploadState !== 'complete' &&
                uploadState !== 'error' &&
                uploadState !== 'extracting' &&
                uploadState !== 'uploading-file' &&
                uploadState !== 'reconnecting';

            if (shouldReset) {
                setUploadState('idle');
                setUploadResult(null);
                setValidationError(null);
                setFile(null);
                setDocumentId(null);
                setJobId(null);
                setSseUrl('');
                setSseConnected(false);
                setFallbackAttempted(false);
                // Reset SSE hook state to initial values
                sseResetRef.current();
            }
        }
    }, [open, urlExtracting, uploadState]);

    // Connect to SSE when URL is set and we're in extracting state (only once)
    // Using a ref to track connection attempt to avoid cleanup race condition
    const connectionAttemptedRef = useRef(false);
    const sseConnectRef = useRef(sse.connect);
    sseConnectRef.current = sse.connect;

    useEffect(() => {
        console.log('SSE useEffect triggered:', { sseUrl, uploadState, sseConnected });
        if (sseUrl && uploadState === 'extracting' && !connectionAttemptedRef.current) {
            connectionAttemptedRef.current = true;
            setSseConnected(true); // Mark that we've attempted connection (for UI)

            // Connect immediately since we're using a ref to prevent duplicates
            console.log('Connecting to SSE:', sseUrl);
            sseConnectRef.current();
        }
    }, [sseUrl, uploadState, sseConnected]);

    // Reset the refs when dialog closes
    useEffect(() => {
        if (!open) {
            connectionAttemptedRef.current = false;
            isFreshUpload.current = false;
            hasCheckedReconnection.current = false;
        }
    }, [open]);

    // Clear URL state after completion/error state is rendered
    // This prevents the reset logic from running before the user sees the completion UI
    useEffect(() => {
        if ((uploadState === 'complete' || uploadState === 'error') && urlExtracting === 'true') {
            // Use setTimeout to ensure the completion UI is rendered first
            const timer = setTimeout(() => {
                clearUrlState();
            }, 100);
            return () => clearTimeout(timer);
        }
    }, [uploadState, urlExtracting, clearUrlState]);

    // Handle SSE error by falling back to non-streaming (only once)
    // Use ref for handleFallbackExtraction to avoid dependency issues
    const handleFallbackExtractionRef = useRef(handleFallbackExtraction);
    handleFallbackExtractionRef.current = handleFallbackExtraction;

    useEffect(() => {
        if (sse.status === 'error' && uploadState === 'extracting' && documentId && !fallbackAttempted) {
            setFallbackAttempted(true);
            console.log('SSE failed, attempting fallback extraction');
            // Clear the SSE error since we're falling back
            // Don't show error UI - the fallback will handle it
            handleFallbackExtractionRef.current();
        }
    }, [sse.status, uploadState, documentId, fallbackAttempted]);

    return (
        <Dialog open={open} onOpenChange={handleOpenChange}>
            <DialogContent className="sm:max-w-2xl max-h-[90vh] flex flex-col overflow-hidden">
                <DialogHeader className="pb-4 border-b">
                    <div className="flex items-start gap-3">
                        <motion.div
                            className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10 shrink-0"
                            animate={isProcessing ? { scale: [1, 1.05, 1] } : {}}
                            transition={{ duration: 1, repeat: isProcessing ? Infinity : 0 }}
                        >
                            {isProcessing ? (
                                <Sparkles className="h-5 w-5 text-primary" />
                            ) : (
                                <Upload className="h-5 w-5 text-primary" />
                            )}
                        </motion.div>
                        <div className="flex-1 min-w-0">
                            <DialogTitle className="text-lg">
                                {isProcessing ? 'Processing Syllabus' : 'Upload Syllabus'}
                            </DialogTitle>
                            {semester && (
                                <div className="flex flex-wrap items-center gap-2 mt-1">
                                    <Badge variant="outline" className="text-xs">
                                        {semester.name}
                                    </Badge>
                                </div>
                            )}
                            <DialogDescription className="mt-2">
                                {isProcessing
                                    ? 'AI is analyzing your syllabus and extracting subjects...'
                                    : 'Upload a syllabus PDF to automatically create subjects and extract course content'
                                }
                            </DialogDescription>
                        </div>
                    </div>
                </DialogHeader>

                <div className="flex-1 min-h-0 mt-4">
                    <AnimatePresence mode="wait">
                        {!semester && !semesterId ? (
                            <motion.div
                                key="no-semester"
                                initial={{ opacity: 0, y: 10 }}
                                animate={{ opacity: 1, y: 0 }}
                                exit={{ opacity: 0, y: -10 }}
                                className="flex flex-col items-center justify-center h-[350px] text-center text-muted-foreground"
                            >
                                <AlertCircle className="h-12 w-12 mb-3 opacity-50" />
                                <p className="font-medium">No Semester Selected</p>
                                <p className="text-sm mt-1">
                                    Please select a semester first
                                </p>
                                <Button
                                    variant="outline"
                                    className="mt-4"
                                    onClick={() => onOpenChange(false)}
                                >
                                    Close
                                </Button>
                            </motion.div>
                        ) : !isAuthenticated ? (
                            <motion.div
                                key="not-authenticated"
                                initial={{ opacity: 0, y: 10 }}
                                animate={{ opacity: 1, y: 0 }}
                                exit={{ opacity: 0, y: -10 }}
                                className="flex flex-col items-center justify-center h-[350px] text-center text-muted-foreground"
                            >
                                <FolderOpen className="h-12 w-12 mb-3 opacity-50" />
                                <p className="font-medium">Login Required</p>
                                <p className="text-sm mt-1">
                                    Please log in to upload syllabus
                                </p>
                                <Button
                                    variant="outline"
                                    className="mt-4"
                                    onClick={() => {
                                        onOpenChange(false);
                                        window.location.href = '/login';
                                    }}
                                >
                                    Go to Login
                                </Button>
                            </motion.div>
                        ) : uploadState === 'complete' && uploadResult ? (
                            <motion.div
                                key="complete"
                                initial={{ opacity: 0, scale: 0.95 }}
                                animate={{ opacity: 1, scale: 1 }}
                                exit={{ opacity: 0, scale: 0.95 }}
                                className="flex flex-col items-center justify-center h-[350px] text-center"
                            >
                                <motion.div
                                    initial={{ scale: 0 }}
                                    animate={{ scale: 1 }}
                                    transition={{ type: "spring", stiffness: 200, delay: 0.1 }}
                                >
                                    <CheckCircle className="h-16 w-16 mb-4 text-emerald-500" />
                                </motion.div>
                                <h3 className="text-lg font-semibold mb-2">Syllabus Processed Successfully!</h3>
                                <p className="text-sm text-muted-foreground mb-4">
                                    Created {uploadResult.subjectsCreated.length} subject{uploadResult.subjectsCreated.length !== 1 ? 's' : ''} from the syllabus
                                </p>

                                {uploadResult.subjectsCreated.length > 0 && (
                                    <ScrollArea className="w-full max-h-48 mb-4">
                                        <div className="space-y-2">
                                            {uploadResult.subjectsCreated.map((subject, index) => (
                                                <motion.div
                                                    key={subject.id}
                                                    className="flex items-center justify-between p-2 border rounded"
                                                    initial={{ opacity: 0, x: -20 }}
                                                    animate={{ opacity: 1, x: 0 }}
                                                    transition={{ delay: index * 0.05 }}
                                                >
                                                    <div className="flex-1 text-left">
                                                        <p className="font-medium text-sm">{subject.name}</p>
                                                        <p className="text-xs text-muted-foreground">{subject.code}</p>
                                                    </div>
                                                    <Badge variant="secondary" className="text-xs">
                                                        {subject.credits} credits
                                                    </Badge>
                                                </motion.div>
                                            ))}
                                        </div>
                                    </ScrollArea>
                                )}

                                <Button onClick={handleClose}>
                                    Done
                                </Button>
                            </motion.div>
                        ) : isProcessing ? (
                            <motion.div
                                key="processing"
                                initial={{ opacity: 0, y: 10 }}
                                animate={{ opacity: 1, y: 0 }}
                                exit={{ opacity: 0, y: -10 }}
                                className="flex flex-col h-full min-h-0"
                            >
                                <ScrollArea className="flex-1 h-[calc(90vh-180px)] max-h-[500px] pr-2">
                                    <div className="px-2 py-1">
                                        <ExtractionProgress
                                            progress={sse.progress || (uploadState === 'uploading-file' ? 5 : 10)}
                                            phase={sse.phase || (uploadState === 'uploading-file' ? 'download' : 'extraction')}
                                            message={fallbackAttempted ? 'Processing with AI (fallback mode)...' : (sse.message || (uploadState === 'uploading-file' ? 'Uploading file...' : 'Processing with AI...'))}
                                            events={sse.events}
                                            latestEvent={sse.latestEvent}
                                            isComplete={sse.isComplete}
                                            error={fallbackAttempted ? null : sse.error}
                                            totalChunks={sse.latestEvent?.total_chunks}
                                            completedChunks={sse.latestEvent?.completed_chunks}
                                        />
                                    </div>
                                </ScrollArea>
                            </motion.div>
                        ) : (
                            <motion.div
                                key="upload-form"
                                initial={{ opacity: 0, y: 10 }}
                                animate={{ opacity: 1, y: 0 }}
                                exit={{ opacity: 0, y: -10 }}
                            >
                                <ScrollArea className="h-[350px] pr-4">
                                    <div className="space-y-4">
                                        <div className="rounded-md border border-blue-500/20 bg-blue-500/5 p-3">
                                            <p className="text-sm text-blue-700 dark:text-blue-400">
                                                <strong>Note:</strong> Upload a syllabus PDF and we'll automatically extract subject information and create subjects for you.
                                            </p>
                                        </div>

                                        {/* Drop Zone */}
                                        <motion.div
                                            className={`relative border-2 border-dashed rounded-lg p-6 transition-colors ${dragActive
                                                    ? 'border-primary bg-primary/5'
                                                    : file
                                                        ? 'border-green-500 bg-green-500/5'
                                                        : 'border-muted-foreground/25 hover:border-muted-foreground/50'
                                                }`}
                                            onDragEnter={handleDrag}
                                            onDragLeave={handleDrag}
                                            onDragOver={handleDrag}
                                            onDrop={handleDrop}
                                            whileHover={{ scale: 1.01 }}
                                            whileTap={{ scale: 0.99 }}
                                        >
                                            <input
                                                type="file"
                                                className="absolute inset-0 w-full h-full opacity-0 cursor-pointer"
                                                onChange={handleFileInput}
                                                accept=".pdf"
                                                disabled={isProcessing}
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
                                                    <Button variant="ghost" size="sm" onClick={clearFile} disabled={isProcessing}>
                                                        <X className="h-4 w-4" />
                                                    </Button>
                                                </div>
                                            ) : (
                                                <div className="text-center">
                                                    <Upload className="h-10 w-10 mx-auto text-muted-foreground" />
                                                    <p className="mt-2 text-sm font-medium">
                                                        Drop your syllabus PDF here or click to browse
                                                    </p>
                                                    <p className="mt-1 text-xs text-muted-foreground">
                                                        Max {MAX_FILE_SIZE / 1024 / 1024}MB - PDF only
                                                    </p>
                                                </div>
                                            )}
                                        </motion.div>

                                        {/* Validation Error */}
                                        <AnimatePresence>
                                            {validationError && (
                                                <motion.div
                                                    initial={{ opacity: 0, height: 0 }}
                                                    animate={{ opacity: 1, height: 'auto' }}
                                                    exit={{ opacity: 0, height: 0 }}
                                                    className="flex items-center gap-2 text-destructive text-sm"
                                                >
                                                    <AlertCircle className="h-4 w-4" />
                                                    {validationError}
                                                </motion.div>
                                            )}
                                        </AnimatePresence>

                                        {/* Upload Button */}
                                        <Button
                                            className="w-full"
                                            onClick={handleUpload}
                                            disabled={!file || isProcessing}
                                        >
                                            <Sparkles className="mr-2 h-4 w-4" />
                                            Upload & Extract with AI
                                        </Button>

                                        <div className="text-xs text-muted-foreground space-y-1">
                                            <p><strong>What happens next:</strong></p>
                                            <ul className="list-disc list-inside space-y-1 ml-2">
                                                <li>Syllabus will be analyzed using AI</li>
                                                <li>Subjects will be automatically created</li>
                                                <li>Course content and structure will be extracted</li>
                                                <li>You can then upload PYQs and notes to each subject</li>
                                            </ul>
                                        </div>
                                    </div>
                                </ScrollArea>
                            </motion.div>
                        )}
                    </AnimatePresence>
                </div>
            </DialogContent>
        </Dialog>
    );
}
