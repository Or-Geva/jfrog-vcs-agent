package utils

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/jfrog/jfrog-client-go/artifactory/buildinfo"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	// Default remote name for the cloned repository.
	defaultRemote = "origin"
)

func CheckoutBranch(branch string, r *git.Repository) error {
	log.Info("Checkout to '" + branch + "' branch")
	w, err := r.Worktree()
	if err != nil {
		return err
	}
	return w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewRemoteReferenceName(defaultRemote, branch),
		Force:  true,
	})
}

func CheckoutHash(hash string, r *git.Repository) error {
	log.Info("Checkout to '" + hash + "' commmit")
	w, err := r.Worktree()
	if err != nil {
		return err
	}
	return w.Checkout(&git.CheckoutOptions{
		Hash:  plumbing.NewHash(hash),
		Force: true,
	})
}

// Returns the commits bwtween fromSha - HEAD.
// Due to 'Force push',the commit may be missing. As a result, the latest commit will be returned.
func GetCommitsRange(fromSha string, r *git.Repository) (commits []object.Commit, err error) {
	_, err = r.CommitObject(plumbing.NewHash(fromSha))
	getLatestCommit := false
	if err != nil {
		log.Info("Commit sha: '" + fromSha + "' wasn't found in the commits log. This may be the result of force push command. As a result, scanning only the latest commit on this branch.")
		getLatestCommit = true
	}
	cIter, err := r.Log(&git.LogOptions{})
	if err != nil {
		return
	}
	// Iterates over the commits from top to buttom. Save the commit hash till 'fromSha' is found.
	err = cIter.ForEach(func(c *object.Commit) error {
		found := c.Hash.String() == fromSha
		if !found {
			commits = append([]object.Commit{*c}, commits...)
		}
		if found || getLatestCommit {
			return storer.ErrStop
		}
		return nil
	})
	return
}

// Clone a vcs repository into the path.
// If the path is not empty, ErrRepositoryAlreadyExists is returned.
func Clone(path string, vcs *Vcs) (gitRepo *git.Repository, err error) {
	cloneOption := &git.CloneOptions{
		URL:  vcs.Url,
		Auth: createCredentials(vcs),
		// Enable git submodules clone.
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
	}
	gitRepo, err = git.PlainClone(path, false, cloneOption)
	return
}

func createCredentials(c *Vcs) (auth transport.AuthMethod) {
	password := c.Token
	if password == "" {
		password = c.Password
	}
	return &http.BasicAuth{Username: c.User, Password: password}
}

func GetCommitsToScan(bi *buildinfo.BuildInfo, r *git.Repository, vcsUrl string) ([]object.Commit, error) {
	log.Info("Searching the latest commit revision in the build-info...")
	sha, err := getBuildCommitSha(bi, vcsUrl)
	if err != nil {
		return nil, err
	}
	commits, err := GetCommitsRange(sha, r)
	if commits == nil {
		log.Info("No new commits since the last run. Skipping... ")
	} else {
		log.Info(fmt.Sprintf("Found %v new commits that haven't been scanned", len(commits)))
	}
	return commits, err
}

func ToShortCommitHash(hash string) string {
	return hash[:8]
}

// Create a local directory for the project that is being cloned.
// Default path is at /agent_home/project/.
// Override if exist.
func CreateCloneDir() (string, error) {
	// Create clone dir.
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	path := filepath.Join(wd, "project")
	exists, err := fileutils.IsDirExists(path, false)
	if err != nil {
		return "", err
	}
	if exists {
		err = os.RemoveAll(path)
		if err != nil {
			return "", err
		}
	}
	err = os.Mkdir(path, 0755)
	return path, err
}
