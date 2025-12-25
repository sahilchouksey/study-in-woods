'use client';

import { useState, useMemo, useEffect } from 'react';
import { useQueryState, parseAsString, parseAsInteger } from 'nuqs';
import { useQueryClient } from '@tanstack/react-query';
import { BookOpen, GraduationCap, MapPin, CheckCircle, BookMarked, FileText, Search, ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight, Star } from 'lucide-react';
import { LoadingSpinner, InlineSpinner } from '@/components/ui/loading-spinner';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { useUniversities, useCoursesByUniversity, useSemesters, useSubjects, useToggleSubjectStar } from '@/lib/api/hooks/useCourses';
import { useAuth } from '@/providers/auth-provider';
import { useUpdateProfile } from '@/lib/api/hooks/useAuth';
import { useDeleteUniversity, useDeleteCourse, useDeleteSemester, useDeleteSubject, useDeleteAllSubjects } from '@/lib/api/hooks/useAdminMutations';
import { AdminActionButtons } from '@/components/admin/AdminActionButtons';
import { UniversityFormDialog } from '@/components/admin/UniversityFormDialog';
import { CourseFormDialog } from '@/components/admin/CourseFormDialog';
import { SemesterFormDialog } from '@/components/admin/SemesterFormDialog';
import { SubjectFormDialog } from '@/components/admin/SubjectFormDialog';
import { DeleteConfirmationDialog } from '@/components/admin/DeleteConfirmationDialog';
import { SubjectDocumentsDialog, SemesterSyllabusUploadDialog } from '@/components/documents';
import { CoursesBreadcrumb } from '@/components/CoursesBreadcrumb';
import { toast } from 'sonner';
import type { University, Course, Semester, Subject } from '@/lib/api/courses';

