/**
 * AI Settings Local Storage Management
 * 
 * This module handles persistence of AI settings to localStorage with:
 * - Global settings (applied to all subjects)
 * - Subject-specific settings (override global for specific subjects)
 * - Automatic migration and validation
 * - Type safety and error handling
 */

import type { AISettings } from '@/lib/api/chat';
import { DEFAULT_AI_SETTINGS } from '@/lib/api/chat';

// Storage keys
const STORAGE_KEYS = {
  GLOBAL_SETTINGS: 'ai_settings_global',
  SUBJECT_SETTINGS: 'ai_settings_subjects',
  LAST_USED_SETTINGS: 'ai_settings_last_used',
  SETTINGS_VERSION: 'ai_settings_version',
} as const;

// Current version for migration purposes
const CURRENT_VERSION = 1;

/**
 * Check if code is running in browser environment
 * Used to prevent localStorage access during SSR
 */
function isBrowser(): boolean {
  return typeof window !== 'undefined';
}

/**
 * Creates a default storage structure for SSR/non-browser environments
 */
function createDefaultStorage(): AISettingsStorage {
  const defaultStored = createStoredSettings(DEFAULT_AI_SETTINGS, false);
  return {
    version: CURRENT_VERSION,
    global: defaultStored,
    subjects: {},
    last_used: defaultStored,
  };
}

// Extended settings with metadata
export interface StoredAISettings extends AISettings {
  /** When these settings were last updated */
  updated_at?: string;
  /** Whether these are custom settings or defaults */
  is_custom?: boolean;
}

// Subject-specific settings storage
export interface SubjectSettingsMap {
  [subjectId: string]: StoredAISettings;
}

// Settings storage structure
export interface AISettingsStorage {
  version: number;
  global: StoredAISettings;
  subjects: SubjectSettingsMap;
  last_used: StoredAISettings;
}

/**
 * Validates AI settings object and ensures all required fields exist
 */
function validateSettings(settings: Partial<AISettings>): AISettings {
  return {
    system_prompt: typeof settings.system_prompt === 'string' ? settings.system_prompt : DEFAULT_AI_SETTINGS.system_prompt,
    include_citations: typeof settings.include_citations === 'boolean' ? settings.include_citations : DEFAULT_AI_SETTINGS.include_citations,
    max_tokens: typeof settings.max_tokens === 'number' && 
                settings.max_tokens >= 256 && 
                settings.max_tokens <= 8192 
                  ? settings.max_tokens 
                  : DEFAULT_AI_SETTINGS.max_tokens,
  };
}

/**
 * Creates a stored settings object with metadata
 */
function createStoredSettings(settings: AISettings, isCustom = true): StoredAISettings {
  return {
    ...validateSettings(settings),
    updated_at: new Date().toISOString(),
    is_custom: isCustom,
  };
}

/**
 * Gets the current storage structure, initializing if needed
 */
function getStorage(): AISettingsStorage {
  // Return default storage structure during SSR
  if (!isBrowser()) {
    return createDefaultStorage();
  }
  
  try {
    const version = localStorage.getItem(STORAGE_KEYS.SETTINGS_VERSION);
    
    // Initialize if no version found
    if (!version) {
      return initializeStorage();
    }
    
    const currentVersion = parseInt(version, 10);
    
    // Migrate if needed
    if (currentVersion < CURRENT_VERSION) {
      return migrateStorage(currentVersion);
    }
    
    // Load existing storage
    const globalRaw = localStorage.getItem(STORAGE_KEYS.GLOBAL_SETTINGS);
    const subjectsRaw = localStorage.getItem(STORAGE_KEYS.SUBJECT_SETTINGS);
    const lastUsedRaw = localStorage.getItem(STORAGE_KEYS.LAST_USED_SETTINGS);
    
    const global = globalRaw ? JSON.parse(globalRaw) : createStoredSettings(DEFAULT_AI_SETTINGS, false);
    const subjects = subjectsRaw ? JSON.parse(subjectsRaw) : {};
    const last_used = lastUsedRaw ? JSON.parse(lastUsedRaw) : global;
    
    return {
      version: currentVersion,
      global: validateStoredSettings(global),
      subjects: validateSubjectSettings(subjects),
      last_used: validateStoredSettings(last_used),
    };
  } catch (error) {
    console.warn('[AI Settings] Failed to load from localStorage, reinitializing:', error);
    return initializeStorage();
  }
}

