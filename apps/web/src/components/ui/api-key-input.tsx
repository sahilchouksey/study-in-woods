'use client';

import * as React from 'react';
import { Eye, EyeOff, Shield } from 'lucide-react';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { cn } from '@/lib/utils';

interface ApiKeyInputProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  className?: string;
  disabled?: boolean;
  error?: string;
  isValid?: boolean;
  showValidation?: boolean;
}

export function ApiKeyInput({
  label,
  value,
  onChange,
  placeholder = 'Enter your API key...',
  className,
  disabled = false,
  error,
  isValid,
  showValidation = false,
}: ApiKeyInputProps) {
  const [showKey, setShowKey] = React.useState(false);
  const [isFocused, setIsFocused] = React.useState(false);

  const maskValue = (val: string) => {
    if (!val || showKey) return val;
    if (val.length <= 8) return '•'.repeat(val.length);
    return val.slice(0, 4) + '•'.repeat(val.length - 8) + val.slice(-4);
  };

  const getValidationColor = () => {
    if (!showValidation || !value) return '';
    if (error) return 'border-destructive ring-destructive/20';
    if (isValid) return 'border-green-500 ring-green-500/20';
    return 'border-yellow-500 ring-yellow-500/20';
  };

  return (
    <div className={cn('space-y-2', className)}>
      <div className="flex items-center gap-2">
        <Shield className="h-4 w-4 text-muted-foreground" />
        <Label className="text-sm font-medium">{label}</Label>
        {showValidation && isValid && (
          <div className="flex items-center gap-1">
            <div className="h-2 w-2 rounded-full bg-green-500" />
            <span className="text-xs text-green-600">Valid</span>
          </div>
        )}
        {showValidation && error && (
          <div className="flex items-center gap-1">
            <div className="h-2 w-2 rounded-full bg-destructive" />
            <span className="text-xs text-destructive">Invalid</span>
          </div>
        )}
      </div>
      
      <div className="relative">
        <Input
          type={showKey ? 'text' : 'password'}
          value={isFocused || showKey ? value : maskValue(value)}
          onChange={(e) => onChange(e.target.value)}
          onFocus={() => setIsFocused(true)}
          onBlur={() => setIsFocused(false)}
          placeholder={placeholder}
          disabled={disabled}
          className={cn(
            'pr-12 font-mono text-sm',
            getValidationColor(),
            disabled && 'opacity-50 cursor-not-allowed'
          )}
        />
        
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="absolute right-1 top-1/2 -translate-y-1/2 h-7 w-7 p-0"
          onClick={() => setShowKey(!showKey)}
          disabled={disabled || !value}
          tabIndex={-1}
        >
          {showKey ? (
            <EyeOff className="h-3 w-3" />
          ) : (
            <Eye className="h-3 w-3" />
          )}
        </Button>
      </div>
      
      {error && (
        <p className="text-xs text-destructive">{error}</p>
      )}
      
      <p className="text-xs text-muted-foreground">
        Your API key is encrypted and stored locally on your device only.
      </p>
    </div>
  );
}