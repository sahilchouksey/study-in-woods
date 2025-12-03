'use client';

import { useState } from 'react';
import { Bell, Key, User, Globe } from 'lucide-react';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Button } from '@/components/ui/button';
import { Separator } from '@/components/ui/separator';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { WebCapabilitiesSection } from '@/components/settings/WebCapabilitiesSection';

type AIProvider = 'gemini' | 'chatgpt' | 'claude' | 'custom';

export function SettingsTab() {
  const [selectedProvider, setSelectedProvider] = useState<AIProvider>('gemini');

  return (
    <div className="flex flex-col h-full">
      <div className="border-b border-border p-6">
        <h2 className="text-foreground">Settings</h2>
        <p className="text-muted-foreground mt-1">Manage your preferences and data</p>
      </div>

      <div className="flex-1 overflow-auto">
        <div className="max-w-2xl mx-auto p-6 space-y-8">
          <section className="space-y-4">
            <div className="flex items-center gap-3">
              <User className="h-5 w-5 text-muted-foreground" />
              <h3 className="text-foreground">Profile</h3>
            </div>
            <div className="space-y-4 pl-8">
              <div className="space-y-2">
                <Label htmlFor="name">Name</Label>
                <Input
                  id="name"
                  placeholder="Your name"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="email">Email</Label>
                <Input
                  id="email"
                  type="email"
                  placeholder="your.email@example.com"
                />
              </div>
            </div>
          </section>

          <Separator />

          <section className="space-y-4">
            <div className="flex items-center gap-3">
              <Globe className="h-5 w-5 text-muted-foreground" />
              <h3 className="text-foreground">Web Search & Scraping APIs</h3>
            </div>
            <WebCapabilitiesSection />
          </section>

          <Separator />

          <section className="space-y-4">
            <div className="flex items-center gap-3">
              <Key className="h-5 w-5 text-muted-foreground" />
              <h3 className="text-foreground">AI Provider</h3>
            </div>
            <div className="space-y-4 pl-8">
              <div className="space-y-2">
                <Label htmlFor="provider">Select AI Provider</Label>
                <Select value={selectedProvider} onValueChange={(value: AIProvider) => setSelectedProvider(value)}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="gemini">Google Gemini</SelectItem>
                    <SelectItem value="chatgpt">OpenAI ChatGPT</SelectItem>
                    <SelectItem value="claude">Anthropic Claude</SelectItem>
                    <SelectItem value="custom">Custom Endpoint</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              
              <div className="space-y-2">
                <Label htmlFor="apiKey">API Key</Label>
                <Input
                  id="apiKey"
                  type="password"
                  placeholder="Enter your API key"
                />
                <p className="text-xs text-muted-foreground">
                  Your API key is stored locally and never shared
                </p>
              </div>

              {selectedProvider === 'custom' && (
                <div className="space-y-2">
                  <Label htmlFor="endpoint">Custom Endpoint</Label>
                  <Input
                    id="endpoint"
                    placeholder="https://your-custom-endpoint.com/api"
                  />
                </div>
              )}
            </div>
          </section>

          <Separator />

          <section className="space-y-4">
            <div className="flex items-center gap-3">
              <Bell className="h-5 w-5 text-muted-foreground" />
              <h3 className="text-foreground">Notifications</h3>
            </div>
            <div className="space-y-4 pl-8">
              <div className="flex items-center justify-between">
                <div>
                  <Label htmlFor="study-reminders">Study Reminders</Label>
                  <p className="text-sm text-muted-foreground">Get reminded to study regularly</p>
                </div>
                <Switch id="study-reminders" />
              </div>
              
              <div className="flex items-center justify-between">
                <div>
                  <Label htmlFor="new-questions">New PYQ Alerts</Label>
                  <p className="text-sm text-muted-foreground">Notify when new questions are available</p>
                </div>
                <Switch id="new-questions" />
              </div>
              
              <div className="flex items-center justify-between">
                <div>
                  <Label htmlFor="ai-updates">AI Model Updates</Label>
                  <p className="text-sm text-muted-foreground">Get notified about AI improvements</p>
                </div>
                <Switch id="ai-updates" />
              </div>
            </div>
          </section>

          <Separator />

          <section className="space-y-4">
            <h3 className="text-foreground">Study Preferences</h3>
            <div className="space-y-4 pl-8">
              <div className="space-y-2">
                <Label htmlFor="difficulty">Default Difficulty Level</Label>
                <Select>
                  <SelectTrigger>
                    <SelectValue placeholder="Select difficulty" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="beginner">Beginner</SelectItem>
                    <SelectItem value="intermediate">Intermediate</SelectItem>
                    <SelectItem value="advanced">Advanced</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              
              <div className="space-y-2">
                <Label htmlFor="session-length">Study Session Length</Label>
                <Select>
                  <SelectTrigger>
                    <SelectValue placeholder="Select duration" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="30">30 minutes</SelectItem>
                    <SelectItem value="60">1 hour</SelectItem>
                    <SelectItem value="90">1.5 hours</SelectItem>
                    <SelectItem value="120">2 hours</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              
              <div className="flex items-center justify-between">
                <div>
                  <Label htmlFor="detailed-explanations">Detailed Explanations</Label>
                  <p className="text-sm text-muted-foreground">Show step-by-step solutions</p>
                </div>
                <Switch id="detailed-explanations" defaultChecked />
              </div>
            </div>
          </section>

          <Separator />

          <section className="space-y-4">
            <h3 className="text-foreground">Data Management</h3>
            <div className="space-y-4 pl-8">
              <div className="space-y-2">
                <Button variant="outline" className="w-full">
                  Export Chat History
                </Button>
                <p className="text-xs text-muted-foreground">
                  Download all your conversations as JSON
                </p>
              </div>
              
              <div className="space-y-2">
                <Button variant="outline" className="w-full">
                  Clear All Data
                </Button>
                <p className="text-xs text-muted-foreground">
                  Remove all conversations and preferences
                </p>
              </div>
            </div>
          </section>

          <div className="flex justify-end">
            <Button>Save Settings</Button>
          </div>
        </div>
      </div>
    </div>
  );
}