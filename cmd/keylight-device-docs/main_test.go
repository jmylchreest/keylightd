package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdateGuide(t *testing.T) {
	t.Chdir(t.TempDir())
	require.NoError(t, os.MkdirAll(filepath.Dir(guidePath), 0o700))
	require.ErrorIs(t, updateGuide(true), os.ErrNotExist)
	write := func(contents string) {
		t.Helper()
		require.NoError(t, os.WriteFile(guidePath, []byte(contents), 0o600))
	}
	const begin = "{/* BEGIN GENERATED EXPERIMENTAL DEVICES */}"
	const end = "{/* END GENERATED EXPERIMENTAL DEVICES */}"
	write("no markers")
	require.ErrorContains(t, updateGuide(false), "missing opening marker")
	write(begin)
	require.ErrorContains(t, updateGuide(false), "missing closing marker")

	stale := "Introduction\n" + begin + "\nOld list\n" + end + "\nTesters welcome.\n"
	write(stale)
	require.ErrorContains(t, updateGuide(true), "is stale")
	data, err := os.ReadFile(guidePath)
	require.NoError(t, err)
	require.Equal(t, stale, string(data))

	require.NoError(t, updateGuide(false))
	require.NoError(t, updateGuide(true))
	data, err = os.ReadFile(guidePath)
	require.NoError(t, err)
	require.Contains(t, string(data), "- **Elgato Key Light Mini** (board 202)")
	require.Contains(t, string(data), "- **Elgato Key Light Neo** (board 210)")
	require.Contains(t, string(data), "Introduction\n"+begin)
	require.Contains(t, string(data), end+"\nTesters welcome.")
	require.NotContains(t, string(data), "Old list")
}
