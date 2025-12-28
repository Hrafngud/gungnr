package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"go-notes/internal/middleware"
	"go-notes/internal/models"
	"go-notes/internal/repository"
	"go-notes/internal/service"
)

type fakeNoteRepo struct {
	notes  map[uint]models.Note
	nextID uint
}

func newFakeNoteRepo() *fakeNoteRepo {
	return &fakeNoteRepo{
		notes:  make(map[uint]models.Note),
		nextID: 1,
	}
}

func (f *fakeNoteRepo) Create(note *models.Note) error {
	note.ID = f.nextID
	f.nextID++
	f.notes[note.ID] = *note
	return nil
}

func (f *fakeNoteRepo) GetByID(id uint) (*models.Note, error) {
	if note, ok := f.notes[id]; ok {
		return &note, nil
	}
	return nil, repository.ErrNotFound
}

func (f *fakeNoteRepo) List() ([]models.Note, error) {
	result := make([]models.Note, 0, len(f.notes))
	for _, n := range f.notes {
		result = append(result, n)
	}
	return result, nil
}

func (f *fakeNoteRepo) Update(note *models.Note) error {
	if _, ok := f.notes[note.ID]; !ok {
		return repository.ErrNotFound
	}
	f.notes[note.ID] = *note
	return nil
}

func (f *fakeNoteRepo) Delete(id uint) error {
	if _, ok := f.notes[id]; !ok {
		return repository.ErrNotFound
	}
	delete(f.notes, id)
	return nil
}

func setupTestServer() (*gin.Engine, *fakeNoteRepo) {
	gin.SetMode(gin.TestMode)
	repo := newFakeNoteRepo()
	svc := service.NewNoteService(repo)
	noteCtrl := NewNoteController(svc)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORSMiddleware([]string{"*"}))
	NewHealthController().Register(r)
	noteCtrl.Register(r.Group("/api/v1"))

	return r, repo
}

func TestCreateNoteHandler(t *testing.T) {
	router, _ := setupTestServer()

	body := `{"title":"Test Note","content":"Content"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notes", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusCreated, resp.Code)

	var res noteResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &res))
	require.Equal(t, "Test Note", res.Title)
	require.NotZero(t, res.ID)
}

func TestGetNoteHandler(t *testing.T) {
	router, repo := setupTestServer()

	// create seed note directly in repo
	require.NoError(t, repo.Create(&models.Note{Title: "Saved", Content: "body"}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/notes/1", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	var res noteResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &res))
	require.Equal(t, uint(1), res.ID)
}

func TestListNotesHandler(t *testing.T) {
	router, repo := setupTestServer()

	require.NoError(t, repo.Create(&models.Note{Title: "First"}))
	require.NoError(t, repo.Create(&models.Note{Title: "Second"}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/notes", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	var res []noteResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &res))
	require.Len(t, res, 2)
}

func TestUpdateNoteHandlerNotFound(t *testing.T) {
	router, _ := setupTestServer()

	body := `{"title":"Updated","content":"Changed"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/notes/99", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusNotFound, resp.Code)
}

func TestDeleteNoteHandler(t *testing.T) {
	router, repo := setupTestServer()

	require.NoError(t, repo.Create(&models.Note{Title: "To delete"}))

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/notes/1", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusNoContent, resp.Code)
}