export function CoursesTab() {
  const queryClient = useQueryClient();
  const { user, isAdmin, isAuthenticated } = useAuth();
  const { data: universities = [], isLoading: universitiesLoading } = useUniversities();
  
  // Use nuqs for URL state management - enables shareable URLs and persistence
  const [selectedUniversityId, setSelectedUniversityId] = useQueryState('university', parseAsString);
  
  const { data: courses = [], isLoading: coursesLoading } = useCoursesByUniversity(
    selectedUniversityId || user?.university_id || null
  );
  const updateProfileMutation = useUpdateProfile();

  const [selectedCourseId, setSelectedCourseId] = useQueryState('course', parseAsString);
  
  const { data: semesters = [], isLoading: semestersLoading } = useSemesters(
    selectedCourseId || user?.course_id || null
  );
  
  const [selectedSemesterId, setSelectedSemesterId] = useQueryState('semester', parseAsString);
  
  // Subject search and pagination - URL-persisted state
  const [subjectSearchQuery, setSubjectSearchQuery] = useQueryState('search', parseAsString);
  const [subjectPage, setSubjectPage] = useQueryState('subjectPage', parseAsInteger.withDefault(1));
  
  // Debounced search value for API calls
  const [debouncedSearch, setDebouncedSearch] = useState(subjectSearchQuery || '');
  
  // Debounce search input
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearch(subjectSearchQuery || '');
    }, 300);
    return () => clearTimeout(timer);
  }, [subjectSearchQuery]);
  
  // Reset to page 1 when search changes
  useEffect(() => {
    if (subjectSearchQuery !== null) {
      setSubjectPage(1);
    }
  }, [subjectSearchQuery, setSubjectPage]);
  
  const { data: subjectsData, isLoading: subjectsLoading } = useSubjects(
    selectedSemesterId,
    {
      page: subjectPage,
      per_page: 10,
      search: debouncedSearch || undefined,
    }
  );
  
  const subjects = subjectsData?.data || [];
  const subjectsPagination = subjectsData?.pagination;

  // Admin mutations
  const deleteUniversityMutation = useDeleteUniversity();
  const deleteCourseMutation = useDeleteCourse();
  const deleteSemesterMutation = useDeleteSemester();
  const deleteSubjectMutation = useDeleteSubject();
  const deleteAllSubjectsMutation = useDeleteAllSubjects();
  const toggleSubjectStarMutation = useToggleSubjectStar();

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

  // Semester upload dialog state
  const [semesterUploadDialog, setSemesterUploadDialog] = useState<{ open: boolean; semester: Semester | null }>({
    open: false,
    semester: null,
  });

  // URL state for extraction reconnection (survives page refresh)
  const [urlExtracting] = useQueryState('extracting', parseAsString);

  // Delete confirmation dialog states
  const [deleteDialog, setDeleteDialog] = useState<{
    open: boolean;
    type: 'university' | 'course' | 'semester' | 'subject' | 'all-subjects' | null;
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

  // Search/filter states (university and course remain client-side)
  const [universitySearch, setUniversitySearch] = useState('');
  const [courseSearch, setCourseSearch] = useState('');

  const selectedUniversity = universities.find((u) => u.id === selectedUniversityId);
  const selectedCourse = courses.find((c) => c.id === selectedCourseId);
  const selectedSemester = semesters.find((s) => s.id === selectedSemesterId);

  // Auto-open syllabus upload dialog if URL has extracting=true (reconnection after page refresh)
  useEffect(() => {
    if (urlExtracting === 'true' && !semesterUploadDialog.open) {
      // Create a minimal semester object for the dialog if we have selectedSemester
      const semester: Semester | null = selectedSemester || (selectedSemesterId ? {
        id: selectedSemesterId,
        course_id: selectedCourseId || '',
        name: `Semester`,
        number: 1,
        created_at: '',
        updated_at: '',
      } : null);
      
      setSemesterUploadDialog({ open: true, semester });
    }
  }, [urlExtracting, selectedSemester, selectedSemesterId, selectedCourseId, semesterUploadDialog.open]);

  // Filtered lists based on search
  const filteredUniversities = useMemo(() => {
    if (!universitySearch) return universities;
    const search = universitySearch.toLowerCase();
    return universities.filter(
      (u) =>
        u.name.toLowerCase().includes(search) ||
        u.code.toLowerCase().includes(search) ||
        u.location.toLowerCase().includes(search)
    );
  }, [universities, universitySearch]);

  const filteredCourses = useMemo(() => {
    if (!courseSearch) return courses;
    const search = courseSearch.toLowerCase();
    return courses.filter(
      (c) =>
        c.name.toLowerCase().includes(search) ||
        c.code.toLowerCase().includes(search) ||
        c.description?.toLowerCase().includes(search)
    );
  }, [courses, courseSearch]);

  // Subjects are now filtered server-side via API params

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
    type: 'university' | 'course' | 'semester' | 'subject' | 'all-subjects',
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

        case 'all-subjects':
          if (deleteDialog.semesterId) {
            const result = await deleteAllSubjectsMutation.mutateAsync(deleteDialog.semesterId);
            if (result.failed_count > 0) {
              toast.warning(`Deleted ${result.deleted_count} subjects, ${result.failed_count} failed`);
            } else {
              toast.success(`All ${result.deleted_count} subjects deleted successfully`);
            }
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
      case 'all-subjects':
        return {
          title: 'Delete All Subjects',
          description: `Are you sure you want to delete all subjects in "${deleteDialog.name}"?`,
          cascadeWarning: 'This will permanently delete ALL subjects and ALL documents in this semester. This action will clean up AI agents and knowledge bases for each subject.',
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
    deleteSubjectMutation.isPending ||
    deleteAllSubjectsMutation.isPending;

  return (
    <div className="h-full flex flex-col min-h-0">
      {/* Header */}
      <div className="border-b border-neutral-200 dark:border-neutral-800 p-6">
        <div className="flex items-center justify-between">
          <div className="flex-1">
            <h2 className="text-2xl font-semibold">My Courses</h2>
            <p className="text-sm text-neutral-600 dark:text-neutral-400 mt-1">
              Select your university, course, and semester for personalized learning
            </p>
            {/* Breadcrumbs */}
            {(selectedUniversity || selectedCourse || selectedSemester) && (
              <div className="mt-3">
                <CoursesBreadcrumb
                  university={selectedUniversity}
                  course={selectedCourse}
                  semester={selectedSemester}
                  onUniversityClick={() => {
                    setSelectedCourseId(null);
                    setSelectedSemesterId(null);
                  }}
                  onCourseClick={() => {
                    setSelectedSemesterId(null);
                  }}
                />
              </div>
            )}
          </div>
          {isAuthenticated && hasChanges && (
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

            {/* Search Input */}
            {universities.length > 3 && (
              <div className="relative">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                <Input
                  placeholder="Search universities..."
                  value={universitySearch}
                  onChange={(e) => setUniversitySearch(e.target.value)}
                  className="pl-9"
                />
              </div>
            )}
            
            {universitiesLoading ? (
              <LoadingSpinner size="lg" centered withPadding />
            ) : (
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {filteredUniversities.length === 0 ? (
                  <div className="col-span-2 text-center py-8 text-muted-foreground">
                    No universities found matching "{universitySearch}"
                  </div>
                ) : (
                  filteredUniversities.map((university) => (
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
                ))
                )}
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
                  {coursesLoading && <InlineSpinner />}
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
                  : coursesLoading 
                  ? 'Loading university...'
                  : 'Choose your course program'}
              </p>

              {/* Search Input */}
              {courses.length > 3 && (
                <div className="relative">
                  <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                  <Input
                    placeholder="Search courses..."
                    value={courseSearch}
                    onChange={(e) => setCourseSearch(e.target.value)}
                    className="pl-9"
                  />
                </div>
              )}
              
              {coursesLoading ? (
                <LoadingSpinner size="lg" centered withPadding />
              ) : courses.length === 0 ? (
                <div className="text-center py-12 text-neutral-600 dark:text-neutral-400">
                  No courses available for this university
                </div>
              ) : (
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  {filteredCourses.length === 0 ? (
                    <div className="col-span-2 text-center py-8 text-muted-foreground">
                      No courses found matching "{courseSearch}"
                    </div>
                  ) : (
                    filteredCourses.map((course) => (
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
                  ))
                  )}
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
                  {semestersLoading && <InlineSpinner />}
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
                {semestersLoading ? 'Loading semesters...' : 'Choose your current semester for targeted content'}
              </p>

              {semestersLoading ? (
                <LoadingSpinner size="lg" centered withPadding />
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
                  {subjectsLoading && <InlineSpinner />}
                </div>
                <div className="flex items-center gap-2">
                  {isAdmin && (subjectsPagination?.total ?? subjects.length) > 0 && selectedSemesterId && (
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => {
                        const semester: Semester | null = selectedSemester || (selectedSemesterId ? {
                          id: selectedSemesterId,
                          course_id: selectedCourseId || '',
                          name: `Semester ${selectedSemesterId}`,
                          number: parseInt(selectedSemesterId) || 1,
                          created_at: '',
                          updated_at: '',
                        } : null);
                        setSemesterUploadDialog({ open: true, semester });
                      }}
                    >
                      <FileText className="mr-2 h-4 w-4" />
                      Upload Syllabus
                    </Button>
                  )}
                  {isAdmin && (
                    <>
                      {(subjectsPagination?.total ?? subjects.length) > 0 && (
                        <Button
                          variant="destructive"
                          size="sm"
                          onClick={() => 
                            openDeleteDialog(
                              'all-subjects',
                              selectedSemesterId!,
                              selectedSemester?.name || 'this semester',
                              { semesterId: selectedSemesterId! }
                            )
                          }
                          disabled={deleteAllSubjectsMutation.isPending}
                        >
                          {deleteAllSubjectsMutation.isPending ? 'Deleting...' : 'Delete All'}
                        </Button>
                      )}
                      <AdminActionButtons
                        showCreate
                        showEdit={false}
                        showDelete={false}
                        createLabel="Add Subject"
                        onCreate={() => setSubjectDialog({ open: true, subject: null })}
                      />
                    </>
                  )}
                </div>
              </div>
              <p className="text-sm text-neutral-600 dark:text-neutral-400">
                {subjectsLoading ? 'Loading subjects...' : `Browse subjects for ${selectedSemester?.name || 'this semester'}`}
              </p>

              {/* Search Input - Always show when there's a semester selected */}
              <div className="relative">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                <Input
                  placeholder="Search subjects by name or code..."
                  value={subjectSearchQuery || ''}
                  onChange={(e) => setSubjectSearchQuery(e.target.value || null)}
                  className="pl-9"
                />
                {subjectSearchQuery && (
                  <button
                    onClick={() => setSubjectSearchQuery(null)}
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                  >
                    <span className="sr-only">Clear search</span>
                    ×
                  </button>
                )}
              </div>

              {subjectsLoading ? (
                <LoadingSpinner size="lg" centered withPadding />
              ) : subjects.length === 0 && !subjectSearchQuery ? (
                <div className="text-center py-12">
                  <BookOpen className="h-12 w-12 mx-auto mb-3 text-muted-foreground opacity-50" />
                  <p className="text-neutral-600 dark:text-neutral-400 font-medium mb-2">
                    No subjects available for this semester
                  </p>
                  <p className="text-sm text-muted-foreground mb-4">
                    {isAdmin 
                      ? 'Upload a syllabus to auto-create subjects, or add subjects manually'
                      : 'No subjects available yet. Please contact an admin to add subjects.'}
                  </p>
                  {isAdmin && (
                    <div className="flex items-center justify-center gap-2">
                      <Button
                        variant="default"
                        onClick={() => {
                          const semester: Semester | null = selectedSemester || (selectedSemesterId ? {
                            id: selectedSemesterId,
                            course_id: selectedCourseId || '',
                            name: `Semester ${selectedSemesterId}`,
                            number: parseInt(selectedSemesterId) || 1,
                            created_at: '',
                            updated_at: '',
                          } : null);
                          setSemesterUploadDialog({ open: true, semester });
                        }}
                      >
                        <FileText className="mr-2 h-4 w-4" />
                        Upload Syllabus
                      </Button>
                      <Button
                        onClick={() => setSubjectDialog({ open: true, subject: null })}
                      >
                        <BookOpen className="mr-2 h-4 w-4" />
                        Add Subject
                      </Button>
                    </div>
                  )}
                </div>
              ) : subjects.length === 0 && subjectSearchQuery ? (
                <div className="text-center py-8 text-muted-foreground">
                  No subjects found matching "{subjectSearchQuery}"
                </div>
              ) : (
                <>
                  <div className="grid grid-cols-1 gap-4">
                    {subjects.map((subject) => (
                      <Card 
                        key={subject.id} 
                        className="hover:shadow-md transition-shadow cursor-pointer hover:border-primary/50"
                        onClick={() => setDocumentsDialog({ open: true, subject })}
                      >
                        <CardHeader>
                          <div className="flex items-start justify-between">
                            <div className="flex-1 flex items-start gap-2">
                              {/* Star indicator for all users */}
                              {subject.is_starred && !isAdmin && (
                                <Star className="h-5 w-5 text-yellow-500 fill-yellow-500 flex-shrink-0 mt-0.5" />
                              )}
                              {/* Star toggle button for admin */}
                              {isAdmin && (
                                <Button
                                  variant="ghost"
                                  size="icon"
                                  className={`h-7 w-7 flex-shrink-0 ${
                                    subject.is_starred 
                                      ? 'text-yellow-500 hover:text-yellow-600' 
                                      : 'text-muted-foreground hover:text-yellow-500'
                                  }`}
                                  onClick={(e) => {
                                    e.stopPropagation();
                                    toggleSubjectStarMutation.mutate({
                                      semesterId: selectedSemesterId!,
                                      subjectId: subject.id,
                                      isStarred: !subject.is_starred,
                                    });
                                  }}
                                  disabled={toggleSubjectStarMutation.isPending}
                                  title={subject.is_starred ? 'Remove from starred' : 'Add to starred'}
                                >
                                  <Star className={`h-4 w-4 ${subject.is_starred ? 'fill-current' : ''}`} />
                                </Button>
                              )}
                              <div>
                                <CardTitle className="text-lg">{subject.name}</CardTitle>
                                <CardDescription className="mt-1">
                                  {subject.code && (
                                    <Badge variant="outline" className="text-xs">
                                      {subject.code}
                                    </Badge>
                                  )}
                                </CardDescription>
                              </div>
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

                  {/* Pagination */}
                  {subjectsPagination && subjectsPagination.total_pages > 1 && (
                    <div className="flex items-center justify-between mt-6 pt-4 border-t">
                      <div className="text-sm text-muted-foreground">
                        Showing {((subjectsPagination.current_page - 1) * subjectsPagination.per_page) + 1} to{' '}
                        {Math.min(subjectsPagination.current_page * subjectsPagination.per_page, subjectsPagination.total)} of{' '}
                        {subjectsPagination.total} subjects
                      </div>
                      <div className="flex items-center gap-1">
                        <Button
                          variant="outline"
                          size="icon"
                          className="h-8 w-8"
                          onClick={() => setSubjectPage(1)}
                          disabled={subjectsPagination.current_page === 1}
                        >
                          <ChevronsLeft className="h-4 w-4" />
                          <span className="sr-only">First page</span>
                        </Button>
                        <Button
                          variant="outline"
                          size="icon"
                          className="h-8 w-8"
                          onClick={() => setSubjectPage(Math.max(1, subjectPage - 1))}
                          disabled={subjectsPagination.current_page === 1}
                        >
                          <ChevronLeft className="h-4 w-4" />
                          <span className="sr-only">Previous page</span>
                        </Button>
                        <div className="flex items-center gap-1 mx-2">
                          {Array.from({ length: Math.min(5, subjectsPagination.total_pages) }, (_, i) => {
                            let pageNum: number;
                            const totalPages = subjectsPagination.total_pages;
                            const currentPage = subjectsPagination.current_page;
                            
                            if (totalPages <= 5) {
                              pageNum = i + 1;
                            } else if (currentPage <= 3) {
                              pageNum = i + 1;
                            } else if (currentPage >= totalPages - 2) {
                              pageNum = totalPages - 4 + i;
                            } else {
                              pageNum = currentPage - 2 + i;
                            }
                            
                            return (
                              <Button
                                key={pageNum}
                                variant={pageNum === currentPage ? 'default' : 'outline'}
                                size="icon"
                                className="h-8 w-8"
                                onClick={() => setSubjectPage(pageNum)}
                              >
                                {pageNum}
                              </Button>
                            );
                          })}
                        </div>
                        <Button
                          variant="outline"
                          size="icon"
                          className="h-8 w-8"
                          onClick={() => setSubjectPage(Math.min(subjectsPagination.total_pages, subjectPage + 1))}
                          disabled={subjectsPagination.current_page === subjectsPagination.total_pages}
                        >
                          <ChevronRight className="h-4 w-4" />
                          <span className="sr-only">Next page</span>
                        </Button>
                        <Button
                          variant="outline"
                          size="icon"
                          className="h-8 w-8"
                          onClick={() => setSubjectPage(subjectsPagination.total_pages)}
                          disabled={subjectsPagination.current_page === subjectsPagination.total_pages}
                        >
                          <ChevronsRight className="h-4 w-4" />
                          <span className="sr-only">Last page</span>
                        </Button>
                      </div>
                    </div>
                  )}
                </>
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
                {isAuthenticated && hasChanges && (
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

      {/* Semester Syllabus Upload Dialog - For uploading syllabus to auto-create subjects */}
      <SemesterSyllabusUploadDialog
        open={semesterUploadDialog.open}
        onOpenChange={(open: boolean) => setSemesterUploadDialog({ open, semester: open ? semesterUploadDialog.semester : null })}
        semester={semesterUploadDialog.semester}
        isAuthenticated={isAuthenticated}
        semesterId={selectedSemesterId || undefined}
        onSubjectsCreated={() => {
          // Invalidate subjects query to refetch the updated list
          if (selectedSemesterId) {
            queryClient.invalidateQueries({ queryKey: ['subjects', selectedSemesterId] });
          }
        }}
      />
    </div>
  );
}
