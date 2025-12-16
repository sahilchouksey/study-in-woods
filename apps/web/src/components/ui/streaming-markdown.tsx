'use client';

import { memo } from 'react';
import { Streamdown } from 'streamdown';
import { cn } from '@/lib/utils';

export interface StreamingMarkdownProps {
  children: string;
  isStreaming?: boolean;
  className?: string;
}

/**
 * StreamingMarkdown - Simple wrapper around Streamdown
 * 
 * Streamdown is designed to handle streaming markdown natively.
 * We should NOT preprocess the content - that breaks its internal handling.
 * 
 * Key props:
 * - isAnimating: Set to true during streaming to disable interactive elements
 * - parseIncompleteMarkdown: Enables remend preprocessor for incomplete syntax
 */
export const StreamingMarkdown = memo(function StreamingMarkdown({
  children,
  isStreaming = false,
  className,
}: StreamingMarkdownProps) {
  return (
    <div className={cn('streaming-markdown', className)}>
      <Streamdown
        isAnimating={isStreaming}
        parseIncompleteMarkdown={true}
        controls={{
          code: !isStreaming,
          table: true,
        }}
      >
        {children || ''}
      </Streamdown>
    </div>
  );
});

export default StreamingMarkdown;
