'use client';

import { useState, useMemo } from 'react';
import {
  FileQuestion,
  ChevronRight,
  Clock,
  RefreshCw,
  AlertCircle,
  CheckCircle2,
  Calendar,
  Award,
  Hash,
  ListOrdered,
  CircleDot,
  Tag,
  Search,
} from 'lucide-react';
import { LoadingSpinner, InlineSpinner } from '@/components/ui/loading-spinner';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Input } from '@/components/ui/input';
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion';
import { usePYQs, usePYQById, useRetryPYQExtraction, useExtractPYQ } from '@/lib/api/hooks/usePYQ';
import { useDocuments } from '@/lib/api/hooks/useDocuments';
import {
  type PYQPaperSummary,
  type PYQQuestion,
  type AvailablePYQPaper,
  PYQ_EXTRACTION_STATUS_CONFIG,
  formatPaperTitle,
} from '@/lib/api/pyq';
import { AvailablePYQPapersSection } from './AvailablePYQPapersSection';
import { PYQBatchUploadDialog } from './PYQBatchUploadDialog';
import type { AISetupStatus } from '@/lib/api/notifications';

interface PYQTabProps {
  subjectId: string;
  subjectCode?: string;
  subjectName?: string;
  isAdmin?: boolean;
  aiSetupStatus?: AISetupStatus;
}

