'use client';

import { useState, useEffect, useCallback } from 'react';
import {
  Cpu,
  Quote,
  Hash,
  Settings2,
  Info,
  RotateCcw,
  Sparkles,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Separator } from '@/components/ui/separator';
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import type { AISettings } from '@/lib/api/chat';
import { DEFAULT_AI_SETTINGS, DEFAULT_SYSTEM_PROMPT } from '@/lib/api/chat';
import { 
  getAISettings,
  saveGlobalAISettings,
  saveSubjectAISettings,
  removeSubjectAISettings,
  hasSubjectCustomSettings,
  getSettingsMetadata,
} from '@/lib/ai-settings-storage';
import { useChatContext } from '@/lib/api/hooks/useChat';
import { cn } from '@/lib/utils';

export function AISettingsTab() {
  // Fetch subjects for the dropdown
  const { data: contextData } = useChatContext();
  const subjects = contextData?.subjects || [];

  // Selected subject for editing (null = global settings)
  const [selectedSubjectId, setSelectedSubjectId] = useState<string | null>(null);
  
  // Current settings being edited
  const [settings, setSettings] = useState<AISettings>({ ...DEFAULT_AI_SETTINGS });
  const [showSaveConfirmation, setShowSaveConfirmation] = useState(false);

  // Load settings when subject selection changes
  useEffect(() => {
    if (typeof window === 'undefined') return;
    
    const loadedSettings = getAISettings(selectedSubjectId || undefined);
    setSettings(loadedSettings);
  }, [selectedSubjectId]);

  // Get metadata for display
  const metadata = selectedSubjectId 
    ? getSettingsMetadata(selectedSubjectId) 
    : getSettingsMetadata();
  const hasCustomSettings = selectedSubjectId 
    ? hasSubjectCustomSettings(selectedSubjectId) 
    : false;

  // Get subject name for display
  const selectedSubject = subjects.find(s => s.id.toString() === selectedSubjectId);
  const subjectName = selectedSubject?.name || 'Global';

  // Handle settings changes
  const handleSettingsChange = useCallback((newSettings: AISettings) => {
    setSettings(newSettings);
    setShowSaveConfirmation(false);
    
    // Auto-save
    if (selectedSubjectId) {
      saveSubjectAISettings(selectedSubjectId, newSettings);
    } else {
      saveGlobalAISettings(newSettings);
    }
    
    setShowSaveConfirmation(true);
    setTimeout(() => setShowSaveConfirmation(false), 2000);
  }, [selectedSubjectId]);

  const handleSystemPromptChange = (value: string) => {
    handleSettingsChange({ ...settings, system_prompt: value });
  };

  const handleCitationsToggle = (checked: boolean) => {
    handleSettingsChange({ ...settings, include_citations: checked });
  };

  const handleMaxTokensChange = (value: string) => {
    const num = parseInt(value, 10);
    if (!isNaN(num)) {
      handleSettingsChange({ ...settings, max_tokens: num });
    }
  };

  const handleMaxTokensBlur = (value: string) => {
    const num = parseInt(value, 10);
    if (isNaN(num) || num < 256) {
      handleSettingsChange({ ...settings, max_tokens: 256 });
    } else if (num > 8192) {
      handleSettingsChange({ ...settings, max_tokens: 8192 });
    }
  };

  const resetToDefaults = () => {
    handleSettingsChange({ ...DEFAULT_AI_SETTINGS });
  };

  const handleRemoveCustomSettings = () => {
    if (selectedSubjectId) {
      removeSubjectAISettings(selectedSubjectId);
      // Reload global settings
      const globalSettings = getAISettings();
      setSettings(globalSettings);
      setShowSaveConfirmation(true);
      setTimeout(() => setShowSaveConfirmation(false), 2000);
    }
  };

  const handleCopyToGlobal = () => {
    saveGlobalAISettings(settings);
    setShowSaveConfirmation(true);
    setTimeout(() => setShowSaveConfirmation(false), 2000);
  };

  // Replace placeholder in default prompt
  const displayDefaultPrompt = DEFAULT_SYSTEM_PROMPT.replace(
    '{subject_name}',
    subjectName
  );

  // Get all subjects with custom settings
  const subjectsWithCustomSettings = subjects.filter(s => 
    hasSubjectCustomSettings(s.id.toString())
  );

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="border-b border-border p-6">
        <div className="flex items-center gap-3">
          <Sparkles className="h-6 w-6 text-primary" />
          <div>
            <h2 className="text-foreground text-xl font-semibold">AI Settings</h2>
            <p className="text-muted-foreground mt-1">
              Configure AI behavior and response settings
            </p>
          </div>
        </div>
      </div>

      <div className="flex-1 overflow-auto">
        <div className="max-w-2xl mx-auto p-6 space-y-8">
          {/* Subject Selector */}
          <section className="space-y-4">
            <div className="flex items-center gap-3">
              <Settings2 className="h-5 w-5 text-muted-foreground" />
              <h3 className="text-foreground font-medium">Settings Scope</h3>
            </div>
            <div className="pl-8 space-y-4">
              <div className="space-y-2">
                <Label>Select Subject or Global</Label>
                <Select
                  value={selectedSubjectId || 'global'}
                  onValueChange={(value) => setSelectedSubjectId(value === 'global' ? null : value)}
                >
                  <SelectTrigger className="w-full">
                    <SelectValue placeholder="Select scope" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="global">
                      <div className="flex items-center gap-2">
                        <span className="h-2 w-2 rounded-full bg-gray-500" />
                        Global Settings (Default for all subjects)
                      </div>
                    </SelectItem>
                    {subjects.map((subject) => (
                      <SelectItem key={subject.id} value={subject.id.toString()}>
                        <div className="flex items-center gap-2">
                          {hasSubjectCustomSettings(subject.id.toString()) && (
                            <span className="h-2 w-2 rounded-full bg-primary" />
                          )}
                          {subject.name} ({subject.code})
                        </div>
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <p className="text-xs text-muted-foreground">
                  {selectedSubjectId 
                    ? 'Settings will apply only to this subject'
                    : 'Global settings apply to all subjects unless overridden'}
                </p>
              </div>

              {/* Settings Status */}
              <div className="flex items-center gap-2 flex-wrap">
                <Badge variant={metadata.source === 'subject' ? 'default' : 'secondary'} className="gap-1.5">
                  <span className={cn(
                    "h-2 w-2 rounded-full",
                    metadata.source === 'subject' ? 'bg-blue-500' : 'bg-gray-500'
                  )} />
                  {metadata.source === 'subject' ? 'Subject-specific' : 
                   metadata.source === 'global' ? 'Global settings' : 'Default settings'}
                </Badge>
                {metadata.updated_at && (
                  <Badge variant="outline" className="text-xs">
                    Updated {new Date(metadata.updated_at).toLocaleDateString()}
                  </Badge>
                )}
                {showSaveConfirmation && (
                  <Badge variant="secondary" className="text-xs animate-pulse">
                    ✓ Saved
                  </Badge>
                )}
              </div>

              {/* Subjects with custom settings */}
              {subjectsWithCustomSettings.length > 0 && !selectedSubjectId && (
                <div className="text-xs text-muted-foreground">
                  <span className="font-medium">{subjectsWithCustomSettings.length}</span> subject(s) have custom settings
                </div>
              )}
            </div>
          </section>

          <Separator />

          {/* Model Info */}
          <section className="space-y-4">
            <div className="flex items-center gap-3">
              <Cpu className="h-5 w-5 text-muted-foreground" />
              <h3 className="text-foreground font-medium">AI Model</h3>
            </div>
            <div className="pl-8 space-y-2">
              <div className="flex items-center gap-2">
                <Badge variant="secondary" className="gap-1.5">
                  <span className="h-2 w-2 rounded-full bg-green-500" />
                  GPT 120B OSS
                </Badge>
                <TooltipProvider>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Badge variant="outline" className="text-xs cursor-help">
                        via DigitalOcean
                      </Badge>
                    </TooltipTrigger>
                    <TooltipContent>
                      <p className="text-xs">Powered by DigitalOcean GenAI Platform</p>
                    </TooltipContent>
                  </Tooltip>
                </TooltipProvider>
              </div>
              <p className="text-xs text-muted-foreground">
                Model selection is managed by the system
              </p>
            </div>
          </section>

          <Separator />

          {/* Citations Toggle */}
          <section className="space-y-4">
            <div className="flex items-center gap-3">
              <Quote className="h-5 w-5 text-muted-foreground" />
              <h3 className="text-foreground font-medium">Citations</h3>
            </div>
            <div className="pl-8 space-y-2">
              <div className="flex items-center justify-between">
                <Label htmlFor="citations-toggle">Include Citations in Responses</Label>
                <Switch
                  id="citations-toggle"
                  checked={settings.include_citations ?? true}
                  onCheckedChange={handleCitationsToggle}
                />
              </div>
              <p className="text-xs text-muted-foreground">
                When enabled, AI responses will include source references from the knowledge base
              </p>
            </div>
          </section>

          <Separator />

          {/* Max Tokens */}
          <section className="space-y-4">
            <div className="flex items-center gap-3">
              <Hash className="h-5 w-5 text-muted-foreground" />
              <h3 className="text-foreground font-medium">Response Length</h3>
            </div>
            <div className="pl-8 space-y-2">
              <div className="flex items-center gap-2">
                <Input
                  id="max-tokens"
                  type="number"
                  min={256}
                  max={8192}
                  step={256}
                  value={settings.max_tokens ?? 2048}
                  onChange={(e) => handleMaxTokensChange(e.target.value)}
                  onBlur={(e) => handleMaxTokensBlur(e.target.value)}
                  className="w-32"
                />
                <span className="text-sm text-muted-foreground">tokens (256 - 8192)</span>
              </div>
              <p className="text-xs text-muted-foreground">
                Controls the maximum length of AI responses. Higher values allow longer answers.
              </p>
            </div>
          </section>

          <Separator />

          {/* System Prompt */}
          <section className="space-y-4">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                <Info className="h-5 w-5 text-muted-foreground" />
                <h3 className="text-foreground font-medium">System Prompt</h3>
              </div>
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button variant="ghost" size="sm" className="h-8 px-2">
                      <Info className="h-4 w-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent side="left" className="max-w-xs">
                    <p className="text-xs">
                      The system prompt defines how the AI behaves. Leave empty to use the default prompt.
                    </p>
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
            </div>
            <div className="pl-8 space-y-2">
              <Textarea
                id="system-prompt"
                placeholder={displayDefaultPrompt}
                value={settings.system_prompt || ''}
                onChange={(e) => handleSystemPromptChange(e.target.value)}
                className="min-h-[200px] text-sm font-mono"
              />
              <p className="text-xs text-muted-foreground">
                {settings.system_prompt 
                  ? 'Using custom system prompt' 
                  : 'Using default system prompt (shown as placeholder)'}
              </p>
            </div>
          </section>

          <Separator />

          {/* Actions */}
          <section className="space-y-4">
            <div className="flex items-center gap-3">
              <Settings2 className="h-5 w-5 text-muted-foreground" />
              <h3 className="text-foreground font-medium">Actions</h3>
            </div>
            <div className="pl-8 space-y-3">
              <Button
                variant="outline"
                size="sm"
                onClick={resetToDefaults}
                className="w-full sm:w-auto gap-2"
              >
                <RotateCcw className="h-4 w-4" />
                Reset to Defaults
              </Button>

              {selectedSubjectId && hasCustomSettings && (
                <div className="flex flex-col sm:flex-row gap-2">
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={handleCopyToGlobal}
                    className="gap-2"
                  >
                    <Settings2 className="h-4 w-4" />
                    Copy to Global
                  </Button>
                  <Button
                    variant="destructive"
                    size="sm"
                    onClick={handleRemoveCustomSettings}
                    className="gap-2"
                  >
                    <RotateCcw className="h-4 w-4" />
                    Remove Custom Settings
                  </Button>
                </div>
              )}

              {selectedSubjectId && hasCustomSettings && (
                <p className="text-xs text-muted-foreground">
                  Copy current settings to global or remove custom settings to use global defaults
                </p>
              )}
            </div>
          </section>
        </div>
      </div>
    </div>
  );
}
