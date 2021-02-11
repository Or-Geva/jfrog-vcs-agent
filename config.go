package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/artifactory/auth"
	"github.com/jfrog/jfrog-client-go/config"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
)

const (
	// An env var to override the build number. See getNextBuildNumber func.
	buildNumber = "BUILD_NUMBER"
	// The build name & number env vars that will be publish to Artifactory.
	jfrogBuildName   = "JFROG_CLI_BUILD_NAME"
	jfrogBuildNumber = "JFROG_CLI_BUILD_NUMBER"

	configFilePath = "config/config.json"
)

type Credentials struct {
	Url         string
	User        string
	Password    string
	AccessToken string
}

type Vcs struct {
	*Credentials
	Branch map[string]string
}

type BuildTool string

const (
	Maven  = "maven"
	Gradle = "gradle"
	Npm    = "npm"
)

// Define the 'config.json' file.
type BuildConfig struct {
	BuildCommand         string
	BuildToolsRepository map[BuildTool]string
	Vcs                  *Vcs
	JfrogCredentials     *Credentials
}

// Set jfrog cli build-name and build-number in env vars.
func setBuildProps(buildName, commitSha, prevBuildNumber, runNumber string) (err error) {
	if err := os.Setenv(jfrogBuildName, buildName); err != nil {
		return err
	}
	nbn, err := getNextBuildNumber(prevBuildNumber)
	if err != nil {
		return err
	}
	return os.Setenv(jfrogBuildNumber, fmt.Sprintf("%s.%s-%s", nbn, runNumber, commitSha))
}

// Return the build number to publish for the current run according to the 'BUILD_NUMBER' env var.
// However, if it doesn't exist, uses the last build number+1 as a failback.
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

func unsetBuildProps() error {
	if err := os.Unsetenv(jfrogBuildName); err != nil {
		return err
	}
	return os.Unsetenv(jfrogBuildNumber)
}

// Load the build configuration. It except to be inside config/config.json on the root project.
func getBuildConfig() (*BuildConfig, artifactory.ArtifactoryServicesManager, error) {
	configPath, err := filepath.Abs(configFilePath)
	if err != nil {
		return nil, nil, err
	}
	exists, err := fileutils.IsFileExists(configPath, false)
	if err != nil {
		return nil, nil, err
	}
	if !exists {
		return nil, nil, errors.New("file 'config.json' is not found in the local directory")
	}
	content, err := fileutils.ReadFile(configPath)
	if err != nil {
		return nil, nil, err
	}
	data := new(BuildConfig)
	err = json.Unmarshal(content, data)
	if err != nil {
		return nil, nil, err
	}
	sm, err := createServiceManager(data)
	if err != nil {
		return nil, nil, err
	}
	return data, sm, err
}

func createServiceManager(c *BuildConfig) (artifactory.ArtifactoryServicesManager, error) {
	rtDetails := auth.NewArtifactoryDetails()
	rtDetails.SetUrl(c.JfrogCredentials.Url)
	rtDetails.SetUser(c.JfrogCredentials.User)
	rtDetails.SetPassword(c.JfrogCredentials.Password)
	rtDetails.SetAccessToken(c.JfrogCredentials.AccessToken)

	serviceConfig, err := config.NewConfigBuilder().
		SetServiceDetails(rtDetails).
		SetDryRun(false).
		Build()
	if err != nil {
		return nil, err
	}
	return artifactory.New(&rtDetails, serviceConfig)
}
