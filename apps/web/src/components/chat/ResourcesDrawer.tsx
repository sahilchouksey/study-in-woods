'use client';

import { useState } from 'react';
import {
  BookOpen,
  FileQuestion,
  ChevronRight,
  Clock,
  Award,
  ListOrdered,
  Hash,
  Calendar,
  BookMarked,
  GraduationCap,
  Settings2,
  Cpu,
  Quote,
  RotateCcw,
  Info,
} from 'lucide-react';
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from '@/components/ui/sheet';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion';
import { LoadingSpinner } from '@/components/ui/loading-spinner';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import { useSyllabus } from '@/lib/api/hooks/useSyllabus';
import { usePYQs, usePYQById } from '@/lib/api/hooks/usePYQ';
import type { SyllabusUnit, SyllabusTopic, BookReference } from '@/lib/api/syllabus';
import type { PYQPaperSummary, PYQPaper, PYQQuestion } from '@/lib/api/pyq';
import { BOOK_TYPE_LABELS } from '@/lib/api/syllabus';
import type { AISettings } from '@/lib/api/chat';
import { DEFAULT_AI_SETTINGS, DEFAULT_SYSTEM_PROMPT } from '@/lib/api/chat';
import { cn } from '@/lib/utils';
import { 
  hasSubjectCustomSettings, 
  getSettingsMetadata, 
  saveGlobalAISettings, 
  removeSubjectAISettings 
} from '@/lib/ai-settings-storage';

type ResourceType = 'syllabus' | 'pyqs' | 'settings';

interface ResourcesDrawerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  resourceType: ResourceType;
  subjectId: string;
  subjectName?: string;
  onSelectQuestion: (question: string) => void;
  // AI Settings props
  aiSettings?: AISettings;
  onAISettingsChange?: (settings: AISettings) => void;
}

export function ResourcesDrawer({
  open,
  onOpenChange,
  resourceType,
  subjectId,
  subjectName,
  onSelectQuestion,
  aiSettings,
  onAISettingsChange,
}: ResourcesDrawerProps) {
  const getTitle = () => {
    switch (resourceType) {
      case 'syllabus':
        return { icon: <BookOpen className="h-5 w-5 text-primary" />, text: 'Syllabus' };
      case 'pyqs':
        return { icon: <FileQuestion className="h-5 w-5 text-primary" />, text: 'Previous Year Questions' };
      case 'settings':
        return { icon: <Settings2 className="h-5 w-5 text-primary" />, text: 'AI Settings' };
    }
  };

  const getDescription = () => {
    switch (resourceType) {
      case 'syllabus':
        return 'Click on any topic to ask about it';
      case 'pyqs':
        return 'Click on any question to ask the AI';
      case 'settings':
        return 'Configure AI behavior and response settings';
    }
  };

  const title = getTitle();

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="w-full sm:max-w-md flex flex-col">
        <SheetHeader className="shrink-0 pb-4">
          <SheetTitle className="flex items-center gap-2">
            {title.icon}
            {title.text}
          </SheetTitle>
          <SheetDescription>
            {getDescription()}
          </SheetDescription>
        </SheetHeader>

        <div className="flex-1 overflow-y-auto">
          {resourceType === 'syllabus' && (
            <SyllabusContent
              subjectId={subjectId}
              onSelectTopic={onSelectQuestion}
              onClose={() => onOpenChange(false)}
            />
          )}
          {resourceType === 'pyqs' && (
            <PYQsContent
              subjectId={subjectId}
              onSelectQuestion={onSelectQuestion}
              onClose={() => onOpenChange(false)}
            />
          )}
          {resourceType === 'settings' && aiSettings && onAISettingsChange && (
            <AISettingsContent
              settings={aiSettings}
              onSettingsChange={onAISettingsChange}
              subjectId={subjectId}
              subjectName={subjectName}
            />
          )}
        </div>
      </SheetContent>
    </Sheet>
  );
}

// ============= Syllabus Content =============

interface SyllabusContentProps {
  subjectId: string;
  onSelectTopic: (topic: string) => void;
  onClose: () => void;
}

