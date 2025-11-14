package validation

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
)

var (
	// EmailRegex is a simple email validation regex
	EmailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

	// PasswordMinLength is the minimum password length
	PasswordMinLength = 8
)

// Validator wraps the go-playground validator
type Validator struct {
	validate *validator.Validate
}

// NewValidator creates a new validator instance
func NewValidator() *Validator {
	return &Validator{
		validate: validator.New(),
	}
}

// ValidateStruct validates a struct using struct tags
func (v *Validator) ValidateStruct(s interface{}) error {
	return v.validate.Struct(s)
}

// FormatValidationErrors converts validation errors to a user-friendly format
func FormatValidationErrors(err error) map[string]string {
	errors := make(map[string]string)

	if validationErrs, ok := err.(validator.ValidationErrors); ok {
		for _, e := range validationErrs {
			field := strings.ToLower(e.Field())
			switch e.Tag() {
			case "required":
				errors[field] = fmt.Sprintf("%s is required", e.Field())
			case "email":
				errors[field] = "Invalid email format"
			case "min":
				errors[field] = fmt.Sprintf("%s must be at least %s characters", e.Field(), e.Param())
			case "max":
				errors[field] = fmt.Sprintf("%s must be at most %s characters", e.Field(), e.Param())
			case "gte":
				errors[field] = fmt.Sprintf("%s must be greater than or equal to %s", e.Field(), e.Param())
			case "lte":
				errors[field] = fmt.Sprintf("%s must be less than or equal to %s", e.Field(), e.Param())
			default:
				errors[field] = fmt.Sprintf("%s is invalid", e.Field())
			}
		}
	}

	return errors
}

// ValidateEmail checks if an email is valid
func ValidateEmail(email string) bool {
	if len(email) < 3 || len(email) > 254 {
		return false
	}
	return EmailRegex.MatchString(email)
}

// ValidatePassword checks if a password meets minimum requirements
func ValidatePassword(password string) (bool, []string) {
	errors := []string{}

	if len(password) < PasswordMinLength {
		errors = append(errors, fmt.Sprintf("Password must be at least %d characters", PasswordMinLength))
	}

	// Check for at least one letter
	hasLetter := false
	for _, char := range password {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') {
			hasLetter = true
			break
		}
	}
	if !hasLetter {
		errors = append(errors, "Password must contain at least one letter")
	}

	return len(errors) == 0, errors
}

// ValidatePasswordStrength checks password strength (optional strict validation)
func ValidatePasswordStrength(password string) (bool, []string) {
	errors := []string{}

	if len(password) < PasswordMinLength {
		errors = append(errors, fmt.Sprintf("Password must be at least %d characters", PasswordMinLength))
	}

	hasUpper := false
	hasLower := false
	hasNumber := false
	hasSpecial := false

	for _, char := range password {
		switch {
		case char >= 'A' && char <= 'Z':
			hasUpper = true
		case char >= 'a' && char <= 'z':
			hasLower = true
		case char >= '0' && char <= '9':
			hasNumber = true
		case strings.ContainsRune("!@#$%^&*()_+-=[]{}|;:,.<>?", char):
			hasSpecial = true
		}
	}

	if !hasUpper {
		errors = append(errors, "Password must contain at least one uppercase letter")
	}
	if !hasLower {
		errors = append(errors, "Password must contain at least one lowercase letter")
	}
	if !hasNumber {
		errors = append(errors, "Password must contain at least one number")
	}
	if !hasSpecial {
		errors = append(errors, "Password must contain at least one special character")
	}

	return len(errors) == 0, errors
}

// ValidateUsername checks if a username is valid
func ValidateUsername(username string) (bool, string) {
	if len(username) < 3 {
		return false, "Username must be at least 3 characters"
	}
	if len(username) > 30 {
		return false, "Username must be at most 30 characters"
	}

	// Only alphanumeric, underscore, and hyphen
	validUsername := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !validUsername.MatchString(username) {
		return false, "Username can only contain letters, numbers, underscores, and hyphens"
	}

	return true, ""
}

// SanitizeString removes potentially dangerous characters
func SanitizeString(s string) string {
	// Remove null bytes
	s = strings.ReplaceAll(s, "\x00", "")
	// Trim whitespace
	s = strings.TrimSpace(s)
	return s
}
