'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import Image from 'next/image';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion';
import { Search, BookOpen } from 'lucide-react';
import { storePendingQuery } from '@/lib/utils/sessionStorage';
import { useAuth } from '@/providers/auth-provider';
import Link from 'next/link';
import { useQueryState } from 'nuqs';

// Example questions from syllabus
const EXAMPLE_QUESTIONS = [
  "Explain Apriori algorithm in Data Mining",
  "What is difference between Data Warehouse and OLAP?",
  "Describe backpropagation in Neural Networks",
  "What are production systems in AI?",
  "Explain Django MVT architecture",
  "What is difference between TCP and UDP?",
  "Describe AJAX and XMLHttpRequest",
  "What is MapReduce in Hadoop?",
];



export default function HomePage() {
  const [question, setQuestion] = useQueryState('q', { defaultValue: '' });
  const [university, setUniversity] = useState('RGPV University');
  const [course, setCourse] = useState('MCA');
  const [semester, setSemester] = useState('Semester 3');
  const [subject, setSubject] = useState('Distributed Systems');
  const [placeholderIndex, setPlaceholderIndex] = useState(0);
  const [showRayquaza, setShowRayquaza] = useState(false);
  const [clickCount, setClickCount] = useState(0);

  const router = useRouter();
  const { isAuthenticated, isLoading } = useAuth();

  // Load Cedarville Cursive font
  useEffect(() => {
    const link = document.createElement('link');
    link.href = 'https://fonts.googleapis.com/css2?family=Cedarville+Cursive&display=swap';
    link.rel = 'stylesheet';
    document.head.appendChild(link);

    return () => {
      document.head.removeChild(link);
    };
  }, []);

  // Rotate placeholder text every 3 seconds
  useEffect(() => {
    const interval = setInterval(() => {
      setPlaceholderIndex((prev) => (prev + 1) % EXAMPLE_QUESTIONS.length);
    }, 3000);
    return () => clearInterval(interval);
  }, []);



  // Easter egg - Rayquaza appears on 5 clicks on title
  const handleTitleClick = () => {
    setClickCount((prev) => {
      const newCount = prev + 1;
      if (newCount === 5) {
        setShowRayquaza(true);
        setTimeout(() => setShowRayquaza(false), 5000);
        return 0;
      }
      return newCount;
    });
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();

    if (!question.trim()) return;

    // Store question in sessionStorage
    storePendingQuery(question.trim());

    // Redirect to login if not authenticated, otherwise to dashboard
    if (!isAuthenticated) {
      router.push('/login');
    } else {
      router.push('/dashboard');
    }
  };

  return (
    <>
      <style jsx>{`
        .cedarville-cursive {
          font-family: "Cedarville Cursive", cursive;
          font-weight: 400;
          font-style: normal;
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

            {!isLoading && (
              <div className="flex items-center gap-3">
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
              </div>
            )}
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

            {/* Rayquaza Easter Egg */}
            {showRayquaza && (
              <div
                className="fixed pointer-events-none z-50"
                style={{
                  top: '20%',
                  left: 0,
                  animation: 'rayquaza-fly 5s ease-in-out forwards',
                }}
              >
                <Image
                  src="/rayquaza.png"
                  alt="Rayquaza"
                  width={200}
                  height={200}
                  className="drop-shadow-2xl"
                />
              </div>
            )}



            {/* Heading with Cursive Font - Rotated with Woods Texture & Sparkles */}
            <div className="text-center space-y-4 relative ">
              <h1
                className="text-7xl sm:text-8xl lg:text-[10rem] cedarville-cursive font-black woods-text-effect leading-none cursor-pointer select-none "
                style={{ transform: 'rotate(-5deg)' }}
                onClick={handleTitleClick}
              >
                <span >
                    Study in Woods
                </span>
                {/* Sparkles */}
                <span className="sparkle" style={{ top: '10%', left: '15%', animationDelay: '0s' }}></span>
                <span className="sparkle" style={{ top: '20%', right: '20%', animationDelay: '0.3s' }}></span>
                <span className="sparkle" style={{ bottom: '25%', left: '25%', animationDelay: '0.6s' }}></span>
                <span className="sparkle" style={{ top: '30%', right: '15%', animationDelay: '0.9s' }}></span>
                <span className="sparkle" style={{ bottom: '15%', right: '30%', animationDelay: '1.2s' }}></span>
              </h1>
            </div>

            {/* Context Selectors - Equal Width & Disabled */}
            <div className="mt-[56px] max-w-4xl mx-auto">
              <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                <Select value={university} disabled>
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="RGPV University">RGPV University</SelectItem>
                  </SelectContent>
                </Select>

                <Select value={course} disabled>
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="MCA">MCA</SelectItem>
                  </SelectContent>
                </Select>

                <Select value={semester} disabled>
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="Semester 3">Semester 3</SelectItem>
                  </SelectContent>
                </Select>

                <Select value={subject} disabled>
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="Distributed Systems">Distributed Systems</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>

            {/* Search Bar */}
            <form onSubmit={handleSubmit} className="max-w-4xl mx-auto mb-8">
              <div className="relative group">
                <Input
                  type="text"
                  value={question}
                  onChange={(e) => setQuestion(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' && !e.shiftKey) {
                      e.preventDefault();
                      handleSubmit(e);
                    }
                  }}
                  placeholder={EXAMPLE_QUESTIONS[placeholderIndex]}
                  className="h-16 pl-6 pr-14 text-base shadow-lg border-2 focus-visible:shadow-xl transition-all"
                />
                <button
                  type="submit"
                  disabled={!question.trim()}
                  className="absolute right-4 top-1/2 -translate-y-1/2 p-2 rounded-lg transition-all disabled:opacity-30 disabled:cursor-not-allowed enabled:hover:bg-neutral-100 dark:enabled:hover:bg-neutral-800 enabled:active:scale-95"
                  aria-label="Submit question"
                >
                  <Search className="h-5 w-5 text-neutral-600 dark:text-neutral-400" />
                </button>
                {question.trim() && (
                  <div className="absolute left-6 -bottom-7 text-xs text-neutral-500 dark:text-neutral-400 animate-in fade-in duration-200">
                    Press Enter to submit
                  </div>
                )}
              </div>

              {/* Quick Action Question Chips */}
              <div className="flex flex-wrap gap-2 mt-6 justify-center">
                {['Explain Apriori algorithm', 'What is OLAP?', 'Describe Backpropagation', 'What is MapReduce?'].map((quickQ) => (
                  <button
                    key={quickQ}
                    type="button"
                    onClick={() => setQuestion(quickQ)}
                    className="px-4 py-2 rounded-full text-sm bg-white text-black hover:bg-neutral-100 dark:bg-neutral-200 dark:hover:bg-neutral-300 dark:text-black transition-all shadow-md border border-neutral-200 dark:border-neutral-300"
                  >
                    {quickQ}
                  </button>
                ))}
              </div>
            </form>
          </div>
        </div>

        {/* FAQ Section */}
        <div className="py-20 border-t border-neutral-200 dark:border-neutral-800">
          <div className="max-w-3xl mx-auto">
            <h2 className="text-3xl font-bold text-center mb-12">
              Frequently Asked Questions
            </h2>

            <Accordion type="single" collapsible className="w-full space-y-4">
              <AccordionItem value="item-1" className="border border-neutral-200 dark:border-neutral-800 rounded-lg px-6">
                <AccordionTrigger className="text-left font-medium hover:no-underline">
                  How does Study in Woods answer my questions?
                </AccordionTrigger>
                <AccordionContent className="text-neutral-600 dark:text-neutral-400">
                  Study in Woods uses advanced AI technology to analyze your course materials,
                  including lecture notes, textbooks, and previous year question papers. When you
                  ask a question, our AI searches through these materials and provides accurate,
                  context-aware answers specifically tailored to your university, course, and semester.
                  The system understands the context of your syllabus and provides relevant explanations
                  that match your curriculum.
                </AccordionContent>
              </AccordionItem>

              <AccordionItem value="item-2" className="border border-neutral-200 dark:border-neutral-800 rounded-lg px-6">
                <AccordionTrigger className="text-left font-medium hover:no-underline">
                  Where does the data come from?
                </AccordionTrigger>
                <AccordionContent className="text-neutral-600 dark:text-neutral-400">
                  All the data comes from verified academic sources that you and other students upload
                  to the platform. This includes:
                  <ul className="list-disc pl-6 mt-2 space-y-1">
                    <li>Official lecture notes from professors</li>
                    <li>University-approved textbooks and reference materials</li>
                    <li>Previous year question papers (PYQs) with solutions</li>
                    <li>Assignment solutions and study guides</li>
                  </ul>
                  <p className="mt-2">
                    We ensure all content is relevant to your specific university and course by organizing
                    materials by institution, program, semester, and subject. Your data remains secure
                    and is only used to help you and your peers learn better.
                  </p>
                </AccordionContent>
              </AccordionItem>

              <AccordionItem value="item-3" className="border border-neutral-200 dark:border-neutral-800 rounded-lg px-6">
                <AccordionTrigger className="text-left font-medium hover:no-underline">
                  Is my data secure?
                </AccordionTrigger>
                <AccordionContent className="text-neutral-600 dark:text-neutral-400">
                  Yes, absolutely! We take data security seriously. All your documents and personal
                  information are encrypted and stored securely. Your uploaded materials are only
                  accessible to users in the same course context and are never shared with third
                  parties. We comply with all data protection regulations to ensure your privacy.
                </AccordionContent>
              </AccordionItem>

              <AccordionItem value="item-4" className="border border-neutral-200 dark:border-neutral-800 rounded-lg px-6">
                <AccordionTrigger className="text-left font-medium hover:no-underline">
                  Can I upload my own study materials?
                </AccordionTrigger>
                <AccordionContent className="text-neutral-600 dark:text-neutral-400">
                  Yes! You can upload your lecture notes, solved assignments, and study materials to
                  help both yourself and your peers. The more quality content in the system, the better
                  the AI can answer questions. All uploads are reviewed to ensure they're relevant and
                  helpful for the community.
                </AccordionContent>
              </AccordionItem>
            </Accordion>
          </div>
        </div>

        {/* Footer */}
        <footer className="border-t border-neutral-200 dark:border-neutral-800 py-8 mt-20">
          <div className="text-center space-y-3">
            <div className="text-sm text-neutral-600 dark:text-neutral-400">
              © 2025 Study in Woods. All rights reserved.
            </div>
            <div className="flex items-center justify-center gap-3 text-xs text-neutral-500">
              <span>Made by</span>
              <a
                href="https://github.com/sahilchouksey"
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-3 hover:text-neutral-900 dark:hover:text-neutral-100 transition-colors group"
              >
                <pre
                  className="m-0 p-0 group-hover:opacity-100 transition-opacity text-cyan-500 dark:text-cyan-400 whitespace-pre font-bold"
                  style={{
                    fontFamily: "'Courier New', monospace",
                    fontSize: '5.2px',
                    lineHeight: '0.7',
                    opacity: 0.7,
                  }}
                >{`               .
              .=.    .
       ..     :+-    :-
      .=.     -+=.   :+:
      -+:    .=++:   :+=:
     :++:    :+++=.  :++=.
    .=++:   .=++++:  :+++=.
    -+++-   .=++++:  :++++-
   .=+++-   .-+++=.  .+++=.
    -+++-    -+++-.  .=++:
    .-++-    :+++:   .=+=.
     .=+=.   .=+=.   .==.
      :==.   .=+-    .=:
       :=.   .-+:     :
        .     -=.
              ::`}</pre>
                <span className="font-semibold">xix3r</span>
              </a>
            </div>
          </div>
        </footer>
      </main>
    </div>
    </>
  );
}
