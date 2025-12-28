package controller

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"go-notes/internal/models"
	"go-notes/internal/service"
)

type NoteController struct {
	service *service.NoteService
}

type noteResponse struct {
	ID        uint      `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Tags      string    `json:"tags"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type createNoteRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Tags    string `json:"tags"`
}

type updateNoteRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Tags    string `json:"tags"`
}

func NewNoteController(service *service.NoteService) *NoteController {
	return &NoteController{service: service}
}

func (c *NoteController) Register(router *gin.RouterGroup) {
	router.GET("/notes", c.ListNotes)
	router.POST("/notes", c.CreateNote)
	router.GET("/notes/:id", c.GetNote)
	router.PUT("/notes/:id", c.UpdateNote)
	router.DELETE("/notes/:id", c.DeleteNote)
}

func (c *NoteController) CreateNote(ctx *gin.Context) {
	var req createNoteRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	note, err := c.service.CreateNote(service.NoteInput{
		Title:   req.Title,
		Content: req.Content,
		Tags:    req.Tags,
	})
	if err != nil {
		handleServiceError(ctx, err)
		return
	}

	ctx.JSON(http.StatusCreated, toNoteResponse(note))
}

func (c *NoteController) GetNote(ctx *gin.Context) {
	id, err := parseID(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid note id"})
		return
	}

	note, err := c.service.GetNote(id)
	if err != nil {
		handleServiceError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, toNoteResponse(note))
}

func (c *NoteController) ListNotes(ctx *gin.Context) {
	notes, err := c.service.ListNotes()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list notes"})
		return
	}

	responses := make([]noteResponse, 0, len(notes))
	for _, n := range notes {
		note := n
		responses = append(responses, toNoteResponse(&note))
	}

	ctx.JSON(http.StatusOK, responses)
}

func (c *NoteController) UpdateNote(ctx *gin.Context) {
	id, err := parseID(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid note id"})
		return
	}

	var req updateNoteRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	note, err := c.service.UpdateNote(id, service.NoteInput{
		Title:   req.Title,
		Content: req.Content,
		Tags:    req.Tags,
	})
	if err != nil {
		handleServiceError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, toNoteResponse(note))
}

func (c *NoteController) DeleteNote(ctx *gin.Context) {
	id, err := parseID(ctx.Param("id"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid note id"})
		return
	}

	if err := c.service.DeleteNote(id); err != nil {
		handleServiceError(ctx, err)
		return
	}

	ctx.Status(http.StatusNoContent)
}

func parseID(idParam string) (uint, error) {
	val, err := strconv.ParseUint(idParam, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(val), nil
}

func toNoteResponse(note *models.Note) noteResponse {
	return noteResponse{
		ID:        note.ID,
		Title:     note.Title,
		Content:   note.Content,
		Tags:      note.Tags,
		CreatedAt: note.CreatedAt,
		UpdatedAt: note.UpdatedAt,
	}
}

func handleServiceError(ctx *gin.Context, err error) {
	var validationErr service.ValidationError
	switch {
	case errors.As(err, &validationErr):
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error":  "validation failed",
			"fields": validationErr.Fields,
		})
	case errors.Is(err, service.ErrNoteNotFound):
		ctx.JSON(http.StatusNotFound, gin.H{"error": "note not found"})
	default:
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
