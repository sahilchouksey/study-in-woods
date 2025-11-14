'use client';

import * as React from 'react';
import { Globe, Search, Download, Upload, Shield, Lock, Eye } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Separator } from '@/components/ui/separator';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Label } from '@/components/ui/label';
import { Badge } from '@/components/ui/badge';
import { ApiKeyInput } from '@/components/ui/api-key-input';
import { ConnectionTester } from './ConnectionTester';
import { ApiProvider, WebCapabilitiesConfig, ApiKeyTestResult } from '@/types/api-keys';
import { 
  saveApiKey, 
  deleteApiKey, 
  getApiKey, 
  getWebCapabilitiesConfig, 
  saveWebCapabilitiesConfig,
  clearAllApiKeys,
  exportConfiguration,
  importConfiguration,
  PROVIDER_INFO 
} from '@/lib/api-keys';

export function WebCapabilitiesSection() {
  const [config, setConfig] = React.useState<WebCapabilitiesConfig | null>(null);
  const [apiKeys, setApiKeys] = React.useState<Record<ApiProvider, string>>({
    tavily: '',
    exa: '',
    firecrawl: '',
  });
  const [validationStates, setValidationStates] = React.useState<Record<ApiProvider, {
    isValid?: boolean;
    error?: string;
    lastTested?: Date;
  }>>({
    tavily: {},
    exa: {},
    firecrawl: {},
  });
  const [isLoading, setIsLoading] = React.useState(true);

  // Load configuration and API keys on mount
  React.useEffect(() => {
    loadConfiguration();
  }, []);

  const loadConfiguration = async () => {
    try {
      setIsLoading(true);
      const loadedConfig = await getWebCapabilitiesConfig();
      setConfig(loadedConfig);

      // Load existing API keys
      const loadedKeys: Record<ApiProvider, string> = {
        tavily: '',
        exa: '',
        firecrawl: '',
      };

      for (const provider of ['tavily', 'exa', 'firecrawl'] as ApiProvider[]) {
        const key = await getApiKey(provider);
        if (key) {
          loadedKeys[provider] = key;
          
          // Load validation state from config
          let apiKeyConfig;
          if (provider === 'tavily' || provider === 'exa') {
            apiKeyConfig = loadedConfig.search[`${provider}Config`];
          } else if (provider === 'firecrawl') {
            apiKeyConfig = loadedConfig.scraping.firecrawlConfig;
          }

          if (apiKeyConfig) {
            setValidationStates(prev => ({
              ...prev,
              [provider]: {
                isValid: apiKeyConfig.isValid,
                lastTested: apiKeyConfig.lastTested,
              }
            }));
          }
        }
      }

      setApiKeys(loadedKeys);
    } catch (error) {
      console.error('Failed to load configuration:', error);
    } finally {
      setIsLoading(false);
    }
  };

  const handleApiKeyChange = (provider: ApiProvider, value: string) => {
    setApiKeys(prev => ({ ...prev, [provider]: value }));
    
    // Clear validation state when key changes
    setValidationStates(prev => ({
      ...prev,
      [provider]: {
        isValid: undefined,
        error: undefined,
      }
    }));
  };

  const handleSaveApiKey = async (provider: ApiProvider) => {
    const apiKey = apiKeys[provider];
    if (!apiKey.trim()) return;

    try {
      await saveApiKey(provider, apiKey);
      // Reload config to reflect changes
      await loadConfiguration();
    } catch (error) {
      setValidationStates(prev => ({
        ...prev,
        [provider]: {
          ...prev[provider],
          error: error instanceof Error ? error.message : 'Failed to save API key'
        }
      }));
    }
  };

  const handleDeleteApiKey = async (provider: ApiProvider) => {
    try {
      await deleteApiKey(provider);
      setApiKeys(prev => ({ ...prev, [provider]: '' }));
      setValidationStates(prev => ({
        ...prev,
        [provider]: {}
      }));
      await loadConfiguration();
    } catch (error) {
      console.error('Failed to delete API key:', error);
    }
  };

  const handleTestComplete = (provider: ApiProvider, result: ApiKeyTestResult) => {
    setValidationStates(prev => ({
      ...prev,
      [provider]: {
        isValid: result.success,
        error: result.success ? undefined : result.message,
        lastTested: new Date(),
      }
    }));
  };

  const handleExportConfig = async () => {
    try {
      const exportData = await exportConfiguration();
      const blob = new Blob([exportData], { type: 'application/json' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `study-woods-config-${new Date().toISOString().split('T')[0]}.json`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
    } catch (error) {
      console.error('Export failed:', error);
    }
  };

  const handleImportConfig = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) return;

    const reader = new FileReader();
    reader.onload = async (e) => {
      try {
        const configData = e.target?.result as string;
        await importConfiguration(configData);
        await loadConfiguration();
      } catch (error) {
        console.error('Import failed:', error);
        // You might want to show a toast notification here
      }
    };
    reader.readAsText(file);
  };

  const handleClearAll = async () => {
    if (confirm('Are you sure you want to clear all API keys and settings? This cannot be undone.')) {
      try {
        await clearAllApiKeys();
        setApiKeys({ tavily: '', exa: '', firecrawl: '' });
        setValidationStates({ tavily: {}, exa: {}, firecrawl: {} });
        await loadConfiguration();
      } catch (error) {
        console.error('Failed to clear all data:', error);
      }
    }
  };

  if (isLoading || !config) {
    return <div className="flex items-center justify-center p-8">Loading...</div>;
  }

  return (
    <div className="space-y-6">
      {/* Security Message */}
      <div className="bg-blue-50 dark:bg-blue-950/30 border border-blue-200 dark:border-blue-800 rounded-lg p-4">
        <div className="flex items-start gap-3">
          <Shield className="h-5 w-5 text-blue-600 dark:text-blue-400 mt-0.5 flex-shrink-0" />
          <div className="space-y-2">
            <h4 className="font-medium text-blue-900 dark:text-blue-100">
              🔒 Your Keys Are Safe With Us
            </h4>
            <div className="text-sm text-blue-800 dark:text-blue-200 space-y-1">
              <p className="flex items-center gap-2">
                <Lock className="h-4 w-4" />
                All your data is encrypted and can only be decrypted by you
              </p>
              <p className="flex items-center gap-2">
                <Eye className="h-4 w-4" />
                No one else, including us, can access your API keys
              </p>
              <p className="flex items-center gap-2">
                <Shield className="h-4 w-4" />
                Keys are stored locally on your device using military-grade encryption
              </p>
            </div>
          </div>
        </div>
      </div>

      <Tabs defaultValue="search" className="w-full">
        <TabsList className="grid w-full grid-cols-3">
          <TabsTrigger value="search">Search Providers</TabsTrigger>
          <TabsTrigger value="scraping">Scraping Provider</TabsTrigger>
          <TabsTrigger value="settings">Settings</TabsTrigger>
        </TabsList>

        {/* Search Providers Tab */}
        <TabsContent value="search" className="space-y-6">
          <div className="space-y-4">
            <div className="flex items-center gap-2">
              <Search className="h-5 w-5 text-muted-foreground" />
              <h3 className="text-lg font-medium">Web Search APIs</h3>
            </div>
            <p className="text-sm text-muted-foreground">
              Configure API keys for web search providers to enable real-time information retrieval.
            </p>
          </div>

          {/* Tavily Configuration */}
          <div className="border rounded-lg p-4 space-y-4">
            <div className="flex items-center justify-between">
              <div>
                <h4 className="font-medium">Tavily API</h4>
                <p className="text-sm text-muted-foreground">Real-time web search and news discovery</p>
              </div>
              <Badge variant="outline">Search Engine</Badge>
            </div>

            <ApiKeyInput
              label="Tavily API Key"
              value={apiKeys.tavily}
              onChange={(value) => handleApiKeyChange('tavily', value)}
              placeholder="tvly-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
              error={validationStates.tavily.error}
              isValid={validationStates.tavily.isValid}
              showValidation={!!apiKeys.tavily}
            />

            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => handleSaveApiKey('tavily')}
                disabled={!apiKeys.tavily.trim()}
              >
                Save Key
              </Button>
              {apiKeys.tavily && (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => handleDeleteApiKey('tavily')}
                >
                  Delete Key
                </Button>
              )}
            </div>

            {apiKeys.tavily && (
              <ConnectionTester
                provider="tavily"
                apiKey={apiKeys.tavily}
                onTestComplete={(result) => handleTestComplete('tavily', result)}
              />
            )}
          </div>

          {/* Exa Configuration */}
          <div className="border rounded-lg p-4 space-y-4">
            <div className="flex items-center justify-between">
              <div>
                <h4 className="font-medium">Exa API</h4>
                <p className="text-sm text-muted-foreground">Semantic search and content discovery</p>
              </div>
              <Badge variant="outline">Semantic Search</Badge>
            </div>

            <ApiKeyInput
              label="Exa API Key"
              value={apiKeys.exa}
              onChange={(value) => handleApiKeyChange('exa', value)}
              placeholder="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
              error={validationStates.exa.error}
              isValid={validationStates.exa.isValid}
              showValidation={!!apiKeys.exa}
            />

            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => handleSaveApiKey('exa')}
                disabled={!apiKeys.exa.trim()}
              >
                Save Key
              </Button>
              {apiKeys.exa && (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => handleDeleteApiKey('exa')}
                >
                  Delete Key
                </Button>
              )}
            </div>

            {apiKeys.exa && (
              <ConnectionTester
                provider="exa"
                apiKey={apiKeys.exa}
                onTestComplete={(result) => handleTestComplete('exa', result)}
              />
            )}
          </div>
        </TabsContent>

        {/* Scraping Provider Tab */}
        <TabsContent value="scraping" className="space-y-6">
          <div className="space-y-4">
            <div className="flex items-center gap-2">
              <Globe className="h-5 w-5 text-muted-foreground" />
              <h3 className="text-lg font-medium">Web Scraping API</h3>
            </div>
            <p className="text-sm text-muted-foreground">
              Configure API key for web scraping, crawling, and content extraction.
            </p>
          </div>

          {/* Firecrawl Configuration */}
          <div className="border rounded-lg p-4 space-y-4">
            <div className="flex items-center justify-between">
              <div>
                <h4 className="font-medium">Firecrawl API</h4>
                <p className="text-sm text-muted-foreground">Web scraping, crawling, and content extraction</p>
              </div>
              <Badge variant="outline">Web Scraper</Badge>
            </div>

            <ApiKeyInput
              label="Firecrawl API Key"
              value={apiKeys.firecrawl}
              onChange={(value) => handleApiKeyChange('firecrawl', value)}
              placeholder="fc-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
              error={validationStates.firecrawl.error}
              isValid={validationStates.firecrawl.isValid}
              showValidation={!!apiKeys.firecrawl}
            />

            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => handleSaveApiKey('firecrawl')}
                disabled={!apiKeys.firecrawl.trim()}
              >
                Save Key
              </Button>
              {apiKeys.firecrawl && (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => handleDeleteApiKey('firecrawl')}
                >
                  Delete Key
                </Button>
              )}
            </div>

            {apiKeys.firecrawl && (
              <ConnectionTester
                provider="firecrawl"
                apiKey={apiKeys.firecrawl}
                onTestComplete={(result) => handleTestComplete('firecrawl', result)}
              />
            )}
          </div>
        </TabsContent>

        {/* Settings Tab */}
        <TabsContent value="settings" className="space-y-6">
          <div className="space-y-4">
            <h3 className="text-lg font-medium">Preferences & Data Management</h3>
          </div>

          {/* Search Preferences */}
          <div className="border rounded-lg p-4 space-y-4">
            <h4 className="font-medium">Search Preferences</h4>
            
            <div className="space-y-3">
              <div className="space-y-2">
                <Label htmlFor="defaultProvider">Default Search Provider</Label>
                <Select value={config.search.defaultProvider}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="tavily">Tavily (Real-time)</SelectItem>
                    <SelectItem value="exa">Exa (Semantic)</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
          </div>

          {/* Data Management */}
          <div className="border rounded-lg p-4 space-y-4">
            <h4 className="font-medium">Data Management</h4>
            
            <div className="space-y-3">
              <div className="flex items-center gap-3">
                <Button variant="outline" onClick={handleExportConfig}>
                  <Download className="h-4 w-4 mr-2" />
                  Export Configuration
                </Button>
                
                <div>
                  <input
                    type="file"
                    accept=".json"
                    onChange={handleImportConfig}
                    className="hidden"
                    id="import-config"
                  />
                  <Button variant="outline" asChild>
                    <label htmlFor="import-config" className="cursor-pointer">
                      <Upload className="h-4 w-4 mr-2" />
                      Import Configuration
                    </label>
                  </Button>
                </div>
              </div>
              
              <Separator />
              
              <div className="space-y-2">
                <p className="text-sm text-muted-foreground">
                  Danger Zone: This will permanently delete all stored API keys and settings.
                </p>
                <Button variant="destructive" onClick={handleClearAll}>
                  Clear All Data
                </Button>
              </div>
            </div>
          </div>
        </TabsContent>
      </Tabs>
    </div>
  );
}