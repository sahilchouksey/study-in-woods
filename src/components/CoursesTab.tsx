'use client';

import { useState } from 'react';
import { Plus, Upload, Globe, FileText, CheckCircle } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { ScrollArea } from '@/components/ui/scroll-area';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Badge } from '@/components/ui/badge';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';

interface Semester {
  id: string;
  number: number;
  hasSyllabus: boolean;
  hasPYQs: boolean;
  syllabusStatus?: 'uploaded' | 'processing' | 'vectorized';
  pyqStatus?: 'uploaded' | 'processing' | 'vectorized';
}

interface Course {
  id: string;
  name: string;
  semesters: Semester[];
}

export function CoursesTab() {
  const [courses, setCourses] = useState<Course[]>([
    {
      id: '1',
      name: 'MCA',
      semesters: [
        { id: '1', number: 1, hasSyllabus: true, hasPYQs: true, syllabusStatus: 'vectorized', pyqStatus: 'vectorized' },
        { id: '2', number: 3, hasSyllabus: true, hasPYQs: false, syllabusStatus: 'vectorized' },
      ],
    },
  ]);

  const [selectedCourse, setSelectedCourse] = useState<Course>(courses[0]);
  const [selectedSemester, setSelectedSemester] = useState<Semester | null>(null);
  const [isAddCourseOpen, setIsAddCourseOpen] = useState(false);
  const [isAddSemesterOpen, setIsAddSemesterOpen] = useState(false);
  const [newCourseName, setNewCourseName] = useState('');
  const [newSemesterNumber, setNewSemesterNumber] = useState('');

  const addCourse = () => {
    if (!newCourseName.trim()) return;
    
    const newCourse: Course = {
      id: Date.now().toString(),
      name: newCourseName,
      semesters: [],
    };
    
    setCourses([...courses, newCourse]);
    setNewCourseName('');
    setIsAddCourseOpen(false);
  };

  const addSemester = () => {
    if (!newSemesterNumber.trim()) return;
    
    const newSemester: Semester = {
      id: Date.now().toString(),
      number: parseInt(newSemesterNumber),
      hasSyllabus: false,
      hasPYQs: false,
    };
    
    const updatedCourses = courses.map(course =>
      course.id === selectedCourse.id
        ? { ...course, semesters: [...course.semesters, newSemester] }
        : course
    );
    
    setCourses(updatedCourses);
    setSelectedCourse({ ...selectedCourse, semesters: [...selectedCourse.semesters, newSemester] });
    setNewSemesterNumber('');
    setIsAddSemesterOpen(false);
  };

  const getStatusBadge = (status?: string) => {
    if (!status) return null;
    
    switch (status) {
      case 'uploaded':
        return <Badge variant="secondary">Uploaded</Badge>;
      case 'processing':
        return <Badge variant="outline">Processing</Badge>;
      case 'vectorized':
        return <Badge className="bg-green-100 text-green-800">Ready</Badge>;
      default:
        return null;
    }
  };

  return (
    <div className="flex flex-col h-full">
      <div className="border-b border-border p-6">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-foreground">Course Management</h2>
            <p className="text-muted-foreground mt-1">Upload syllabus and PYQs for your courses</p>
          </div>
          <Dialog open={isAddCourseOpen} onOpenChange={setIsAddCourseOpen}>
            <DialogTrigger asChild>
              <Button>
                <Plus className="h-4 w-4 mr-2" />
                Add Course
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Add New Course</DialogTitle>
                <DialogDescription>
                  Create a new course to manage syllabus and previous year questions.
                </DialogDescription>
              </DialogHeader>
              <div className="space-y-4">
                <div>
                  <Label htmlFor="courseName">Course Name</Label>
                  <Input
                    id="courseName"
                    value={newCourseName}
                    onChange={(e) => setNewCourseName(e.target.value)}
                    placeholder="e.g., MCA, BCA, B.Tech"
                  />
                </div>
                <div className="flex justify-end gap-2">
                  <Button variant="outline" onClick={() => setIsAddCourseOpen(false)}>
                    Cancel
                  </Button>
                  <Button onClick={addCourse}>Add Course</Button>
                </div>
              </div>
            </DialogContent>
          </Dialog>
        </div>
      </div>

      <div className="flex flex-1 overflow-hidden">
        {/* Course List */}
        <div className="w-64 border-r border-border bg-muted">
          <div className="p-4 border-b border-border">
            <h3 className="font-medium text-foreground">Courses</h3>
          </div>
          <ScrollArea className="h-full">
            <div className="p-2 space-y-1">
              {courses.map((course) => (
                <Button
                  key={course.id}
                  variant={selectedCourse.id === course.id ? 'default' : 'ghost'}
                  className="w-full justify-start"
                  onClick={() => setSelectedCourse(course)}
                >
                  {course.name}
                  <Badge variant="outline" className="ml-auto">
                    {course.semesters.length}
                  </Badge>
                </Button>
              ))}
            </div>
          </ScrollArea>
        </div>

        {/* Course Details */}
        <div className="flex-1 flex flex-col">
          <div className="border-b border-border p-6">
            <div className="flex items-center justify-between">
              <h3 className="text-lg font-medium text-foreground">{selectedCourse.name}</h3>
              <Dialog open={isAddSemesterOpen} onOpenChange={setIsAddSemesterOpen}>
                <DialogTrigger asChild>
                  <Button variant="outline">
                    <Plus className="h-4 w-4 mr-2" />
                    Add Semester
                  </Button>
                </DialogTrigger>
                <DialogContent>
                  <DialogHeader>
                    <DialogTitle>Add New Semester</DialogTitle>
                    <DialogDescription>
                      Add a new semester to {selectedCourse.name}.
                    </DialogDescription>
                  </DialogHeader>
                  <div className="space-y-4">
                    <div>
                      <Label htmlFor="semesterNumber">Semester Number</Label>
                      <Input
                        id="semesterNumber"
                        type="number"
                        value={newSemesterNumber}
                        onChange={(e) => setNewSemesterNumber(e.target.value)}
                        placeholder="e.g., 1, 2, 3..."
                      />
                    </div>
                    <div className="flex justify-end gap-2">
                      <Button variant="outline" onClick={() => setIsAddSemesterOpen(false)}>
                        Cancel
                      </Button>
                      <Button onClick={addSemester}>Add Semester</Button>
                    </div>
                  </div>
                </DialogContent>
              </Dialog>
            </div>
          </div>

          <ScrollArea className="flex-1">
            <div className="p-6">
              {selectedCourse.semesters.length === 0 ? (
                <div className="text-center py-12">
                  <BookOpen className="h-12 w-12 text-muted-foreground/50 mx-auto mb-4" />
                  <h4 className="text-lg font-medium text-muted-foreground mb-2">No semesters added</h4>
                  <p className="text-muted-foreground/70 mb-4">Start by adding a semester to this course.</p>
                  <Button onClick={() => setIsAddSemesterOpen(true)}>
                    <Plus className="h-4 w-4 mr-2" />
                    Add First Semester
                  </Button>
                </div>
              ) : (
                <div className="grid gap-6">
                  {selectedCourse.semesters
                    .sort((a, b) => a.number - b.number)
                    .map((semester) => (
                      <div key={semester.id} className="border border-border rounded-lg p-6">
                        <div className="flex items-center justify-between mb-4">
                          <h4 className="text-lg font-medium text-foreground">
                            Semester {semester.number}
                          </h4>
                          <div className="flex gap-2">
                            {semester.hasSyllabus && getStatusBadge(semester.syllabusStatus)}
                            {semester.hasPYQs && getStatusBadge(semester.pyqStatus)}
                          </div>
                        </div>

                        <Tabs defaultValue="syllabus" className="w-full">
                          <TabsList className="grid w-full grid-cols-2">
                            <TabsTrigger value="syllabus">Syllabus</TabsTrigger>
                            <TabsTrigger value="pyqs">Previous Year Questions</TabsTrigger>
                          </TabsList>

                          <TabsContent value="syllabus" className="space-y-4">
                            {semester.hasSyllabus ? (
                              <div className="flex items-center gap-3 p-4 bg-green-50 rounded-lg">
                                <CheckCircle className="h-5 w-5 text-green-600" />
                                <div className="flex-1">
                                  <p className="font-medium text-green-800">Syllabus uploaded</p>
                                  <p className="text-sm text-green-600">Ready for AI assistant</p>
                                </div>
                                {getStatusBadge(semester.syllabusStatus)}
                              </div>
                            ) : (
                              <div className="space-y-4">
                                <div className="border-2 border-dashed border-border rounded-lg p-8 text-center">
                                  <Upload className="h-8 w-8 text-muted-foreground mx-auto mb-3" />
                                  <p className="text-muted-foreground mb-2">Upload syllabus document</p>
                                  <p className="text-sm text-muted-foreground/70 mb-4">PDF, DOC, or DOCX files</p>
                                  <Button>Choose File</Button>
                                </div>
                                <div className="text-center">
                                  <p className="text-black/40 text-sm mb-2">Or import from web</p>
                                  <Button variant="outline" className="w-full">
                                    <Globe className="h-4 w-4 mr-2" />
                                    Import from URL
                                  </Button>
                                </div>
                              </div>
                            )}
                          </TabsContent>

                          <TabsContent value="pyqs" className="space-y-4">
                            {semester.hasPYQs ? (
                              <div className="flex items-center gap-3 p-4 bg-green-50 rounded-lg">
                                <CheckCircle className="h-5 w-5 text-green-600" />
                                <div className="flex-1">
                                  <p className="font-medium text-green-800">PYQs uploaded</p>
                                  <p className="text-sm text-green-600">Ready for AI assistant</p>
                                </div>
                                {getStatusBadge(semester.pyqStatus)}
                              </div>
                            ) : (
                              <div className="space-y-4">
                                <div className="border-2 border-dashed border-black/20 rounded-lg p-8 text-center">
                                  <FileText className="h-8 w-8 text-black/40 mx-auto mb-3" />
                                  <p className="text-black/60 mb-2">Upload previous year questions</p>
                                  <p className="text-sm text-black/40 mb-4">PDF, DOC, or DOCX files</p>
                                  <Button>Choose Files</Button>
                                </div>
                                <div className="text-center">
                                  <p className="text-black/40 text-sm mb-2">Or import from web</p>
                                  <Button variant="outline" className="w-full">
                                    <Globe className="h-4 w-4 mr-2" />
                                    Import from URL
                                  </Button>
                                </div>
                              </div>
                            )}
                          </TabsContent>
                        </Tabs>
                      </div>
                    ))}
                </div>
              )}
            </div>
          </ScrollArea>
        </div>
      </div>
    </div>
  );
}