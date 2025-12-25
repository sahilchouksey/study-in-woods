'use client';

import { useState, useEffect, useRef, memo } from 'react';
import Image from 'next/image';
import { createPortal } from 'react-dom';
import { Button } from '@/components/ui/button';
import { ThemeToggle } from '@/components/theme-toggle';
import { useAuth } from '@/providers/auth-provider';
import Link from 'next/link';
import { ChevronDown, Search, Zap, BookOpen, ZoomIn, X } from 'lucide-react';

// FAQ Data
// Note: Add images to /public/faq/ directory when available:
// - settings-tavily-setup.png, settings-tavily-connected.png (for enable-search)
// - retrieval-dropdown.png (for retrieval-methods)
const faqData = [
  {
    id: 'enable-search',
    icon: Search,
    question: 'How do I enable web search for real-time information?',
    answer: `To enable web search capabilities in your chat:

1. Go to **Settings** from the sidebar
2. Navigate to the **Search Providers** tab
3. Add your **Tavily API Key** (get one free at tavily.com)
4. Click **Save Key** and then **Test Connection**
5. Once connected, you'll see "Valid" status with available capabilities

When enabled, the AI can search the web for latest information, news, and real-time data to supplement your course materials.`,
    images: ['/faq/settings-tavily-setup.png', '/faq/settings-tavily-connected.png', '/faq/web-search-tool.png'],
  },
  {
    id: 'retrieval-methods',
    icon: Zap,
    question: 'What are the different retrieval methods and when should I use them?',
    answer: `Study in Woods offers multiple retrieval methods accessible via the dropdown next to the send button:

• **Default** - Standard retrieval using course materials
• **Rewrite** - Rewrites your query for better search results
• **Step Back** - Breaks down complex questions into simpler parts
• **Sub Queries** - Generates multiple sub-queries for comprehensive answers

For most study questions, "Default" works great. Use "Sub Queries" for complex topics that need information from multiple sources.`,
    images: ['/faq/retrieval-dropdown.png'],
  },
  {
    id: 'response-quality',
    icon: BookOpen,
    question: 'How accurate are the AI responses? What sources are used?',
    answer: `We've prioritized response quality and accuracy:

• **Comprehensive Knowledge Base** - We've indexed almost all recommended textbooks and course materials for each subject
• **Citation-Based Responses** - Every response includes clickable citations [[C1]], [[C2]] etc. linking to the exact source passages
• **Page Numbers** - Citations show the exact page number from the source document
• **Multiple Sources** - Responses synthesize information from multiple textbooks for comprehensive answers

The AI draws from standard DBMS textbooks (Silberschatz, Ramakrishnan, Date), official course materials, and past year question papers to provide exam-focused, accurate responses.`,
    images: [] as string[],
  },
];

// Image Lightbox Component
function ImageLightbox({ 
  src, 
  alt, 
  isOpen, 
  onClose 
}: { 
  src: string; 
  alt: string; 
  isOpen: boolean; 
  onClose: () => void;
}) {
  useEffect(() => {
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    if (isOpen) {
      document.addEventListener('keydown', handleEscape);
      document.body.style.overflow = 'hidden';
    }
    return () => {
      document.removeEventListener('keydown', handleEscape);
      document.body.style.overflow = '';
    };
  }, [isOpen, onClose]);

  if (!isOpen) return null;

  return createPortal(
    <div 
      className="fixed inset-0 z-[9999] flex items-center justify-center bg-black/80 backdrop-blur-sm animate-in fade-in duration-200"
      onClick={onClose}
    >
      {/* Close button */}
      <button
        onClick={onClose}
        className="absolute top-4 right-4 p-2 rounded-full bg-white/10 hover:bg-white/20 text-white transition-colors"
        aria-label="Close"
      >
        <X className="w-6 h-6" />
      </button>
      
      {/* Image container */}
      <div 
        className="relative max-w-[90vw] max-h-[90vh] animate-in zoom-in-95 duration-200"
        onClick={(e) => e.stopPropagation()}
      >
        <Image
          src={src}
          alt={alt}
          width={1200}
          height={800}
          className="w-auto h-auto max-w-full max-h-[90vh] object-contain rounded-lg shadow-2xl"
          priority
        />
      </div>
    </div>,
    document.body
  );
}

