package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/go-git/go-git/v5"
	cliLog "github.com/jfrog/jfrog-cli-core/utils/log"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

func init() {
	cliLog.SetDefaultLogger()
}

// This program runs a series JFrog CLI commands in order to scan git repository. A high level flow overview:
// 1. Load user config.
// 2. Clone & build the git repository.
// 3. Publish & scan the build.
func main() {
	c, sm, err := loadBuildConfig()
	checkIfError(err)
	r, projectPath, cleanup, err := setupAgent(c, sm)
	checkIfError(err)
	defer cleanup()
	for _, name := range c.Vcs.Branches {
		checkIfError(scanBranch(name, projectPath, c, r, sm))
	}
	log.Info(fmt.Sprintf("Git repository scan completed"))
}

// Setup the agent for scanning the git repository.
// 1. Clone the project.
// 2. Pre-configured the project with the Artifactory server and repositories.
// 3. Set build envarament varbles
// Returns (git repository details, local path to project, cleanup func, error).
func setupAgent(c *BuildConfig, sm artifactory.ArtifactoryServicesManager) (*git.Repository, string, func(), error) {
	// Create artifactory server on agent.
	if err := createArtServer(c); err != nil {
		return nil, "", nil, err
	}
	cloneDir, err := createCloneDir()
	if err != nil {
		return nil, "", nil, err
	}
	log.Info("Cloning project '" + c.Vcs.Url + "' to '" + cloneDir + "'")
	r, err := clone(cloneDir, c.Vcs)
	if err != nil {
		return nil, "", nil, err
	}
	log.Info("Configure the Artifactory server and repositories for each technology")
	if err := createBuildToolConfigs(cloneDir, c); err != nil {
		return nil, "", nil, err
	}
	log.Info("The agent is fully setup.")
	return r, cloneDir, func() {
		if err := os.RemoveAll(cloneDir); err != nil {
			log.Error(err.Error())
		}
		if err := unsetJfrogBuildProps(); err != nil {
			log.Error(err.Error())
		}
		if err := deleteArtServer(); err != nil {
			log.Error(err.Error())
		}
	}, nil
}

func scanBranch(branch, projectPath string, c *BuildConfig, r *git.Repository, sm artifactory.ArtifactoryServicesManager) error {
	if err := checkoutBranch(branch, r); err != nil {
		return err
	}
	buildName := getBranchBuildName(branch, c)
	bi, err := getLatestBuildInfo(sm, buildName)
	if err != nil {
		return err
	}
	commits, err := getCommitsToScan(bi, r, c.Vcs.Url)
	if err != nil {
		return err
	}
	for i, commit := range commits {
		if err := checkoutHash(commit.Hash.String(), r); err != nil {
			return err
		}
		setBuildProps(buildName, shortCommitHash(commit.Hash.String()), bi.Number, strconv.Itoa(i))
		if err := build(c.BuildCommand, projectPath); err != nil {
			log.Info("Failed to build commit '" + commit.Hash.String() + "' skipping to the next commit...")
			continue
		}
		if err := bag(projectPath); err != nil {
			return err
		}
		if err := publish(); err != nil {
			return err
		}
		if err := buildScan(); err != nil {
			return err
		}
	}
	return nil
}

func checkIfError(err error) {
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}
}
