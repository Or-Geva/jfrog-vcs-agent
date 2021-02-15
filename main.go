package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-git/go-git/v5"
	cliLog "github.com/jfrog/jfrog-cli-core/utils/log"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	defaultName = "project"
)

func init() {
	cliLog.SetDefaultLogger()
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
		log.Info("Checkout to '" + name + "' branch")
		checkIfError(checkoutToBranch(name, r))
		bi, err := getLatestBuildInfo(sm, buildName)
		checkIfError(err)
		commits, err := getCommitsToBuild(r, bi)
		checkIfError(err)
		if commits == nil {
			log.Info("'" + name + "' branch has no new commits since the last run. Skipping... ")
		}
		for i, commit := range commits {
			checkIfError(checkoutToHash(commit.Hash.String(), r))
			setBuildProps(buildName, commit.Hash.String()[:8], bi.Number, strconv.Itoa(i))
			log.Info("Generating build " + os.Getenv(jfrogBuildName) + "/" + os.Getenv(jfrogBuildNumber))
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
	wd, err := os.Getwd()
	if err != nil {
		return nil, "", nil, err
	}
	cloneDir := filepath.Join(wd, defaultName)
	err = os.Mkdir(cloneDir, 0755)
	if err != nil {
		return nil, "", nil, err
	}
	if err != nil {
		return nil, "", nil, err
	}
	log.Info("Cloning project '" + c.Vcs.Url + "' to '" + cloneDir + "'")
	r, err := cloneProject(cloneDir, c.Vcs)
	if err != nil {
		return nil, "", nil, err
	}
	// Create build tools config.
	if err := createBuildToolConfigs("vcs-superhighway", c); err != nil {
		return nil, "", nil, err
	}
	return r, cloneDir, func() {
		if err := os.RemoveAll(cloneDir); err != nil {
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
