package pyq

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/sahilchouksey/go-init-setup/model"
	"github.com/sahilchouksey/go-init-setup/services"
	"github.com/sahilchouksey/go-init-setup/utils/middleware"
	"github.com/sahilchouksey/go-init-setup/utils/response"
	"gorm.io/gorm"
)

// PYQHandler handles PYQ-related requests
type PYQHandler struct {
	db         *gorm.DB
	pyqService *services.PYQService
}

// NewPYQHandler creates a new PYQ handler
func NewPYQHandler(db *gorm.DB, pyqService *services.PYQService) *PYQHandler {
	return &PYQHandler{
		db:         db,
		pyqService: pyqService,
	}
}

// GetPYQsBySubject handles GET /api/v1/subjects/:subject_id/pyqs
func (h *PYQHandler) GetPYQsBySubject(c *fiber.Ctx) error {
	subjectID := c.Params("subject_id")

	// Parse subject ID
	subID, err := strconv.ParseUint(subjectID, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid subject ID")
	}

	// Verify subject exists
	var subject model.Subject
	if err := h.db.First(&subject, subID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Subject not found")
		}
		return response.InternalServerError(c, "Failed to fetch subject")
	}

	// Get PYQ papers
	papers, err := h.pyqService.GetPYQsBySubject(c.Context(), uint(subID))
	if err != nil {
		return response.InternalServerError(c, "Failed to fetch PYQ papers")
	}

	// Convert to summaries
	summaries := make([]model.PYQPaperSummary, len(papers))
	for i, paper := range papers {
		summaries[i] = paper.ToSummary()
	}

	return response.Success(c, model.PYQPapersListResponse{
		Papers: summaries,
		Total:  len(summaries),
	})
}

// GetPYQById handles GET /api/v1/pyqs/:id
func (h *PYQHandler) GetPYQById(c *fiber.Ctx) error {
	pyqID := c.Params("id")

	// Parse PYQ ID
	pID, err := strconv.ParseUint(pyqID, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid PYQ ID")
	}

	// Get PYQ paper with questions and choices
	paper, err := h.pyqService.GetPYQByID(c.Context(), uint(pID))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "PYQ paper not found")
		}
		return response.InternalServerError(c, "Failed to fetch PYQ paper")
	}

	return response.Success(c, paper.ToResponse())
}

// ExtractPYQ handles POST /api/v1/documents/:document_id/extract-pyq
// Triggers PYQ extraction from a document
func (h *PYQHandler) ExtractPYQ(c *fiber.Ctx) error {
	documentID := c.Params("document_id")

	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Parse document ID
	docID, err := strconv.ParseUint(documentID, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid document ID")
	}

	// Verify document exists and is a PYQ type
	var document model.Document
	if err := h.db.Preload("Subject").First(&document, docID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Document not found")
		}
		return response.InternalServerError(c, "Failed to fetch document")
	}

	// Verify document is PYQ type
	if document.Type != model.DocumentTypePYQ {
		return response.BadRequest(c, "Document is not a PYQ type. Only PYQ documents can be extracted.")
	}

	// Check user permission (admin or document uploader)
	if user.Role != "admin" && document.UploadedByUserID != user.ID {
		return response.Forbidden(c, "You don't have permission to extract this PYQ")
	}

	// Check if async extraction is requested
	async := c.Query("async", "false") == "true"

	if async {
		// Trigger async extraction
		h.pyqService.TriggerExtractionAsync(uint(docID))
		return response.Success(c, fiber.Map{
			"message":    "PYQ extraction started in background",
			"status":     "processing",
			"subject_id": document.SubjectID,
		})
	}

	// Synchronous extraction
	paper, err := h.pyqService.ExtractPYQFromDocument(c.Context(), uint(docID))
	if err != nil {
		return response.InternalServerError(c, "Failed to extract PYQ: "+err.Error())
	}

	// Reload with relationships
	paper, _ = h.pyqService.GetPYQByID(c.Context(), paper.ID)

	return response.Success(c, fiber.Map{
		"message": "PYQ extracted successfully",
		"pyq":     paper.ToResponse(),
	})
}

// GetExtractionStatus handles GET /api/v1/pyqs/:id/status
// Returns the current extraction status
func (h *PYQHandler) GetExtractionStatus(c *fiber.Ctx) error {
	pyqID := c.Params("id")

	// Parse PYQ ID
	pID, err := strconv.ParseUint(pyqID, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid PYQ ID")
	}

	status, errMsg, err := h.pyqService.GetExtractionStatus(c.Context(), uint(pID))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "PYQ paper not found")
		}
		return response.InternalServerError(c, "Failed to fetch status")
	}

	responseData := fiber.Map{
		"status": status,
	}
	if errMsg != "" {
		responseData["error"] = errMsg
	}

	return response.Success(c, responseData)
}

