import axios, { AxiosInstance, AxiosError, InternalAxiosRequestConfig } from 'axios';

const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

/**
 * Create axios instance with base configuration
 */
export const apiClient: AxiosInstance = axios.create({
  baseURL: API_URL,
  timeout: 30000, // 30 seconds
  headers: {
    'Content-Type': 'application/json',
  },
});

/**
 * Request interceptor - Add auth token if available
 */
apiClient.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    // Get access token from localStorage
    const accessToken = localStorage.getItem('access_token');
    
    if (accessToken && config.headers) {
      config.headers.Authorization = `Bearer ${accessToken}`;
    }
    
    return config;
  },
  (error: AxiosError) => {
    return Promise.reject(error);
  }
);

/**
 * Response interceptor - Handle errors globally
 */
apiClient.interceptors.response.use(
  (response) => {
    return response;
  },
  (error: AxiosError) => {
    // Handle different error types
    if (error.response) {
      // Server responded with error status
      const status = error.response.status;
      const data = error.response.data as any;

      switch (status) {
        case 401:
          // Unauthorized - clear tokens and redirect to login
          localStorage.removeItem('access_token');
          localStorage.removeItem('refresh_token');
          
          // Only redirect if not already on auth pages
          if (typeof window !== 'undefined' && 
              !window.location.pathname.startsWith('/login') &&
              !window.location.pathname.startsWith('/register')) {
            window.location.href = '/login';
          }
          break;
        case 403:
          console.error('Forbidden access');
          break;
        case 404:
          console.error('Resource not found');
          break;
        case 500:
          console.error('Internal server error');
          break;
        default:
          console.error(`Error ${status}:`, data?.message || error.message);
      }

      // Return structured error
      return Promise.reject({
        status,
        message: data?.message || error.message,
        data: data,
      });
    } else if (error.request) {
      // Request made but no response
      console.error('Network error - no response from server');
      return Promise.reject({
        status: 0,
        message: 'Network error. Please check your connection.',
      });
    } else {
      // Something else happened
      console.error('Request error:', error.message);
      return Promise.reject({
        status: -1,
        message: error.message,
      });
    }
  }
);

export default apiClient;
