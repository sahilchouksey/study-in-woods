# Phase 12: Database Seeding

## Overview
Phase 12 implements comprehensive database seeding functionality to populate the database with initial data including admin users, universities, courses, subjects, and application settings.

## Features Implemented

### 1. Seed Script (`database/seed.go`)
- **Idempotent seeding**: All seed functions check if data exists before insertion
- **Structured seeding**: Seeds are run in correct order respecting foreign key constraints
- **Error handling**: Comprehensive error reporting for failed seeds
- **Logging**: Clear console output showing seeding progress

### 2. Seed Data Includes:

#### Admin User
- **Email**: `admin@studyinwoods.com`
- **Password**: `Admin123!`
- **Role**: admin
- **Note**: ⚠️ Change password after first login!

#### Universities (5)
1. Dr. A.P.J. Abdul Kalam Technical University (AKTU) - Lucknow, UP
2. University of Delhi (DU) - Delhi
3. Jawaharlal Nehru University (JNU) - New Delhi
4. Banaras Hindu University (BHU) - Varanasi, UP
5. Indian Institute of Technology Kanpur (IITK) - Kanpur, UP

#### Courses (12 across universities)
- **AKTU**: MCA, BCA, BTECH-CS
- **DU**: BSC-CS, MSC-CS
- **JNU**: MCA-JNU, MTECH-CS
- **BHU**: BCA-BHU, BTECH-IT
- **IITK**: BTECH-CS-IITK, MTECH-CS-IITK, DD-CS-IITK

#### Semesters
- Automatically generated for all courses based on their duration
- Total: 66 semesters across all courses

#### Subjects (20 for MCA)
- **Semester 1**: Programming in C, Computer Organization, Discrete Mathematics, DBMS, Communication Skills
- **Semester 2**: Data Structures, OOP with C++, Computer Networks, Operating Systems, Software Engineering
- **Semester 3**: Algorithms, Web Technologies, Theory of Computation, Python Programming, Cyber Security
- **Semester 4**: Machine Learning, Cloud Computing, Mobile App Development, Big Data Analytics, Artificial Intelligence

#### Application Settings (19 settings)
- **System**: name, version, maintenance_mode
- **Rate Limits**: API requests, chat messages, document uploads
- **Features**: registration, chat, document upload, external API
- **Upload**: max file size, allowed extensions
- **Chat**: max message length, session timeout
- **Security**: password length, JWT expiry, login attempts, lockout duration
- **Analytics**: retention days

## Usage

### Method 1: Make Command (Recommended)
```bash
make db-seed
```

### Method 2: Direct Execution
```bash
go run cmd/seed/main.go
```

### Method 3: Build and Run
```bash
go build -o bin/seed cmd/seed/main.go
./bin/seed
```

## Prerequisites
- Database must be running and accessible
- Environment variables configured in `.env`:
  ```env
  DB_HOST=localhost
  DB_PORT=5432
  DB_NAME=study_in_woods
  DB_USER_NAME=postgres
  DB_PASSWORD=yourpassword
  DB_SSL_MODE=disable
  ```
- Database migrations must be run first: `make db-migrate`

## Expected Output
```
============================================================
Study in Woods - Database Seeding
============================================================

🌱 Starting database seeding...
✅ Created admin user: admin@studyinwoods.com (password: Admin123!)
✅ Created 5 universities
✅ Created 12 courses
✅ Created 66 semesters
✅ Created 20 subjects
✅ Created 19 app settings
✅ Database seeding completed successfully!

============================================================
🎉 Seeding completed successfully!
============================================================

Default Admin Credentials:
  Email:    admin@studyinwoods.com
  Password: Admin123!

⚠️  Please change the admin password after first login!
```

## Idempotency
The seeder is **fully idempotent** - you can run it multiple times safely:
- If admin user exists: skips creation
- If universities exist: skips creation
- If courses exist: skips creation
- If semesters exist: skips creation
- If subjects exist: skips creation
- If app settings exist: skips creation

This makes it safe to run in production without duplicating data.

## File Structure
```
study-in-woods/
├── database/
│   └── seed.go              # Main seeding logic
├── cmd/
│   └── seed/
│       └── main.go          # CLI entry point
└── Makefile                 # Updated with db-seed command
```

