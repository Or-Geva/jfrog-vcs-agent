package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/stretchr/testify/assert"
)

func init() {
	log.SetLogger(log.NewLogger(log.INFO, nil))
}
func TestLoadConfigByFile(t *testing.T) {
	// Verify env doesn't exists.
	if fromEnv := os.Getenv(configEnvVar); fromEnv != "" {
		defer func() { err := os.Setenv(configEnvVar, fromEnv); assert.NoError(t, err) }()
		err := os.Setenv(configEnvVar, "")
		assert.NoError(t, err)
	}
	oldPath := configPath
	var err error
	configPath, err = filepath.Abs(filepath.Join("testdata", "config.yaml"))
	assert.NoError(t, err)
	defer func() { configPath = oldPath }()
	runConfigValidation(t)
}

func TestLoadConfigByEnv(t *testing.T) {
	if fromEnv := os.Getenv(configEnvVar); fromEnv != "" {
		defer func() { err := os.Setenv(configEnvVar, fromEnv); assert.NoError(t, err) }()
	}
	err := os.Setenv(configEnvVar, "cHJvamVjdE5hbWU6IG5wbS1leGFtcGxlCmJ1aWxkQ29tbWFuZDogbnBtIGkKdmNzOgogIHVybDogaHR0cHM6Ly9naXRodWIuY29tL09yLUdldmEvbnBtLWV4YW1wbGUuZ2l0CiAgdXNlcjogdGVzdAogIHBhc3N3b3JkOiAiIgogIHRva2VuOiA3ZTI3Mjk2N2FkYTRkNGJlNDkyMGMxYmQ3YWMwZmQ5ODhhNzdlNzJiCiAgYnJhbmNoZXM6CiAgLSBtYWluCiAgLSBkZXYKamZyb2c6CiAgYXJ0VXJsOiBodHRwOi8vbG9jYWxob3N0OjgwODAvYXJ0aWZhY3RvcnkvCiAgdXNlcjogYWRtaW4KICBwYXNzd29yZDogcGFzc3dvcmQKICByZXBvc2l0b3JpZXM6CiAgICBucG06IG5wbS12aXJ0dWFsCiAgICBtdm46IG12bi12aXJ0dWFsCiAgICBncmFkbGU6IGdyYWRsZS12aXJ0dWFsCiAgYnVpbGROYW1lOiAke3Byb2plY3ROYW1lfS0ke2JyYW5jaH0=")
	assert.NoError(t, err)
	runConfigValidation(t)
}

func runConfigValidation(t *testing.T) {
	buildConfig, ArtifactoryServicesManager, err := LoadBuildConfig()
	assert.NoError(t, err)
	assert.Equal(t, excpectedConfig(), buildConfig)
	assert.Equal(t, "http://localhost:8080/artifactory/", ArtifactoryServicesManager.GetConfig().GetServiceDetails().GetUrl())
	assert.Equal(t, "admin", ArtifactoryServicesManager.GetConfig().GetServiceDetails().GetUser())
	assert.Equal(t, "password", ArtifactoryServicesManager.GetConfig().GetServiceDetails().GetPassword())
}

func excpectedConfig() *BuildConfig {
	return &BuildConfig{
		ProjectName:  "npm-example",
		BuildCommand: "npm i",
		Vcs: &Vcs{
			Url:      "https://github.com/Or-Geva/npm-example.git",
			User:     "test",
			Password: "",
			Token:    "7e272967ada4d4be4920c1bd7ac0fd988a77e72b",
			Branches: []string{"main", "dev"},
		},
		Jfrog: &JfrogDetails{
			ArtUrl:       "http://localhost:8080/artifactory/",
			User:         "admin",
			Password:     "password",
			Repositories: map[BuildTool]string{"npm": "npm-virtual", "mvn": "mvn-virtual", "gradle": "gradle-virtual"},
			BuildName:    "${projectName}-${branch}",
		},
	}
}
