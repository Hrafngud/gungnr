package apierror

import (
	"fmt"
	"testing"

	"go-notes/internal/errs"
	"go-notes/internal/repository"

	"github.com/stretchr/testify/require"
)

func TestClassifyUsesFallbackCodeForRepositoryNotFound(t *testing.T) {
	code, message, options := classify(repository.ErrNotFound, errs.CodeJobNotFound, "job not found")

	require.Equal(t, errs.CodeJobNotFound, code)
	require.Equal(t, "job not found", message)
	require.Nil(t, options)
}

func TestClassifyUsesFallbackCodeForWrappedRepositoryNotFound(t *testing.T) {
	err := fmt.Errorf("lookup failed: %w", repository.ErrNotFound)

	code, message, options := classify(err, errs.CodeUserNotFound, "user not found")

	require.Equal(t, errs.CodeUserNotFound, code)
	require.Equal(t, "user not found", message)
	require.Nil(t, options)
}

func TestClassifyPrefersTypedErrorCode(t *testing.T) {
	err := errs.Wrap(errs.CodeJobNotFound, "job not found", repository.ErrNotFound)

	code, message, options := classify(err, errs.CodeUserNotFound, "user not found")

	require.Equal(t, errs.CodeJobNotFound, code)
	require.Equal(t, "job not found", message)
	require.NotNil(t, options)
}
