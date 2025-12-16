'use client';

import { useState } from 'react';
import {
  BookOpen,
  ChevronRight,
  Clock,
  FileText,
  RefreshCw,
  AlertCircle,
  CheckCircle2,
  BookMarked,
  GraduationCap,
  Hash,
} from 'lucide-react';
import { LoadingSpinner, InlineSpinner } from '@/components/ui/loading-spinner';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { ScrollArea } from '@/components/ui/scroll-area';
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion';
import { useSyllabus, useRetryExtraction, useExtractSyllabus } from '@/lib/api/hooks/useSyllabus';
import { useDocuments } from '@/lib/api/hooks/useDocuments';
import {
  type SyllabusUnit,
  type BookReference,
  EXTRACTION_STATUS_CONFIG,
  BOOK_TYPE_LABELS,
} from '@/lib/api/syllabus';

interface SyllabusTabProps {
  subjectId: string;
  isAdmin?: boolean;
}

export function SyllabusTab({ subjectId }: SyllabusTabProps) {
  const { data: syllabus, isLoading, refetch } = useSyllabus(subjectId);
  const { data: documentsData } = useDocuments(subjectId);
  const retryMutation = useRetryExtraction();
  const extractMutation = useExtractSyllabus();

  // Find syllabus documents that can be used for extraction
  const syllabusDocuments = documentsData?.data?.filter(
    (doc) => doc.type === 'syllabus' && doc.indexing_status === 'completed'
  ) || [];

  const isProcessing = syllabus?.extraction_status === 'processing' || syllabus?.extraction_status === 'pending';

  // Poll for status when processing
  const { refetch: refetchStatus } = useSyllabus(subjectId);

  // Auto-refresh when processing
  useState(() => {
    if (isProcessing) {
      const interval = setInterval(() => {
        refetchStatus();
      }, 3000);
      return () => clearInterval(interval);
    }
  });

  const handleRetry = () => {
    if (syllabus) {
      retryMutation.mutate({ syllabusId: String(syllabus.id), subjectId });
    }
  };

  const handleExtract = (documentId: number) => {
    extractMutation.mutate({ documentId: String(documentId), subjectId });
  };

  if (isLoading) {
    return (
      <div className="flex flex-col items-center justify-center h-[350px]">
        <LoadingSpinner size="lg" text="Loading syllabus..." centered />
      </div>
    );
  }

  // No syllabus exists yet
  if (!syllabus) {
    return (
      <div className="flex flex-col items-center justify-center h-[350px] text-center">
        <BookOpen className="h-12 w-12 mb-3 text-muted-foreground opacity-50" />
        <p className="font-medium text-muted-foreground">No Syllabus Extracted</p>
        <p className="text-sm text-muted-foreground mt-1 max-w-sm">
          Upload a syllabus document and extract it to see the structured content here.
        </p>
        
        {syllabusDocuments.length > 0 && (
          <div className="mt-4 space-y-2">
            <p className="text-sm text-muted-foreground">Available syllabus documents:</p>
            {syllabusDocuments.map((doc) => (
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
                  <FileText className="h-4 w-4 mr-2" />
                )}
                Extract from {doc.filename}
              </Button>
            ))}
          </div>
        )}
      </div>
    );
  }

  // Syllabus is processing
  if (isProcessing) {
    const statusConfig = EXTRACTION_STATUS_CONFIG[syllabus.extraction_status];
    return (
      <div className="flex flex-col items-center justify-center h-[350px] text-center">
        <div className="relative">
          <LoadingSpinner size="xl" className="text-primary" />
          <div className={`absolute -top-1 -right-1 h-3 w-3 rounded-full ${statusConfig.color}`} />
        </div>
        <p className="font-medium mt-4">{statusConfig.label}</p>
        <p className="text-sm text-muted-foreground mt-1">
          {statusConfig.description}
        </p>
        <p className="text-xs text-muted-foreground mt-2">
          This may take a few minutes...
        </p>
      </div>
    );
  }

  // Extraction failed
  if (syllabus.extraction_status === 'failed') {
    return (
      <div className="flex flex-col items-center justify-center h-[350px] text-center">
        <AlertCircle className="h-12 w-12 text-destructive mb-3" />
        <p className="font-medium text-destructive">Extraction Failed</p>
        <p className="text-sm text-muted-foreground mt-1 max-w-sm">
          {syllabus.extraction_error || 'An error occurred during extraction.'}
        </p>
        <Button
          variant="outline"
          className="mt-4"
          onClick={handleRetry}
          disabled={retryMutation.isPending}
        >
          {retryMutation.isPending ? (
            <InlineSpinner className="mr-2" />
          ) : (
            <RefreshCw className="h-4 w-4 mr-2" />
          )}
          Retry Extraction
        </Button>
      </div>
    );
  }

  // Successfully extracted - show the syllabus content
  return (
    <ScrollArea className="h-[350px] pr-4">
      <div className="space-y-4">
        {/* Header with subject info */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <CheckCircle2 className="h-4 w-4 text-green-500" />
            <span className="text-sm text-muted-foreground">
              Extracted successfully
            </span>
          </div>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => refetch()}
          >
            <RefreshCw className="h-4 w-4" />
          </Button>
        </div>

        {/* Subject metadata */}
        {(syllabus.subject_code || syllabus.total_credits > 0) && (
          <div className="flex flex-wrap gap-2">
            {syllabus.subject_code && (
              <Badge variant="outline" className="gap-1">
                <Hash className="h-3 w-3" />
                {syllabus.subject_code}
              </Badge>
            )}
            {syllabus.total_credits > 0 && (
              <Badge variant="outline" className="gap-1">
                <GraduationCap className="h-3 w-3" />
                {syllabus.total_credits} Credits
              </Badge>
            )}
          </div>
        )}

        {/* Units Section */}
        {syllabus.units && syllabus.units.length > 0 && (
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-base flex items-center gap-2">
                <BookOpen className="h-4 w-4" />
                Units ({syllabus.units.length})
              </CardTitle>
            </CardHeader>
            <CardContent className="pt-0">
              <Accordion type="multiple" className="w-full">
                {syllabus.units.map((unit) => (
                  <UnitAccordion key={unit.id} unit={unit} />
                ))}
              </Accordion>
            </CardContent>
          </Card>
        )}

        {/* Books Section */}
        {syllabus.books && syllabus.books.length > 0 && (
          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-base flex items-center gap-2">
                <BookMarked className="h-4 w-4" />
                References ({syllabus.books.length})
              </CardTitle>
            </CardHeader>
            <CardContent className="pt-0">
              <div className="space-y-3">
                {syllabus.books.map((book) => (
                  <BookCard key={book.id} book={book} />
                ))}
              </div>
            </CardContent>
          </Card>
        )}
      </div>
    </ScrollArea>
  );
}

