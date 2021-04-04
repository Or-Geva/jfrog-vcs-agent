package utils

import (
	"encoding/json"
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
	// Name of downloaded build-info file from Artifactory.
	buildInfoFile = "buildinfo.json"
	// A unique ID for a new Artifactory server configuration.
	serverId = "vcs-superhighway"

	// Environment variables
	// The next build number to be published by JFrog CLI (Optional).
	buildNumber = "BUILD_NUMBER"
	// The build name & number to be used by JFrog CLI commands.
	jfrogBuildName   = "JFROG_CLI_BUILD_NAME"
	jfrogBuildNumber = "JFROG_CLI_BUILD_NUMBER"
)

// Configure JFrog CLI with Artifactory servers, which can later be used in the other commands.
func CreateArtServer(c *BuildConfig) error {
	log.Info("Setting up Artifactory server on agent")
	configCmd := fmt.Sprintf("jfrog rt c %s --interactive=false --url=%s --user=%s --password=%s ", serverId, c.Jfrog.ArtUrl, c.Jfrog.User, c.Jfrog.Password)
	return RunCmd("", configCmd)
}

// Runs build command at 'projectPath'.
// Build-name & build-number are expected to be set as env vars
func Build(buildCommand, projectPath string) error {
	log.Info("Executing build command '%s'...", buildCommand)
	return RunCmd(projectPath, buildCommand)
}

// Build-name & build-number are expected to be set as env vars
func Bag(projectPath string) error {
	log.Info("Collecting VCS details...")
	return RunCmd(projectPath, "jfrog rt bag --server-id="+serverId)
}

// Build-name & build-number are expected to be set as env vars
func Publish() error {
	log.Info("Publishing the build to Artifactory...")
	return RunCmd("", "jfrog rt bp --server-id="+serverId)
}

// Build-name & build-number are expected to be set as env vars
func BuildScan() error {
	log.Info("Scanning the published build with Xray...")
	return RunCmd("", "jfrog rt bs --server-id="+serverId)
}

// Run a command in the bash shell. If 'runAt' is specified, the command will be executed at this path context.
func RunCmd(runAt string, cmd string) error {
	cmds := exec.Command("bash", "-c", cmd)
	if runAt != "" {
		cmds.Dir = runAt
	}
	cmds.Stdout, cmds.Stderr = os.Stdout, os.Stderr
	return cmds.Run()
}

func DeleteArtServer() error {
	configCmd := fmt.Sprintf("jfrog rt c  delete %s --interactive=false ", serverId)
	return RunCmd("", configCmd)
}

// Before using the mvn/gradle/npm commands, the project needs to be pre-configured with the Artifactory server and repositories, to be used for building and publishing the project
func CreateBuildToolConfigs(runAt string, c *BuildConfig) (err error) {
	for k, repo := range c.Jfrog.Repositories {
		switch k {
		case Maven:
			err = RunCmd(runAt, fmt.Sprintf("jfrog rt mvnc --global --server-id-resolve=%s --server-id-deploy=%s --repo-resolve-releases=%s --repo-resolve-snapshots=%s --repo-deploy-releases=%s --repo-deploy-snapshots=%s", serverId, serverId, repo, repo, repo, repo))
		case Gradle:
			err = RunCmd(runAt, fmt.Sprintf("jfrog rt gradlec --global --server-id-resolve=%s --server-id-deploy=%s --repo-resolve=%s --repo-deploy=%s ", serverId, serverId, repo, repo))
		case Npm:
			err = RunCmd(runAt, fmt.Sprintf("jfrog rt npmc --global --server-id-resolve=%s --server-id-deploy=%s --repo-resolve=%s --repo-deploy=%s ", serverId, serverId, repo, repo))
		}
		if err != nil {
			return
		}
	}
	return
}

// Set jfrog cli build-name and build-number as env vars, to be use by the agent during the build.
func SetBuildProps(buildName, commitSha, prevBuildNumber, runNumber string) (err error) {
	log.Info("Generating JFrog CLI build environment variables...")
	if err := os.Setenv(jfrogBuildName, buildName); err != nil {
		return err
	}
	buildNumber, err := GetNextBuildNumber(prevBuildNumber)
	if err != nil {
		return err
	}
	return os.Setenv(jfrogBuildNumber, fmt.Sprintf("%s.%s-%s", buildNumber, runNumber, commitSha))
}

// Return the value of 'BUILD_NUMBER' env var.
// If not configured, return the last run build number incremented by 1.
func GetNextBuildNumber(prevBuildNumber string) (nextBuildNumber string, err error) {
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
	nextBuildNumber = strconv.Itoa(bn)
	return
}

func UnsetJfrogBuildProps() error {
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
func GetLatestBuildInfo(ArtifactoryServicesManager artifactory.ArtifactoryServicesManager, buildName string) (buildInfo *buildinfo.BuildInfo, err error) {
	params := services.NewDownloadParams()
	params.Pattern = "artifactory-build-info/" + buildName + "/*"
	params.Target = buildInfoFile
	params.SortBy = []string{"created"}
	params.SortOrder = "desc"
	params.Limit = 1
	params.Flat = true
	log.Info("Searching the latest build for '%s' build...", buildName)
	var totalDownloaded int
	totalDownloaded, _, err = ArtifactoryServicesManager.DownloadFiles(params)
	if err != nil {
		return nil, fmt.Errorf("failed to download build '%s' from Artifactory, Error: '%s'", buildName, err.Error())
	}
	if totalDownloaded == 0 {
		return nil, fmt.Errorf("build '%s' is not found in Artifactory", buildName)
	}
	defer func() {
		if e := os.Remove(buildInfoFile); err == nil {
			err = e
		}
	}()
	var data []byte
	buildInfo = new(buildinfo.BuildInfo)
	data, err = ioutil.ReadFile(buildInfoFile)
	if err != nil {
		return
	}
	err = json.Unmarshal(data, buildInfo)
	return
}

// Create the branch build name from the build config.
// replace '${projectName}' with BuildConfig.ProjectName and '${branch}' with branch name.
func GetBranchBuildName(branch string, c *BuildConfig) string {
	buildName := strings.Replace(c.Jfrog.BuildName, "${projectName}", c.ProjectName, -1)
	buildName = strings.Replace(buildName, "${branch}", branch, -1)
	log.Info("The associate branch build-name is '%s'", buildName)
	return buildName
}

func createServiceManager(buildConfig *BuildConfig) (artifactory.ArtifactoryServicesManager, error) {
	rtDetails := auth.NewArtifactoryDetails()
	rtDetails.SetUrl(buildConfig.Jfrog.ArtUrl)
	rtDetails.SetUser(buildConfig.Jfrog.User)
	rtDetails.SetPassword(buildConfig.Jfrog.Password)

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
func getBuildCommitSha(bi *buildinfo.BuildInfo, vcsUrl string) (string, error) {
	for _, vcs := range bi.VcsList {
		if vcs.Url == vcsUrl {
			return vcs.Revision, nil
		}
	}
	return "", fmt.Errorf("No revision is found for git repository: '%s'", vcsUrl)
}