function SyllabusContent({ subjectId, onSelectTopic, onClose }: SyllabusContentProps) {
  const { data: syllabus, isLoading } = useSyllabus(subjectId);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-full">
        <LoadingSpinner size="lg" text="Loading syllabus..." centered />
      </div>
    );
  }

  if (!syllabus) {
    return (
      <div className="flex flex-col items-center justify-center h-full text-center px-4">
        <BookOpen className="h-12 w-12 mb-3 text-muted-foreground opacity-50" />
        <p className="font-medium text-muted-foreground">No Syllabus Available</p>
        <p className="text-sm text-muted-foreground mt-1">
          Upload a syllabus document to see the content here.
        </p>
      </div>
    );
  }

  const handleTopicClick = (unit: SyllabusUnit, topic: SyllabusTopic) => {
    const question = `Explain "${topic.title}" from Unit ${unit.unit_number}: ${unit.title}`;
    onSelectTopic(question);
    onClose();
  };

  const handleUnitClick = (unit: SyllabusUnit) => {
    const question = `Give me an overview of Unit ${unit.unit_number}: ${unit.title}`;
    onSelectTopic(question);
    onClose();
  };

  return (
    <div className="space-y-4 pr-2">
      {/* Subject Info */}
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

        {/* Units */}
        {syllabus.units && syllabus.units.length > 0 && (
          <Accordion type="multiple" className="w-full">
            {syllabus.units.map((unit) => (
              <AccordionItem key={unit.id} value={`unit-${unit.id}`}>
                <AccordionTrigger className="hover:no-underline">
                  <div className="flex items-center gap-2 text-left">
                    <Badge variant="secondary" className="shrink-0">
                      Unit {unit.unit_number}
                    </Badge>
                    <span className="font-medium text-sm">{unit.title}</span>
                    {unit.hours && unit.hours > 0 && (
                      <Badge variant="outline" className="ml-auto shrink-0 gap-1 text-xs">
                        <Clock className="h-3 w-3" />
                        {unit.hours}h
                      </Badge>
                    )}
                  </div>
                </AccordionTrigger>
                <AccordionContent>
                  <div className="pl-2 space-y-1">
                    {/* Unit overview button */}
                    <Button
                      variant="ghost"
                      size="sm"
                      className="w-full justify-start text-left h-auto py-2 text-muted-foreground hover:text-foreground"
                      onClick={() => handleUnitClick(unit)}
                    >
                      <ChevronRight className="h-3 w-3 mr-2 shrink-0" />
                      <span className="text-xs">Ask about this entire unit</span>
                    </Button>

                    {/* Topics */}
                    {unit.topics && unit.topics.length > 0 && (
                      <div className="space-y-1 border-l-2 border-muted ml-1 pl-3">
                        {unit.topics.map((topic) => (
                          <Button
                            key={topic.id}
                            variant="ghost"
                            size="sm"
                            className="w-full justify-start text-left h-auto py-2 hover:bg-primary/10"
                            onClick={() => handleTopicClick(unit, topic)}
                          >
                            <div className="flex items-start gap-2 w-full">
                              <span className="text-xs text-muted-foreground shrink-0 font-mono">
                                {topic.topic_number}.
                              </span>
                              <span className="text-sm flex-1">{topic.title}</span>
                            </div>
                          </Button>
                        ))}
                      </div>
                    )}
                  </div>
                </AccordionContent>
              </AccordionItem>
            ))}
          </Accordion>
        )}

        {/* Books Section */}
        {syllabus.books && syllabus.books.length > 0 && (
          <div className="mt-4 pt-4 border-t">
            <h4 className="text-sm font-medium flex items-center gap-2 mb-3">
              <BookMarked className="h-4 w-4" />
              Reference Books
            </h4>
            <div className="space-y-2">
              {syllabus.books.map((book) => (
                <BookCard key={book.id} book={book} />
              ))}
            </div>
          </div>
        )}
    </div>
  );
}

