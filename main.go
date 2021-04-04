package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/go-git/go-git/v5"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/jfrog/jfrog-vcs-agent/utils"
)

func init() {
	log.SetLogger(log.NewLogger(log.INFO, nil))
}

// This program runs a series JFrog CLI commands in order to scan git repository. A high level flow overview:
// 1. Load config.
// 2. Clone & build the git repository.
// 3. Publish & scan the build.
func main() {
	buildConfig, ArtifactoryServicesManager, err := utils.LoadBuildConfig()
	assertNoError(err)
	gitRepo, projectPath, cleanup, err := setupAgent(buildConfig, ArtifactoryServicesManager)
	assertNoError(err)
	defer cleanup()
	for _, name := range buildConfig.Vcs.Branches {
		assertNoError(scanBranch(name, projectPath, buildConfig, gitRepo, ArtifactoryServicesManager))
	}
	log.Info(fmt.Sprintf("Git repository scan completed"))
}

// Setup the agent for scanning the git repository.
// 1. Clone the project.
// 2. Pre-configured the project with the Artifactory server and repositories.
// 3. Set build envarament varbles
// Returns (git repository details, local path to project, cleanup func, error).
func setupAgent(buildConfig *utils.BuildConfig, ArtifactoryServicesManager artifactory.ArtifactoryServicesManager) (*git.Repository, string, func(), error) {
	// Create artifactory server on agent.
	if err := utils.CreateArtServer(buildConfig); err != nil {
		return nil, "", nil, err
	}
	cloneDir, err := utils.CreateCloneDir()
	if err != nil {
		return nil, "", nil, err
	}
	log.Info("Cloning project '" + buildConfig.Vcs.Url + "' to '" + cloneDir + "'")
	gitRepo, err := utils.Clone(cloneDir, buildConfig.Vcs)
	if err != nil {
		return nil, "", nil, err
	}
	log.Info("Configure the Artifactory server and repositories for each technology")
	if err := utils.CreateBuildToolConfigs(cloneDir, buildConfig); err != nil {
		return nil, "", nil, err
	}
	log.Info("The agent is fully setup.")
	return gitRepo, cloneDir, func() {
		if err := os.RemoveAll(cloneDir); err != nil {
			log.Error(err.Error())
		}
		if err := utils.UnsetJfrogBuildProps(); err != nil {
			log.Error(err.Error())
		}
		if err := utils.DeleteArtServer(); err != nil {
			log.Error(err.Error())
		}
	}, nil
}

func scanBranch(branch, projectPath string, buildConfig *utils.BuildConfig, gitRepo *git.Repository, ArtifactoryServicesManager artifactory.ArtifactoryServicesManager) error {
	if err := utils.CheckoutBranch(branch, gitRepo); err != nil {
		return err
	}
	buildName := utils.GetBranchBuildName(branch, buildConfig)
	bi, err := utils.GetLatestBuildInfo(ArtifactoryServicesManager, buildName)
	if err != nil {
		return err
	}
	commits, err := utils.GetCommitsToScan(bi, gitRepo, buildConfig.Vcs.Url)
	if err != nil {
		return err
	}
	for i, commit := range commits {
		if err := utils.CheckoutHash(commit.Hash.String(), gitRepo); err != nil {
			return err
		}
		utils.SetBuildProps(buildName, utils.ToShortCommitHash(commit.Hash.String()), bi.Number, strconv.Itoa(i))
		if err := utils.Build(buildConfig.BuildCommand, projectPath); err != nil {
			log.Info("Failed to build commit '" + commit.Hash.String() + "' skipping to the next commit...")
			continue
		}
		if err := utils.Bag(projectPath); err != nil {
			return err
		}
		if err := utils.Publish(); err != nil {
			return err
		}
		if err := utils.BuildScan(); err != nil {
			return err
		}
	}
	return nil
}

func assertNoError(err error) {
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}
}
