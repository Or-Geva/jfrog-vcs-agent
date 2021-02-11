package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/buildinfo"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

// runs build command at 'projectPath'.
// Build-name & build-number expect to be in env vars
func build(buildCommand, projectPath string) error {
	return runCmd(projectPath, buildCommand)
}

// Build-name & build-number expect to be in env vars
func bag(projectPath string) error {
	return runCmd(projectPath, "jfrog rt bag")
}

// Build-name & build-number expect to be in env vars
func publish() error {
	return runCmd("", "jfrog rt bp")
}

// Build-name & build-number expect to be in env vars
func buildScan() error {
	return runCmd("", "jfrog rt bs")
}

// Run a command in the bash shell. If 'runAt' is specified, the command will be executed at this path context.
func runCmd(runAt string, cmd string) error {
	cmds := exec.Command("bash", "-c", cmd)
	if runAt != "" {
		cmds.Dir = runAt
	}
	cmds.Stdout, cmds.Stderr = os.Stdout, os.Stderr
	return cmds.Run()
}

// Runs 'jfrog rt c..'.
func createArtServer(serverId string, c *BuildConfig) error {
	log.Info("Setting up Artifactory server")
	configCmd := fmt.Sprintf("jfrog rt c %s --interactive=false --url=%s --user=%s --password=%s", serverId, c.JfrogCredentials.Url, c.JfrogCredentials.User, c.JfrogCredentials.Password)

	err := runCmd("", configCmd)
	if err != nil {
		log.Info("error: " + err.Error())

	}
	return err
}

func createBuildToolConfigs(serverId string, c *BuildConfig) (err error) {
	for k, repo := range c.BuildToolsRepository {
		switch k {
		case Maven:
			err = runCmd("", fmt.Sprintf("jfrog rt mvnc --global --server-id-resolve=%s --server-id-deploy=%s --repo-resolve-releases=%s --repo-resolve-snapshots=%s --repo-deploy-releases=%s --repo-deploy-snapshots=%s", serverId, serverId, repo, repo, repo, repo))
		case Gradle:
			err = runCmd("", fmt.Sprintf("jfrog rt gradlec --global --server-id-resolve=%s --server-id-deploy=%s --repo-resolve=%s --repo-deploy=%s ", serverId, serverId, repo, repo))
		case Npm:
			err = runCmd("", fmt.Sprintf("jfrog rt npmc --global --server-id-resolve=%s --server-id-deploy=%s --repo-resolve=%s --repo-deploy=%s ", serverId, serverId, repo, repo))
		}
		if err != nil {
			return
		}
	}
	return nil
}

// Gets the latest build number for 'buildName' from Artifactory.
// If the build does not exist, return an error.
func getLatestBuildInfo(sm artifactory.ArtifactoryServicesManager, buildName string) (*buildinfo.PublishedBuildInfo, error) {
	previousBuild, found, err := sm.GetBuildInfo(services.BuildInfoParams{BuildName: buildName, BuildNumber: "LATEST"})
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errors.New(fmt.Sprintf("build %s is not found in Artifactory", buildName))
	}
	return previousBuild, nil
}
