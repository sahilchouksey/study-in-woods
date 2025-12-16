'use client';

import { CheckCircle, Plus, Loader2, Calendar } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent } from '@/components/ui/card';
import type { AvailablePYQPaper } from '@/lib/api/pyq';

interface AvailablePYQCardProps {
  paper: AvailablePYQPaper;
  isIngested: boolean;
  onIngest: (paper: AvailablePYQPaper) => void;
  isIngesting: boolean;
  /** Whether this paper has a different subject code (from older syllabus) */
  isUnmatched?: boolean;
}

export function AvailablePYQCard({
  paper,
  isIngested,
  onIngest,
  isIngesting,
  isUnmatched = false,
}: AvailablePYQCardProps) {
  return (
    <Card className={`transition-all overflow-hidden ${isIngested ? 'opacity-60' : 'hover:shadow-md hover:border-primary/30'} ${isUnmatched ? 'border-orange-500/30 bg-orange-500/5 dark:border-orange-400/30 dark:bg-orange-400/5' : ''}`}>
      <CardContent className="py-3 px-4">
        <div className="flex items-start justify-between gap-3">
          {/* Left side: Paper info */}
          <div className="flex-1 min-w-0 overflow-hidden">
            <div className="flex items-start gap-2 mb-2">
              <Calendar className="h-4 w-4 text-muted-foreground shrink-0 mt-0.5" />
              <h4 className="font-medium text-sm leading-tight break-words line-clamp-2">{paper.title}</h4>
            </div>
            
            {/* Metadata badges - compact layout */}
            <div className="flex flex-wrap items-center gap-1.5">
              <Badge variant="secondary" className="text-xs shrink-0">
                {paper.year}
              </Badge>
              {paper.month && (
                <Badge variant="outline" className="text-xs shrink-0">
                  {paper.month}
                </Badge>
              )}
              {/* Show subject code for unmatched papers */}
              {isUnmatched && paper.subject_code && (
                <Badge variant="outline" className="text-xs text-orange-600 dark:text-orange-400 border-orange-400/50 dark:border-orange-400/50 gap-1 shrink-0">
                  {paper.subject_code}
                </Badge>
              )}
              <Badge variant="outline" className="text-xs text-blue-600 dark:text-blue-400 border-blue-400/50 dark:border-blue-400/50 shrink-0">
                {paper.source_name}
              </Badge>
            </div>
          </div>

          {/* Right side: Action button */}
          <div className="shrink-0">
            {isIngested ? (
              <Button
                variant="secondary"
                size="sm"
                disabled
                className="gap-1.5 cursor-not-allowed"
              >
                <CheckCircle className="h-4 w-4" />
                Added
              </Button>
            ) : (
              <Button
                variant={isUnmatched ? 'outline' : 'default'}
                size="sm"
                onClick={() => onIngest(paper)}
                disabled={isIngesting}
                className="gap-1.5"
              >
                {isIngesting ? (
                  <>
                    <Loader2 className="h-4 w-4 animate-spin" />
                    Adding...
                  </>
                ) : (
                  <>
                    <Plus className="h-4 w-4" />
                    Add
                  </>
                )}
              </Button>
            )}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
