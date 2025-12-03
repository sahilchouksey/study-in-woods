import { AxiosError } from 'axios';
import { toast } from 'sonner';

/**
 * API Error response structure
 */
export interface ApiErrorResponse {
  error: string;
  message: string;
  statusCode?: number;
  details?: Record<string, unknown>;
}

/**
 * Custom Application Error class
 */
export class AppError extends Error {
  public readonly statusCode: number;
  public readonly isOperational: boolean;
  public readonly details?: Record<string, unknown>;

  constructor(
    message: string,
    statusCode: number = 500,
    isOperational: boolean = true,
    details?: Record<string, unknown>
  ) {
    super(message);
    this.statusCode = statusCode;
    this.isOperational = isOperational;
    this.details = details;

    Object.setPrototypeOf(this, AppError.prototype);
    Error.captureStackTrace(this, this.constructor);
  }
}

/**
 * Parse error from various sources
 */
export function parseError(error: unknown): ApiErrorResponse {
  // Axios error
  if (error instanceof AxiosError) {
    const response = error.response;
    
    if (response?.data) {
      return {
        error: response.data.error || 'API Error',
        message: response.data.message || error.message || 'An unexpected error occurred',
        statusCode: response.status,
        details: response.data.details,
      };
    }

    // Network error (no response)
    if (error.request) {
      return {
        error: 'Network Error',
        message: 'Unable to connect to the server. Please check your internet connection.',
        statusCode: 0,
      };
    }

    // Request setup error
    return {
      error: 'Request Error',
      message: error.message || 'Failed to make the request',
      statusCode: 0,
    };
  }

  // AppError
  if (error instanceof AppError) {
    return {
      error: 'Application Error',
      message: error.message,
      statusCode: error.statusCode,
      details: error.details,
    };
  }

  // Standard Error
  if (error instanceof Error) {
    return {
      error: 'Error',
      message: error.message,
      statusCode: 500,
    };
  }

  // Unknown error type
  return {
    error: 'Unknown Error',
    message: 'An unexpected error occurred',
    statusCode: 500,
  };
}

/**
 * Get user-friendly error message based on status code
 */
export function getErrorMessage(error: unknown): string {
  const parsed = parseError(error);

  switch (parsed.statusCode) {
    case 400:
      return parsed.message || 'Invalid request. Please check your input.';
    case 401:
      return 'You are not authenticated. Please login.';
    case 403:
      return 'You do not have permission to perform this action.';
    case 404:
      return 'The requested resource was not found.';
    case 409:
      return parsed.message || 'A conflict occurred. The resource may already exist.';
    case 422:
      return parsed.message || 'Validation failed. Please check your input.';
    case 429:
      return 'Too many requests. Please try again later.';
    case 500:
      return 'Internal server error. Please try again later.';
    case 503:
      return 'Service temporarily unavailable. Please try again later.';
    case 0:
      return parsed.message; // Network/Request errors already have good messages
    default:
      return parsed.message || 'An unexpected error occurred. Please try again.';
  }
}

/**
 * Error toast notification helper
 */
export function showErrorToast(error: unknown, title?: string): void {
  const message = getErrorMessage(error);
  toast.error(title || 'Error', {
    description: message,
    duration: 5000,
  });
}

/**
 * Success toast notification helper
 */
export function showSuccessToast(message: string, title?: string): void {
  toast.success(title || 'Success', {
    description: message,
    duration: 3000,
  });
}

/**
 * Info toast notification helper
 */
export function showInfoToast(message: string, title?: string): void {
  toast.info(title || 'Info', {
    description: message,
    duration: 3000,
  });
}

/**
 * Warning toast notification helper
 */
export function showWarningToast(message: string, title?: string): void {
  toast.warning(title || 'Warning', {
    description: message,
    duration: 4000,
  });
}

/**
 * Check if error is authentication error
 */
export function isAuthError(error: unknown): boolean {
  const parsed = parseError(error);
  return parsed.statusCode === 401;
}

/**
 * Check if error is permission error
 */
export function isPermissionError(error: unknown): boolean {
  const parsed = parseError(error);
  return parsed.statusCode === 403;
}

/**
 * Check if error is validation error
 */
export function isValidationError(error: unknown): boolean {
  const parsed = parseError(error);
  return parsed.statusCode === 422 || parsed.statusCode === 400;
}

/**
 * Check if error is network error
 */
export function isNetworkError(error: unknown): boolean {
  const parsed = parseError(error);
  return parsed.statusCode === 0;
}

/**
 * Global error handler for unhandled errors
 */
export function handleGlobalError(error: unknown): void {
  console.error('Global error:', error);
  
  // Don't show toast for auth errors (let components handle redirect)
  if (isAuthError(error)) {
    return;
  }

  showErrorToast(error);
}

/**
 * Log error to monitoring service (placeholder for future integration)
 */
export function logErrorToService(error: unknown, context?: Record<string, unknown>): void {
  // TODO: Integrate with error monitoring service (e.g., Sentry)
  console.error('Error logged:', {
    error: parseError(error),
    context,
    timestamp: new Date().toISOString(),
  });
}

/**
 * Retry function with exponential backoff
 */
export async function retryWithBackoff<T>(
  fn: () => Promise<T>,
  maxRetries: number = 3,
  initialDelay: number = 1000
): Promise<T> {
  let lastError: unknown;

  for (let i = 0; i < maxRetries; i++) {
    try {
      return await fn();
    } catch (error) {
      lastError = error;
      
      // Don't retry auth/permission errors
      if (isAuthError(error) || isPermissionError(error)) {
        throw error;
      }

      // Last attempt failed
      if (i === maxRetries - 1) {
        break;
      }

      // Wait with exponential backoff
      const delay = initialDelay * Math.pow(2, i);
      await new Promise((resolve) => setTimeout(resolve, delay));
    }
  }

  throw lastError;
}