function UnitAccordion({ unit }: { unit: SyllabusUnit }) {
  return (
    <AccordionItem value={`unit-${unit.id}`}>
      <AccordionTrigger className="hover:no-underline">
        <div className="flex items-center gap-2 text-left">
          <Badge variant="secondary" className="shrink-0">
            Unit {unit.unit_number}
          </Badge>
          <span className="font-medium">{unit.title}</span>
          {unit.hours && unit.hours > 0 && (
            <Badge variant="outline" className="ml-auto shrink-0 gap-1">
              <Clock className="h-3 w-3" />
              {unit.hours}h
            </Badge>
          )}
        </div>
      </AccordionTrigger>
      <AccordionContent>
        <div className="pl-4 space-y-2">
          {unit.description && (
            <p className="text-sm text-muted-foreground mb-3">
              {unit.description}
            </p>
          )}
          {unit.topics && unit.topics.length > 0 && (
            <div className="space-y-2">
              {unit.topics.map((topic) => (
                <div
                  key={topic.id}
                  className="flex items-start gap-2 text-sm"
                >
                  <ChevronRight className="h-4 w-4 mt-0.5 text-muted-foreground shrink-0" />
                  <div>
                    <span className="font-medium">
                      {topic.topic_number}. {topic.title}
                    </span>
                    {topic.description && (
                      <p className="text-muted-foreground text-xs mt-0.5">
                        {topic.description}
                      </p>
                    )}
                    {topic.keywords && (
                      <div className="flex flex-wrap gap-1 mt-1">
                        {topic.keywords.split(',').map((keyword, idx) => (
                          <Badge
                            key={idx}
                            variant="outline"
                            className="text-xs py-0 px-1"
                          >
                            {keyword.trim()}
                          </Badge>
                        ))}
                      </div>
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

function BookCard({ book }: { book: BookReference }) {
  return (
    <div className="flex items-start gap-3 p-2 rounded-lg border bg-card">
      <div className="shrink-0">
        <BookMarked className="h-5 w-5 text-muted-foreground" />
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-start justify-between gap-2">
          <p className="font-medium text-sm leading-tight">{book.title}</p>
          <Badge
            variant={book.is_textbook ? 'default' : 'secondary'}
            className="shrink-0 text-xs"
          >
            {BOOK_TYPE_LABELS[book.book_type] || book.book_type}
          </Badge>
        </div>
        <p className="text-xs text-muted-foreground mt-0.5">
          {book.authors}
        </p>
        <div className="flex flex-wrap gap-x-3 gap-y-0.5 mt-1 text-xs text-muted-foreground">
          {book.publisher && <span>{book.publisher}</span>}
          {book.edition && <span>{book.edition}</span>}
          {book.year && book.year > 0 && <span>{book.year}</span>}
          {book.isbn && <span>ISBN: {book.isbn}</span>}
        </div>
      </div>
    </div>
  );
}
