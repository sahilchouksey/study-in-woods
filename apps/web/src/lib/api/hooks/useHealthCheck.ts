import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@/lib/api/client';

interface HealthCheckResponse {
  status: string;
}

/**
 * Fetch health check status
 */
async function fetchHealthCheck(): Promise<HealthCheckResponse> {
  const response = await apiClient.get<HealthCheckResponse>('/ping');
  return response.data;
}

/**
 * Health check hook
 */
export function useHealthCheck(enabled: boolean = true) {
  return useQuery({
    queryKey: ['health'],
    queryFn: fetchHealthCheck,
    enabled,
    retry: 2,
    staleTime: 30000, // 30 seconds
  });
}