// RetryExtraction handles POST /api/v1/pyqs/:id/retry
// Retries a failed extraction
func (h *PYQHandler) RetryExtraction(c *fiber.Ctx) error {
	pyqID := c.Params("id")

	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Parse PYQ ID
	pID, err := strconv.ParseUint(pyqID, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid PYQ ID")
	}

	// Get PYQ paper to check ownership
	var paper model.PYQPaper
	if err := h.db.Preload("Document").First(&paper, pID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "PYQ paper not found")
		}
		return response.InternalServerError(c, "Failed to fetch PYQ paper")
	}

	// Check permission
	if user.Role != "admin" && paper.Document.UploadedByUserID != user.ID {
		return response.Forbidden(c, "You don't have permission to retry this extraction")
	}

	// Check if async is requested
	async := c.Query("async", "false") == "true"

	if async {
		// Trigger async retry
		h.pyqService.TriggerExtractionAsync(paper.DocumentID)
		return response.Success(c, fiber.Map{
			"message": "PYQ extraction retry started in background",
			"status":  "processing",
		})
	}

	// Synchronous retry
	result, err := h.pyqService.RetryExtraction(c.Context(), uint(pID))
	if err != nil {
		return response.InternalServerError(c, "Failed to retry extraction: "+err.Error())
	}

	// Reload with relationships
	result, _ = h.pyqService.GetPYQByID(c.Context(), result.ID)

	return response.Success(c, fiber.Map{
		"message": "PYQ extraction completed",
		"pyq":     result.ToResponse(),
	})
}

// DeletePYQ handles DELETE /api/v1/pyqs/:id
func (h *PYQHandler) DeletePYQ(c *fiber.Ctx) error {
	pyqID := c.Params("id")

	// Get user from context
	user, ok := middleware.GetUser(c)
	if !ok || user == nil {
		return response.Unauthorized(c, "User not authenticated")
	}

	// Only admin can delete PYQ
	if user.Role != "admin" {
		return response.Forbidden(c, "Only administrators can delete PYQ data")
	}

	// Parse PYQ ID
	pID, err := strconv.ParseUint(pyqID, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid PYQ ID")
	}

	// Delete PYQ
	if err := h.pyqService.DeletePYQ(c.Context(), uint(pID)); err != nil {
		return response.InternalServerError(c, "Failed to delete PYQ paper")
	}

	return response.Success(c, fiber.Map{
		"message": "PYQ paper deleted successfully",
	})
}

// ListQuestions handles GET /api/v1/pyqs/:id/questions
func (h *PYQHandler) ListQuestions(c *fiber.Ctx) error {
	pyqID := c.Params("id")

	// Parse PYQ ID
	pID, err := strconv.ParseUint(pyqID, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid PYQ ID")
	}

	// Get questions
	var questions []model.PYQQuestion
	if err := h.db.Preload("Choices").
		Where("paper_id = ?", pID).
		Order("question_number ASC").
		Find(&questions).Error; err != nil {
		return response.InternalServerError(c, "Failed to fetch questions")
	}

	// Convert to response format
	questionsResp := make([]model.PYQQuestionResponse, len(questions))
	for i, q := range questions {
		questionsResp[i] = model.PYQQuestionResponse{
			ID:             q.ID,
			QuestionNumber: q.QuestionNumber,
			SectionName:    q.SectionName,
			QuestionText:   q.QuestionText,
			Marks:          q.Marks,
			IsCompulsory:   q.IsCompulsory,
			HasChoices:     q.HasChoices,
			ChoiceGroup:    q.ChoiceGroup,
			UnitNumber:     q.UnitNumber,
			TopicKeywords:  q.TopicKeywords,
			Choices:        make([]model.PYQQuestionChoiceResponse, len(q.Choices)),
		}
		for j, choice := range q.Choices {
			questionsResp[i].Choices[j] = model.PYQQuestionChoiceResponse{
				ID:          choice.ID,
				ChoiceLabel: choice.ChoiceLabel,
				ChoiceText:  choice.ChoiceText,
				Marks:       choice.Marks,
			}
		}
	}

	return response.Success(c, questionsResp)
}

// SearchQuestions handles GET /api/v1/subjects/:subject_id/pyqs/search
// Search questions by keywords
func (h *PYQHandler) SearchQuestions(c *fiber.Ctx) error {
	subjectID := c.Params("subject_id")
	query := c.Query("q", "")

	if query == "" {
		return response.BadRequest(c, "Search query is required")
	}

	// Parse subject ID
	subID, err := strconv.ParseUint(subjectID, 10, 32)
	if err != nil {
		return response.BadRequest(c, "Invalid subject ID")
	}

	// Verify subject exists
	var subject model.Subject
	if err := h.db.First(&subject, subID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return response.NotFound(c, "Subject not found")
		}
		return response.InternalServerError(c, "Failed to fetch subject")
	}

	// Search questions
	questions, err := h.pyqService.SearchQuestions(c.Context(), uint(subID), query)
	if err != nil {
		return response.InternalServerError(c, "Failed to search questions")
	}

	// Convert to response format
	questionsResp := make([]model.PYQQuestionResponse, len(questions))
	for i, q := range questions {
		questionsResp[i] = model.PYQQuestionResponse{
			ID:             q.ID,
			QuestionNumber: q.QuestionNumber,
			SectionName:    q.SectionName,
			QuestionText:   q.QuestionText,
			Marks:          q.Marks,
			IsCompulsory:   q.IsCompulsory,
			HasChoices:     q.HasChoices,
			ChoiceGroup:    q.ChoiceGroup,
			UnitNumber:     q.UnitNumber,
			TopicKeywords:  q.TopicKeywords,
		}
	}

	return response.Success(c, fiber.Map{
		"query":   query,
		"count":   len(questionsResp),
		"results": questionsResp,
	})
}
