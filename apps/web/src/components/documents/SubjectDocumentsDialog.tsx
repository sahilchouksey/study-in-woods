'use client';

import { useState } from 'react';
import { BookOpen, Upload, List, FolderOpen, FileText, FileQuestion } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Badge } from '@/components/ui/badge';
import { DocumentList } from './DocumentComponents';
import { MultiFileUploadForm } from './MultiFileUploadForm';
import { SyllabusTab } from './SyllabusTab';
import { PYQTab } from './PYQTab';
import { useDocuments } from '@/lib/api/hooks/useDocuments';
import { useSyllabus } from '@/lib/api/hooks/useSyllabus';
import { usePYQs } from '@/lib/api/hooks/usePYQ';

interface Subject {
  id: string;
  name: string;
  code: string;
  credits?: number;
  description?: string;
}

interface SubjectDocumentsDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  subject: Subject | null;
  userId?: string;
  isAdmin: boolean;
  isAuthenticated: boolean;
}

export function SubjectDocumentsDialog({
  open,
  onOpenChange,
  subject,
  userId,
  isAdmin,
  isAuthenticated,
}: SubjectDocumentsDialogProps) {
  const [activeTab, setActiveTab] = useState<'documents' | 'syllabus' | 'pyqs' | 'upload'>('documents');
  
  // Get document count for the badge
  const { data: documentsData } = useDocuments(subject?.id || null);
  const documentCount = documentsData?.data?.length || 0;

  // Get syllabus status for the badge
  const { data: syllabus } = useSyllabus(subject?.id || null);
  const hasSyllabus = syllabus?.extraction_status === 'completed';

  // Get PYQs count for the badge
  const { data: pyqsData } = usePYQs(subject?.id || null);
  const pyqCount = pyqsData?.total || 0;

  if (!subject) return null;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-2xl max-h-[85vh] flex flex-col">
        <DialogHeader className="pb-4 border-b">
          <div className="flex items-start gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10 shrink-0">
              <BookOpen className="h-5 w-5 text-primary" />
            </div>
            <div className="flex-1 min-w-0">
              <DialogTitle className="text-lg truncate">{subject.name}</DialogTitle>
              <div className="flex flex-wrap items-center gap-2 mt-1">
                <Badge variant="outline" className="text-xs">
                  {subject.code}
                </Badge>
                {subject.credits && (
                  <span className="text-xs text-muted-foreground">
                    {subject.credits} credits
                  </span>
                )}
                {documentCount > 0 && (
                  <Badge variant="secondary" className="text-xs">
                    {documentCount} document{documentCount !== 1 ? 's' : ''}
                  </Badge>
                )}
              </div>
              {subject.description && (
                <p className="text-sm text-muted-foreground mt-2 line-clamp-2">
                  {subject.description}
                </p>
              )}
            </div>
          </div>
        </DialogHeader>

        <Tabs
          value={activeTab}
          onValueChange={(v) => setActiveTab(v as 'documents' | 'syllabus' | 'pyqs' | 'upload')}
          className="flex-1 flex flex-col min-h-0"
        >
          <TabsList className={`grid w-full ${isAdmin ? 'grid-cols-4' : 'grid-cols-3'}`}>
            <TabsTrigger value="documents" className="gap-1.5">
              <List className="h-4 w-4" />
              <span className="hidden sm:inline">Docs</span>
            </TabsTrigger>
            <TabsTrigger value="syllabus" className="gap-1.5">
              <FileText className="h-4 w-4" />
              <span className="hidden sm:inline">Syllabus</span>
              {hasSyllabus && (
                <span className="h-2 w-2 rounded-full bg-green-500" />
              )}
            </TabsTrigger>
            <TabsTrigger value="pyqs" className="gap-1.5">
              <FileQuestion className="h-4 w-4" />
              <span className="hidden sm:inline">PYQs</span>
              {pyqCount > 0 && (
                <Badge variant="secondary" className="h-5 px-1.5 text-xs">
                  {pyqCount}
                </Badge>
              )}
            </TabsTrigger>
            {/* Upload tab - Admin only */}
            {isAdmin && (
              <TabsTrigger 
                value="upload" 
                className="gap-1.5"
                disabled={!isAuthenticated}
                title={!isAuthenticated ? 'Login to upload documents' : undefined}
              >
                <Upload className="h-4 w-4" />
                <span className="hidden sm:inline">Upload</span>
              </TabsTrigger>
            )}
          </TabsList>

          <div className="flex-1 min-h-0 mt-4">
            <TabsContent value="documents" className="h-full m-0">
              <ScrollArea className="h-[350px] pr-4">
                <DocumentList
                  subjectId={subject.id}
                  userId={userId}
                  isAdmin={isAdmin}
                />
              </ScrollArea>
            </TabsContent>

            <TabsContent value="syllabus" className="h-full m-0">
              <SyllabusTab subjectId={subject.id} />
            </TabsContent>

            <TabsContent value="pyqs" className="h-full m-0">
              <PYQTab subjectId={subject.id} subjectCode={subject.code} subjectName={subject.name} isAdmin={isAdmin} />
            </TabsContent>

            {/* Upload tab content - Admin only */}
            {isAdmin && (
              <TabsContent value="upload" className="h-full m-0">
                {isAuthenticated ? (
                  <ScrollArea className="h-[350px] pr-4">
                    <MultiFileUploadForm
                      subjectId={subject.id}
                      subjectName={subject.name}
                      onSuccess={() => setActiveTab('documents')}
                      excludeTypes={['syllabus']}
                    />
                  </ScrollArea>
                ) : (
                  <div className="flex flex-col items-center justify-center h-[350px] text-center text-muted-foreground">
                    <FolderOpen className="h-12 w-12 mb-3 opacity-50" />
                    <p className="font-medium">Login Required</p>
                    <p className="text-sm mt-1">
                      Please log in to upload documents to this subject
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
                  </div>
                )}
              </TabsContent>
            )}
          </div>
        </Tabs>
      </DialogContent>
    </Dialog>
  );
}