export function PYQTab({ subjectId, subjectCode, subjectName, isAdmin = false, aiSetupStatus }: PYQTabProps) {
  const { data: pyqsData, isLoading, refetch } = usePYQs(subjectId);
  const { data: documentsData } = useDocuments(subjectId);
  const extractMutation = useExtractPYQ();
  
  const [selectedPaperId, setSelectedPaperId] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  
  // Batch upload dialog state
  const [batchUploadOpen, setBatchUploadOpen] = useState(false);
  const [selectedExternalPapers, setSelectedExternalPapers] = useState<AvailablePYQPaper[]>([]);

  // Find PYQ documents that can be used for extraction
  const pyqDocuments = useMemo(
    () =>
      documentsData?.data?.filter(
        (doc) => doc.type === 'pyq' && doc.indexing_status === 'completed'
      ) || [],
    [documentsData]
  );

  const papers = pyqsData?.papers || [];

  // Calculate ingested paper IDs for deduplication
  const ingestedPaperIds = useMemo(() => {
    return new Set(papers.map(p => `${p.year}-${p.month || ''}`));
  }, [papers]);

  const handleExtract = (documentId: number) => {
    extractMutation.mutate({ documentId: String(documentId), subjectId, async: true });
  };

  // Handle proceeding to batch upload
  const handleProceedToUpload = (papers: AvailablePYQPaper[]) => {
    setSelectedExternalPapers(papers);
    setBatchUploadOpen(true);
  };

  // Handle batch upload complete
  const handleBatchUploadComplete = () => {
    setBatchUploadOpen(false);
    setSelectedExternalPapers([]);
    refetch();
  };

  if (isLoading) {
    return (
      <div className="flex flex-col items-center justify-center h-[350px]">
        <LoadingSpinner size="lg" text="Loading PYQ papers..." centered />
      </div>
    );
  }

  // No PYQ papers exist yet
  if (papers.length === 0) {
    return (
      <ScrollArea className="h-[350px] pr-4">
        <div className="space-y-6">
          {/* Empty state message */}
          <div className="flex flex-col items-center justify-center py-8 text-center">
            <FileQuestion className="h-12 w-12 mb-3 text-muted-foreground opacity-50" />
            <p className="font-medium text-muted-foreground">No Question Papers Extracted</p>
            <p className="text-sm text-muted-foreground mt-1 max-w-sm">
              Upload a PYQ document and extract it, or search for external papers below.
            </p>
            
            {isAdmin && pyqDocuments.length > 0 && (
              <div className="mt-4 space-y-2">
                <p className="text-sm text-muted-foreground">Available PYQ documents:</p>
                {pyqDocuments.map((doc) => (
                  <Button
                    key={doc.id}
                    variant="outline"
                    size="sm"
                    onClick={() => handleExtract(doc.id)}
                    disabled={extractMutation.isPending}
                  >
                    {extractMutation.isPending ? (
                      <InlineSpinner className="mr-2" />
                    ) : (
                      <FileQuestion className="h-4 w-4 mr-2" />
                    )}
                    Extract from {doc.filename}
                  </Button>
                ))}
              </div>
            )}
          </div>

          {/* Admin-only: Search External Papers */}
          {isAdmin && (
            <>
              {/* Divider */}
              <div className="relative">
                <div className="absolute inset-0 flex items-center">
                  <span className="w-full border-t" />
                </div>
                <div className="relative flex justify-center text-xs uppercase">
                  <span className="bg-background px-2 text-muted-foreground">
                    Search External Papers
                  </span>
                </div>
              </div>

              {/* Available PYQ Papers Section - show even when no papers exist */}
              <AvailablePYQPapersSection
                subjectId={subjectId}
                subjectCode={subjectCode || 'MCA'}
                subjectName={subjectName || 'Subject'}
                ingestedPaperIds={ingestedPaperIds}
                onProceedToUpload={handleProceedToUpload}
              />
            </>
          )}
        </div>

        {/* Batch Upload Dialog - Admin only */}
        {isAdmin && (
          <PYQBatchUploadDialog
            open={batchUploadOpen}
            onOpenChange={setBatchUploadOpen}
            selectedPapers={selectedExternalPapers}
            subjectId={subjectId}
            subjectName={subjectName || 'Subject'}
            onComplete={handleBatchUploadComplete}
            aiSetupStatus={aiSetupStatus}
          />
        )}
      </ScrollArea>
    );
  }

  // If a paper is selected, show its details
  if (selectedPaperId) {
    return (
      <PYQPaperDetail
        paperId={selectedPaperId}
        subjectId={subjectId}
        onBack={() => setSelectedPaperId(null)}
      />
    );
  }

  // Show list of papers
  return (
    <ScrollArea className="h-[350px] pr-4">
      <div className="space-y-4">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <FileQuestion className="h-4 w-4 text-muted-foreground" />
            <span className="text-sm text-muted-foreground">
              {papers.length} question paper{papers.length !== 1 ? 's' : ''} available
            </span>
          </div>
          <Button variant="ghost" size="sm" onClick={() => refetch()}>
            <RefreshCw className="h-4 w-4" />
          </Button>
        </div>

        {/* Search */}
        <div className="relative">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="Search by year..."
            className="pl-9"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
          />
        </div>

        {/* Papers List */}
        <div className="space-y-2">
          {papers
            .filter((paper) => {
              if (!searchQuery) return true;
              const title = formatPaperTitle(paper).toLowerCase();
              return title.includes(searchQuery.toLowerCase());
            })
            .map((paper) => (
              <PaperCard
                key={paper.id}
                paper={paper}
                onClick={() => setSelectedPaperId(String(paper.id))}
              />
            ))}
        </div>

        {/* Extract more button - hidden for now (auto-extraction handles this) */}
        {/* {pyqDocuments.length > 0 && (
          <Card className="border-dashed">
            <CardContent className="py-3">
              <p className="text-sm text-muted-foreground mb-2">
                {pyqDocuments.length} more PYQ document{pyqDocuments.length !== 1 ? 's' : ''} available for extraction
              </p>
              <div className="flex flex-wrap gap-2">
                {pyqDocuments.slice(0, 3).map((doc) => (
                  <Button
                    key={doc.id}
                    variant="outline"
                    size="sm"
                    onClick={() => handleExtract(doc.id)}
                    disabled={extractMutation.isPending}
                  >
                    {extractMutation.isPending ? (
                      <Loader2 className="h-3 w-3 mr-1 animate-spin" />
                    ) : (
                      <FileQuestion className="h-3 w-3 mr-1" />
                    )}
                    {doc.filename.length > 20 ? doc.filename.slice(0, 20) + '...' : doc.filename}
                  </Button>
                ))}
              </div>
            </CardContent>
          </Card>
        )} */}

        {/* Admin-only: Search External Papers */}
        {isAdmin && (
          <>
            {/* Divider */}
            <div className="relative">
              <div className="absolute inset-0 flex items-center">
                <span className="w-full border-t" />
              </div>
              <div className="relative flex justify-center text-xs uppercase">
                <span className="bg-background px-2 text-muted-foreground">
                  Search External Papers
                </span>
              </div>
            </div>

            {/* Available PYQ Papers Section */}
            <AvailablePYQPapersSection
              subjectId={subjectId}
              subjectCode={subjectCode || 'MCA'}
              subjectName={subjectName || 'Subject'}
              ingestedPaperIds={ingestedPaperIds}
              onProceedToUpload={handleProceedToUpload}
            />
          </>
        )}
      </div>

      {/* Batch Upload Dialog - Admin only */}
      {isAdmin && (
        <PYQBatchUploadDialog
          open={batchUploadOpen}
          onOpenChange={setBatchUploadOpen}
          selectedPapers={selectedExternalPapers}
          subjectId={subjectId}
          subjectName={subjectName || 'Subject'}
          onComplete={handleBatchUploadComplete}
          aiSetupStatus={aiSetupStatus}
        />
      )}
    </ScrollArea>
  );
}

