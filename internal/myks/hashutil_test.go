package myks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashDirectory_Determinism(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("world"), 0o600))

	h1, err := hashDirectory(dir)
	require.NoError(t, err)
	h2, err := hashDirectory(dir)
	require.NoError(t, err)

	assert.Equal(t, h1, h2, "same directory should produce the same hash each time")
}

func TestHashDirectory_ContentChange(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	require.NoError(t, os.WriteFile(file, []byte("original"), 0o600))

	h1, err := hashDirectory(dir)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(file, []byte("modified"), 0o600))
	h2, err := hashDirectory(dir)
	require.NoError(t, err)

	assert.NotEqual(t, h1, h2, "modifying a file should change the hash")
}

func TestHashDirectory_FileAddition(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o600))

	h1, err := hashDirectory(dir)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("new"), 0o600))
	h2, err := hashDirectory(dir)
	require.NoError(t, err)

	assert.NotEqual(t, h1, h2, "adding a file should change the hash")
}

func TestHashDirectory_FileDeletion(t *testing.T) {
	dir := t.TempDir()
	fileA := filepath.Join(dir, "a.txt")
	fileB := filepath.Join(dir, "b.txt")
	require.NoError(t, os.WriteFile(fileA, []byte("hello"), 0o600))
	require.NoError(t, os.WriteFile(fileB, []byte("world"), 0o600))

	h1, err := hashDirectory(dir)
	require.NoError(t, err)

	require.NoError(t, os.Remove(fileB))
	h2, err := hashDirectory(dir)
	require.NoError(t, err)

	assert.NotEqual(t, h1, h2, "removing a file should change the hash")
}

func TestHashDirectory_FileRename(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "original.txt"), []byte("content"), 0o600))

	h1, err := hashDirectory(dir)
	require.NoError(t, err)

	require.NoError(t, os.Rename(
		filepath.Join(dir, "original.txt"),
		filepath.Join(dir, "renamed.txt"),
	))
	h2, err := hashDirectory(dir)
	require.NoError(t, err)

	assert.NotEqual(t, h1, h2, "renaming a file should change the hash")
}

func TestHashDirectory_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	h, err := hashDirectory(dir)
	require.NoError(t, err)
	assert.NotEmpty(t, h, "empty directory should return a valid (non-empty) hash string")
}

func TestHashDirectory_NonExistentDirectory(t *testing.T) {
	_, err := hashDirectory("/does/not/exist/at/all")
	assert.Error(t, err, "non-existent directory should return an error")
}