// FAQ Item Component
function FAQItem({ item, isOpen, onToggle }: { 
  item: typeof faqData[0]; 
  isOpen: boolean; 
  onToggle: () => void;
}) {
  const Icon = item.icon;
  const [lightboxImage, setLightboxImage] = useState<{ src: string; alt: string } | null>(null);
  
  return (
    <>
      <div className="border border-neutral-200 dark:border-neutral-800 rounded-xl overflow-hidden bg-white/50 dark:bg-neutral-900/50 backdrop-blur-sm">
        <button
          onClick={onToggle}
          className="w-full flex items-center gap-4 p-5 text-left hover:bg-neutral-50 dark:hover:bg-neutral-800/50 transition-colors"
        >
          <div className="flex-shrink-0 w-10 h-10 rounded-lg bg-green-100 dark:bg-green-900/30 flex items-center justify-center">
            <Icon className="w-5 h-5 text-green-600 dark:text-green-400" />
          </div>
          <span className="flex-1 font-medium text-neutral-900 dark:text-neutral-100">
            {item.question}
          </span>
          <ChevronDown 
            className={`w-5 h-5 text-neutral-500 transition-transform duration-200 ${isOpen ? 'rotate-180' : ''}`} 
          />
        </button>
        
        <div 
          className={`overflow-hidden transition-all duration-300 ${isOpen ? 'max-h-[1000px] opacity-100' : 'max-h-0 opacity-0'}`}
        >
          <div className="px-5 pb-5 pt-2 space-y-4">
            <div className="text-neutral-600 dark:text-neutral-400 whitespace-pre-line leading-relaxed">
              {item.answer.split('**').map((part, i) => 
                i % 2 === 1 ? <strong key={i} className="text-neutral-900 dark:text-neutral-100">{part}</strong> : part
              )}
            </div>
            
            {item.images.length > 0 && (
              <div className={`grid gap-4 ${item.images.length === 1 ? 'grid-cols-1' : item.images.length === 2 ? 'grid-cols-1 md:grid-cols-2' : 'grid-cols-1 md:grid-cols-2 lg:grid-cols-3'}`}>
                {item.images.map((src, idx) => (
                  <button
                    key={idx}
                    onClick={() => setLightboxImage({ src, alt: `${item.question} - Screenshot ${idx + 1}` })}
                    className="relative group rounded-lg overflow-hidden border border-neutral-200 dark:border-neutral-700 shadow-sm cursor-zoom-in focus:outline-none focus:ring-2 focus:ring-green-500 focus:ring-offset-2"
                  >
                    <Image
                      src={src}
                      alt={`${item.question} - Screenshot ${idx + 1}`}
                      width={600}
                      height={400}
                      className="w-full h-auto object-contain bg-neutral-100 dark:bg-neutral-800 transition-transform duration-300 group-hover:scale-[1.02]"
                    />
                    {/* Hover overlay with zoom icon */}
                    <div className="absolute inset-0 bg-black/0 group-hover:bg-black/40 transition-all duration-300 flex items-center justify-center">
                      <div className="opacity-0 group-hover:opacity-100 transition-opacity duration-300 bg-white/90 dark:bg-neutral-800/90 p-3 rounded-full shadow-lg">
                        <ZoomIn className="w-6 h-6 text-neutral-700 dark:text-neutral-200" />
                      </div>
                    </div>
                  </button>
                ))}
              </div>
            )}
          </div>
        </div>
      </div>
      
      {/* Lightbox */}
      {lightboxImage && (
        <ImageLightbox
          src={lightboxImage.src}
          alt={lightboxImage.alt}
          isOpen={!!lightboxImage}
          onClose={() => setLightboxImage(null)}
        />
      )}
    </>
  );
}

// Google Fonts URL for Cedarville Cursive - using woff2 for better performance
const CEDARVILLE_FONT_URL = 'https://fonts.gstatic.com/s/cedarvillecursive/v17/yYL00g_a2veiudhUmxjo5VKkoqA-B_neJbBxw8BeTg.woff2';

// Memoized Rayquaza component to prevent re-renders during animation
const RayquazaAnimation = memo(function RayquazaAnimation({ animationKey }: { animationKey: number }) {
  return (
    <div
      key={animationKey}
      className="fixed pointer-events-none"
      style={{
        top: '30%',
        left: 0,
        zIndex: 9999,
        animation: 'rayquaza-fly 5s cubic-bezier(0.25, 0.46, 0.45, 0.94) forwards',
        willChange: 'transform, opacity',
      }}
    >
      <Image
        src="/rayquaza.png"
        alt="Rayquaza"
        width={80}
        height={80}
        className="drop-shadow-lg opacity-90"
        style={{ filter: 'drop-shadow(0 4px 12px rgba(0,0,0,0.3))' }}
        priority
      />
    </div>
  );
});

