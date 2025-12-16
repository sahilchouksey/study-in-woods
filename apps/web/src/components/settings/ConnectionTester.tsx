'use client';

import * as React from 'react';
import { CheckCircle, XCircle, Loader2, Wifi, WifiOff } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { ApiProvider, ApiKeyTestResult } from '@/types/api-keys';
import { testApiKey, PROVIDER_INFO } from '@/lib/api-keys';

interface ConnectionTesterProps {
  provider: ApiProvider;
  apiKey: string;
  disabled?: boolean;
  onTestComplete?: (result: ApiKeyTestResult) => void;
}

export function ConnectionTester({
  provider,
  apiKey,
  disabled = false,
  onTestComplete,
}: ConnectionTesterProps) {
  const [isLoading, setIsLoading] = React.useState(false);
  const [lastResult, setLastResult] = React.useState<ApiKeyTestResult | null>(null);

  const handleTest = async () => {
    if (!apiKey.trim() || disabled) return;

    setIsLoading(true);
    try {
      const result = await testApiKey(provider, apiKey);
      setLastResult(result);
      onTestComplete?.(result);
    } catch (error) {
      const errorResult: ApiKeyTestResult = {
        success: false,
        message: error instanceof Error ? error.message : 'Test failed',
      };
      setLastResult(errorResult);
      onTestComplete?.(errorResult);
    } finally {
      setIsLoading(false);
    }
  };

  const getStatusIcon = () => {
    if (isLoading) {
      return <Loader2 className="h-4 w-4 animate-spin" />;
    }
    
    if (!lastResult) {
      return <Wifi className="h-4 w-4" />;
    }
    
    return lastResult.success ? (
      <CheckCircle className="h-4 w-4 text-green-600" />
    ) : (
      <XCircle className="h-4 w-4 text-destructive" />
    );
  };

  const getButtonVariant = (): "default" | "destructive" | "outline" | "secondary" | "ghost" | "link" => {
    if (!lastResult) return 'outline';
    return lastResult.success ? 'outline' : 'outline';
  };

  const getButtonText = () => {
    if (isLoading) return 'Testing...';
    if (!lastResult) return 'Test Connection';
    return lastResult.success ? 'Test Again' : 'Retry Test';
  };

  const providerInfo = PROVIDER_INFO[provider];

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-3">
        <Button
          variant={getButtonVariant()}
          size="sm"
          onClick={handleTest}
          disabled={!apiKey.trim() || disabled || isLoading}
          className="min-w-[120px]"
        >
          {getStatusIcon()}
          {getButtonText()}
        </Button>
        
        {lastResult && (
          <div className="flex-1">
            {lastResult.success ? (
              <div className="flex items-center gap-2">
                <Badge variant="outline" className="bg-green-50 text-green-700 border-green-200">
                  <CheckCircle className="h-3 w-3 mr-1" />
                  Connected
                </Badge>
              </div>
            ) : (
              <div className="flex items-center gap-2">
                <Badge variant="outline" className="bg-red-50 text-red-700 border-red-200">
                  <WifiOff className="h-3 w-3 mr-1" />
                  Failed
                </Badge>
              </div>
            )}
          </div>
        )}
      </div>

      {lastResult && (
        <div className="text-sm">
          {lastResult.success ? (
            <div className="space-y-2">
              <p className="text-green-600">{lastResult.message}</p>
              {lastResult.capabilities && (
                <div className="space-y-1">
                  <p className="text-muted-foreground">Available capabilities:</p>
                  <div className="flex flex-wrap gap-1">
                    {lastResult.capabilities.map((capability) => (
                      <Badge key={capability} variant="secondary" className="text-xs">
                        {capability}
                      </Badge>
                    ))}
                  </div>
                </div>
              )}
            </div>
          ) : (
            <p className="text-destructive">{lastResult.message}</p>
          )}
        </div>
      )}

      <div className="text-xs text-muted-foreground">
        <p className="font-medium">{providerInfo.name} - {providerInfo.description}</p>
        <p>Capabilities: {providerInfo.capabilities.join(', ')}</p>
      </div>
    </div>
  );
}