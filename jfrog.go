package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/auth"
	"github.com/jfrog/jfrog-client-go/artifactory/buildinfo"
	"github.com/jfrog/jfrog-client-go/artifactory/services"
	"github.com/jfrog/jfrog-client-go/config"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	// Default name for downlowing build-info from Artifactory
	buildInfoFile = "buildinfo.json"
	// A unique ID for the new Artifactory server configuration.
	serverId = "vcs-superhighway"
	// Set the default buildNumber for every build branch.
	buildNumber = "BUILD_NUMBER"
	// The build name & number env vars to be used by JFrog CLI commands.
	jfrogBuildName   = "JFROG_CLI_BUILD_NAME"
	jfrogBuildNumber = "JFROG_CLI_BUILD_NUMBER"
)

// Configure JFrog CLI with Artifactory servers, which can later be used in the other commands.
func createArtServer(c *BuildConfig) error {
	log.Info("Setting up Artifactory server on agent")
	configCmd := fmt.Sprintf("jfrog rt c %s --interactive=false --url=%s --user=%s --password=%s ", serverId, c.Jfrog.ArtUrl, c.Jfrog.User, c.Jfrog.Password)
	return runCmd("", configCmd)
}

// runs build command at 'projectPath'.
// Build-name & build-number expect to be in env vars
func build(buildCommand, projectPath string) error {
	log.Info("Execute build command " + buildCommand)
	return runCmd(projectPath, buildCommand)
}

// Build-name & build-number expect to be in env vars
func bag(projectPath string) error {
	log.Info("Collect VCS details at '" + projectPath + "'")
	return runCmd(projectPath, "jfrog rt bag --server-id="+serverId)
}

// Build-name & build-number expect to be in env vars
func publish() error {
	log.Info("Publish the build to Artifactory")
	return runCmd("", "jfrog rt bp --server-id="+serverId)
}

// Build-name & build-number expect to be in env vars
func buildScan() error {
	log.Info("Scan the published build with Xray")
	return runCmd("", "jfrog rt bs --server-id="+serverId)
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

func deleteArtServer() error {
	configCmd := fmt.Sprintf("jfrog rt c  delete %s --interactive=false ", serverId)
	return runCmd("", configCmd)
}

// Before using the mvn/gradle/npm commands, the project needs to be pre-configured with the Artifactory server and repositories, to be used for building and publishing the project
func createBuildToolConfigs(runAt string, c *BuildConfig) (err error) {
	for k, repo := range c.Jfrog.Repositories {
		switch k {
		case Maven:
			err = runCmd(runAt, fmt.Sprintf("jfrog rt mvnc --global --server-id-resolve=%s --server-id-deploy=%s --repo-resolve-releases=%s --repo-resolve-snapshots=%s --repo-deploy-releases=%s --repo-deploy-snapshots=%s", serverId, serverId, repo, repo, repo, repo))
		case Gradle:
			err = runCmd(runAt, fmt.Sprintf("jfrog rt gradlec --global --server-id-resolve=%s --server-id-deploy=%s --repo-resolve=%s --repo-deploy=%s ", serverId, serverId, repo, repo))
		case Npm:
			err = runCmd(runAt, fmt.Sprintf("jfrog rt npmc --global --server-id-resolve=%s --server-id-deploy=%s --repo-resolve=%s --repo-deploy=%s ", serverId, serverId, repo, repo))
		}
		if err != nil {
			return
		}
	}
	return nil
}

// Set jfrog cli build-name and build-number in env vars to be use by the agent during the build.
func setBuildProps(buildName, commitSha, prevBuildNumber, runNumber string) (err error) {
	log.Info("Generating JFrog CLI build environment variables")
	if err := os.Setenv(jfrogBuildName, buildName); err != nil {
		return err
	}
	nbn, err := getNextBuildNumber(prevBuildNumber)
	if err != nil {
		return err
	}
	return os.Setenv(jfrogBuildNumber, fmt.Sprintf("%s.%s-%s", nbn, runNumber, commitSha))
}

// Return the value of 'BUILD_NUMBER' env var.
// If not configured, return the last run build number incremented by 1.
func getNextBuildNumber(prevBuildNumber string) (nbn string, err error) {
	bn := 0
	if buildNumber := os.Getenv(buildNumber); buildNumber != "" {
		bn, err = strconv.Atoi(buildNumber)
		if err != nil {
			return
		}
	} else {
		prevBuildNumberIdx := strings.Index(prevBuildNumber, ".")
		bn, err = strconv.Atoi(prevBuildNumber[:prevBuildNumberIdx])
		if err != nil {
			return
		}
		bn++
	}
	nbn = strconv.Itoa(bn)
	return
}

func unsetJfrogBuildProps() error {
	var err error
	for _, env := range []string{jfrogBuildName, jfrogBuildNumber} {
		if os.Getenv(env) == "" {
			continue
		}
		if err = os.Unsetenv(env); err != nil {
			return err
		}
	}
	return nil
}

// Gets the latest build number from Artifactory.
// If the build does not exist, return an error.
func getLatestBuildInfo(sm artifactory.ArtifactoryServicesManager, buildName string) (bi *buildinfo.BuildInfo, err error) {
	params := services.NewDownloadParams()
	params.Pattern = "artifactory-build-info/" + buildName + "/*"
	params.Target = buildInfoFile
	params.SortBy = []string{"created"}
	params.SortOrder = "desc"
	params.Limit = 1
	params.Flat = true
	log.Info("Searching the latest build for '" + buildName + "' build")
	var totalDownloaded int
	totalDownloaded, _, err = sm.DownloadFiles(params)
	if err != nil || totalDownloaded == 0 {
		return nil, errors.New(fmt.Sprintf("build '%s' is not found in Artifactory", buildName))
	}
	defer func() {
		if e := os.Remove(buildInfoFile); err == nil {
			err = e
		}
	}()
	var d []byte
	bi = new(buildinfo.BuildInfo)
	d, err = ioutil.ReadFile(buildInfoFile)
	if err != nil {
		return
	}
	err = json.Unmarshal(d, bi)
	return
}

// Create the branch build name from the build config.
// replace '${projectName}' with BuildConfig.ProjectName and '${branch}' with branch name.
func getBranchBuildName(branch string, c *BuildConfig) string {
	buildName := strings.Replace(c.Jfrog.BuildName, "${projectName}", c.ProjectName, -1)
	buildName = strings.Replace(buildName, "${branch}", branch, -1)
	log.Info("Associate build to the branch is '" + buildName + "'")
	return buildName
}

func createServiceManager(c *BuildConfig) (artifactory.ArtifactoryServicesManager, error) {
	rtDetails := auth.NewArtifactoryDetails()
	rtDetails.SetUrl(c.Jfrog.ArtUrl)
	rtDetails.SetUser(c.Jfrog.User)
	rtDetails.SetPassword(c.Jfrog.Password)

	serviceConfig, err := config.NewConfigBuilder().
		SetServiceDetails(rtDetails).
		SetDryRun(false).
		Build()
	if err != nil {
		return nil, err
	}
	return artifactory.New(&rtDetails, serviceConfig)
}

// Returns the vcs revision from build-info.
func getLatestCommitSha(bi *buildinfo.BuildInfo, vcsUrl string) (string, error) {
	for _, vcs := range bi.VcsList {
		if vcs.Url == vcsUrl {
			return vcs.Revision, nil
		}
	}
	return "", errors.New("No revision is found for git repository: '" + vcsUrl + "'")
}
