'use client';

import { useState, useEffect } from 'react';
import { User, Globe, Check } from 'lucide-react';
import { InlineSpinner } from '@/components/ui/loading-spinner';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Separator } from '@/components/ui/separator';
import { WebCapabilitiesSection } from '@/components/settings/WebCapabilitiesSection';
import { useAuth } from '@/providers/auth-provider';
import { apiClient } from '@/lib/api/client';
import { toast } from 'sonner';

export function SettingsTab() {
  const { user, refetchUser } = useAuth();
  const [name, setName] = useState('');
  const [isSaving, setIsSaving] = useState(false);
  const [hasChanges, setHasChanges] = useState(false);

  // Initialize form with user data
  useEffect(() => {
    if (user) {
      setName(user.name || '');
    }
  }, [user]);

  // Track changes
  useEffect(() => {
    if (user) {
      setHasChanges(name !== (user.name || ''));
    }
  }, [name, user]);

  const handleSaveProfile = async () => {
    if (!hasChanges || isSaving) return;

    setIsSaving(true);
    try {
      await apiClient.put('/api/v1/auth/profile', { name });
      refetchUser();
      toast.success('Profile updated successfully');
      setHasChanges(false);
    } catch (error: any) {
      toast.error(error.message || 'Failed to update profile');
    } finally {
      setIsSaving(false);
    }
  };

  return (
    <div className="flex flex-col h-full">
      <div className="border-b border-border p-6">
        <h2 className="text-foreground text-xl font-semibold">Settings</h2>
        <p className="text-muted-foreground mt-1">Manage your profile and API keys</p>
      </div>

      <div className="flex-1 overflow-auto">
        <div className="max-w-2xl mx-auto p-6 space-y-8">
          {/* Profile Section */}
          <section className="space-y-4">
            <div className="flex items-center gap-3">
              <User className="h-5 w-5 text-muted-foreground" />
              <h3 className="text-foreground font-medium">Profile</h3>
            </div>
            <div className="space-y-4 pl-8">
              <div className="space-y-2">
                <Label htmlFor="name">Display Name</Label>
                <Input
                  id="name"
                  placeholder="Your name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                />
                <p className="text-xs text-muted-foreground">
                  This is how you'll appear in the app
                </p>
              </div>
              <div className="space-y-2">
                <Label htmlFor="email">Email</Label>
                <Input
                  id="email"
                  type="email"
                  value={user?.email || ''}
                  disabled
                  className="bg-muted"
                />
                <p className="text-xs text-muted-foreground">
                  Email cannot be changed
                </p>
              </div>
              <Button 
                onClick={handleSaveProfile}
                disabled={!hasChanges || isSaving}
                size="sm"
              >
                {isSaving ? (
                  <>
                    <InlineSpinner className="mr-2" />
                    Saving...
                  </>
                ) : hasChanges ? (
                  'Save Profile'
                ) : (
                  <>
                    <Check className="h-4 w-4 mr-2" />
                    Saved
                  </>
                )}
              </Button>
            </div>
          </section>

          <Separator />

          {/* Web Search & Scraping APIs Section */}
          <section className="space-y-4">
            <div className="flex items-center gap-3">
              <Globe className="h-5 w-5 text-muted-foreground" />
              <h3 className="text-foreground font-medium">Web Search & Scraping APIs</h3>
            </div>
            <p className="text-sm text-muted-foreground pl-8">
              Add your own API keys to enable web search capabilities in chat. These keys are stored securely and sent with your chat requests.
            </p>
            <div className="pl-8">
              <WebCapabilitiesSection />
            </div>
          </section>
        </div>
      </div>
    </div>
  );
}
