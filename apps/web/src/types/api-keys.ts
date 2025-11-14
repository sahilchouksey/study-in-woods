export type ApiProvider = 'tavily' | 'exa' | 'firecrawl';

export interface ApiKeyConfig {
  provider: ApiProvider;
  apiKey: string;
  isActive: boolean;
  lastTested?: Date;
  isValid?: boolean;
  capabilities: string[];
}

export interface SearchPreferences {
  maxResults: number;
  searchType: 'general' | 'news' | 'academic';
  language?: string;
}

export interface ScrapingPreferences {
  formats: ('markdown' | 'html' | 'text')[];
  maxAge: number;
  onlyMainContent: boolean;
  timeout: number;
}

export interface WebCapabilitiesConfig {
  search: {
    defaultProvider: 'tavily' | 'exa';
    tavilyConfig?: ApiKeyConfig;
    exaConfig?: ApiKeyConfig;
    preferences: SearchPreferences;
  };
  scraping: {
    firecrawlConfig?: ApiKeyConfig;
    preferences: ScrapingPreferences;
  };
}

export interface ApiKeyTestResult {
  success: boolean;
  message: string;
  capabilities?: string[];
}

export interface ProviderInfo {
  name: string;
  description: string;
  capabilities: string[];
  testEndpoint: string;
  keyPattern?: RegExp;
}