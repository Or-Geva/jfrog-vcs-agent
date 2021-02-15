package main

import (
	"errors"
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/jfrog/jfrog-client-go/artifactory/buildinfo"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	defaultRemote = "origin"
)

func checkoutToBranch(branch string, r *git.Repository) error {
	w, err := r.Worktree()
	if err != nil {
		return err
	}
	return w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewRemoteReferenceName(defaultRemote, branch),
		Force:  true,
	})
}

func checkoutToHash(hash string, r *git.Repository) error {
	w, err := r.Worktree()
	if err != nil {
		return err
	}
	return w.Checkout(&git.CheckoutOptions{
		Hash:  plumbing.NewHash(hash),
		Force: true,
	})
}

// Returns a list of commits that haven't been scanned since the last run.
func getCommitsToBuild(r *git.Repository, bi *buildinfo.BuildInfo) (commitsToBuild []object.Commit, err error) {
	sha, err := getLatestCommitSha(bi)
	if err != nil {
		return nil, err
	}
	cIter, err := r.Log(&git.LogOptions{})
	if err != nil {
		err = errors.New("git revision '" + sha + "' was not found in git log. Error: " + err.Error())
		return
	}
	// Iterates over the commits from top to buttom. Save the commit hash till 'fromCommitSha' is found.
	found := false
	err = cIter.ForEach(func(c *object.Commit) error {
		if c.Hash.String() != sha {
			commitsToBuild = append([]object.Commit{*c}, commitsToBuild...)
			return nil
		}
		found = true
		return storer.ErrStop
	})
	if found == false {
		commitsToBuild = commitsToBuild[len(commitsToBuild)-1:]
		log.Info("Commit sha: '" + sha + "' wasn't found in the commits log. This may be the result of force push command. As a result, scanning only the latest commit on this branch.")
	}
	if len(commitsToBuild) > 0 {
		log.Info(fmt.Sprintf("Found %v commit(s) that haven't been scanned yet... ", len(commitsToBuild)))
	}
	return
}

// Returns the vcs revision in build-info.
func getLatestCommitSha(bi *buildinfo.BuildInfo) (string, error) {
	if len(bi.VcsList) == 0 {
		return "", errors.New("no vcs data in build info")
	}
	return bi.VcsList[0].Revision, nil
}

func cloneProject(runAt string, vcs *Vcs) (r *git.Repository, err error) {
	cloneOption := &git.CloneOptions{
		URL:  vcs.Url,
		Auth: createCredentials(vcs.Credentials),
		// Enable git submodules clone if there any.
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth,
	}
	r, err = git.PlainClone(runAt, false, cloneOption)
	return
}

func createCredentials(c *Credentials) (auth transport.AuthMethod) {
	password := c.AccessToken
	if password == "" {
		password = c.Password
	}
	return &http.BasicAuth{Username: c.User, Password: password}
}
