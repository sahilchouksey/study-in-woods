# Study in Woods - Presentation Content

> **Design Notes for AI Presentation Maker:**
> - **Theme**: Black and white only (monochrome)
> - **Background Image**: Use `woods-background.png` (pixelated forest) for the title slide
> - **Font Style**: Clean, modern sans-serif
> - **Accent**: Use subtle gray gradients for visual hierarchy
> - **Total Slides**: 8-10 slides

---

## SLIDE 1: Title Slide
**[Use woods-background.png as background with dark overlay]**

### Study in Woods
**AI-Powered Study Companion Platform**

*Transforming how students learn with intelligent document processing and conversational AI*

---

**Developed by:** Sahil Chouksey  
**Duration:** 6 months (June - December 2024)  
**Project Type:** Minor Project

---

## SLIDE 2: The Problem

### Challenges Students Face Today

- **Scattered Resources**: Study materials spread across multiple platforms
- **Unstructured Syllabi**: PDF syllabi are hard to navigate and search
- **No Contextual Help**: Generic AI tools lack course-specific knowledge
- **Exam Preparation**: Difficulty connecting past questions to syllabus topics
- **Time Wasted**: Hours spent organizing instead of learning

> *"Students need a single platform that understands their curriculum and helps them study smarter, not harder."*

---

## SLIDE 3: Our Solution

### Study in Woods - Key Features

| Feature | Description |
|---------|-------------|
| **AI Syllabus Extraction** | Upload PDF → Auto-extract units, topics, and structure |
| **Smart Knowledge Base** | Documents indexed for instant AI retrieval |
| **Contextual AI Chat** | Ask questions, get answers with citations from your materials |
| **PYQ Integration** | Past year questions linked to syllabus topics |
| **Academic Hierarchy** | University → Course → Semester → Subject organization |

**Core Value**: *Your course materials become an intelligent, searchable knowledge base*

---

## SLIDE 4: System Architecture

### Three-Tier Microservices Architecture

```
┌─────────────────────────────────────────────────┐
│           PRESENTATION LAYER                     │
│        Next.js 15 + React 19 + TypeScript       │
└─────────────────────┬───────────────────────────┘
                      │ HTTPS/REST + SSE
                      ▼
┌─────────────────────────────────────────────────┐
│           APPLICATION LAYER                      │
│           Go 1.24 + Fiber Framework             │
└───────┬─────────┬─────────┬─────────┬───────────┘
        ▼         ▼         ▼         ▼
   PostgreSQL   Redis    DO Spaces   DO AI
   (30+ Tables) (Cache)  (Storage)  (Llama 3.3)
```

**Key Design Decisions:**
- Go backend: 3x faster than Node.js
- PostgreSQL: ACID compliance for academic data
- DigitalOcean: India data center (15-30ms latency)

---

## SLIDE 5: Technology Stack

### Modern, Scalable Technologies

**Frontend**
- Next.js 15 (App Router, SSR)
- React 19 (Server Components)
- TypeScript + Tailwind CSS v4
- TanStack Query (State Management)

**Backend**
- Go 1.24 (High Performance)
- Fiber v2 (Web Framework)
- GORM (ORM)
- JWT + bcrypt (Security)

**Infrastructure**
- PostgreSQL 15 (Primary DB)
- Redis 7 (Cache/Sessions)
- DigitalOcean Spaces (S3 Storage)
- GradientAI (Llama 3.3 70B)

**Total**: 25+ technologies integrated

---

## SLIDE 6: AI-Powered Features

### Intelligent Document Processing

**1. Syllabus Extraction (85% Accuracy)**
```
PDF Upload → OCR Processing → AI Analysis → Structured Data
                                    ↓
                        Units → Topics → Keywords
```

**2. Knowledge Base Integration**
- Documents auto-indexed to vector database
- RAG (Retrieval-Augmented Generation) for accurate responses
- Citations from source materials in every answer

**3. Conversational AI**
- Subject-specific chat sessions
- Real-time streaming responses (SSE)
- Context-aware with conversation memory

