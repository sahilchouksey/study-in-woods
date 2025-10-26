import { ApiProvider, ApiKeyConfig, WebCapabilitiesConfig, ApiKeyTestResult, ProviderInfo } from '@/types/api-keys';
import { encryptData, decryptData } from './crypto';

const STORAGE_KEYS = {
  WEB_CAPABILITIES: 'study_woods_web_capabilities',
  ENCRYPTION_VERSION: 'study_woods_encryption_version',
} as const;

const CURRENT_VERSION = '1.0';

// Provider information and test endpoints
export const PROVIDER_INFO: Record<ApiProvider, ProviderInfo> = {
  tavily: {
    name: 'Tavily',
    description: 'Real-time web search and news discovery',
    capabilities: ['Web Search', 'News Search', 'Real-time Data'],
    testEndpoint: 'https://api.tavily.com/search',
    keyPattern: /^tvly-[a-zA-Z0-9]{32,}$/,
  },
  exa: {
    name: 'Exa',
    description: 'Semantic search and content discovery',
    capabilities: ['Semantic Search', 'Content Discovery', 'Research'],
    testEndpoint: 'https://api.exa.ai/search',
    keyPattern: /^[a-zA-Z0-9]{40,}$/,
  },
  firecrawl: {
    name: 'Firecrawl',
    description: 'Web scraping, crawling, and content extraction',
    capabilities: ['Web Scraping', 'Site Crawling', 'Content Extraction', 'Site Mapping'],
    testEndpoint: 'https://api.firecrawl.dev/v1/scrape',
    keyPattern: /^fc-[a-zA-Z0-9]{32,}$/,
  },
};

/**
 * Get default web capabilities configuration
 */
const getDefaultConfig = (): WebCapabilitiesConfig => ({
  search: {
    defaultProvider: 'tavily',
    preferences: {
      maxResults: 10,
      searchType: 'general',
      language: 'en',
    },
  },
  scraping: {
    preferences: {
      formats: ['markdown'],
      maxAge: 172800000, // 48 hours
      onlyMainContent: true,
      timeout: 30000, // 30 seconds
    },
  },
});

/**
 * Save API key securely in encrypted storage
 */
export const saveApiKey = async (provider: ApiProvider, apiKey: string): Promise<void> => {
  try {
    const config = await getWebCapabilitiesConfig();
    const encryptedKey = encryptData(apiKey);
    
    const apiKeyConfig: ApiKeyConfig = {
      provider,
      apiKey: encryptedKey,
      isActive: true,
      lastTested: new Date(),
      isValid: false, // Will be validated separately
      capabilities: PROVIDER_INFO[provider].capabilities,
    };

    if (provider === 'tavily' || provider === 'exa') {
      config.search[`${provider}Config`] = apiKeyConfig;
    } else if (provider === 'firecrawl') {
      config.scraping.firecrawlConfig = apiKeyConfig;
    }

    const encryptedConfig = encryptData(JSON.stringify(config));
    localStorage.setItem(STORAGE_KEYS.WEB_CAPABILITIES, encryptedConfig);
    localStorage.setItem(STORAGE_KEYS.ENCRYPTION_VERSION, CURRENT_VERSION);
  } catch (error) {
    console.error('Failed to save API key:', error);
    throw new Error('Failed to save API key securely');
  }
};

/**
 * Get decrypted API key for a provider
 */
export const getApiKey = async (provider: ApiProvider): Promise<string | null> => {
  try {
    const config = await getWebCapabilitiesConfig();
    let apiKeyConfig: ApiKeyConfig | undefined;

    if (provider === 'tavily' || provider === 'exa') {
      apiKeyConfig = config.search[`${provider}Config`];
    } else if (provider === 'firecrawl') {
      apiKeyConfig = config.scraping.firecrawlConfig;
    }

    if (!apiKeyConfig?.apiKey) {
      return null;
    }

    return decryptData(apiKeyConfig.apiKey);
  } catch (error) {
    console.error('Failed to retrieve API key:', error);
    return null;
  }
};