function BookCard({ book }: { book: BookReference }) {
  return (
    <div className="flex items-start gap-2 p-2 rounded-md bg-muted/50 text-sm">
      <BookMarked className="h-4 w-4 text-muted-foreground shrink-0 mt-0.5" />
      <div className="flex-1 min-w-0">
        <p className="font-medium text-xs leading-tight">{book.title}</p>
        <p className="text-xs text-muted-foreground">{book.authors}</p>
      </div>
      <Badge variant="outline" className="text-[10px] shrink-0">
        {BOOK_TYPE_LABELS[book.book_type] || book.book_type}
      </Badge>
    </div>
  );
}

// ============= PYQs Content =============

interface PYQsContentProps {
  subjectId: string;
  onSelectQuestion: (question: string) => void;
  onClose: () => void;
}

function PYQsContent({ subjectId, onSelectQuestion, onClose }: PYQsContentProps) {
  const { data: pyqsData, isLoading } = usePYQs(subjectId);
  const [selectedPaperId, setSelectedPaperId] = useState<string | null>(null);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-full">
        <LoadingSpinner size="lg" text="Loading PYQs..." centered />
      </div>
    );
  }

  const papers = pyqsData?.papers || [];

  if (papers.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center h-full text-center px-4">
        <FileQuestion className="h-12 w-12 mb-3 text-muted-foreground opacity-50" />
        <p className="font-medium text-muted-foreground">No Question Papers</p>
        <p className="text-sm text-muted-foreground mt-1">
          Upload PYQ documents to see questions here.
        </p>
      </div>
    );
  }

  // If a paper is selected, show its questions
  if (selectedPaperId) {
    return (
      <PYQPaperQuestions
        paperId={selectedPaperId}
        onBack={() => setSelectedPaperId(null)}
        onSelectQuestion={(q) => {
          onSelectQuestion(q);
          onClose();
        }}
      />
    );
  }

  // Show list of papers
  return (
    <div className="space-y-2 pr-2">
      <p className="text-xs text-muted-foreground mb-3">
        {papers.length} question paper{papers.length !== 1 ? 's' : ''} available
      </p>
      {papers.map((paper) => (
        <PaperCard
          key={paper.id}
          paper={paper}
          onClick={() => setSelectedPaperId(String(paper.id))}
        />
      ))}
    </div>
  );
}

function PaperCard({ paper, onClick }: { paper: PYQPaperSummary; onClick: () => void }) {
  return (
    <Button
      variant="outline"
      className="w-full h-auto py-3 px-4 justify-start text-left"
      onClick={onClick}
    >
      <div className="flex items-center justify-between w-full">
        <div className="flex items-center gap-3">
          <Calendar className="h-4 w-4 text-muted-foreground" />
          <div>
            <p className="font-medium text-sm">
              {paper.year} {paper.month && `- ${paper.month}`}
            </p>
            <div className="flex items-center gap-2 mt-0.5">
              {paper.total_marks > 0 && (
                <span className="text-xs text-muted-foreground flex items-center gap-1">
                  <Award className="h-3 w-3" />
                  {paper.total_marks}m
                </span>
              )}
              {paper.total_questions > 0 && (
                <span className="text-xs text-muted-foreground flex items-center gap-1">
                  <ListOrdered className="h-3 w-3" />
                  {paper.total_questions}Q
                </span>
              )}
            </div>
          </div>
        </div>
        <ChevronRight className="h-4 w-4 text-muted-foreground" />
      </div>
    </Button>
  );
}

interface PYQPaperQuestionsProps {
  paperId: string;
  onBack: () => void;
  onSelectQuestion: (question: string) => void;
}

