package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/stretchr/testify/assert"
)

func TestCheckoutBranch(t *testing.T) {
	path, cleanup := setupTmpDir(t, "checkout")
	defer cleanup()
	// We instantiate a new repository targeting the given path (the .git folder)
	r, err := git.PlainOpen(path)
	assert.NoError(t, err)

	err = CheckoutBranch("dev", r)
	assert.NoError(t, err)
	cIter, err := r.Log(&git.LogOptions{})
	err = cIter.ForEach(func(c *object.Commit) error {
		assert.Equal(t, "Checkout succeeded\n", c.Message)
		return nil
	})
}
func setupTmpDir(t *testing.T, dir string) (string, func()) {
	tmpDir, err := fileutils.CreateTempDir()
	assert.NoError(t, err)
	cleanup := func() { assert.NoError(t, os.RemoveAll(tmpDir)) }
	testDataDir, err := filepath.Abs(filepath.Join("testdata", "git", dir))
	assert.NoError(t, err)
	assert.NoError(t, fileutils.CopyDir(testDataDir, tmpDir, true, nil))
	return tmpDir, cleanup
}