/**
 * Validates stored settings and ensures they have proper structure
 */
function validateStoredSettings(settings: any): StoredAISettings {
  if (!settings || typeof settings !== 'object') {
    return createStoredSettings(DEFAULT_AI_SETTINGS, false);
  }
  
  return {
    ...validateSettings(settings),
    updated_at: typeof settings.updated_at === 'string' ? settings.updated_at : new Date().toISOString(),
    is_custom: typeof settings.is_custom === 'boolean' ? settings.is_custom : true,
  };
}

/**
 * Validates subject settings map
 */
function validateSubjectSettings(subjects: any): SubjectSettingsMap {
  if (!subjects || typeof subjects !== 'object') {
    return {};
  }
  
  const validated: SubjectSettingsMap = {};
  
  for (const [subjectId, settings] of Object.entries(subjects)) {
    if (typeof subjectId === 'string' && settings) {
      validated[subjectId] = validateStoredSettings(settings);
    }
  }
  
  return validated;
}

/**
 * Initializes storage with default values
 */
function initializeStorage(): AISettingsStorage {
  const defaultStored = createStoredSettings(DEFAULT_AI_SETTINGS, false);
  
  const storage: AISettingsStorage = {
    version: CURRENT_VERSION,
    global: defaultStored,
    subjects: {},
    last_used: defaultStored,
  };
  
  saveStorage(storage);
  return storage;
}

/**
 * Migrates storage from older versions
 */
function migrateStorage(fromVersion: number): AISettingsStorage {
  console.log(`[AI Settings] Migrating from version ${fromVersion} to ${CURRENT_VERSION}`);
  
  // For now, just reinitialize - add migration logic here if needed in future
  return initializeStorage();
}

/**
 * Saves storage to localStorage
 */
function saveStorage(storage: AISettingsStorage): void {
  // No-op during SSR - localStorage is not available
  if (!isBrowser()) {
    return;
  }
  
  try {
    localStorage.setItem(STORAGE_KEYS.SETTINGS_VERSION, storage.version.toString());
    localStorage.setItem(STORAGE_KEYS.GLOBAL_SETTINGS, JSON.stringify(storage.global));
    localStorage.setItem(STORAGE_KEYS.SUBJECT_SETTINGS, JSON.stringify(storage.subjects));
    localStorage.setItem(STORAGE_KEYS.LAST_USED_SETTINGS, JSON.stringify(storage.last_used));
    
    // Debug: Verify the save
    console.log('[AI Settings Storage] Saved to localStorage:', {
      version: storage.version,
      globalKeys: Object.keys(storage.global),
      subjectIds: Object.keys(storage.subjects),
      subjects: storage.subjects,
    });
  } catch (error) {
    console.error('[AI Settings Storage] Failed to save to localStorage:', error);
  }
}

// ==================== Public API ====================

/**
 * Gets AI settings for a specific subject
 * Priority: Subject-specific > Global > Defaults
 */
export function getAISettings(subjectId?: string): AISettings {
  const storage = getStorage();
  
  console.log('[AI Settings Storage] getAISettings called:', {
    subjectId,
    hasSubjectSettings: subjectId ? !!storage.subjects[subjectId] : false,
    availableSubjects: Object.keys(storage.subjects),
    globalSettings: storage.global,
  });
  
  // If subject ID provided, check for subject-specific settings first
  if (subjectId && storage.subjects[subjectId]) {
    console.log('[AI Settings Storage] Returning subject-specific settings for:', subjectId);
    return validateSettings(storage.subjects[subjectId]);
  }
  
  // Fall back to global settings
  console.log('[AI Settings Storage] Returning global settings');
  return validateSettings(storage.global);
}

/**
 * Gets the last used AI settings (for new sessions)
 */
export function getLastUsedAISettings(): AISettings {
  const storage = getStorage();
  return validateSettings(storage.last_used);
}

/**
 * Saves AI settings globally (applies to all subjects unless overridden)
 */