function PYQPaperQuestions({ paperId, onBack, onSelectQuestion }: PYQPaperQuestionsProps) {
  const { data: paper, isLoading } = usePYQById(paperId);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-full">
        <LoadingSpinner size="md" text="Loading questions..." centered />
      </div>
    );
  }

  if (!paper) {
    return (
      <div className="flex flex-col items-center justify-center h-full">
        <p className="text-muted-foreground">Paper not found</p>
        <Button variant="outline" className="mt-4" onClick={onBack}>
          Go Back
        </Button>
      </div>
    );
  }

  const handleQuestionClick = (question: PYQQuestion) => {
    // Create a natural question from the PYQ
    const questionText = question.question_text.length > 200
      ? question.question_text.slice(0, 200) + '...'
      : question.question_text;
    const prompt = `Explain and answer: "${questionText}"`;
    onSelectQuestion(prompt);
  };

  return (
    <div className="pr-2">
      {/* Header */}
      <div className="mb-4">
        <Button variant="ghost" size="sm" onClick={onBack} className="mb-2">
          <ChevronRight className="h-4 w-4 mr-1 rotate-180" />
          Back to Papers
        </Button>
        <div className="flex items-center gap-2 flex-wrap">
          <Badge variant="secondary">
            {paper.year} {paper.month && `- ${paper.month}`}
          </Badge>
          {paper.total_marks > 0 && (
            <Badge variant="outline" className="gap-1 text-xs">
              <Award className="h-3 w-3" />
              {paper.total_marks} marks
            </Badge>
          )}
        </div>
      </div>

      {/* Questions */}
      <div className="space-y-2">
        {paper.questions && paper.questions.length > 0 ? (
          paper.questions.map((question) => (
            <QuestionCard
              key={question.id}
              question={question}
              onClick={() => handleQuestionClick(question)}
            />
          ))
        ) : (
          <p className="text-sm text-muted-foreground text-center py-4">
            No questions extracted yet
          </p>
        )}
      </div>
    </div>
  );
}

function QuestionCard({ question, onClick }: { question: PYQQuestion; onClick: () => void }) {
  return (
    <Button
      variant="ghost"
      className={cn(
        "w-full h-auto py-3 px-3 justify-start text-left",
        "hover:bg-primary/10 border border-transparent hover:border-primary/20"
      )}
      onClick={onClick}
    >
      <div className="flex items-start gap-2 w-full">
        <Badge variant="secondary" className="shrink-0 text-xs">
          Q{question.question_number}
        </Badge>
        <div className="flex-1 min-w-0">
          <p className="text-sm line-clamp-3 whitespace-pre-wrap">
            {question.question_text}
          </p>
          <div className="flex items-center gap-2 mt-1">
            {question.marks > 0 && (
              <span className="text-xs text-muted-foreground">
                {question.marks} marks
              </span>
            )}
            {question.unit_number && question.unit_number > 0 && (
              <Badge variant="outline" className="text-[10px]">
                Unit {question.unit_number}
              </Badge>
            )}
          </div>
        </div>
      </div>
    </Button>
  );
}

// ============= AI Settings Content =============

interface AISettingsContentProps {
  settings: AISettings;
  onSettingsChange: (settings: AISettings) => void;
  subjectId?: string;
  subjectName?: string;
}

