package chat

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/services"
	"github.com/sahilchouksey/go-init-setup/utils/response"
)

// ChatContextHandler handles chat context selection requests
type ChatContextHandler struct {
	chatContextService *services.ChatContextService
}

// NewChatContextHandler creates a new chat context handler
func NewChatContextHandler(chatContextService *services.ChatContextService) *ChatContextHandler {
	return &ChatContextHandler{
		chatContextService: chatContextService,
	}
}

// GetChatContext handles GET /api/v1/chat/context
// Returns all dropdown data in a single call for chat setup
func (h *ChatContextHandler) GetChatContext(c *fiber.Ctx) error {
	context, err := h.chatContextService.GetChatContext(c.Context())
	if err != nil {
		return response.InternalServerError(c, "Failed to fetch chat context: "+err.Error())
	}

	return response.Success(c, context)
}

// GetUniversities handles GET /api/v1/chat/context/universities
func (h *ChatContextHandler) GetUniversities(c *fiber.Ctx) error {
	universities, err := h.chatContextService.GetUniversities(c.Context())
	if err != nil {
		return response.InternalServerError(c, "Failed to fetch universities: "+err.Error())
	}

	return response.Success(c, universities)
}

// GetCourses handles GET /api/v1/chat/context/universities/:university_id/courses
func (h *ChatContextHandler) GetCourses(c *fiber.Ctx) error {
	universityIDStr := c.Params("university_id")
	universityID, err := strconv.ParseUint(universityIDStr, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid university ID")
	}

	courses, err := h.chatContextService.GetCoursesByUniversity(c.Context(), uint(universityID))
	if err != nil {
		return response.InternalServerError(c, "Failed to fetch courses: "+err.Error())
	}

	return response.Success(c, courses)
}

// GetSemesters handles GET /api/v1/chat/context/courses/:course_id/semesters
func (h *ChatContextHandler) GetSemesters(c *fiber.Ctx) error {
	courseIDStr := c.Params("course_id")
	courseID, err := strconv.ParseUint(courseIDStr, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid course ID")
	}

	semesters, err := h.chatContextService.GetSemestersByCourse(c.Context(), uint(courseID))
	if err != nil {
		return response.InternalServerError(c, "Failed to fetch semesters: "+err.Error())
	}

	return response.Success(c, semesters)
}

// GetSubjects handles GET /api/v1/chat/context/semesters/:semester_id/subjects
// Only returns subjects that have both KnowledgeBaseUUID and AgentUUID set
func (h *ChatContextHandler) GetSubjects(c *fiber.Ctx) error {
	semesterIDStr := c.Params("semester_id")
	semesterID, err := strconv.ParseUint(semesterIDStr, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid semester ID")
	}

	subjects, err := h.chatContextService.GetSubjectsBySemester(c.Context(), uint(semesterID))
	if err != nil {
		return response.InternalServerError(c, "Failed to fetch subjects: "+err.Error())
	}

	return response.Success(c, subjects)
}

// GetSubjectSyllabusContext handles GET /api/v1/chat/context/subjects/:subject_id/syllabus
// Returns syllabus data formatted for chat prompts
func (h *ChatContextHandler) GetSubjectSyllabusContext(c *fiber.Ctx) error {
	subjectIDStr := c.Params("subject_id")
	subjectID, err := strconv.ParseUint(subjectIDStr, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid subject ID")
	}

	syllabusCtx, err := h.chatContextService.GetSubjectSyllabusContext(c.Context(), uint(subjectID))
	if err != nil {
		return response.InternalServerError(c, "Failed to fetch syllabus context: "+err.Error())
	}

	if syllabusCtx == nil {
		return response.Success(c, fiber.Map{
			"has_syllabus": false,
			"message":      "No syllabus data available for this subject",
		})
	}

	return response.Success(c, fiber.Map{
		"has_syllabus": true,
		"syllabus":     syllabusCtx,
	})
}
