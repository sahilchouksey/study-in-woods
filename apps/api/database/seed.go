package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/utils/auth"
	"gorm.io/gorm"
)

// Seeder handles database seeding operations
type Seeder struct {
	db *gorm.DB
}

// NewSeeder creates a new seeder instance
func NewSeeder(db *gorm.DB) *Seeder {
	return &Seeder{db: db}
}

// SeedAll runs all seed functions
func (s *Seeder) SeedAll() error {
	log.Println("🌱 Starting database seeding...")

	// Run seeds in order (respecting foreign key constraints)
	if err := s.SeedAdminUser(); err != nil {
		return fmt.Errorf("failed to seed admin user: %w", err)
	}

	if err := s.SeedUniversities(); err != nil {
		return fmt.Errorf("failed to seed universities: %w", err)
	}

	if err := s.SeedCourses(); err != nil {
		return fmt.Errorf("failed to seed courses: %w", err)
	}

	if err := s.SeedSemesters(); err != nil {
		return fmt.Errorf("failed to seed semesters: %w", err)
	}

	if err := s.SeedSubjects(); err != nil {
		return fmt.Errorf("failed to seed subjects: %w", err)
	}

	if err := s.SeedAppSettings(); err != nil {
		return fmt.Errorf("failed to seed app settings: %w", err)
	}

	log.Println("✅ Database seeding completed successfully!")
	return nil
}