/**
 * Delete API key for a provider
 */
export const deleteApiKey = async (provider: ApiProvider): Promise<void> => {
  try {
    const config = await getWebCapabilitiesConfig();

    if (provider === 'tavily' || provider === 'exa') {
      delete config.search[`${provider}Config`];
    } else if (provider === 'firecrawl') {
      delete config.scraping.firecrawlConfig;
    }

    const encryptedConfig = encryptData(JSON.stringify(config));
    localStorage.setItem(STORAGE_KEYS.WEB_CAPABILITIES, encryptedConfig);
  } catch (error) {
    console.error('Failed to delete API key:', error);
    throw new Error('Failed to delete API key');
  }
};

/**
 * Get the complete web capabilities configuration
 */
export const getWebCapabilitiesConfig = async (): Promise<WebCapabilitiesConfig> => {
  try {
    const encryptedConfig = localStorage.getItem(STORAGE_KEYS.WEB_CAPABILITIES);
    
    if (!encryptedConfig) {
      return getDefaultConfig();
    }

    const decryptedConfig = decryptData(encryptedConfig);
    return JSON.parse(decryptedConfig);
  } catch (error) {
    console.error('Failed to load configuration, using defaults:', error);
    return getDefaultConfig();
  }
};

/**
 * Save web capabilities configuration
 */
export const saveWebCapabilitiesConfig = async (config: WebCapabilitiesConfig): Promise<void> => {
  try {
    const encryptedConfig = encryptData(JSON.stringify(config));
    localStorage.setItem(STORAGE_KEYS.WEB_CAPABILITIES, encryptedConfig);
  } catch (error) {
    console.error('Failed to save configuration:', error);
    throw new Error('Failed to save configuration');
  }
};

/**
 * Test API key connection for a provider
 */
export const testApiKey = async (provider: ApiProvider, apiKey: string): Promise<ApiKeyTestResult> => {
  const providerInfo = PROVIDER_INFO[provider];
  
  try {
    // Validate key format first
    if (providerInfo.keyPattern && !providerInfo.keyPattern.test(apiKey)) {
      return {
        success: false,
        message: `Invalid ${providerInfo.name} API key format`,
      };
    }

    // Test the actual API connection
    const result = await testProviderConnection(provider, apiKey);
    
    if (result.success) {
      // Update the stored config with validation result
      const config = await getWebCapabilitiesConfig();
      let apiKeyConfig: ApiKeyConfig | undefined;

      if (provider === 'tavily' || provider === 'exa') {
        apiKeyConfig = config.search[`${provider}Config`];
      } else if (provider === 'firecrawl') {
        apiKeyConfig = config.scraping.firecrawlConfig;
      }

      if (apiKeyConfig) {
        apiKeyConfig.isValid = true;
        apiKeyConfig.lastTested = new Date();
        await saveWebCapabilitiesConfig(config);
      }
    }

    return result;
  } catch (error) {
    return {
      success: false,
      message: `Connection test failed: ${error instanceof Error ? error.message : 'Unknown error'}`,
    };
  }
};

/**
 * Test provider-specific API connections
 */
const testProviderConnection = async (provider: ApiProvider, apiKey: string): Promise<ApiKeyTestResult> => {
  switch (provider) {
    case 'tavily':
      return testTavilyConnection(apiKey);
    case 'exa':
      return testExaConnection(apiKey);
    case 'firecrawl':
      return testFirecrawlConnection(apiKey);
    default:
      throw new Error(`Unknown provider: ${provider}`);
  }
};

/**
 * Test Tavily API connection
 */