## Implementation Details

### Seeder Structure
```go
type Seeder struct {
    db *gorm.DB
}

func (s *Seeder) SeedAll() error {
    // Seeds in order:
    // 1. Admin User
    // 2. Universities
    // 3. Courses
    // 4. Semesters
    // 5. Subjects
    // 6. App Settings
}
```

### Key Functions
- `SeedAdminUser()` - Creates default admin account
- `SeedUniversities()` - Creates 5 sample universities
- `SeedCourses()` - Creates 12 courses across universities
- `SeedSemesters()` - Auto-generates semesters for all courses
- `SeedSubjects()` - Creates subjects for MCA Semester 1-4
- `SeedAppSettings()` - Creates 19 configuration settings

## Development Notes

### Adding More Seed Data
To add more seed data, extend the respective functions in `database/seed.go`:

```go
// Example: Add more subjects
func (s *Seeder) SeedSubjects() error {
    // ... existing code ...
    
    // Add subjects for Semester 5-6
    subjectsBySemester[5] = []model.Subject{
        {Name: "...", Code: "...", Credits: 4, Description: "..."},
    }
}
```

### Subject AI Setup
**Important**: The seeder creates subjects WITHOUT AI configuration:
- `KnowledgeBaseUUID`: empty (should be set via API after document uploads)
- `AgentUUID`: empty (should be set via API when creating AI agent)

Use the Subject API endpoints to configure AI after seeding:
```bash
# 1. Create subject with AI
POST /api/v1/semesters/:semester_id/subjects
{
  "name": "...",
  "code": "...",
  "credits": 4,
  "description": "..."
}

# 2. Upload documents
POST /api/v1/subjects/:subject_id/documents

# 3. Subject will auto-configure AI via DigitalOcean integration
```

## Testing
To verify seeding worked correctly:

```bash
# Check admin user
psql -d study_in_woods -c "SELECT email, name, role FROM users WHERE role='admin';"

# Check universities
psql -d study_in_woods -c "SELECT name, code FROM universities;"

# Check courses count
psql -d study_in_woods -c "SELECT COUNT(*) FROM courses;"

# Check semesters count
psql -d study_in_woods -c "SELECT COUNT(*) FROM semesters;"

# Check subjects count
psql -d study_in_woods -c "SELECT COUNT(*) FROM subjects;"

# Check app settings
psql -d study_in_woods -c "SELECT key, value, category FROM app_settings ORDER BY category, key;"
```

## Security Considerations
1. **Change default admin password** immediately after first deployment
2. Consider using environment variables for admin credentials in production
3. The default password `Admin123!` meets minimum security requirements but should be changed
4. Admin creation is logged and can be audited via system logs

## Production Deployment
For production deployments:

1. **First-time setup**:
   ```bash
   make db-migrate  # Run migrations
   make db-seed     # Seed initial data
   ```

2. **Update `.env` with production values** before seeding

3. **Change admin password** via API after seeding:
   ```bash
   POST /api/v1/auth/change-password
   Authorization: Bearer <admin-jwt-token>
   {
     "old_password": "Admin123!",
     "new_password": "<secure-password>"
   }
   ```

## Troubleshooting

### Error: "no universities found"
- Ensure migrations ran successfully first
- Check database connection in `.env`

### Error: "Failed to connect to database"
- Verify database is running
- Check `.env` configuration
- Ensure PostgreSQL is accessible

### Duplicate key errors
- The seeder is idempotent and should skip existing data
- If you see duplicate errors, check the idempotency logic

### Wrong admin credentials
- Default: `admin@studyinwoods.com` / `Admin123!`
- Case-sensitive email and password
- Check if password was already changed

## Future Enhancements
Potential improvements for future iterations:
- [ ] Support for custom seed data via JSON files
- [ ] CLI flags for selective seeding (`--only=users,universities`)
- [ ] Seed data versioning
- [ ] Production-ready admin credentials from environment variables
- [ ] More diverse sample data (multiple admin users, test students)
- [ ] Sample chat sessions and documents for testing

---

**Phase 12 Status**: ✅ Complete
**Total Lines**: 478 lines across 2 files
**Files Created**: 2 (database/seed.go, cmd/seed/main.go)
**Files Modified**: 1 (Makefile)
