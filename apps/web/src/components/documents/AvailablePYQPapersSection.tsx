'use client';

import { useState, useMemo } from 'react';
import { AlertCircle, RefreshCw, Filter, FileSearch, CheckCircle2, Clock, Check, Upload } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Card, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Checkbox } from '@/components/ui/checkbox';
import { useSearchAvailablePYQs } from '@/lib/api/hooks/usePYQ';
import type { AvailablePYQPaper } from '@/lib/api/pyq';

interface AvailablePYQPapersSectionProps {
  subjectId: string;
  subjectCode: string;
  subjectName: string;
  ingestedPaperIds: Set<string>;
  /** Callback when user wants to proceed to upload with selected papers */
  onProceedToUpload?: (selectedPapers: AvailablePYQPaper[]) => void;
}

const CURRENT_YEAR = new Date().getFullYear();
const YEARS = Array.from({ length: 10 }, (_, i) => CURRENT_YEAR - i);
const MONTHS = [
  'January', 'February', 'March', 'April', 'May', 'June',
  'July', 'August', 'September', 'October', 'November', 'December'
];

function extractCourseFromCode(code: string): string {
  const match = code.match(/^([A-Za-z]+)/);
  return match ? match[1].toUpperCase() : 'MCA';
}

/**
 * Extract semester from subject code
 * "MCA 303 (1)" -> 3, "MCA-505" -> 5, "MCA 101" -> 1
 * The first digit after the course prefix indicates the semester
 */
function extractSemesterFromCode(code: string): number | undefined {
  // Normalize: remove spaces, hyphens
  const normalized = code.toUpperCase().replace(/[\s-]/g, '');
  // Find the first digit in the code (after letters)
  const match = normalized.match(/[A-Z]+(\d)/);
  if (match && match[1]) {
    const semester = parseInt(match[1], 10);
    // Validate it's a reasonable semester (1-8)
    if (semester >= 1 && semester <= 8) {
      return semester;
    }
  }
  return undefined;
}

/** Generate unique ID for a paper */
function getPaperId(paper: AvailablePYQPaper): string {
  return `${paper.pdf_url}-${paper.year}-${paper.month}`;
}

