package service

import (
	"testing"

	"github.com/stretchr/testify/require"

	"go-notes/internal/models"
	"go-notes/internal/repository"
)

type fakeNoteRepo struct {
	notes  map[uint]models.Note
	nextID uint
}

func newFakeRepo() *fakeNoteRepo {
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
	n, ok := f.notes[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return &n, nil
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

func TestCreateNote(t *testing.T) {
	repo := newFakeRepo()
	svc := NewNoteService(repo)

	note, err := svc.CreateNote(NoteInput{
		Title:   "Hello",
		Content: "World",
	})
	require.NoError(t, err)
	require.NotNil(t, note)
	require.Equal(t, uint(1), note.ID)
	require.Equal(t, "Hello", note.Title)
}

func TestCreateNoteValidation(t *testing.T) {
	repo := newFakeRepo()
	svc := NewNoteService(repo)

	_, err := svc.CreateNote(NoteInput{})
	require.Error(t, err)
	var valErr ValidationError
	require.ErrorAs(t, err, &valErr)
	require.Contains(t, valErr.Fields, "title")
}

func TestGetNoteNotFound(t *testing.T) {
	repo := newFakeRepo()
	svc := NewNoteService(repo)

	_, err := svc.GetNote(123)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrNoteNotFound)
}

func TestUpdateNote(t *testing.T) {
	repo := newFakeRepo()
	svc := NewNoteService(repo)

	created, err := svc.CreateNote(NoteInput{Title: "Old"})
	require.NoError(t, err)

	updated, err := svc.UpdateNote(created.ID, NoteInput{Title: "New", Content: "Body"})
	require.NoError(t, err)
	require.Equal(t, "New", updated.Title)
	require.Equal(t, "Body", updated.Content)
}

func TestUpdateNoteNotFound(t *testing.T) {
	repo := newFakeRepo()
	svc := NewNoteService(repo)

	_, err := svc.UpdateNote(999, NoteInput{Title: "X"})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrNoteNotFound)
}

func TestDeleteNote(t *testing.T) {
	repo := newFakeRepo()
	svc := NewNoteService(repo)

	created, err := svc.CreateNote(NoteInput{Title: "Delete me"})
	require.NoError(t, err)

	require.NoError(t, svc.DeleteNote(created.ID))
	_, err = svc.GetNote(created.ID)
	require.ErrorIs(t, err, ErrNoteNotFound)
}

func TestDeleteNoteNotFound(t *testing.T) {
	repo := newFakeRepo()
	svc := NewNoteService(repo)

	err := svc.DeleteNote(42)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrNoteNotFound)
}
