'use client';

import { useState } from 'react';
import { BookOpen, GraduationCap, MapPin, CheckCircle, BookMarked, FileText } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { useUniversities, useCoursesByUniversity, useSemesters, useSubjects } from '@/lib/api/hooks/useCourses';
import { useAuth } from '@/providers/auth-provider';
import { useUpdateProfile } from '@/lib/api/hooks/useAuth';
import { useDeleteUniversity, useDeleteCourse, useDeleteSemester, useDeleteSubject } from '@/lib/api/hooks/useAdminMutations';
import { AdminActionButtons } from '@/components/admin/AdminActionButtons';
import { UniversityFormDialog } from '@/components/admin/UniversityFormDialog';
import { CourseFormDialog } from '@/components/admin/CourseFormDialog';
import { SemesterFormDialog } from '@/components/admin/SemesterFormDialog';
import { SubjectFormDialog } from '@/components/admin/SubjectFormDialog';
import { DeleteConfirmationDialog } from '@/components/admin/DeleteConfirmationDialog';
import { SubjectDocumentsDialog } from '@/components/documents/SubjectDocumentsDialog';
import { toast } from 'sonner';
import type { University, Course, Semester, Subject } from '@/lib/api/courses';

export function CoursesTab() {
  const { user, isAdmin, isAuthenticated } = useAuth();
  const { data: universities = [], isLoading: universitiesLoading } = useUniversities();
  const [selectedUniversityId, setSelectedUniversityId] = useState<string | null>(
    user?.university_id || null
  );
  const { data: courses = [], isLoading: coursesLoading } = useCoursesByUniversity(
    selectedUniversityId
  );
  const updateProfileMutation = useUpdateProfile();

  const [selectedCourseId, setSelectedCourseId] = useState<string | null>(
    user?.course_id || null
  );
  
  const { data: semesters = [], isLoading: semestersLoading } = useSemesters(selectedCourseId);
  
  const [selectedSemesterId, setSelectedSemesterId] = useState<string | null>(null);
  
  const { data: subjects = [], isLoading: subjectsLoading } = useSubjects(selectedSemesterId);

  // Admin mutations
  const deleteUniversityMutation = useDeleteUniversity();
  const deleteCourseMutation = useDeleteCourse();
  const deleteSemesterMutation = useDeleteSemester();
  const deleteSubjectMutation = useDeleteSubject();

  // Dialog states
  const [universityDialog, setUniversityDialog] = useState<{ open: boolean; university: University | null }>({
    open: false,
    university: null,
  });
  const [courseDialog, setCourseDialog] = useState<{ open: boolean; course: Course | null }>({
    open: false,
    course: null,
  });
  const [semesterDialog, setSemesterDialog] = useState<{ open: boolean; semester: Semester | null }>({
    open: false,
    semester: null,
  });
  const [subjectDialog, setSubjectDialog] = useState<{ open: boolean; subject: Subject | null }>({
    open: false,
    subject: null,
  });

  // Subject documents dialog state
  const [documentsDialog, setDocumentsDialog] = useState<{ open: boolean; subject: Subject | null }>({
    open: false,
    subject: null,
  });

  // Delete confirmation dialog states
  const [deleteDialog, setDeleteDialog] = useState<{
    open: boolean;
    type: 'university' | 'course' | 'semester' | 'subject' | null;
    id: string | null;
    name: string;
    courseId?: string;
    semesterNumber?: number;
    semesterId?: string;
  }>({
    open: false,
    type: null,
    id: null,
    name: '',
  });

  const selectedUniversity = universities.find((u) => u.id === selectedUniversityId);
  const selectedCourse = courses.find((c) => c.id === selectedCourseId);
  const selectedSemester = semesters.find((s) => s.id === selectedSemesterId);

  const handleSaveProfile = async () => {
    if (!selectedUniversityId || !selectedCourseId || !selectedSemesterId) {
      return;
    }

    // Find the semester number to save
    const semester = semesters.find(s => s.id === selectedSemesterId);
    if (!semester) return;

    try {
      await updateProfileMutation.mutateAsync({
        university_id: selectedUniversityId,
        course_id: selectedCourseId,
        semester: `${semester.number}${semester.number === 1 ? 'st' : semester.number === 2 ? 'nd' : semester.number === 3 ? 'rd' : 'th'} Semester`,
      });
    } catch (error) {
      console.error('Failed to update profile:', error);
    }
  };

  const hasChanges =
    selectedUniversityId !== user?.university_id ||
    selectedCourseId !== user?.course_id;

  // Open delete confirmation dialog
  const openDeleteDialog = (
    type: 'university' | 'course' | 'semester' | 'subject',
    id: string,
    name: string,
    extra?: { courseId?: string; semesterNumber?: number; semesterId?: string }
  ) => {
    setDeleteDialog({
      open: true,
      type,
      id,
      name,
      ...extra,
    });
  };

  const closeDeleteDialog = () => {
    setDeleteDialog({
      open: false,
      type: null,
      id: null,
      name: '',
    });
  };

  // Execute delete based on dialog type
  const handleConfirmDelete = async () => {
    if (!deleteDialog.type || !deleteDialog.id) return;

    try {
      switch (deleteDialog.type) {
        case 'university':
          await deleteUniversityMutation.mutateAsync(deleteDialog.id);
          toast.success('University and all associated data deleted successfully');
          if (selectedUniversityId === deleteDialog.id) {
            setSelectedUniversityId(null);
            setSelectedCourseId(null);
            setSelectedSemesterId(null);
          }
          break;

        case 'course':
          await deleteCourseMutation.mutateAsync(deleteDialog.id);
          toast.success('Course and all associated data deleted successfully');
          if (selectedCourseId === deleteDialog.id) {
            setSelectedCourseId(null);
            setSelectedSemesterId(null);
          }
          break;

        case 'semester':
          if (deleteDialog.courseId && deleteDialog.semesterNumber) {
            await deleteSemesterMutation.mutateAsync({
              courseId: deleteDialog.courseId,
              number: deleteDialog.semesterNumber,
            });
            toast.success('Semester and all associated subjects deleted successfully');
            setSelectedSemesterId(null);
          }
          break;

        case 'subject':
          if (deleteDialog.semesterId) {
            await deleteSubjectMutation.mutateAsync({
              semesterId: deleteDialog.semesterId,
              subjectId: deleteDialog.id,
            });
            toast.success('Subject deleted successfully');
          }
          break;
      }
      closeDeleteDialog();
    } catch (error: unknown) {
      const err = error as { message?: string; data?: { error?: { message?: string } } };
      const errorMessage = err.data?.error?.message || err.message || `Failed to delete ${deleteDialog.type}`;
      toast.error(errorMessage);
      console.error(`Failed to delete ${deleteDialog.type}:`, error);
    }
  };

  // Get delete dialog content based on type
  const getDeleteDialogContent = () => {
    switch (deleteDialog.type) {
      case 'university':
        return {
          title: 'Delete University',
          description: `Are you sure you want to delete "${deleteDialog.name}"?`,
          cascadeWarning: 'This will permanently delete all courses, semesters, subjects, and documents associated with this university.',
        };
      case 'course':
        return {
          title: 'Delete Course',
          description: `Are you sure you want to delete "${deleteDialog.name}"?`,
          cascadeWarning: 'This will permanently delete all semesters, subjects, and documents associated with this course.',
        };
      case 'semester':
        return {
          title: 'Delete Semester',
          description: `Are you sure you want to delete "${deleteDialog.name}"?`,
          cascadeWarning: 'This will permanently delete all subjects and documents associated with this semester.',
        };
      case 'subject':
        return {
          title: 'Delete Subject',
          description: `Are you sure you want to delete "${deleteDialog.name}"?`,
          cascadeWarning: 'This will permanently delete all documents associated with this subject.',
        };
      default:
        return {
          title: 'Delete',
          description: 'Are you sure you want to delete this item?',
        };
    }
  };

  const deleteDialogContent = getDeleteDialogContent();
  const isDeleting =
    deleteUniversityMutation.isPending ||
    deleteCourseMutation.isPending ||
    deleteSemesterMutation.isPending ||
    deleteSubjectMutation.isPending;

  return (
    <div className="h-full flex flex-col min-h-0">
      {/* Header */}
      <div className="border-b border-neutral-200 dark:border-neutral-800 p-6">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-2xl font-semibold">My Courses</h2>
            <p className="text-sm text-neutral-600 dark:text-neutral-400 mt-1">
              Select your university, course, and semester for personalized learning
            </p>
          </div>
          {hasChanges && (
            <Button
              onClick={handleSaveProfile}
              disabled={
                !selectedUniversityId ||
                !selectedCourseId ||
                !selectedSemesterId ||
                updateProfileMutation.isPending
              }
              className="bg-black hover:bg-neutral-800 dark:bg-white dark:hover:bg-neutral-200 dark:text-black"
            >
              {updateProfileMutation.isPending ? 'Saving...' : 'Save Changes'}
            </Button>
          )}
        </div>
      </div>

      <ScrollArea className="flex-1 min-h-0 p-6">
        <div className="max-w-4xl mx-auto space-y-8">
          {/* University Selection */}
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <GraduationCap className="h-5 w-5" />
                <h3 className="text-lg font-semibold">Select University</h3>
              </div>
              {isAdmin && (
                <AdminActionButtons
                  showCreate
                  showEdit={false}
                  showDelete={false}
                  createLabel="Add University"
                  onCreate={() => setUniversityDialog({ open: true, university: null })}
                />
              )}
            </div>
            <p className="text-sm text-neutral-600 dark:text-neutral-400">
              Choose your university to see available courses
            </p>
            
            {universitiesLoading ? (
              <div className="flex items-center justify-center py-12">
                <div className="animate-spin rounded-full h-8 w-8 border-t-2 border-b-2 border-black dark:border-white" />
              </div>
            ) : (
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {universities.map((university) => (
                  <div
                    key={university.id}
                    role="button"
                    tabIndex={0}
                    className={`text-left p-4 rounded-lg border transition-all relative cursor-pointer ${
                      selectedUniversityId === university.id
                        ? 'border-black dark:border-white bg-black/5 dark:bg-white/5'
                        : 'border-neutral-200 dark:border-neutral-800 hover:border-neutral-400 dark:hover:border-neutral-600'
                    }`}
                    onClick={() => {
                      setSelectedUniversityId(university.id);
                      setSelectedCourseId(null);
                      setSelectedSemesterId(null);
                    }}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault();
                        setSelectedUniversityId(university.id);
                        setSelectedCourseId(null);
                        setSelectedSemesterId(null);
                      }
                    }}
                  >
                    <div className="flex items-start justify-between gap-4">
                      <div className="space-y-1 flex-1">
                        <h4 className="font-medium">{university.name}</h4>
                        <p className="text-sm text-neutral-600 dark:text-neutral-400 flex items-center gap-1">
                          <MapPin className="h-3 w-3" />
                          {university.location}
                        </p>
                        <Badge variant="secondary" className="text-xs mt-2">
                          {university.code}
                        </Badge>
                      </div>
                      <div className="flex items-center gap-2">
                        {selectedUniversityId === university.id && (
                          <CheckCircle className="h-5 w-5 flex-shrink-0" />
                        )}
                        {isAdmin && (
                          <div onClick={(e) => e.stopPropagation()}>
                            <AdminActionButtons
                              showCreate={false}
                              onEdit={() => setUniversityDialog({ open: true, university })}
                              onDelete={() => openDeleteDialog('university', university.id, university.name)}
                              isDeleting={deleteUniversityMutation.isPending}
                            />
                          </div>
                        )}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Course Selection */}
          {selectedUniversityId && (
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <BookOpen className="h-5 w-5" />
                  <h3 className="text-lg font-semibold">Select Course</h3>
                </div>
                {isAdmin && (
                  <AdminActionButtons
                    showCreate
                    showEdit={false}
                    showDelete={false}
                    createLabel="Add Course"
                    onCreate={() => setCourseDialog({ open: true, course: null })}
                  />
                )}
              </div>
              <p className="text-sm text-neutral-600 dark:text-neutral-400">
                {selectedUniversity
                  ? `Available courses at ${selectedUniversity.name}`
                  : 'Choose your course program'}
              </p>
              
              {coursesLoading ? (
                <div className="flex items-center justify-center py-12">
                  <div className="animate-spin rounded-full h-8 w-8 border-t-2 border-b-2 border-black dark:border-white" />
                </div>
              ) : courses.length === 0 ? (
                <div className="text-center py-12 text-neutral-600 dark:text-neutral-400">
                  No courses available for this university
                </div>
              ) : (
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  {courses.map((course) => (
                    <div
                      key={course.id}
                      role="button"
                      tabIndex={0}
                      className={`text-left p-4 rounded-lg border transition-all cursor-pointer ${
                        selectedCourseId === course.id
                          ? 'border-black dark:border-white bg-black/5 dark:bg-white/5'
                          : 'border-neutral-200 dark:border-neutral-800 hover:border-neutral-400 dark:hover:border-neutral-600'
                      }`}
                      onClick={() => {
                        setSelectedCourseId(course.id);
                        setSelectedSemesterId(null);
                      }}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter' || e.key === ' ') {
                          e.preventDefault();
                          setSelectedCourseId(course.id);
                          setSelectedSemesterId(null);
                        }
                      }}
                    >
                      <div className="flex items-start justify-between gap-4">
                        <div className="space-y-2 flex-1">
                          <h4 className="font-medium">{course.name}</h4>
                          <Badge variant="secondary" className="text-xs">
                            {course.code}
                          </Badge>
                          <p className="text-sm text-neutral-600 dark:text-neutral-400">
                            {course.duration} semesters
                          </p>
                          {course.description && (
                            <p className="text-xs text-neutral-600 dark:text-neutral-400 line-clamp-2">
                              {course.description}
                            </p>
                          )}
                        </div>
                        <div className="flex items-center gap-2">
                          {selectedCourseId === course.id && (
                            <CheckCircle className="h-5 w-5 flex-shrink-0" />
                          )}
                          {isAdmin && (
                            <div onClick={(e) => e.stopPropagation()}>
                              <AdminActionButtons
                                showCreate={false}
                                onEdit={() => setCourseDialog({ open: true, course })}
                                onDelete={() => openDeleteDialog('course', course.id, course.name)}
                                isDeleting={deleteCourseMutation.isPending}
                              />
                            </div>
                          )}
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* Semester Selection */}
          {selectedCourseId && (
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <BookMarked className="h-5 w-5" />
                  <h3 className="text-lg font-semibold">Select Semester</h3>
                </div>
                {isAdmin && (
                  <AdminActionButtons
                    showCreate
                    showEdit={false}
                    showDelete={false}
                    createLabel="Add Semester"
                    onCreate={() => setSemesterDialog({ open: true, semester: null })}
                  />
                )}
              </div>
              <p className="text-sm text-neutral-600 dark:text-neutral-400">
                Choose your current semester for targeted content
              </p>

              {semestersLoading ? (
                <div className="flex items-center justify-center py-12">
                  <div className="animate-spin rounded-full h-8 w-8 border-t-2 border-b-2 border-black dark:border-white" />
                </div>
              ) : semesters.length === 0 ? (
                <div className="text-center py-12 text-neutral-600 dark:text-neutral-400">
                  No semesters available for this course
                </div>
              ) : (
                <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                  {semesters.map((semester) => (
                    <div key={semester.id} className="relative">
                      <Button
                        variant={selectedSemesterId === semester.id ? 'default' : 'outline'}
                        className="h-auto py-4 w-full"
                        onClick={() => setSelectedSemesterId(semester.id)}
                      >
                        <div className="flex flex-col items-center gap-1">
                          <span className="font-semibold">{semester.number}</span>
                          <span className="text-xs">{semester.name}</span>
                        </div>
                      </Button>
                      {isAdmin && selectedSemesterId === semester.id && (
                        <div className="absolute top-1 right-1">
                          <AdminActionButtons
                            showCreate={false}
                            onEdit={() => setSemesterDialog({ open: true, semester })}
                            onDelete={() => openDeleteDialog('semester', semester.id, semester.name, {
                              courseId: selectedCourseId!,
                              semesterNumber: semester.number,
                            })}
                            isDeleting={deleteSemesterMutation.isPending}
                          />
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* Subject List */}
          {selectedSemesterId && (
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <BookOpen className="h-5 w-5" />
                  <h3 className="text-lg font-semibold">Subjects</h3>
                </div>
                {isAdmin && (
                  <AdminActionButtons
                    showCreate
                    showEdit={false}
                    showDelete={false}
                    createLabel="Add Subject"
                    onCreate={() => setSubjectDialog({ open: true, subject: null })}
                  />
                )}
              </div>
              <p className="text-sm text-neutral-600 dark:text-neutral-400">
                Browse subjects for {selectedSemester?.name}
              </p>

              {subjectsLoading ? (
                <div className="flex items-center justify-center py-12">
                  <div className="animate-spin rounded-full h-8 w-8 border-t-2 border-b-2 border-black dark:border-white" />
                </div>
              ) : subjects.length === 0 ? (
                <div className="text-center py-12 text-neutral-600 dark:text-neutral-400">
                  No subjects available for this semester
                </div>
              ) : (
                <div className="grid grid-cols-1 gap-4">
                  {subjects.map((subject) => (
                    <Card 
                      key={subject.id} 
                      className="hover:shadow-md transition-shadow cursor-pointer hover:border-primary/50"
                      onClick={() => setDocumentsDialog({ open: true, subject })}
                    >
                      <CardHeader>
                        <div className="flex items-start justify-between">
                          <div className="flex-1">
                            <CardTitle className="text-lg">{subject.name}</CardTitle>
                            <CardDescription className="mt-1">
                              <Badge variant="outline" className="text-xs">
                                {subject.code}
                              </Badge>
                              {subject.credits && (
                                <span className="ml-2 text-xs">
                                  {subject.credits} credits
                                </span>
                              )}
                            </CardDescription>
                          </div>
                          <div className="flex items-center gap-2">
                            <Button
                              variant="ghost"
                              size="sm"
                              className="h-8 gap-1 text-muted-foreground hover:text-foreground"
                              onClick={(e) => {
                                e.stopPropagation();
                                setDocumentsDialog({ open: true, subject });
                              }}
                            >
                              <FileText className="h-4 w-4" />
                              <span className="text-xs">Documents</span>
                            </Button>
                            {isAdmin && (
                              <div onClick={(e) => e.stopPropagation()}>
                                <AdminActionButtons
                                  showCreate={false}
                                  onEdit={() => setSubjectDialog({ open: true, subject })}
                                  onDelete={() => openDeleteDialog('subject', subject.id, subject.name, {
                                    semesterId: selectedSemesterId!,
                                  })}
                                  isDeleting={deleteSubjectMutation.isPending}
                                />
                              </div>
                            )}
                          </div>
                        </div>
                      </CardHeader>
                      {subject.description && (
                        <CardContent>
                          <p className="text-sm text-neutral-600 dark:text-neutral-400">
                            {subject.description}
                          </p>
                        </CardContent>
                      )}
                    </Card>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* Current Selection Summary */}
          {selectedUniversityId && selectedCourseId && selectedSemesterId && (
            <Card className="bg-primary/5 border-primary/20">
              <CardHeader>
                <CardTitle className="text-lg">Your Selection</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="space-y-2">
                  <div className="flex items-center gap-2">
                    <GraduationCap className="h-4 w-4 text-muted-foreground" />
                    <span className="font-medium">{selectedUniversity?.name}</span>
                  </div>
                  <div className="flex items-center gap-2">
                    <BookOpen className="h-4 w-4 text-muted-foreground" />
                    <span className="font-medium">{selectedCourse?.name}</span>
                  </div>
                  <div className="flex items-center gap-2">
                    <CheckCircle className="h-4 w-4 text-muted-foreground" />
                    <span className="font-medium">{selectedSemester?.name}</span>
                  </div>
                </div>
                {hasChanges && (
                  <p className="text-sm text-muted-foreground mt-4">
                    Don't forget to save your changes!
                  </p>
                )}
              </CardContent>
            </Card>
          )}
        </div>
      </ScrollArea>

      {/* Admin Dialogs */}
      {isAdmin && (
        <>
          <UniversityFormDialog
            open={universityDialog.open}
            onOpenChange={(open) => setUniversityDialog({ open, university: null })}
            university={universityDialog.university}
          />
          <CourseFormDialog
            open={courseDialog.open}
            onOpenChange={(open) => setCourseDialog({ open, course: null })}
            course={courseDialog.course}
            universityId={selectedUniversityId || ''}
          />
          <SemesterFormDialog
            open={semesterDialog.open}
            onOpenChange={(open) => setSemesterDialog({ open, semester: null })}
            semester={semesterDialog.semester}
            courseId={selectedCourseId || ''}
          />
          <SubjectFormDialog
            open={subjectDialog.open}
            onOpenChange={(open) => setSubjectDialog({ open, subject: null })}
            subject={subjectDialog.subject}
            semesterId={selectedSemesterId || ''}
          />
          <DeleteConfirmationDialog
            open={deleteDialog.open}
            onOpenChange={(open) => !open && closeDeleteDialog()}
            onConfirm={handleConfirmDelete}
            title={deleteDialogContent.title}
            description={deleteDialogContent.description}
            cascadeWarning={deleteDialogContent.cascadeWarning}
            isDeleting={isDeleting}
          />
        </>
      )}

      {/* Subject Documents Dialog - Available to all users */}
      <SubjectDocumentsDialog
        open={documentsDialog.open}
        onOpenChange={(open) => setDocumentsDialog({ open, subject: open ? documentsDialog.subject : null })}
        subject={documentsDialog.subject}
        userId={user?.id}
        isAdmin={isAdmin}
        isAuthenticated={isAuthenticated}
      />
    </div>
  );
}