export function AvailablePYQPapersSection({
  subjectId,
  subjectCode,
  subjectName,
  ingestedPaperIds,
  onProceedToUpload,
}: AvailablePYQPapersSectionProps) {
  const [showFilters, setShowFilters] = useState(false);
  const [filters, setFilters] = useState<{
    year?: number;
    month?: string;
    search?: string;
  }>({});
  
  // Multi-select state
  const [selectedPapers, setSelectedPapers] = useState<Map<string, AvailablePYQPaper>>(new Map());

  const course = extractCourseFromCode(subjectCode);
  const semester = extractSemesterFromCode(subjectCode);

  const {
    data: searchData,
    isLoading,
    error,
    refetch,
  } = useSearchAvailablePYQs(
    subjectId, 
    { ...filters, course, semester }, 
    true
  );

  const handleRefresh = () => {
    refetch();
  };

  // Toggle paper selection
  const togglePaperSelection = (paper: AvailablePYQPaper) => {
    const paperId = getPaperId(paper);
    const newSelected = new Map(selectedPapers);
    
    if (newSelected.has(paperId)) {
      newSelected.delete(paperId);
    } else {
      newSelected.set(paperId, paper);
    }
    
    setSelectedPapers(newSelected);
  };

  // Select all non-ingested papers
  const selectAllAvailable = () => {
    const newSelected = new Map<string, AvailablePYQPaper>();
    const allPapers = [...(searchData?.matched_papers || []), ...(searchData?.unmatched_papers || [])];
    
    allPapers.forEach(paper => {
      const ingestKey = `${paper.year}-${paper.month}`;
      if (!ingestedPaperIds.has(ingestKey)) {
        newSelected.set(getPaperId(paper), paper);
      }
    });
    
    setSelectedPapers(newSelected);
  };

  // Clear all selections
  const clearSelection = () => {
    setSelectedPapers(new Map());
  };

  // Handle proceed to upload
  const handleProceedToUpload = () => {
    if (onProceedToUpload && selectedPapers.size > 0) {
      onProceedToUpload(Array.from(selectedPapers.values()));
    }
  };

  const matchedPapers = searchData?.matched_papers || [];
  const unmatchedPapers = searchData?.unmatched_papers || [];
  const totalPapers = matchedPapers.length + unmatchedPapers.length;
  
  // Count available (non-ingested) papers
  const availableCount = useMemo(() => {
    const allPapers = [...matchedPapers, ...unmatchedPapers];
    return allPapers.filter(p => !ingestedPaperIds.has(`${p.year}-${p.month}`)).length;
  }, [matchedPapers, unmatchedPapers, ingestedPaperIds]);

  return (
    <div className="space-y-4 relative">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <FileSearch className="h-4 w-4 text-muted-foreground" />
          <h3 className="text-sm font-medium">Available Papers</h3>
          <Badge variant="outline" className="text-xs">
            {subjectCode}
          </Badge>
        </div>
        <div className="flex items-center gap-1">
          {availableCount > 0 && selectedPapers.size < availableCount && (
            <Button
              variant="ghost"
              size="sm"
              onClick={selectAllAvailable}
              className="text-xs"
            >
              Select All
            </Button>
          )}
          {selectedPapers.size > 0 && (
            <Button
              variant="ghost"
              size="sm"
              onClick={clearSelection}
              className="text-xs"
            >
              Clear
            </Button>
          )}
          <Button
            variant="ghost"
            size="sm"
            onClick={handleRefresh}
            disabled={isLoading}
          >
            <RefreshCw className={`h-4 w-4 ${isLoading ? 'animate-spin' : ''}`} />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setShowFilters(!showFilters)}
          >
            <Filter className={`h-4 w-4 ${showFilters ? 'text-primary' : ''}`} />
          </Button>
        </div>
      </div>

      {/* Filters (collapsible) */}
      {showFilters && (
        <Card>
          <CardContent className="pt-4 space-y-3">
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label htmlFor="year" className="text-xs">Year</Label>
                <Select
                  value={filters.year?.toString() || ''}
                  onValueChange={(value) =>
                    setFilters({ ...filters, year: value ? parseInt(value) : undefined })
                  }
                >
                  <SelectTrigger id="year" className="h-9">
                    <SelectValue placeholder="All years" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="">All years</SelectItem>
                    {YEARS.map((year) => (
                      <SelectItem key={year} value={year.toString()}>
                        {year}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="month" className="text-xs">Month</Label>
                <Select
                  value={filters.month || ''}
                  onValueChange={(value) =>
                    setFilters({ ...filters, month: value || undefined })
                  }
                >
                  <SelectTrigger id="month" className="h-9">
                    <SelectValue placeholder="All months" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="">All months</SelectItem>
                    {MONTHS.map((month) => (
                      <SelectItem key={month} value={month}>
                        {month}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="search" className="text-xs">Search by name</Label>
              <Input
                id="search"
                placeholder={`e.g., ${subjectName.split(' ')[0].toLowerCase()}`}
                value={filters.search || ''}
                onChange={(e) => setFilters({ ...filters, search: e.target.value })}
                className="h-9"
              />
            </div>
          </CardContent>
        </Card>
      )}

      {/* Results */}
      <div className="space-y-3 pb-16"> {/* padding for floating CTA */}
        {/* Loading skeleton */}
        {isLoading && (
          <div className="space-y-2">
            {[1, 2, 3].map((i) => (
              <Card key={i} className="animate-pulse">
                <CardContent className="py-3 px-4">
                  <div className="h-4 bg-muted rounded w-3/4 mb-2" />
                  <div className="h-3 bg-muted rounded w-1/2" />
                </CardContent>
              </Card>
            ))}
          </div>
        )}

        {/* Error state */}
        {error && !isLoading && (
          <Card className="border-destructive/50">
            <CardContent className="py-4 px-4">
              <div className="flex items-start gap-2 text-destructive">
                <AlertCircle className="h-4 w-4 mt-0.5 shrink-0" />
                <div className="space-y-1">
                  <p className="text-sm font-medium">Search Failed</p>
                  <p className="text-xs">
                    {(error as Error)?.message || 'Unable to search for papers. Please try again.'}
                  </p>
                  <Button variant="outline" size="sm" onClick={handleRefresh} className="mt-2">
                    <RefreshCw className="h-3 w-3 mr-1" />
                    Retry
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>
        )}

        {/* Empty state */}
        {!isLoading && !error && totalPapers === 0 && (
          <Card>
            <CardContent className="py-8 text-center">
              <FileSearch className="h-8 w-8 mx-auto mb-2 text-muted-foreground opacity-50" />
              <p className="text-sm font-medium text-muted-foreground">No papers found</p>
              <p className="text-xs text-muted-foreground mt-1">
                No PYQ papers available for {subjectName} ({subjectCode})
              </p>
            </CardContent>
          </Card>
        )}

        {/* Papers list - Categorized */}
        {!isLoading && !error && totalPapers > 0 && (
          <>
            {/* Summary */}
            <div className="flex items-center justify-between text-xs text-muted-foreground">
              <span>
                Found {totalPapers} papers{' '}
                {searchData?.ingested_count ? `(${searchData.ingested_count} already added)` : ''}
              </span>
              {selectedPapers.size > 0 && (
                <Badge variant="default" className="text-xs">
                  {selectedPapers.size} selected
                </Badge>
              )}
            </div>

            {/* Matched Papers Section */}
            {matchedPapers.length > 0 && (
              <div className="space-y-2">
                <div className="flex items-center gap-2">
                  <CheckCircle2 className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
                  <span className="text-sm font-medium text-emerald-700 dark:text-emerald-400">
                    Current Subject Code
                  </span>
                  <Badge variant="secondary" className="text-xs">
                    {matchedPapers.length}
                  </Badge>
                </div>
                <div className="space-y-2">
                  {matchedPapers.map((paper) => (
                    <SelectablePYQCard
                      key={getPaperId(paper)}
                      paper={paper}
                      isIngested={ingestedPaperIds.has(`${paper.year}-${paper.month}`)}
                      isSelected={selectedPapers.has(getPaperId(paper))}
                      onToggle={() => togglePaperSelection(paper)}
                    />
                  ))}
                </div>
              </div>
            )}

            {/* Unmatched Papers Section */}
            {unmatchedPapers.length > 0 && (
              <div className="space-y-2">
                <div className="flex items-center gap-2">
                  <Clock className="h-4 w-4 text-orange-600 dark:text-orange-400" />
                  <span className="text-sm font-medium text-orange-700 dark:text-orange-400">
                    From Older Syllabus
                  </span>
                  <Badge variant="outline" className="text-xs text-orange-600 dark:text-orange-400 border-orange-400/50">
                    {unmatchedPapers.length}
                  </Badge>
                </div>
                <p className="text-xs text-muted-foreground">
                  These papers have a different subject code and may be from a previous syllabus
                </p>
                <div className="space-y-2">
                  {unmatchedPapers.map((paper) => (
                    <SelectablePYQCard
                      key={getPaperId(paper)}
                      paper={paper}
                      isIngested={ingestedPaperIds.has(`${paper.year}-${paper.month}`)}
                      isSelected={selectedPapers.has(getPaperId(paper))}
                      onToggle={() => togglePaperSelection(paper)}
                      isUnmatched
                    />
                  ))}
                </div>
              </div>
            )}
          </>
        )}
      </div>

      {/* Floating CTA */}
      {selectedPapers.size > 0 && (
        <div className="sticky bottom-0 left-0 right-0 p-4 bg-background/95 backdrop-blur border-t">
          <Button 
            onClick={handleProceedToUpload}
            className="w-full gap-2"
            size="lg"
          >
            <Upload className="h-4 w-4" />
            Go to Upload ({selectedPapers.size} selected)
          </Button>
        </div>
      )}
    </div>
  );
}

/** Individual selectable PYQ card */
interface SelectablePYQCardProps {
  paper: AvailablePYQPaper;
  isIngested: boolean;
  isSelected: boolean;
  onToggle: () => void;
  isUnmatched?: boolean;
}

function SelectablePYQCard({
  paper,
  isIngested,
  isSelected,
  onToggle,
  isUnmatched = false,
}: SelectablePYQCardProps) {
  return (
    <Card 
      className={`transition-all overflow-hidden cursor-pointer ${
        isIngested 
          ? 'opacity-50 cursor-not-allowed' 
          : isSelected 
            ? 'border-primary bg-primary/5 ring-1 ring-primary' 
            : 'hover:border-primary/50'
      } ${isUnmatched ? 'border-orange-500/30 dark:border-orange-400/30' : ''}`}
      onClick={() => !isIngested && onToggle()}
    >
      <CardContent className="py-3 px-4">
        <div className="flex items-start gap-3">
          {/* Checkbox */}
          <div className="pt-0.5">
            {isIngested ? (
              <div className="h-4 w-4 rounded-sm bg-muted flex items-center justify-center">
                <Check className="h-3 w-3 text-muted-foreground" />
              </div>
            ) : (
              <Checkbox 
                checked={isSelected} 
                onCheckedChange={() => onToggle()}
                onClick={(e) => e.stopPropagation()}
              />
            )}
          </div>
          
          {/* Paper info */}
          <div className="flex-1 min-w-0 overflow-hidden">
            <h4 className="font-medium text-sm leading-tight break-words line-clamp-2">
              {paper.title}
            </h4>
            
            {/* Metadata badges */}
            <div className="flex flex-wrap items-center gap-1.5 mt-2">
              <Badge variant="secondary" className="text-xs shrink-0">
                {paper.year}
              </Badge>
              {paper.month && (
                <Badge variant="outline" className="text-xs shrink-0">
                  {paper.month}
                </Badge>
              )}
              {isUnmatched && paper.subject_code && (
                <Badge variant="outline" className="text-xs text-orange-600 dark:text-orange-400 border-orange-400/50 shrink-0">
                  {paper.subject_code}
                </Badge>
              )}
              <Badge variant="outline" className="text-xs text-blue-600 dark:text-blue-400 border-blue-400/50 shrink-0">
                {paper.source_name}
              </Badge>
              {isIngested && (
                <Badge variant="secondary" className="text-xs shrink-0">
                  Already Added
                </Badge>
              )}
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