export function saveGlobalAISettings(settings: AISettings): void {
  console.log('[AI Settings Storage] saveGlobalAISettings called:', settings);
  const storage = getStorage();
  storage.global = createStoredSettings(settings, true);
  storage.last_used = storage.global;
  saveStorage(storage);
}

/**
 * Saves AI settings for a specific subject
 */
export function saveSubjectAISettings(subjectId: string, settings: AISettings): void {
  console.log('[AI Settings Storage] saveSubjectAISettings called:', { subjectId, settings });
  const storage = getStorage();
  storage.subjects[subjectId] = createStoredSettings(settings, true);
  storage.last_used = storage.subjects[subjectId];
  saveStorage(storage);
  
  // Verify the save immediately
  const verifyStorage = getStorage();
  console.log('[AI Settings Storage] Verified after save:', {
    subjectId,
    savedSettings: verifyStorage.subjects[subjectId],
    allSubjects: Object.keys(verifyStorage.subjects),
  });
}

/**
 * Updates last used settings (called when settings are changed in UI)
 */
export function updateLastUsedSettings(settings: AISettings): void {
  const storage = getStorage();
  storage.last_used = createStoredSettings(settings, true);
  saveStorage(storage);
}

/**
 * Removes subject-specific settings (falls back to global)
 */
export function removeSubjectAISettings(subjectId: string): void {
  const storage = getStorage();
  delete storage.subjects[subjectId];
  saveStorage(storage);
}

/**
 * Resets all settings to defaults
 */
export function resetAllAISettings(): void {
  // No-op during SSR - localStorage is not available
  if (!isBrowser()) {
    return;
  }
  
  try {
    localStorage.removeItem(STORAGE_KEYS.GLOBAL_SETTINGS);
    localStorage.removeItem(STORAGE_KEYS.SUBJECT_SETTINGS);
    localStorage.removeItem(STORAGE_KEYS.LAST_USED_SETTINGS);
    localStorage.removeItem(STORAGE_KEYS.SETTINGS_VERSION);
  } catch (error) {
    console.error('[AI Settings] Failed to reset settings:', error);
  }
}

/**
 * Gets all subject-specific settings (for management UI)
 */
export function getAllSubjectSettings(): SubjectSettingsMap {
  const storage = getStorage();
  return storage.subjects;
}

/**
 * Gets global settings
 */
export function getGlobalAISettings(): StoredAISettings {
  const storage = getStorage();
  return storage.global;
}

/**
 * Checks if a subject has custom settings
 */
export function hasSubjectCustomSettings(subjectId: string): boolean {
  const storage = getStorage();
  return subjectId in storage.subjects;
}

/**
 * Gets settings metadata for debugging/display
 */
export function getSettingsMetadata(subjectId?: string): {
  source: 'subject' | 'global' | 'default';
  updated_at?: string;
  is_custom?: boolean;
} {
  const storage = getStorage();
  
  if (subjectId && storage.subjects[subjectId]) {
    return {
      source: 'subject',
      updated_at: storage.subjects[subjectId].updated_at,
      is_custom: storage.subjects[subjectId].is_custom,
    };
  }
  
  if (storage.global.is_custom) {
    return {
      source: 'global',
      updated_at: storage.global.updated_at,
      is_custom: storage.global.is_custom,
    };
  }
  
  return {
    source: 'default',
  };
}

/**
 * Exports all settings as JSON (for backup/sharing)
 */
export function exportAISettings(): string {
  const storage = getStorage();
  return JSON.stringify(storage, null, 2);
}

/**
 * Imports settings from JSON (for restore/sharing)
 */
export function importAISettings(jsonData: string): boolean {
  try {
    const imported = JSON.parse(jsonData) as AISettingsStorage;
    
    // Validate structure
    if (!imported.version || !imported.global) {
      throw new Error('Invalid settings format');
    }
    
    // Validate and save
    const validatedStorage: AISettingsStorage = {
      version: CURRENT_VERSION, // Always use current version
      global: validateStoredSettings(imported.global),
      subjects: validateSubjectSettings(imported.subjects || {}),
      last_used: validateStoredSettings(imported.last_used || imported.global),
    };
    
    saveStorage(validatedStorage);
    return true;
  } catch (error) {
    console.error('[AI Settings] Failed to import settings:', error);
    return false;
  }
}