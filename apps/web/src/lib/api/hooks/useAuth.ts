import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { authService, type LoginRequest, type RegisterRequest, type User } from '@/lib/api/auth';
import { showErrorToast, showSuccessToast } from '@/lib/utils/errors';
import { useRouter } from 'next/navigation';

/**
 * Hook for user login
 */
export function useLogin() {
  const router = useRouter();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: LoginRequest) => authService.login(data),
    onSuccess: (data) => {
      // Update user in cache
      queryClient.setQueryData(['user'], data.user);
      showSuccessToast('Welcome back!', 'Login Successful');
      router.push('/dashboard');
    },
    onError: (error) => {
      showErrorToast(error, 'Login Failed');
    },
  });
}

/**
 * Hook for user registration
 */
export function useRegister() {
  const router = useRouter();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: RegisterRequest) => authService.register(data),
    onSuccess: (data) => {
      // Update user in cache
      queryClient.setQueryData(['user'], data.user);
      showSuccessToast('Account created successfully!', 'Registration Successful');
      router.push('/dashboard');
    },
    onError: (error) => {
      showErrorToast(error, 'Registration Failed');
    },
  });
}

/**
 * Hook for user logout
 */
export function useLogout() {
  const router = useRouter();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: () => authService.logout(),
    onSuccess: () => {
      // Clear all cached data
      queryClient.clear();
      showSuccessToast('You have been logged out', 'Logout Successful');
      router.push('/login');
    },
    onError: (error) => {
      // Still clear tokens and redirect even if API call fails
      authService.clearTokens();
      queryClient.clear();
      showErrorToast(error, 'Logout Error');
      router.push('/login');
    },
  });
}

/**
 * Hook to get current user
 */
export function useUser() {
  return useQuery({
    queryKey: ['user'],
    queryFn: () => authService.getProfile(),
    enabled: authService.isAuthenticated(),
    retry: false,
    staleTime: 5 * 60 * 1000, // 5 minutes
  });
}

/**
 * Hook to update user profile
 */
export function useUpdateProfile() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: Partial<User>) => authService.updateProfile(data),
    onSuccess: (data) => {
      // Update user in cache
      queryClient.setQueryData(['user'], data);
      showSuccessToast('Profile updated successfully');
    },
    onError: (error) => {
      showErrorToast(error, 'Profile Update Failed');
    },
  });
}

/**
 * Hook for forgot password
 */
export function useForgotPassword() {
  return useMutation({
    mutationFn: (email: string) => authService.forgotPassword({ email }),
    onSuccess: () => {
      showSuccessToast(
        'Password reset instructions have been sent to your email',
        'Email Sent'
      );
    },
    onError: (error) => {
      showErrorToast(error, 'Request Failed');
    },
  });
}

/**
 * Hook for reset password
 */
export function useResetPassword() {
  const router = useRouter();

  return useMutation({
    mutationFn: (data: { token: string; password: string }) =>
      authService.resetPassword(data),
    onSuccess: () => {
      showSuccessToast(
        'Your password has been reset. Please login with your new password.',
        'Password Reset Successful'
      );
      router.push('/login');
    },
    onError: (error) => {
      showErrorToast(error, 'Password Reset Failed');
    },
  });
}

/**
 * Hook for change password (authenticated)
 */
export function useChangePassword() {
  return useMutation({
    mutationFn: (data: { currentPassword: string; newPassword: string }) =>
      authService.changePassword(data),
    onSuccess: () => {
      showSuccessToast('Password changed successfully');
    },
    onError: (error) => {
      showErrorToast(error, 'Password Change Failed');
    },
  });
}