export default function HomePage() {
  const [showRayquaza, setShowRayquaza] = useState(false);
  const [fontLoaded, setFontLoaded] = useState(false);
  const [portalContainer, setPortalContainer] = useState<HTMLElement | null>(null);
  const [openFAQ, setOpenFAQ] = useState<string | null>(null);
  const fontLoadAttempted = useRef(false);

  const { isAuthenticated, isLoading } = useAuth();

  // Set up portal container on mount
  useEffect(() => {
    setPortalContainer(document.body);
  }, []);

  // Load Cedarville Cursive font using FontFace API for precise loading detection
  useEffect(() => {
    // Prevent double loading in strict mode
    if (fontLoadAttempted.current) return;
    fontLoadAttempted.current = true;

    // Check if font is already loaded (e.g., from cache)
    if (document.fonts.check('1em "Cedarville Cursive"')) {
      setFontLoaded(true);
      return;
    }

    // Create FontFace and load it
    const font = new FontFace(
      'Cedarville Cursive',
      `url(${CEDARVILLE_FONT_URL})`,
      { style: 'normal', weight: '400' }
    );

    font.load().then((loadedFont) => {
      // Add font to document
      document.fonts.add(loadedFont);
      setFontLoaded(true);
    }).catch((err) => {
      console.error('Failed to load Cedarville Cursive font:', err);
      // Still show the text with fallback font after error
      setFontLoaded(true);
    });
  }, []);

  // Easter egg - Rayquaza appears on 5 clicks on title
  // Using refs to completely avoid re-render issues during animation
  const isAnimating = useRef(false);
  const clickCountRef = useRef(0);
  const clickResetTimer = useRef<NodeJS.Timeout | null>(null);
  const rayquazaKeyRef = useRef(0);
  const [rayquazaKey, setRayquazaKey] = useState(0);
  const CLICK_RESET_MS = 2000;
  
  const handleTitleClick = () => {
    // Completely ignore clicks while animation is playing
    if (isAnimating.current) return;
    
    // Clear any existing reset timer
    if (clickResetTimer.current) {
      clearTimeout(clickResetTimer.current);
      clickResetTimer.current = null;
    }
    
    clickCountRef.current += 1;
    
    if (clickCountRef.current >= 5) {
      // Lock and trigger animation
      isAnimating.current = true;
      clickCountRef.current = 0;
      rayquazaKeyRef.current += 1;
      setRayquazaKey(rayquazaKeyRef.current);
      setShowRayquaza(true);
      
      // Hide after animation completes (5s animation + 500ms buffer)
      setTimeout(() => {
        setShowRayquaza(false);
        // Small delay before allowing new triggers
        setTimeout(() => {
          isAnimating.current = false;
        }, 500);
      }, 5500);
      return;
    }
    
    // Reset click count after inactivity
    clickResetTimer.current = setTimeout(() => {
      clickCountRef.current = 0;
    }, CLICK_RESET_MS);
  };

  return (
    <>
      <style jsx>{`
        .cedarville-cursive {
          font-family: "Cedarville Cursive", cursive;
          font-weight: 400;
          font-style: normal;
        }
        
        /* Font loading skeleton animation */
        .font-skeleton {
          position: relative;
          overflow: hidden;
        }
        
        .font-skeleton-bar {
          height: 0.8em;
          border-radius: 0.4em;
          background: linear-gradient(
            90deg,
            rgba(34, 197, 94, 0.2) 0%,
            rgba(34, 197, 94, 0.4) 50%,
            rgba(34, 197, 94, 0.2) 100%
          );
          background-size: 200% 100%;
          animation: shimmer 1.5s ease-in-out infinite;
        }
        
        @keyframes shimmer {
          0% {
            background-position: 200% 0;
          }
          100% {
            background-position: -200% 0;
          }
        }
        
        /* Fade in animation for loaded font */
        .font-loaded {
          animation: fadeIn 0.5s ease-out forwards;
        }
        
        @keyframes fadeIn {
          from {
            opacity: 0;
            transform: rotate(-5deg) scale(0.95);
          }
          to {
            opacity: 1;
            transform: rotate(-5deg) scale(1);
          }
        }
      `}</style>

      <div className="min-h-screen bg-white dark:bg-black">
      {/* Navigation */}
      <nav className="border-b border-neutral-200 dark:border-neutral-800">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex justify-between items-center h-16">
            <div className="flex items-center gap-2">
              <span className="font-semibold text-lg">Study in Woods 🪵</span>
            </div>

            <div className="flex items-center gap-3">
              {/* GitHub Repository Link */}
              <a
                href="https://github.com/sahilchouksey/study-in-woods"
                target="_blank"
                rel="noopener noreferrer"
                className="p-2 rounded-md hover:bg-neutral-100 dark:hover:bg-neutral-800 transition-colors"
                title="View on GitHub"
              >
                <svg
                  viewBox="0 0 24 24"
                  className="h-5 w-5 text-neutral-700 dark:text-neutral-300"
                  fill="currentColor"
                >
                  <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
                </svg>
              </a>
              <ThemeToggle />
              {!isLoading && (
                <>
                  {isAuthenticated ? (
                    <Link href="/dashboard">
                      <Button>Go to Dashboard</Button>
                    </Link>
                  ) : (
                    <>
                      <Link href="/login">
                        <Button variant="outline">Login</Button>
                      </Link>
                      <Link href="/register">
                        <Button>Signup</Button>
                      </Link>
                    </>
                  )}
                </>
              )}
            </div>
          </div>
        </div>
      </nav>

      {/* Hero Section with Pixelated Forest Background */}
      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="relative py-12 sm:py-16 h-[calc(100vh-4rem)] max-h-[1066px] flex items-center">
          {/* Pixelated Forest Background */}
          <div
            className="absolute inset-0 rounded-3xl overflow-hidden border border-neutral-200 dark:border-neutral-800 bg-cover bg-center bg-no-repeat pixelated-bg"
            style={{
              zIndex: 0,
              backgroundImage: 'url(/woods-background.png)',
            }}
          >


            {/* Overlay to make content readable */}
            <div className="absolute inset-0 bg-white/70 dark:bg-black/70 pointer-events-none" />
          </div>

          <div className="relative w-full space-y-8 sm:space-y-12">

            {/* Rayquaza Easter Egg - rendered via portal to isolate from main component tree */}
            {showRayquaza && portalContainer && createPortal(
              <RayquazaAnimation animationKey={rayquazaKey} />,
              portalContainer
            )}



            {/* Heading with Cursive Font - Rotated with Woods Texture & Sparkles */}
            <div className="text-center space-y-4 relative ">
              {!fontLoaded ? (
                /* Skeleton loader while font is loading */
                <div 
                  className="font-skeleton flex flex-col items-center gap-2 sm:gap-3"
                  style={{ transform: 'rotate(-5deg)' }}
                >
                  {/* "Study in" skeleton */}
                  <div className="font-skeleton-bar w-[280px] sm:w-[380px] lg:w-[520px] h-[60px] sm:h-[80px] lg:h-[120px]" />
                  {/* "Woods" skeleton */}
                  <div className="font-skeleton-bar w-[200px] sm:w-[280px] lg:w-[380px] h-[60px] sm:h-[80px] lg:h-[120px]" />
                </div>
              ) : (
                <h1
                  className="text-7xl sm:text-8xl lg:text-[10rem] cedarville-cursive font-black woods-text-effect leading-none cursor-pointer select-none font-loaded"
                  style={{ transform: 'rotate(-5deg)' }}
                  onClick={handleTitleClick}
                >
                  <span>
                    Study in Woods
                  </span>
                  {/* Sparkles */}
                  <span className="sparkle" style={{ top: '10%', left: '15%', animationDelay: '0s' }}></span>
                  <span className="sparkle" style={{ top: '20%', right: '20%', animationDelay: '0.3s' }}></span>
                  <span className="sparkle" style={{ bottom: '25%', left: '25%', animationDelay: '0.6s' }}></span>
                  <span className="sparkle" style={{ top: '30%', right: '15%', animationDelay: '0.9s' }}></span>
                  <span className="sparkle" style={{ bottom: '15%', right: '30%', animationDelay: '1.2s' }}></span>
                </h1>
              )}
            </div>

          </div>
        </div>

        {/* FAQ Section */}
        <section className="py-16 sm:py-24">
          <div className="text-center mb-12">
            <h2 className="text-3xl sm:text-4xl font-bold text-neutral-900 dark:text-neutral-100 mb-4">
              Frequently Asked Questions
            </h2>
            <p className="text-neutral-600 dark:text-neutral-400 max-w-2xl mx-auto">
              Learn how to get the most out of Study in Woods with these common questions about features and capabilities.
            </p>
          </div>
          
          <div className="max-w-3xl mx-auto space-y-4">
            {faqData.map((item) => (
              <FAQItem
                key={item.id}
                item={item}
                isOpen={openFAQ === item.id}
                onToggle={() => setOpenFAQ(openFAQ === item.id ? null : item.id)}
              />
            ))}
          </div>
        </section>

        {/* Footer */}
        <footer className="border-t border-neutral-200 dark:border-neutral-800 py-8">
          <div className="text-center space-y-2">
            <div className="text-sm text-neutral-600 dark:text-neutral-400">
              © 2025 Study in Woods. All rights reserved.
            </div>
            <div className="text-xs text-neutral-500">
              Made by{' '}
              <a
                href="https://sahilchouksey.in"
                target="_blank"
                rel="noopener noreferrer"
                className="font-medium text-neutral-700 dark:text-neutral-300 hover:text-neutral-900 dark:hover:text-neutral-100 transition-colors"
              >
                @sahilchouksey
              </a>
            </div>
          </div>
        </footer>
      </main>
    </div>
    </>
  );
}
