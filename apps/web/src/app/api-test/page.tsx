
'use client';

import { useHealthCheck } from '@/lib/api/hooks/useHealthCheck';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';

export default function ApiTestPage() {
  const { data, isLoading, isError, error, refetch } = useHealthCheck();

  return (
    <div className="flex items-center justify-center min-h-screen bg-background p-8">
      <div className="w-full max-w-2xl space-y-6">
        <div className="text-center space-y-2">
          <h1 className="text-4xl font-bold">API Connection Test</h1>
          <p className="text-muted-foreground">
            Testing connection to backend API
          </p>
        </div>

        <div className="border rounded-lg p-6 space-y-4 bg-card">
          <div className="flex items-center justify-between">
            <h2 className="text-xl font-semibold">Health Check</h2>
            <Button onClick={() => refetch()} disabled={isLoading}>
              {isLoading ? 'Checking...' : 'Refresh'}
            </Button>
          </div>

          {isLoading && (
            <div className="text-center py-8">
              <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-solid border-current border-r-transparent motion-reduce:animate-[spin_1.5s_linear_infinite]" />
              <p className="mt-4 text-muted-foreground">Loading...</p>
            </div>
          )}

          {isError && (
            <div className="space-y-3">
              <Badge variant="destructive" className="text-sm">
                Connection Failed
              </Badge>
              <div className="bg-destructive/10 border border-destructive/20 rounded-lg p-4">
                <p className="text-sm text-destructive font-medium">
                  Error: {error instanceof Error ? error.message : 'Unknown error'}
                </p>
              </div>
              <div className="text-sm space-y-2 text-muted-foreground">
                <p><strong>Possible causes:</strong></p>
                <ul className="list-disc list-inside space-y-1 ml-2">
                  <li>Backend server is not running</li>
                  <li>API URL is incorrect ({process.env.NEXT_PUBLIC_API_URL})</li>
                  <li>Network connectivity issues</li>
                  <li>CORS configuration issues</li>
                </ul>
                <p className="mt-4"><strong>To fix:</strong></p>
                <ul className="list-disc list-inside space-y-1 ml-2">
                  <li>Make sure backend is running: <code className="bg-muted px-2 py-1 rounded">cd apps/api && make dev</code></li>
                  <li>Check .env.local has correct API_URL</li>
                  <li>Verify Docker containers are running</li>
                </ul>
              </div>
            </div>
          )}

          {data && !isError && (
            <div className="space-y-3">
              <Badge variant="default" className="text-sm bg-green-600">
                Connected Successfully
              </Badge>
              
              <div className="border rounded-lg p-4 space-y-1">
                <p className="text-xs text-muted-foreground uppercase font-medium">Status</p>
                <p className="text-lg font-semibold">{data.status}</p>
              </div>

              <div className="bg-green-50 dark:bg-green-950/20 border border-green-200 dark:border-green-900 rounded-lg p-4">
                <p className="text-sm text-green-800 dark:text-green-200">
                  ✅ All systems operational. Frontend can communicate with backend.
                </p>
              </div>
            </div>
          )}
        </div>

        <div className="text-center text-sm text-muted-foreground">
          <p>API URL: <code className="bg-muted px-2 py-1 rounded">{process.env.NEXT_PUBLIC_API_URL}</code></p>
        </div>
      </div>
    </div>
  );
}
