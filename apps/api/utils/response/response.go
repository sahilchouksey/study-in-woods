package response

import (
	"github.com/gofiber/fiber/v2"
)

// Response represents a standardized API response
type Response struct {
	Success bool         `json:"success"`
	Message string       `json:"message,omitempty"`
	Data    interface{}  `json:"data,omitempty"`
	Error   *ErrorDetail `json:"error,omitempty"`
}

// ErrorDetail contains error information
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// PaginationMeta contains pagination metadata
type PaginationMeta struct {
	CurrentPage int   `json:"current_page"`
	PerPage     int   `json:"per_page"`
	Total       int64 `json:"total"`
	TotalPages  int   `json:"total_pages"`
}

// PaginatedResponse represents a paginated API response
type PaginatedResponse struct {
	Success    bool           `json:"success"`
	Message    string         `json:"message,omitempty"`
	Data       interface{}    `json:"data"`
	Pagination PaginationMeta `json:"pagination"`
}

// Success returns a successful response
func Success(c *fiber.Ctx, data interface{}) error {
	return c.Status(fiber.StatusOK).JSON(Response{
		Success: true,
		Data:    data,
	})
}

// SuccessWithMessage returns a successful response with a message
func SuccessWithMessage(c *fiber.Ctx, message string, data interface{}) error {
	return c.Status(fiber.StatusOK).JSON(Response{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// Created returns a 201 Created response
func Created(c *fiber.Ctx, data interface{}) error {
	return c.Status(fiber.StatusCreated).JSON(Response{
		Success: true,
		Message: "Resource created successfully",
		Data:    data,
	})
}

// NoContent returns a 204 No Content response
func NoContent(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusNoContent)
}

// Error returns an error response
func Error(c *fiber.Ctx, statusCode int, message string, code string) error {
	return c.Status(statusCode).JSON(Response{
		Success: false,
		Error: &ErrorDetail{
			Code:    code,
			Message: message,
		},
	})
}

// ErrorWithDetails returns an error response with details
func ErrorWithDetails(c *fiber.Ctx, statusCode int, message string, code string, details string) error {
	return c.Status(statusCode).JSON(Response{
		Success: false,
		Error: &ErrorDetail{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}

// BadRequest returns a 400 Bad Request response
func BadRequest(c *fiber.Ctx, message string) error {
	return Error(c, fiber.StatusBadRequest, message, "BAD_REQUEST")
}

// Unauthorized returns a 401 Unauthorized response
func Unauthorized(c *fiber.Ctx, message string) error {
	if message == "" {
		message = "Unauthorized access"
	}
	return Error(c, fiber.StatusUnauthorized, message, "UNAUTHORIZED")
}

// Forbidden returns a 403 Forbidden response
func Forbidden(c *fiber.Ctx, message string) error {
	if message == "" {
		message = "Access forbidden"
	}
	return Error(c, fiber.StatusForbidden, message, "FORBIDDEN")
}

// NotFound returns a 404 Not Found response
func NotFound(c *fiber.Ctx, message string) error {
	if message == "" {
		message = "Resource not found"
	}
	return Error(c, fiber.StatusNotFound, message, "NOT_FOUND")
}

// Conflict returns a 409 Conflict response
func Conflict(c *fiber.Ctx, message string) error {
	return Error(c, fiber.StatusConflict, message, "CONFLICT")
}

// TooManyRequests returns a 429 Too Many Requests response
func TooManyRequests(c *fiber.Ctx, message string) error {
	if message == "" {
		message = "Too many requests"
	}
	return Error(c, fiber.StatusTooManyRequests, message, "TOO_MANY_REQUESTS")
}

// ValidationError returns a 422 Unprocessable Entity response for validation errors
func ValidationError(c *fiber.Ctx, err error) error {
	return ErrorWithDetails(c, fiber.StatusUnprocessableEntity,
		"Validation failed", "VALIDATION_ERROR", err.Error())
}

// InternalServerError returns a 500 Internal Server Error response
func InternalServerError(c *fiber.Ctx, message string) error {
	if message == "" {
		message = "Internal server error"
	}
	return Error(c, fiber.StatusInternalServerError, message, "INTERNAL_ERROR")
}

// ServiceUnavailable returns a 503 Service Unavailable response
func ServiceUnavailable(c *fiber.Ctx, message string) error {
	if message == "" {
		message = "Service temporarily unavailable"
	}
	return Error(c, fiber.StatusServiceUnavailable, message, "SERVICE_UNAVAILABLE")
}

// Paginated returns a paginated response
func Paginated(c *fiber.Ctx, data interface{}, pagination PaginationMeta) error {
	return c.Status(fiber.StatusOK).JSON(PaginatedResponse{
		Success:    true,
		Data:       data,
		Pagination: pagination,
	})
}

// CalculatePagination calculates pagination metadata
func CalculatePagination(page, limit int, total int64) PaginationMeta {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}

	return PaginationMeta{
		CurrentPage: page,
		PerPage:     limit,
		Total:       total,
		TotalPages:  totalPages,
	}
}
