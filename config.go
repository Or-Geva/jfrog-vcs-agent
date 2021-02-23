package main

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"gopkg.in/yaml.v2"
)

const (
	// config file name on the local agent.
	configFile = "config.yaml"
)

type JfrogDetails struct {
	ArtUrl       string               `yaml:"artUrl"`
	User         string               `yaml:"user"`
	Password     string               `yaml:"password"`
	Repositories map[BuildTool]string `yaml:"repositories"`
	BuildName    string               `yaml:"buildName"`
}

type Vcs struct {
	Url      string   `yaml:"url"`
	User     string   `yaml:"user"`
	Password string   `yaml:"password"`
	Token    string   `yaml:"token"`
	Branches []string `yaml:"branches"`
}

type BuildTool string

const (
	Maven  = "maven"
	Gradle = "gradle"
	Npm    = "npm"
)

// Define the file 'config.yaml'.
type BuildConfig struct {
	ProjectName  string        `yaml:"projectName"`
	BuildCommand string        `yaml:"buildCommand"`
	Vcs          *Vcs          `yaml:"vcs"`
	Jfrog        *JfrogDetails `yaml:"jfrog"`
}

// Load the build configuration from a yaml file.
func loadBuildConfig() (*BuildConfig, artifactory.ArtifactoryServicesManager, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, nil, err
	}
	content, err := fileutils.ReadFile(configPath)
	if err != nil {
		return nil, nil, err
	}
	data := new(BuildConfig)
	err = yaml.Unmarshal(content, data)
	if err != nil {
		return nil, nil, err
	}
	sm, err := createServiceManager(data)
	if err != nil {
		return nil, nil, err
	}
	return data, sm, err
}

// Config directory is expected to be in the parent directory.
func getConfigPath() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	configPath := filepath.Join(dir, "..", "config", configFile)
	exists, err := fileutils.IsFileExists(configPath, false)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", errors.New("file '" + configFile + "' is not found in '" + configPath + "'")
	}
	log.Info("Found config file at '" + configPath + "'")
	return configPath, nil
}