function PaperCard({ paper, onClick }: { paper: PYQPaperSummary; onClick: () => void }) {
  const statusConfig = PYQ_EXTRACTION_STATUS_CONFIG[paper.extraction_status];
  const isProcessing = paper.extraction_status === 'processing' || paper.extraction_status === 'pending';
  const isFailed = paper.extraction_status === 'failed';

  return (
    <Card
      className={`cursor-pointer transition-colors hover:bg-accent/50 ${
        isFailed ? 'border-destructive/50' : ''
      }`}
      onClick={onClick}
    >
      <CardContent className="py-3 px-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="flex flex-col">
              <div className="flex items-center gap-2">
                <Calendar className="h-4 w-4 text-muted-foreground" />
                <span className="font-medium">{formatPaperTitle(paper)}</span>
              </div>
              <div className="flex items-center gap-3 mt-1 text-xs text-muted-foreground">
                {paper.total_marks > 0 && (
                  <span className="flex items-center gap-1">
                    <Award className="h-3 w-3" />
                    {paper.total_marks} marks
                  </span>
                )}
                {paper.total_questions > 0 && (
                  <span className="flex items-center gap-1">
                    <ListOrdered className="h-3 w-3" />
                    {paper.total_questions} questions
                  </span>
                )}
              </div>
            </div>
          </div>
          <div className="flex items-center gap-2">
            {isProcessing && (
              <Badge variant="secondary" className="gap-1">
                <LoadingSpinner size="xs" />
                {statusConfig.label}
              </Badge>
            )}
            {isFailed && (
              <Badge variant="destructive" className="gap-1">
                <AlertCircle className="h-3 w-3" />
                Failed
              </Badge>
            )}
            {paper.extraction_status === 'completed' && (
              <Badge variant="outline" className="gap-1 text-green-600 border-green-300">
                <CheckCircle2 className="h-3 w-3" />
                Ready
              </Badge>
            )}
            <ChevronRight className="h-4 w-4 text-muted-foreground" />
          </div>
        </div>
      </CardContent>
    </Card>
  );
}

function PYQPaperDetail({
  paperId,
  subjectId,
  onBack,
}: {
  paperId: string;
  subjectId: string;
  onBack: () => void;
}) {
  const { data: paper, isLoading, refetch } = usePYQById(paperId);
  const retryMutation = useRetryPYQExtraction();

  const handleRetry = () => {
    if (paper) {
      retryMutation.mutate({ pyqId: String(paper.id), subjectId });
    }
  };

  if (isLoading) {
    return (
      <div className="flex flex-col items-center justify-center h-[350px] text-muted-foreground">
        <LoadingSpinner size="lg" text="Loading questions..." centered />
      </div>
    );
  }

  if (!paper) {
    return (
      <div className="flex flex-col items-center justify-center h-[350px]">
        <AlertCircle className="h-8 w-8 mb-3 text-destructive" />
        <p>Paper not found</p>
        <Button variant="outline" className="mt-4" onClick={onBack}>
          Go Back
        </Button>
      </div>
    );
  }

  const isProcessing = paper.extraction_status === 'processing' || paper.extraction_status === 'pending';

  // Processing state
  if (isProcessing) {
    const statusConfig = PYQ_EXTRACTION_STATUS_CONFIG[paper.extraction_status];
    return (
      <div className="flex flex-col items-center justify-center h-[350px] text-center">
        <div className="relative">
          <LoadingSpinner size="xl" className="text-primary" />
          <div className={`absolute -top-1 -right-1 h-3 w-3 rounded-full ${statusConfig.color}`} />
        </div>
        <p className="font-medium mt-4">{statusConfig.label}</p>
        <p className="text-sm text-muted-foreground mt-1">{statusConfig.description}</p>
        <Button variant="outline" className="mt-4" onClick={onBack}>
          Go Back
        </Button>
      </div>
    );
  }

  // Failed state
  if (paper.extraction_status === 'failed') {
    return (
      <div className="flex flex-col items-center justify-center h-[350px] text-center">
        <AlertCircle className="h-12 w-12 text-destructive mb-3" />
        <p className="font-medium text-destructive">Extraction Failed</p>
        <p className="text-sm text-muted-foreground mt-1 max-w-sm">
          {paper.extraction_error || 'An error occurred during extraction.'}
        </p>
        <div className="flex gap-2 mt-4">
          <Button variant="outline" onClick={onBack}>
            Go Back
          </Button>
          <Button
            variant="default"
            onClick={handleRetry}
            disabled={retryMutation.isPending}
          >
            {retryMutation.isPending ? (
              <InlineSpinner className="mr-2" />
            ) : (
              <RefreshCw className="h-4 w-4 mr-2" />
            )}
            Retry
          </Button>
        </div>
      </div>
    );
  }

  // Successfully extracted - show questions
  return (
    <ScrollArea className="h-[350px] pr-4">
      <div className="space-y-4">
        {/* Back button and header */}
        <div className="flex items-center justify-between">
          <Button variant="ghost" size="sm" onClick={onBack}>
            <ChevronRight className="h-4 w-4 mr-1 rotate-180" />
            Back to Papers
          </Button>
          <Button variant="ghost" size="sm" onClick={() => refetch()}>
            <RefreshCw className="h-4 w-4" />
          </Button>
        </div>

        {/* Paper info */}
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base flex items-center gap-2">
              <Calendar className="h-4 w-4" />
              {formatPaperTitle(paper)}
            </CardTitle>
          </CardHeader>
          <CardContent className="pt-0">
            <div className="flex flex-wrap gap-2">
              {paper.total_marks > 0 && (
                <Badge variant="secondary" className="gap-1">
                  <Award className="h-3 w-3" />
                  {paper.total_marks} marks
                </Badge>
              )}
              {paper.duration && (
                <Badge variant="secondary" className="gap-1">
                  <Clock className="h-3 w-3" />
                  {paper.duration}
                </Badge>
              )}
              {paper.total_questions > 0 && (
                <Badge variant="secondary" className="gap-1">
                  <ListOrdered className="h-3 w-3" />
                  {paper.total_questions} questions
                </Badge>
              )}
            </div>
            {paper.instructions && (
              <p className="text-xs text-muted-foreground mt-2 italic">
                {paper.instructions}
              </p>
            )}
          </CardContent>
        </Card>

        {/* Questions */}
        {paper.questions && paper.questions.length > 0 && (
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-base flex items-center gap-2">
                <FileQuestion className="h-4 w-4" />
                Questions
              </CardTitle>
            </CardHeader>
            <CardContent className="pt-0">
              <Accordion type="multiple" className="w-full">
                {paper.questions.map((question) => (
                  <QuestionAccordion key={question.id} question={question} />
                ))}
              </Accordion>
            </CardContent>
          </Card>
        )}
      </div>
    </ScrollArea>
  );
}