// SeedAdminUser creates the default admin user
func (s *Seeder) SeedAdminUser() error {
	// Check if admin already exists
	var count int64
	if err := s.db.Model(&model.User{}).Where("role = ?", "admin").Count(&count).Error; err != nil {
		return err
	}

	if count > 0 {
		log.Println("⏭️  Admin user already exists, skipping...")
		return nil
	}

	// Get admin credentials from environment variables
	adminEmail := os.Getenv("ADMIN_EMAIL")
	adminPassword := os.Getenv("ADMIN_PASSWORD")

	if adminEmail == "" || adminPassword == "" {
		log.Println("⚠️  ADMIN_EMAIL and ADMIN_PASSWORD environment variables not set, skipping admin user creation")
		return nil
	}

	// Hash password
	passwordHash, err := auth.HashPassword(adminPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	admin := &model.User{
		Email:        adminEmail,
		PasswordHash: passwordHash,
		PasswordSalt: []byte("legacy_salt"), // bcrypt handles salt internally
		Name:         "System Administrator",
		Role:         "admin",
		Semester:     0, // Admin doesn't have semester
		TokenVersion: 0,
	}

	if err := s.db.Create(admin).Error; err != nil {
		return err
	}

	log.Printf("✅ Created admin user: %s\n", admin.Email)
	return nil
}

// SeedUniversities creates sample universities
func (s *Seeder) SeedUniversities() error {
	// Check if universities already exist
	var count int64
	if err := s.db.Model(&model.University{}).Count(&count).Error; err != nil {
		return err
	}

	if count > 0 {
		log.Println("⏭️  Universities already exist, skipping...")
		return nil
	}

	universities := []model.University{
		{
			Name:     "Dr. A.P.J. Abdul Kalam Technical University",
			Code:     "AKTU",
			Location: "Lucknow, Uttar Pradesh",
			Website:  "https://aktu.ac.in",
			IsActive: true,
		},
		{
			Name:     "University of Delhi",
			Code:     "DU",
			Location: "Delhi",
			Website:  "https://du.ac.in",
			IsActive: true,
		},
		{
			Name:     "Jawaharlal Nehru University",
			Code:     "JNU",
			Location: "New Delhi",
			Website:  "https://jnu.ac.in",
			IsActive: true,
		},
		{
			Name:     "Banaras Hindu University",
			Code:     "BHU",
			Location: "Varanasi, Uttar Pradesh",
			Website:  "https://bhu.ac.in",
			IsActive: true,
		},
		{
			Name:     "Indian Institute of Technology Kanpur",
			Code:     "IITK",
			Location: "Kanpur, Uttar Pradesh",
			Website:  "https://iitk.ac.in",
			IsActive: true,
		},
	}

	if err := s.db.Create(&universities).Error; err != nil {
		return err
	}

	log.Printf("✅ Created %d universities\n", len(universities))
	return nil
}

// SeedCourses creates sample courses
func (s *Seeder) SeedCourses() error {
	// Check if courses already exist
	var count int64
	if err := s.db.Model(&model.Course{}).Count(&count).Error; err != nil {
		return err
	}

	if count > 0 {
		log.Println("⏭️  Courses already exist, skipping...")
		return nil
	}

	// Get universities
	var universities []model.University
	if err := s.db.Find(&universities).Error; err != nil {
		return err
	}

	if len(universities) == 0 {
		return fmt.Errorf("no universities found, seed universities first")
	}

	courses := []model.Course{
		// AKTU Courses
		{
			UniversityID: universities[0].ID,
			Name:         "Master of Computer Applications",
			Code:         "MCA",
			Description:  "3-year postgraduate program in computer applications",
			Duration:     6,
		},
		{
			UniversityID: universities[0].ID,
			Name:         "Bachelor of Computer Applications",
			Code:         "BCA",
			Description:  "3-year undergraduate program in computer applications",
			Duration:     6,
		},
		{
			UniversityID: universities[0].ID,
			Name:         "Bachelor of Technology - Computer Science",
			Code:         "BTECH-CS",
			Description:  "4-year undergraduate engineering program",
			Duration:     8,
		},
		// DU Courses
		{
			UniversityID: universities[1].ID,
			Name:         "Bachelor of Science - Computer Science",
			Code:         "BSC-CS",
			Description:  "3-year undergraduate science program",
			Duration:     6,
		},
		{
			UniversityID: universities[1].ID,
			Name:         "Master of Science - Computer Science",
			Code:         "MSC-CS",
			Description:  "2-year postgraduate science program",
			Duration:     4,
		},
		// JNU Courses
		{
			UniversityID: universities[2].ID,
			Name:         "Master of Computer Applications",
			Code:         "MCA-JNU",
			Description:  "3-year MCA program at JNU",
			Duration:     6,
		},
		{
			UniversityID: universities[2].ID,
			Name:         "Master of Technology - Computer Science",
			Code:         "MTECH-CS",
			Description:  "2-year MTech program",
			Duration:     4,
		},
		// BHU Courses
		{
			UniversityID: universities[3].ID,
			Name:         "Bachelor of Computer Applications",
			Code:         "BCA-BHU",
			Description:  "3-year BCA program at BHU",
			Duration:     6,
		},
		{
			UniversityID: universities[3].ID,
			Name:         "Bachelor of Technology - Information Technology",
			Code:         "BTECH-IT",
			Description:  "4-year BTech in Information Technology",
			Duration:     8,
		},
		// IITK Courses
		{
			UniversityID: universities[4].ID,
			Name:         "Bachelor of Technology - Computer Science",
			Code:         "BTECH-CS-IITK",
			Description:  "4-year BTech at IIT Kanpur",
			Duration:     8,
		},
		{
			UniversityID: universities[4].ID,
			Name:         "Master of Technology - Computer Science",
			Code:         "MTECH-CS-IITK",
			Description:  "2-year MTech at IIT Kanpur",
			Duration:     4,
		},
		{
			UniversityID: universities[4].ID,
			Name:         "Dual Degree - Computer Science",
			Code:         "DD-CS-IITK",
			Description:  "5-year integrated BTech + MTech program",
			Duration:     10,
		},
	}

	if err := s.db.Create(&courses).Error; err != nil {
		return err
	}

	log.Printf("✅ Created %d courses\n", len(courses))
	return nil
}

// SeedSemesters creates semesters for all courses
func (s *Seeder) SeedSemesters() error {
	// Check if semesters already exist
	var count int64
	if err := s.db.Model(&model.Semester{}).Count(&count).Error; err != nil {
		return err
	}

	if count > 0 {
		log.Println("⏭️  Semesters already exist, skipping...")
		return nil
	}

	// Get all courses
	var courses []model.Course
	if err := s.db.Find(&courses).Error; err != nil {
		return err
	}

	if len(courses) == 0 {
		return fmt.Errorf("no courses found, seed courses first")
	}

	var semesters []model.Semester
	for _, course := range courses {
		for i := 1; i <= course.Duration; i++ {
			semester := model.Semester{
				CourseID: course.ID,
				Number:   i,
				Name:     fmt.Sprintf("Semester %d", i),
			}
			semesters = append(semesters, semester)
		}
	}

	if err := s.db.Create(&semesters).Error; err != nil {
		return err
	}

	log.Printf("✅ Created %d semesters\n", len(semesters))
	return nil
}

// SeedSubjects creates sample subjects for semesters
func (s *Seeder) SeedSubjects() error {
	// Check if subjects already exist
	var count int64
	if err := s.db.Model(&model.Subject{}).Count(&count).Error; err != nil {
		return err
	}

	if count > 0 {
		log.Println("⏭️  Subjects already exist, skipping...")
		return nil
	}

	// Get MCA course semesters from AKTU (first university, first course)
	var semesters []model.Semester
	if err := s.db.Joins("JOIN courses ON courses.id = semesters.course_id").
		Where("courses.code = ?", "MCA").
		Order("semesters.number ASC").
		Limit(6).
		Find(&semesters).Error; err != nil {
		return err
	}

	if len(semesters) == 0 {
		log.Println("⏭️  No MCA semesters found, skipping subject seeding...")
		return nil
	}

	// Sample subjects for first 4 semesters of MCA
	subjectsBySemester := map[int][]model.Subject{
		1: {
			{Name: "Programming in C", Code: "MCA101", Credits: 4, Description: "Introduction to C programming language"},
			{Name: "Computer Organization", Code: "MCA102", Credits: 4, Description: "Computer architecture and organization"},
			{Name: "Discrete Mathematics", Code: "MCA103", Credits: 4, Description: "Mathematical foundations for computer science"},
			{Name: "Database Management Systems", Code: "MCA104", Credits: 4, Description: "Fundamentals of database systems"},
			{Name: "Communication Skills", Code: "MCA105", Credits: 3, Description: "Technical communication and soft skills"},
		},
		2: {
			{Name: "Data Structures", Code: "MCA201", Credits: 4, Description: "Linear and non-linear data structures"},
			{Name: "Object Oriented Programming with C++", Code: "MCA202", Credits: 4, Description: "OOP concepts using C++"},
			{Name: "Computer Networks", Code: "MCA203", Credits: 4, Description: "Network protocols and architectures"},
			{Name: "Operating Systems", Code: "MCA204", Credits: 4, Description: "OS concepts and implementation"},
			{Name: "Software Engineering", Code: "MCA205", Credits: 3, Description: "Software development lifecycle and methodologies"},
		},
		3: {
			{Name: "Design and Analysis of Algorithms", Code: "MCA301", Credits: 4, Description: "Algorithm design techniques and complexity analysis"},
			{Name: "Web Technologies", Code: "MCA302", Credits: 4, Description: "HTML, CSS, JavaScript, and web development"},
			{Name: "Theory of Computation", Code: "MCA303", Credits: 4, Description: "Automata theory and formal languages"},
			{Name: "Python Programming", Code: "MCA304", Credits: 4, Description: "Python programming and applications"},
			{Name: "Cyber Security", Code: "MCA305", Credits: 3, Description: "Information security fundamentals"},
		},
		4: {
			{Name: "Machine Learning", Code: "MCA401", Credits: 4, Description: "ML algorithms and applications"},
			{Name: "Cloud Computing", Code: "MCA402", Credits: 4, Description: "Cloud platforms and services"},
			{Name: "Mobile Application Development", Code: "MCA403", Credits: 4, Description: "Android/iOS app development"},
			{Name: "Big Data Analytics", Code: "MCA404", Credits: 4, Description: "Big data technologies and analytics"},
			{Name: "Artificial Intelligence", Code: "MCA405", Credits: 3, Description: "AI concepts and techniques"},
		},
	}

	var allSubjects []model.Subject
	for i, semester := range semesters {
		if i >= 4 { // Only seed first 4 semesters
			break
		}

		subjects := subjectsBySemester[i+1]
		for j := range subjects {
			subjects[j].SemesterID = semester.ID
			// Note: KnowledgeBaseUUID and AgentUUID are empty - they should be set up via API later
		}
		allSubjects = append(allSubjects, subjects...)
	}

	if len(allSubjects) > 0 {
		if err := s.db.Create(&allSubjects).Error; err != nil {
			return err
		}
		log.Printf("✅ Created %d subjects\n", len(allSubjects))
	}

	return nil
}

// SeedAppSettings creates default application settings
func (s *Seeder) SeedAppSettings() error {
	// Check if settings already exist
	var count int64
	if err := s.db.Model(&model.AppSetting{}).Count(&count).Error; err != nil {
		return err
	}

	if count > 0 {
		log.Println("⏭️  App settings already exist, skipping...")
		return nil
	}

	now := time.Now()
	settings := []model.AppSetting{
		// System Information
		{
			Key:         "system.name",
			Value:       "Study in Woods",
			Type:        "string",
			Description: "Application name",
			IsPublic:    true,
			Category:    "system",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			Key:         "system.version",
			Value:       "1.0.0",
			Type:        "string",
			Description: "Current application version",
			IsPublic:    true,
			Category:    "system",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			Key:         "system.maintenance_mode",
			Value:       "false",
			Type:        "bool",
			Description: "Enable maintenance mode to restrict access",
			IsPublic:    true,
			Category:    "system",
			CreatedAt:   now,
			UpdatedAt:   now,
		},

		// Rate Limiting
		{
			Key:         "rate_limit.api.requests_per_minute",
			Value:       "60",
			Type:        "int",
			Description: "Maximum API requests per minute per user",
			IsPublic:    false,
			Category:    "rate_limit",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			Key:         "rate_limit.chat.messages_per_hour",
			Value:       "100",
			Type:        "int",
			Description: "Maximum chat messages per hour per user",
			IsPublic:    false,
			Category:    "rate_limit",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			Key:         "rate_limit.document.uploads_per_day",
			Value:       "50",
			Type:        "int",
			Description: "Maximum document uploads per day per user",
			IsPublic:    false,
			Category:    "rate_limit",
			CreatedAt:   now,
			UpdatedAt:   now,
		},

		// Feature Flags
		{
			Key:         "feature.registration_enabled",
			Value:       "true",
			Type:        "bool",
			Description: "Allow new user registrations",
			IsPublic:    true,
			Category:    "feature",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			Key:         "feature.chat_enabled",
			Value:       "true",
			Type:        "bool",
			Description: "Enable chat functionality",
			IsPublic:    true,
			Category:    "feature",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			Key:         "feature.document_upload_enabled",
			Value:       "true",
			Type:        "bool",
			Description: "Enable document uploads",
			IsPublic:    true,
			Category:    "feature",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			Key:         "feature.external_api_enabled",
			Value:       "true",
			Type:        "bool",
			Description: "Enable external API key generation",
			IsPublic:    false,
			Category:    "feature",
			CreatedAt:   now,
			UpdatedAt:   now,
		},

		// File Upload Limits
		{
			Key:         "upload.max_file_size_mb",
			Value:       "10",
			Type:        "int",
			Description: "Maximum file size for uploads in MB",
			IsPublic:    true,
			Category:    "upload",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			Key:         "upload.allowed_extensions",
			Value:       "pdf,doc,docx,txt,ppt,pptx",
			Type:        "string",
			Description: "Comma-separated list of allowed file extensions",
			IsPublic:    true,
			Category:    "upload",
			CreatedAt:   now,
			UpdatedAt:   now,
		},

		// Chat Settings
		{
			Key:         "chat.max_message_length",
			Value:       "2000",
			Type:        "int",
			Description: "Maximum characters per chat message",
			IsPublic:    true,
			Category:    "chat",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			Key:         "chat.session_timeout_minutes",
			Value:       "30",
			Type:        "int",
			Description: "Chat session timeout in minutes",
			IsPublic:    false,
			Category:    "chat",
			CreatedAt:   now,
			UpdatedAt:   now,
		},

		// Security Settings
		{
			Key:         "security.password_min_length",
			Value:       "8",
			Type:        "int",
			Description: "Minimum password length",
			IsPublic:    true,
			Category:    "security",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			Key:         "security.jwt_expiry_hours",
			Value:       "24",
			Type:        "int",
			Description: "JWT token expiry time in hours",
			IsPublic:    false,
			Category:    "security",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			Key:         "security.max_login_attempts",
			Value:       "5",
			Type:        "int",
			Description: "Maximum failed login attempts before lockout",
			IsPublic:    false,
			Category:    "security",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			Key:         "security.lockout_duration_minutes",
			Value:       "15",
			Type:        "int",
			Description: "Account lockout duration after max failed attempts",
			IsPublic:    false,
			Category:    "security",
			CreatedAt:   now,
			UpdatedAt:   now,
		},

		// Analytics
		{
			Key:         "analytics.retention_days",
			Value:       "90",
			Type:        "int",
			Description: "Days to retain analytics data",
			IsPublic:    false,
			Category:    "analytics",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	if err := s.db.Create(&settings).Error; err != nil {
		return err
	}

	log.Printf("✅ Created %d app settings\n", len(settings))
	return nil
}

// RunSeeds is a convenience function to run all seeds
func RunSeeds(db *gorm.DB) error {
	seeder := NewSeeder(db)
	return seeder.SeedAll()
}