const testTavilyConnection = async (apiKey: string): Promise<ApiKeyTestResult> => {
  try {
    const response = await fetch('https://api.tavily.com/search', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        api_key: apiKey,
        query: 'test',
        max_results: 1,
      }),
    });

    if (response.ok) {
      return {
        success: true,
        message: 'Tavily API key is valid and working',
        capabilities: PROVIDER_INFO.tavily.capabilities,
      };
    } else if (response.status === 401) {
      return {
        success: false,
        message: 'Invalid Tavily API key',
      };
    } else {
      return {
        success: false,
        message: `Tavily API error: ${response.status}`,
      };
    }
  } catch (error) {
    return {
      success: false,
      message: 'Failed to connect to Tavily API',
    };
  }
};

/**
 * Test Exa API connection
 */
const testExaConnection = async (apiKey: string): Promise<ApiKeyTestResult> => {
  try {
    const response = await fetch('https://api.exa.ai/search', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${apiKey}`,
      },
      body: JSON.stringify({
        query: 'test',
        numResults: 1,
      }),
    });

    if (response.ok) {
      return {
        success: true,
        message: 'Exa API key is valid and working',
        capabilities: PROVIDER_INFO.exa.capabilities,
      };
    } else if (response.status === 401) {
      return {
        success: false,
        message: 'Invalid Exa API key',
      };
    } else {
      return {
        success: false,
        message: `Exa API error: ${response.status}`,
      };
    }
  } catch (error) {
    return {
      success: false,
      message: 'Failed to connect to Exa API',
    };
  }
};

/**
 * Test Firecrawl API connection
 */
const testFirecrawlConnection = async (apiKey: string): Promise<ApiKeyTestResult> => {
  try {
    const response = await fetch('https://api.firecrawl.dev/v1/scrape', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${apiKey}`,
      },
      body: JSON.stringify({
        url: 'https://example.com',
        formats: ['markdown'],
      }),
    });

    if (response.ok) {
      return {
        success: true,
        message: 'Firecrawl API key is valid and working',
        capabilities: PROVIDER_INFO.firecrawl.capabilities,
      };
    } else if (response.status === 401) {
      return {
        success: false,
        message: 'Invalid Firecrawl API key',
      };
    } else {
      return {
        success: false,
        message: `Firecrawl API error: ${response.status}`,
      };
    }
  } catch (error) {
    return {
      success: false,
      message: 'Failed to connect to Firecrawl API',
    };
  }
};

/**
 * Clear all stored API keys and configuration
 */
export const clearAllApiKeys = async (): Promise<void> => {
  try {
    localStorage.removeItem(STORAGE_KEYS.WEB_CAPABILITIES);
    localStorage.removeItem(STORAGE_KEYS.ENCRYPTION_VERSION);
  } catch (error) {
    console.error('Failed to clear API keys:', error);
    throw new Error('Failed to clear API keys');
  }
};

/**
 * Export encrypted configuration for backup
 */
export const exportConfiguration = async (): Promise<string> => {
  const encryptedConfig = localStorage.getItem(STORAGE_KEYS.WEB_CAPABILITIES);
  if (!encryptedConfig) {
    throw new Error('No configuration to export');
  }
  
  return JSON.stringify({
    version: CURRENT_VERSION,
    timestamp: new Date().toISOString(),
    data: encryptedConfig,
  });
};

/**
 * Import encrypted configuration from backup
 */
export const importConfiguration = async (configData: string): Promise<void> => {
  try {
    const backup = JSON.parse(configData);
    
    if (backup.version !== CURRENT_VERSION) {
      throw new Error('Incompatible configuration version');
    }
    
    // Validate that we can decrypt the data
    const testDecrypt = decryptData(backup.data);
    JSON.parse(testDecrypt); // Validate JSON structure
    
    localStorage.setItem(STORAGE_KEYS.WEB_CAPABILITIES, backup.data);
    localStorage.setItem(STORAGE_KEYS.ENCRYPTION_VERSION, backup.version);
  } catch (error) {
    console.error('Failed to import configuration:', error);
    throw new Error('Failed to import configuration - data may be corrupted or from a different device');
  }
};