function QuestionAccordion({ question }: { question: PYQQuestion }) {
  return (
    <AccordionItem value={`q-${question.id}`}>
      <AccordionTrigger className="hover:no-underline">
        <div className="flex items-center gap-2 text-left flex-1">
          <Badge variant="secondary" className="shrink-0">
            Q{question.question_number}
          </Badge>
          <span className="text-sm line-clamp-1 flex-1">
            {question.question_text.slice(0, 80)}
            {question.question_text.length > 80 ? '...' : ''}
          </span>
          <div className="flex items-center gap-2 shrink-0 ml-auto">
            {question.marks > 0 && (
              <Badge variant="outline" className="gap-1">
                <Award className="h-3 w-3" />
                {question.marks}m
              </Badge>
            )}
            {question.has_choices && (
              <Badge variant="outline" className="text-xs">
                OR
              </Badge>
            )}
          </div>
        </div>
      </AccordionTrigger>
      <AccordionContent>
        <div className="pl-4 space-y-3">
          {/* Full question text */}
          <p className="text-sm whitespace-pre-wrap">{question.question_text}</p>

          {/* Metadata */}
          <div className="flex flex-wrap gap-2">
            {question.section_name && (
              <Badge variant="outline" className="text-xs gap-1">
                <Hash className="h-3 w-3" />
                {question.section_name}
              </Badge>
            )}
            {question.unit_number && question.unit_number > 0 && (
              <Badge variant="outline" className="text-xs gap-1">
                Unit {question.unit_number}
              </Badge>
            )}
            {!question.is_compulsory && (
              <Badge variant="outline" className="text-xs text-amber-600">
                Optional
              </Badge>
            )}
          </div>

          {/* Topic keywords */}
          {question.topic_keywords && (
            <div className="flex flex-wrap gap-1">
              <Tag className="h-3 w-3 text-muted-foreground mr-1" />
              {question.topic_keywords.split(',').map((keyword, idx) => (
                <Badge key={idx} variant="secondary" className="text-xs py-0 px-1">
                  {keyword.trim()}
                </Badge>
              ))}
            </div>
          )}

          {/* Choices (OR questions) */}
          {question.has_choices && question.choices && question.choices.length > 0 && (
            <div className="mt-3 space-y-2 border-l-2 border-muted pl-3">
              <p className="text-xs font-medium text-muted-foreground flex items-center gap-1">
                <CircleDot className="h-3 w-3" />
                Choose one:
              </p>
              {question.choices.map((choice) => (
                <div key={choice.id} className="text-sm bg-muted/50 rounded-md p-2">
                  <div className="flex items-start gap-2">
                    <Badge variant="outline" className="shrink-0 text-xs">
                      {choice.choice_label}
                    </Badge>
                    <span className="flex-1">{choice.choice_text}</span>
                    {choice.marks && choice.marks > 0 && (
                      <Badge variant="secondary" className="shrink-0 text-xs">
                        {choice.marks}m
                      </Badge>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </AccordionContent>
    </AccordionItem>
  );
}
