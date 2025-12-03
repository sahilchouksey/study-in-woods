import { apiClient } from './client';
import type { ApiResponse } from '@/types/api';

/**
 * User type
 */
export interface User {
  id: string;
  name: string;
  email: string;
  role: 'user' | 'admin';
  university_id?: string;
  course_id?: string;
  semester?: string;
  phone?: string;
  created_at: string;
  updated_at: string;
}

/**
 * Auth response types
 */
export interface LoginResponse {
  user: User;
  access_token: string;
  refresh_token: string;
}

export interface RegisterResponse {
  user: User;
  access_token: string;
  refresh_token: string;
}

export interface RefreshTokenResponse {
  access_token: string;
  refresh_token: string;
}

/**
 * Auth request types
 */
export interface LoginRequest {
  email: string;
  password: string;
  rememberMe?: boolean;
}

export interface RegisterRequest {
  name: string;
  email: string;
  password: string;
}

export interface ForgotPasswordRequest {
  email: string;
}

export interface ResetPasswordRequest {
  token: string;
  password: string;
}

export interface ChangePasswordRequest {
  currentPassword: string;
  newPassword: string;
}

/**
 * Auth Service
 */
export const authService = {
  /**
   * Login user
   */
  async login(data: LoginRequest): Promise<LoginResponse> {
    const response = await apiClient.post<ApiResponse<LoginResponse>>(
      '/api/v1/auth/login',
      data
    );
    
    // Store tokens
    if (response.data.data) {
      this.setTokens(
        response.data.data.access_token,
        response.data.data.refresh_token
      );
    }
    
    return response.data.data!;
  },

  /**
   * Register new user
   */
  async register(data: RegisterRequest): Promise<RegisterResponse> {
    const response = await apiClient.post<ApiResponse<RegisterResponse>>(
      '/api/v1/auth/register',
      data
    );
    
    // Store tokens
    if (response.data.data) {
      this.setTokens(
        response.data.data.access_token,
        response.data.data.refresh_token
      );
    }
    
    return response.data.data!;
  },

  /**
   * Logout user
   */
  async logout(): Promise<void> {
    try {
      await apiClient.post('/api/v1/auth/logout');
    } finally {
      this.clearTokens();
    }
  },

  /**
   * Get current user profile
   */
  async getProfile(): Promise<User> {
    const response = await apiClient.get<ApiResponse<User>>('/api/v1/profile');
    return response.data.data!;
  },

  /**
   * Update user profile
   */
  async updateProfile(data: Partial<User>): Promise<User> {
    const response = await apiClient.put<ApiResponse<User>>(
      '/api/v1/profile',
      data
    );
    return response.data.data!;
  },

  /**
   * Refresh access token
   */
  async refreshToken(): Promise<RefreshTokenResponse> {
    const refreshToken = this.getRefreshToken();
    
    if (!refreshToken) {
      throw new Error('No refresh token available');
    }

    const response = await apiClient.post<ApiResponse<RefreshTokenResponse>>(
      '/api/v1/auth/refresh',
      { refresh_token: refreshToken }
    );
    
    if (response.data.data) {
      this.setTokens(
        response.data.data.access_token,
        response.data.data.refresh_token
      );
    }
    
    return response.data.data!;
  },

  /**
   * Request password reset
   */
  async forgotPassword(data: ForgotPasswordRequest): Promise<void> {
    await apiClient.post<ApiResponse<{ message: string }>>(
      '/api/v1/auth/forgot-password',
      data
    );
  },

  /**
   * Reset password with token
   */
  async resetPassword(data: ResetPasswordRequest): Promise<void> {
    await apiClient.post<ApiResponse<{ message: string }>>(
      '/api/v1/auth/reset-password',
      data
    );
  },

  /**
   * Change password (authenticated)
   */
  async changePassword(data: ChangePasswordRequest): Promise<void> {
    await apiClient.post<ApiResponse<{ message: string }>>(
      '/api/v1/auth/change-password',
      {
        current_password: data.currentPassword,
        new_password: data.newPassword,
      }
    );
  },

  /**
   * Token management
   */
  setTokens(accessToken: string, refreshToken: string): void {
    if (typeof window !== 'undefined') {
      localStorage.setItem('access_token', accessToken);
      localStorage.setItem('refresh_token', refreshToken);
    }
  },

  getAccessToken(): string | null {
    if (typeof window === 'undefined') return null;
    return localStorage.getItem('access_token');
  },

  getRefreshToken(): string | null {
    if (typeof window === 'undefined') return null;
    return localStorage.getItem('refresh_token');
  },

  clearTokens(): void {
    if (typeof window !== 'undefined') {
      localStorage.removeItem('access_token');
      localStorage.removeItem('refresh_token');
    }
  },

  /**
   * Check if user is authenticated
   */
  isAuthenticated(): boolean {
    return !!this.getAccessToken();
  },
};