function AISettingsContent({ settings, onSettingsChange, subjectId, subjectName }: AISettingsContentProps) {
  const [saveScope, setSaveScope] = useState<'subject' | 'global'>('subject');
  const [showSaveConfirmation, setShowSaveConfirmation] = useState(false);

  // Get current settings metadata
  const metadata = subjectId 
    ? getSettingsMetadata(subjectId) 
    : { source: 'global' as const, updated_at: undefined, is_custom: undefined };
  const hasCustomSettings = subjectId ? hasSubjectCustomSettings(subjectId) : false;

  const handleSystemPromptChange = (value: string) => {
    onSettingsChange({ ...settings, system_prompt: value });
    showSaveConfirmation && setShowSaveConfirmation(false);
  };

  const handleCitationsToggle = (checked: boolean) => {
    onSettingsChange({ ...settings, include_citations: checked });
    showSaveConfirmation && setShowSaveConfirmation(false);
  };

  const handleMaxTokensChange = (value: string) => {
    // Allow any numeric input while typing
    const num = parseInt(value, 10);
    if (!isNaN(num)) {
      onSettingsChange({ ...settings, max_tokens: num });
      showSaveConfirmation && setShowSaveConfirmation(false);
    }
  };

  const handleMaxTokensBlur = (value: string) => {
    // Clamp to valid range on blur
    const num = parseInt(value, 10);
    if (isNaN(num) || num < 256) {
      onSettingsChange({ ...settings, max_tokens: 256 });
    } else if (num > 8192) {
      onSettingsChange({ ...settings, max_tokens: 8192 });
    }
  };

  const resetToDefaults = () => {
    onSettingsChange({ ...DEFAULT_AI_SETTINGS });
    setShowSaveConfirmation(true);
    setTimeout(() => setShowSaveConfirmation(false), 2000);
  };

  const handleSaveAsGlobal = () => {
    saveGlobalAISettings(settings);
    setShowSaveConfirmation(true);
    setTimeout(() => setShowSaveConfirmation(false), 2000);
  };

  const handleRemoveCustomSettings = () => {
    if (subjectId) {
      removeSubjectAISettings(subjectId);
      // Reload settings from global
      window.location.reload();
    }
  };

  // Replace placeholder in default prompt
  const displayDefaultPrompt = DEFAULT_SYSTEM_PROMPT.replace(
    '{subject_name}',
    subjectName || 'this subject'
  );

  return (
    <div className="space-y-6 pr-2">
      {/* Settings Status */}
      <div className="space-y-2">
        <div className="flex items-center gap-2">
          <Info className="h-4 w-4 text-muted-foreground" />
          <Label className="text-sm font-medium">Settings Status</Label>
        </div>
        <div className="flex items-center gap-2 flex-wrap">
          <Badge variant={metadata.source === 'subject' ? 'default' : 'secondary'} className="gap-1.5">
            <span className={cn(
              "h-2 w-2 rounded-full",
              metadata.source === 'subject' ? 'bg-blue-500' : 'bg-gray-500'
            )} />
            {metadata.source === 'subject' ? 'Subject-specific' : 
             metadata.source === 'global' ? 'Global settings' : 'Default settings'}
          </Badge>
          {metadata.updated_at && (
            <Badge variant="outline" className="text-xs">
              Updated {new Date(metadata.updated_at).toLocaleDateString()}
            </Badge>
          )}
          {showSaveConfirmation && (
            <Badge variant="secondary" className="text-xs animate-pulse">
              ✓ Saved
            </Badge>
          )}
        </div>
        <p className="text-xs text-muted-foreground">
          {metadata.source === 'subject' 
            ? `Custom settings for ${subjectName || 'this subject'}. Changes apply only to this subject.`
            : metadata.source === 'global'
            ? 'Global settings apply to all subjects unless overridden.'
            : 'Using default system settings.'}
        </p>
      </div>

      {/* Model Info - Read Only */}
      <div className="space-y-2">
        <div className="flex items-center gap-2">
          <Cpu className="h-4 w-4 text-muted-foreground" />
          <Label className="text-sm font-medium">AI Model</Label>
        </div>
        <div className="flex items-center gap-2">
          <Badge variant="secondary" className="gap-1.5">
            <span className="h-2 w-2 rounded-full bg-green-500" />
            GPT 120B OSS
          </Badge>
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger asChild>
                <Badge variant="outline" className="text-xs cursor-help">
                  via DigitalOcean
                </Badge>
              </TooltipTrigger>
              <TooltipContent>
                <p className="text-xs">Powered by DigitalOcean GenAI Platform</p>
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
        </div>
        <p className="text-xs text-muted-foreground">
          Model selection is managed by the system
        </p>
      </div>

      {/* Citations Toggle */}
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Quote className="h-4 w-4 text-muted-foreground" />
            <Label htmlFor="citations-toggle" className="text-sm font-medium">
              Include Citations
            </Label>
          </div>
          <Switch
            id="citations-toggle"
            checked={settings.include_citations ?? true}
            onCheckedChange={handleCitationsToggle}
          />
        </div>
        <p className="text-xs text-muted-foreground">
          When enabled, AI responses will include source references from the knowledge base
        </p>
      </div>

      {/* Max Tokens */}
      <div className="space-y-2">
        <div className="flex items-center gap-2">
          <Hash className="h-4 w-4 text-muted-foreground" />
          <Label htmlFor="max-tokens" className="text-sm font-medium">
            Max Response Length
          </Label>
        </div>
        <div className="flex items-center gap-2">
          <Input
            id="max-tokens"
            type="number"
            min={256}
            max={8192}
            step={256}
            value={settings.max_tokens ?? 2048}
            onChange={(e) => handleMaxTokensChange(e.target.value)}
            onBlur={(e) => handleMaxTokensBlur(e.target.value)}
            className="w-24"
          />
          <span className="text-xs text-muted-foreground">tokens (256 - 8192)</span>
        </div>
        <p className="text-xs text-muted-foreground">
          Controls the maximum length of AI responses. Higher values allow longer answers.
        </p>
      </div>

      {/* System Prompt */}
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Settings2 className="h-4 w-4 text-muted-foreground" />
            <Label htmlFor="system-prompt" className="text-sm font-medium">
              System Prompt
            </Label>
          </div>
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button variant="ghost" size="sm" className="h-6 px-2">
                  <Info className="h-3 w-3" />
                </Button>
              </TooltipTrigger>
              <TooltipContent side="left" className="max-w-xs">
                <p className="text-xs">
                  The system prompt defines how the AI behaves. Leave empty to use the default prompt.
                </p>
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
        </div>
        <Textarea
          id="system-prompt"
          placeholder={displayDefaultPrompt}
          value={settings.system_prompt || ''}
          onChange={(e) => handleSystemPromptChange(e.target.value)}
          className="min-h-[200px] text-xs font-mono"
        />
        <p className="text-xs text-muted-foreground">
          {settings.system_prompt 
            ? 'Using custom system prompt' 
            : 'Using default system prompt (shown as placeholder)'}
        </p>
      </div>

      {/* Action Buttons */}
      <div className="pt-4 border-t space-y-3">
        {/* Reset Button */}
        <Button
          variant="outline"
          size="sm"
          onClick={resetToDefaults}
          className="w-full gap-2"
        >
          <RotateCcw className="h-4 w-4" />
          Reset to Defaults
        </Button>

        {/* Advanced Actions */}
        {hasCustomSettings && (
          <div className="space-y-2">
            <div className="flex gap-2">
              <Button
                variant="secondary"
                size="sm"
                onClick={handleSaveAsGlobal}
                className="flex-1 gap-2 text-xs"
              >
                <Settings2 className="h-3 w-3" />
                Save as Global
              </Button>
              <Button
                variant="destructive"
                size="sm"
                onClick={handleRemoveCustomSettings}
                className="flex-1 gap-2 text-xs"
              >
                <RotateCcw className="h-3 w-3" />
                Use Global
              </Button>
            </div>
            <p className="text-xs text-muted-foreground text-center">
              Save current settings globally or revert to global settings
            </p>
          </div>
        )}
      </div>
    </div>
  );
}

// ============= Helper function to get recent PYQ questions =============

export function getRecentPYQQuestions(
  papers: PYQPaperSummary[] | undefined,
  fullPapers: Map<number, PYQPaper>,
  limit: number = 3
): { question: string; year: number; month?: string }[] {
  if (!papers || papers.length === 0) return [];

  // Sort papers by year (descending) and get most recent
  const sortedPapers = [...papers]
    .filter((p) => p.extraction_status === 'completed')
    .sort((a, b) => b.year - a.year);

  const questions: { question: string; year: number; month?: string }[] = [];

  for (const paper of sortedPapers) {
    const fullPaper = fullPapers.get(paper.id);
    if (fullPaper?.questions) {
      for (const q of fullPaper.questions) {
        if (questions.length >= limit) break;
        questions.push({
          question: q.question_text.length > 100
            ? q.question_text.slice(0, 100) + '...'
            : q.question_text,
          year: paper.year,
          month: paper.month,
        });
      }
    }
    if (questions.length >= limit) break;
  }

  return questions;
}
