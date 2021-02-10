package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/go-git/go-git/v5"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

func init() {
	log.SetLogger(log.NewLogger(log.DEBUG, nil))
}

// This program runs a series JFrog CLI commands in order to scan git repository. A high level flow overview:
// 1. Load user config from config/config.json.
// 2. Clone & build the git repository.
// 3. Publish & scan the build-info.
func main() {
	c, sm, err := getBuildConfig()
	checkIfError(err)
	r, projectPath, cleanup, err := setupAgent(c, sm)
	checkIfError(err)
	defer cleanup()
	for name, buildName := range c.Vcs.Branch {
		// Checkout to branch.
		checkIfError(checkoutToBranch(name, r))
		bi, err := getLatestBuildInfo(sm, buildName)
		checkIfError(err)
		commits, err := getCommitsToBuild(r, bi)
		checkIfError(err)
		for i, commit := range commits {
			checkIfError(checkoutToHash(commit.Hash.String(), r))
			setBuildProps(buildName, commit.Hash.String(), bi.BuildInfo.Number, strconv.Itoa(i))
			checkIfError(build(c.BuildCommand, projectPath))
			checkIfError(bag(projectPath))
			checkIfError(publish())
			checkIfError(buildScan())
		}
	}
}

// Set the agent before building and publishing the git repository.
// Returns (git repository details, path to project, cleanup func, error).
func setupAgent(c *BuildConfig, sm artifactory.ArtifactoryServicesManager) (*git.Repository, string, func(), error) {
	// Create artifactory server on agent.
	if err := createArtServer("vcs-superhighway", c); err != nil {
		return nil, "", nil, err
	}
	// Create a clone dir.
	tmp, err := fileutils.CreateTempDir()
	if err != nil {
		return nil, "", nil, err
	}
	r, err := cloneProject(tmp, c.Vcs)
	if err != nil {
		return nil, "", nil, err
	}
	// Create build tools config.
	if err := createBuildToolConfigs("vcs-superhighway", c); err != nil {
		return nil, "", nil, err
	}
	return r, tmp, func() {
		if err := os.RemoveAll(tmp); err != nil {
			log.Error(err.Error())
		}
		if err := unsetBuildProps(); err != nil {
			log.Error(err.Error())
		}
	}, nil
}

func checkIfError(err error) {
	if err != nil {
		fmt.Println("ERROR:", err.Error())
		os.Exit(1)
	}
}
