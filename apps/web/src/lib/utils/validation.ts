import { z } from 'zod';

/**
 * Common validation schemas and utilities
 */

// Email validation
export const emailSchema = z.string().email('Invalid email address');

// Password validation (min 8 chars, 1 uppercase, 1 lowercase, 1 number, 1 special char)
export const passwordSchema = z
  .string()
  .min(8, 'Password must be at least 8 characters')
  .regex(/[A-Z]/, 'Password must contain at least one uppercase letter')
  .regex(/[a-z]/, 'Password must contain at least one lowercase letter')
  .regex(/[0-9]/, 'Password must contain at least one number')
  .regex(/[^A-Za-z0-9]/, 'Password must contain at least one special character');

// UUID validation
export const uuidSchema = z.string().uuid('Invalid UUID format');

// Name validation
export const nameSchema = z
  .string()
  .min(2, 'Name must be at least 2 characters')
  .max(100, 'Name must not exceed 100 characters')
  .regex(/^[a-zA-Z\s'-]+$/, 'Name can only contain letters, spaces, hyphens, and apostrophes');

// Phone validation (international format)
export const phoneSchema = z
  .string()
  .regex(/^\+?[1-9]\d{1,14}$/, 'Invalid phone number format');

// URL validation
export const urlSchema = z.string().url('Invalid URL format');

// API Key validation
export const apiKeySchema = z
  .string()
  .min(32, 'API key must be at least 32 characters')
  .max(256, 'API key must not exceed 256 characters');

// Date validation
export const dateSchema = z.coerce.date();

// Pagination validation
export const paginationSchema = z.object({
  page: z.coerce.number().int().positive().default(1),
  limit: z.coerce.number().int().positive().max(100).default(10),
});

// Search query validation
export const searchQuerySchema = z
  .string()
  .min(1, 'Search query cannot be empty')
  .max(500, 'Search query too long');

// Semester validation
export const semesterSchema = z.enum([
  '1st Semester',
  '2nd Semester',
  '3rd Semester',
  '4th Semester',
  '5th Semester',
  '6th Semester',
  '7th Semester',
  '8th Semester',
]);

// Document type validation
export const documentTypeSchema = z.enum([
  'lecture_notes',
  'assignment',
  'previous_year_paper',
  'book',
  'other',
]);

/**
 * Common form validation schemas
 */

// Login form
export const loginSchema = z.object({
  email: emailSchema,
  password: z.string().min(1, 'Password is required'),
  rememberMe: z.boolean().optional(),
});

// Register form
export const registerSchema = z.object({
  name: nameSchema,
  email: emailSchema,
  password: passwordSchema,
  confirmPassword: z.string(),
}).refine((data) => data.password === data.confirmPassword, {
  message: "Passwords don't match",
  path: ['confirmPassword'],
});

// Profile update form
export const profileUpdateSchema = z.object({
  name: nameSchema.optional(),
  email: emailSchema.optional(),
  phone: phoneSchema.optional(),
  university_id: uuidSchema.optional(),
  course_id: uuidSchema.optional(),
  semester: semesterSchema.optional(),
});

// Password reset form
export const passwordResetSchema = z.object({
  email: emailSchema,
});

// New password form
export const newPasswordSchema = z.object({
  token: z.string().min(1, 'Reset token is required'),
  password: passwordSchema,
  confirmPassword: z.string(),
}).refine((data) => data.password === data.confirmPassword, {
  message: "Passwords don't match",
  path: ['confirmPassword'],
});

// Change password form
export const changePasswordSchema = z.object({
  currentPassword: z.string().min(1, 'Current password is required'),
  newPassword: passwordSchema,
  confirmNewPassword: z.string(),
}).refine((data) => data.newPassword === data.confirmNewPassword, {
  message: "Passwords don't match",
  path: ['confirmNewPassword'],
});

// API Key creation form
export const apiKeyCreateSchema = z.object({
  name: z.string().min(3, 'Name must be at least 3 characters').max(100),
  description: z.string().max(500).optional(),
  rate_limit: z.coerce.number().int().positive().max(10000).optional(),
  expires_at: dateSchema.optional(),
});

// Chat message form
export const chatMessageSchema = z.object({
  message: searchQuerySchema,
  session_id: uuidSchema.optional(),
  context: z.object({
    university_id: uuidSchema.optional(),
    course_id: uuidSchema.optional(),
    semester: semesterSchema.optional(),
    subject_name: z.string().optional(),
  }).optional(),
});

// File schema that works in both browser and SSR environments
// On server, File doesn't exist, so we use a custom validation
const fileSchema = z.custom<File>(
  (val) => {
    // During SSR, File global doesn't exist - accept any value as this schema
    // is only used for client-side form validation
    if (typeof File === 'undefined') {
      return true;
    }
    return val instanceof File;
  },
  { message: 'File is required' }
);

// Document upload form
export const documentUploadSchema = z.object({
  title: z.string().min(3, 'Title must be at least 3 characters').max(200),
  description: z.string().max(1000).optional(),
  subject_id: uuidSchema,
  document_type: documentTypeSchema,
  file: fileSchema
    .refine((file) => {
      // Skip validation during SSR
      if (typeof File === 'undefined') return true;
      return file.size <= 10 * 1024 * 1024;
    }, 'File size must be less than 10MB')
    .refine(
      (file) => {
        // Skip validation during SSR
        if (typeof File === 'undefined') return true;
        return ['application/pdf', 'application/msword', 'application/vnd.openxmlformats-officedocument.wordprocessingml.document', 'text/plain'].includes(file.type);
      },
      'Only PDF, DOC, DOCX, and TXT files are allowed'
    ),
});

// University/Course/Subject creation (admin)
export const universityCreateSchema = z.object({
  name: z.string().min(3, 'Name must be at least 3 characters').max(200),
  location: z.string().min(3, 'Location must be at least 3 characters').max(200),
  description: z.string().max(1000).optional(),
});

export const courseCreateSchema = z.object({
  university_id: uuidSchema,
  name: z.string().min(2, 'Name must be at least 2 characters').max(200),
  code: z.string().min(2, 'Code must be at least 2 characters').max(50),
  description: z.string().max(1000).optional(),
});

export const subjectCreateSchema = z.object({
  course_id: uuidSchema,
  name: z.string().min(2, 'Name must be at least 2 characters').max(200),
  code: z.string().min(2, 'Code must be at least 2 characters').max(50),
  semester: semesterSchema,
  description: z.string().max(1000).optional(),
});

/**
 * Type inference helpers
 */
export type LoginInput = z.infer<typeof loginSchema>;
export type RegisterInput = z.infer<typeof registerSchema>;
export type ProfileUpdateInput = z.infer<typeof profileUpdateSchema>;
export type PasswordResetInput = z.infer<typeof passwordResetSchema>;
export type NewPasswordInput = z.infer<typeof newPasswordSchema>;
export type ChangePasswordInput = z.infer<typeof changePasswordSchema>;
export type ApiKeyCreateInput = z.infer<typeof apiKeyCreateSchema>;
export type ChatMessageInput = z.infer<typeof chatMessageSchema>;
export type DocumentUploadInput = z.infer<typeof documentUploadSchema>;
export type UniversityCreateInput = z.infer<typeof universityCreateSchema>;
export type CourseCreateInput = z.infer<typeof courseCreateSchema>;
export type SubjectCreateInput = z.infer<typeof subjectCreateSchema>;
export type PaginationInput = z.infer<typeof paginationSchema>;

/**
 * Validation helper function
 */
export function validateData<T>(schema: z.Schema<T>, data: unknown): { success: true; data: T } | { success: false; errors: z.ZodError } {
  try {
    const validatedData = schema.parse(data);
    return { success: true, data: validatedData };
  } catch (error) {
    if (error instanceof z.ZodError) {
      return { success: false, errors: error };
    }
    throw error;
  }
}

/**
 * Format Zod errors for display
 */
export function formatZodErrors(error: z.ZodError): Record<string, string> {
  const formattedErrors: Record<string, string> = {};
  
  error.issues.forEach((err) => {
    const path = err.path.join('.');
    formattedErrors[path] = err.message;
  });
  
  return formattedErrors;
}
