package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHashAndCheckPassword(t *testing.T) {
	hashed, err := HashPassword("secret")
	require.NoError(t, err)
	require.NotEmpty(t, hashed)

	require.NoError(t, CheckPassword(hashed, "secret"))
	require.Error(t, CheckPassword(hashed, "wrong"))
}