**AI Model**: Llama 3.3 70B via DigitalOcean GradientAI

---

## SLIDE 7: Database Design

### 30+ Tables with Academic Hierarchy

**Core Relationships:**
```
University (1) → (N) Course → (N) Semester → (N) Subject
                                                  ↓
                              Documents, Syllabi, Chat Sessions
```

**Key Tables:**
| Category | Tables | Purpose |
|----------|--------|---------|
| Users | 2 | Authentication, Enrollment |
| Academic | 4 | University → Subject hierarchy |
| Documents | 4 | Files, Syllabus extraction |
| Chat | 4 | Sessions, Messages, Memory |
| System | 6 | API keys, Audit logs, Settings |

**Performance**: B-tree indexes, JSONB for citations, Redis caching

---

## SLIDE 8: Key Screens

### User Interface Highlights

**1. Landing Page**
- Pixelated forest theme (black & white)
- Animated title with cursive font
- Easter egg: Rayquaza flies across on 5 clicks!

**2. Dashboard**
- Welcome header with user context
- Quick access to Chat, Courses, History

**3. AI Chat Interface**
- 3-column layout: Sessions | Chat | Citations
- Real-time streaming with markdown support
- Clickable citation badges [[C1]], [[C2]]

**4. Admin Panel**
- User management with role control
- System settings and audit logs

---

## SLIDE 9: Testing & Quality

### Comprehensive Testing Strategy

| Test Type | Coverage | Tools |
|-----------|----------|-------|
| Unit Tests | 76% (Backend), 68% (Frontend) | Go testify, Jest |
| Integration | 86 test cases | Docker Compose |
| E2E | 5 critical journeys | Playwright |
| Performance | 1000 concurrent users | k6 |

**Key Metrics:**
- ✅ 97.8% overall pass rate
- ✅ API response time: 1.2s (p95)
- ✅ Zero high/critical vulnerabilities
- ✅ CI pipeline: 10 minutes

**Security**: JWT RS256, bcrypt, AES-256 encryption, rate limiting

---

## SLIDE 10: Project Summary

### Study in Woods - By the Numbers

| Metric | Value |
|--------|-------|
| **Lines of Code** | ~14,500 (Backend: 8,500, Frontend: 6,000) |
| **API Endpoints** | 96 documented endpoints |
| **Database Tables** | 30+ tables |
| **Development Time** | 6 months (24 weeks) |
| **Development Phases** | 12 major phases |
| **Test Coverage** | 76% backend, 68% frontend |

### Future Scope
- Mobile app (React Native)
- Collaborative study groups
- Spaced repetition flashcards
- Integration with university LMS

---

**Thank You!**

*Questions?*

**Contact**: sahilchouksey.in  
**Repository**: github.com/sahilchouksey/study-in-woods

---

## DESIGN SPECIFICATIONS FOR AI PRESENTATION MAKER

### Color Palette (Black & White Theme)
- **Primary Background**: #FFFFFF (white) or #000000 (black)
- **Primary Text**: #000000 (black) or #FFFFFF (white)
- **Accent Gray**: #666666, #999999, #CCCCCC
- **Borders**: #E5E5E5 (light) or #333333 (dark)

### Typography
- **Headings**: Bold, sans-serif (like Geist Sans)
- **Body**: Regular weight, clean sans-serif
- **Code/Technical**: Monospace (like Geist Mono)

### Visual Elements
- Use simple line icons (no color)
- Tables with clean borders
- Minimal shadows and effects
- High contrast for readability

### Background Image (Slide 1 Only)
- File: `woods-background.png`
- Style: Pixelated forest illustration
- Apply: Dark overlay (70% opacity) for text readability
- Location: `/apps/web/public/woods-background.png`

### Layout Guidelines
- Generous white space
- Left-aligned text (except titles)
- Maximum 6 bullet points per slide
- Use tables for comparisons
- Code blocks in monospace with subtle background
