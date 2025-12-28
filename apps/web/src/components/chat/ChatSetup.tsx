'use client';

import { useState, useEffect, useMemo } from 'react';
import { useRouter } from 'next/navigation';
import { 
  GraduationCap, 
  BookOpen, 
  Calendar, 
  FileText, 
  MessageSquare, 
  AlertCircle,
  Sparkles,
  Star
} from 'lucide-react';
import { LoadingSpinner, InlineSpinner } from '@/components/ui/loading-spinner';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import {
  useChatContext,
  useChatContextFilters,
  useCreateChatSession,
} from '@/lib/api/hooks/useChat';

export interface ChatSetupProps {
  onSessionCreated?: (sessionId: string) => void;
}

export function ChatSetup({ onSessionCreated }: ChatSetupProps) {
  const router = useRouter();
  const { data: context, isLoading: contextLoading, error: contextError } = useChatContext();
  const { getCoursesForUniversity, getSemestersForCourse, getSubjectsForSemester } = useChatContextFilters(context);
  
  const [selectedUniversityId, setSelectedUniversityId] = useState<number | null>(null);
  const [selectedCourseId, setSelectedCourseId] = useState<number | null>(null);
  const [selectedSemesterId, setSelectedSemesterId] = useState<number | null>(null);
  const [selectedSubjectId, setSelectedSubjectId] = useState<number | null>(null);

  const createSession = useCreateChatSession();

  const universities = context?.universities || [];
  const courses = useMemo(() => 
    selectedUniversityId ? getCoursesForUniversity(selectedUniversityId) : [],
    [selectedUniversityId, getCoursesForUniversity]
  );
  const semesters = useMemo(() => 
    selectedCourseId ? getSemestersForCourse(selectedCourseId) : [],
    [selectedCourseId, getSemestersForCourse]
  );
  const subjects = useMemo(() => 
    selectedSemesterId ? getSubjectsForSemester(selectedSemesterId) : [],
    [selectedSemesterId, getSubjectsForSemester]
  );

  const selectedSubject = useMemo(() => 
    subjects.find(s => s.id === selectedSubjectId),
    [subjects, selectedSubjectId]
  );

  // Auto-select first options
  useEffect(() => {
    if (universities.length > 0 && !selectedUniversityId) {
      setSelectedUniversityId(universities[0].id);
    }
  }, [universities, selectedUniversityId]);

  useEffect(() => {
    if (courses.length > 0) {
      setSelectedCourseId(courses[0].id);
    } else {
      setSelectedCourseId(null);
    }
  }, [courses]);

  useEffect(() => {
    if (semesters.length > 0) {
      setSelectedSemesterId(semesters[0].id);
    } else {
      setSelectedSemesterId(null);
    }
  }, [semesters]);

  useEffect(() => {
    if (subjects.length > 0) {
      setSelectedSubjectId(subjects[0].id);
    } else {
      setSelectedSubjectId(null);
    }
  }, [subjects]);

  const handleUniversityChange = (value: string) => {
    setSelectedUniversityId(parseInt(value, 10));
    setSelectedCourseId(null);
    setSelectedSemesterId(null);
    setSelectedSubjectId(null);
  };

  const handleCourseChange = (value: string) => {
    setSelectedCourseId(parseInt(value, 10));
    setSelectedSemesterId(null);
    setSelectedSubjectId(null);
  };

  const handleSemesterChange = (value: string) => {
    setSelectedSemesterId(parseInt(value, 10));
    setSelectedSubjectId(null);
  };

  const handleSubjectChange = (value: string) => {
    setSelectedSubjectId(parseInt(value, 10));
  };

  const handleStartChatting = async () => {
    if (!selectedSubjectId || !selectedSubject) return;

    try {
      const session = await createSession.mutateAsync({
        subject_id: selectedSubjectId,
      });
      
      const sessionId = String(session.id);
      const targetUrl = `/chat/${sessionId}`;
      
      console.log('Session created:', session);
      console.log('Navigating to:', targetUrl);
      
      // Navigate to the session URL - this persists the session ID in the URL
      router.push(targetUrl);
      
      // Also call the callback if provided (for compatibility)
      onSessionCreated?.(sessionId);
    } catch (error) {
      console.error('Failed to create session:', error);
    }
  };

  if (contextLoading) {
    return (
      <div className="flex items-center justify-center h-full">
        <LoadingSpinner size="xl" text="Loading chat options..." centered />
      </div>
    );
  }

  if (contextError) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-center space-y-4 max-w-md">
          <AlertCircle className="h-12 w-12 text-destructive mx-auto" />
          <h3 className="text-lg font-semibold">Failed to load chat options</h3>
          <p className="text-muted-foreground">
            Please try refreshing the page or <a href="mailto:support@studyinwoods.app" className="underline hover:text-foreground">contact support</a> if the issue persists.
          </p>
        </div>
      </div>
    );
  }

  const canStartChat = selectedSubject !== undefined;
  const noSubjectsAvailable = subjects.length === 0 && selectedSemesterId !== null;

  return (
    <div className="flex items-center justify-center h-full p-6">
      <Card className="w-full max-w-2xl shadow-lg">
        <CardHeader className="text-center pb-2">
          <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-primary/10">
            <Sparkles className="h-8 w-8 text-primary" />
          </div>
          <CardTitle className="text-2xl">Start a Chat Session</CardTitle>
          <CardDescription className="text-base">
            Select your course details to chat with an AI tutor. Only AI-enabled subjects are available.
          </CardDescription>
        </CardHeader>
        
        <CardContent className="space-y-5 pt-4">
          {/* University */}
          <div className="space-y-2">
            <Label htmlFor="university" className="flex items-center gap-2 text-sm font-medium">
              <GraduationCap className="h-4 w-4 text-muted-foreground" />
              University
            </Label>
            <Select
              value={selectedUniversityId?.toString() || ''}
              onValueChange={handleUniversityChange}
            >
              <SelectTrigger id="university" className="w-full">
                <SelectValue placeholder="Select university" />
              </SelectTrigger>
              <SelectContent>
                {universities.map((uni) => (
                  <SelectItem key={uni.id} value={uni.id.toString()}>
                    {uni.name} ({uni.code})
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {/* Course */}
          <div className="space-y-2">
            <Label htmlFor="course" className="flex items-center gap-2 text-sm font-medium">
              <BookOpen className="h-4 w-4 text-muted-foreground" />
              Course
            </Label>
            <Select
              value={selectedCourseId?.toString() || ''}
              onValueChange={handleCourseChange}
              disabled={!selectedUniversityId || courses.length === 0}
            >
              <SelectTrigger id="course" className="w-full">
                <SelectValue placeholder={courses.length === 0 ? "No courses available" : "Select course"} />
              </SelectTrigger>
              <SelectContent>
                {courses.map((course) => (
                  <SelectItem key={course.id} value={course.id.toString()}>
                    {course.name} ({course.code})
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {/* Semester */}
          <div className="space-y-2">
            <Label htmlFor="semester" className="flex items-center gap-2 text-sm font-medium">
              <Calendar className="h-4 w-4 text-muted-foreground" />
              Semester
            </Label>
            <Select
              value={selectedSemesterId?.toString() || ''}
              onValueChange={handleSemesterChange}
              disabled={!selectedCourseId || semesters.length === 0}
            >
              <SelectTrigger id="semester" className="w-full">
                <SelectValue placeholder={semesters.length === 0 ? "No semesters available" : "Select semester"} />
              </SelectTrigger>
              <SelectContent>
                {semesters.map((sem) => (
                  <SelectItem key={sem.id} value={sem.id.toString()}>
                    {sem.name || `Semester ${sem.number}`}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {/* Subject */}
          <div className="space-y-2">
            <Label htmlFor="subject" className="flex items-center gap-2 text-sm font-medium">
              <FileText className="h-4 w-4 text-muted-foreground" />
              Subject
            </Label>
            <Select
              value={selectedSubjectId?.toString() || ''}
              onValueChange={handleSubjectChange}
              disabled={!selectedSemesterId || subjects.length === 0}
            >
              <SelectTrigger id="subject" className="w-full">
                <SelectValue placeholder={noSubjectsAvailable ? "No AI-enabled subjects" : "Select subject"} />
              </SelectTrigger>
              <SelectContent>
                {subjects.map((subject) => (
                  <SelectItem key={subject.id} value={subject.id.toString()}>
                    <span className="flex items-center gap-2">
                      {subject.is_starred && (
                        <Star className="h-3.5 w-3.5 text-yellow-500 fill-yellow-500 flex-shrink-0" />
                      )}
                      {subject.name}
                      <span className="text-muted-foreground">({subject.code})</span>
                      {subject.has_syllabus && (
                        <Badge variant="secondary" className="text-xs ml-1">
                          Syllabus
                        </Badge>
                      )}
                    </span>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {noSubjectsAvailable && (
              <p className="text-sm text-muted-foreground">
                No subjects in this semester have AI agents configured yet.
              </p>
            )}
          </div>

          {/* Selected Subject Preview */}
          {selectedSubject && (
            <div className="rounded-lg border bg-muted/30 p-4 space-y-3">
              <div className="flex items-start justify-between">
                <div className="flex items-start gap-2">
                  {selectedSubject.is_starred && (
                    <Star className="h-5 w-5 text-yellow-500 fill-yellow-500 flex-shrink-0 mt-0.5" />
                  )}
                  <div>
                    <h4 className="font-semibold">{selectedSubject.name}</h4>
                    <p className="text-sm text-muted-foreground">Code: {selectedSubject.code}</p>
                  </div>
                </div>
                <Badge variant="outline">{selectedSubject.credits} Credits</Badge>
              </div>
              {selectedSubject.description && (
                <p className="text-sm text-muted-foreground line-clamp-2">
                  {selectedSubject.description}
                </p>
              )}
              <div className="flex gap-2">
                {selectedSubject.has_syllabus && (
                  <Badge variant="default" className="text-xs">
                    <FileText className="h-3 w-3 mr-1" />
                    Syllabus Context
                  </Badge>
                )}
                <Badge variant="secondary" className="text-xs">
                  <Sparkles className="h-3 w-3 mr-1" />
                  AI Enabled
                </Badge>
              </div>
            </div>
          )}

          {/* Start Button */}
          <Button
            onClick={handleStartChatting}
            disabled={!canStartChat || createSession.isPending}
            className="w-full"
            size="lg"
          >
            {createSession.isPending ? (
              <>
                <InlineSpinner className="mr-2" />
                Creating session...
              </>
            ) : (
              <>
                <MessageSquare className="mr-2 h-4 w-4" />
                Start Chatting
              </>
            )}
          </Button>

          <p className="text-xs text-center text-muted-foreground">
            Responses include citations from the knowledge base.
            {selectedSubject?.has_syllabus && ' Syllabus context enhances answer relevance.'}
          </p>
        </CardContent>
      </Card>
    </div>
  );
}
