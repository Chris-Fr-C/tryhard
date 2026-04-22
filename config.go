package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gookit/config/v2"
	"github.com/gookit/config/v2/ini"
)

type Config struct {
	DBPath           string
	BlobsPath        string
	ScreenshotsPath  string
	GowitnessPath    string
	GowitnessEnabled bool
	RejectedDays     int
	Host             string
	Port             string
}

func expandPath(path string) string {
	if strings.Contains(path, "$HOME") {
		home := os.Getenv("HOME")
		if home != "" {
			path = strings.ReplaceAll(path, "$HOME", home)
		}
	}
	if strings.HasPrefix(path, "~") {
		home := os.Getenv("HOME")
		if home != "" {
			path = strings.Replace(path, "~", home, 1)
		}
	}
	return path
}

func createDefaultConfig(configPath string) error {
	defaultConfig := `[database]
db_path = ./data/jobtracker.sqlite

[storage]
blobs_path = ./data/blobs
screenshots_path = ./data/screenshots

[gowitness]
path = $HOME/go/bin/gowitness
enabled = false

[automation]
rejected_days = 21

[app]
host = 0.0.0.0
port = 8080
`
	return os.WriteFile(configPath, []byte(defaultConfig), 0644)
}

func LoadConfig(configPath string) (*Config, error) {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := createDefaultConfig(configPath); err != nil {
			return nil, err
		}
	}

	config.AddDriver(ini.Driver)
	err := config.LoadFiles(configPath)
	if err != nil {
		return nil, err
	}

	dbPath := config.String("database.db_path", "./data/jobtracker.sqlite")
	blobsPath := config.String("storage.blobs_path", "./data/blobs")
	screenshotsPath := config.String("storage.screenshots_path", "./data/screenshots")
	gowitnessPath := config.String("gowitness.path", "$HOME/go/bin/gowitness")
	gowitnessEnabled := config.Bool("gowitness.enabled", false)
	rejectedDays := config.Int("automation.rejected_days", 21)

	gowitnessPath = expandPath(gowitnessPath)
	host := config.String("app.host", "0.0.0.0")
	port := config.String("app.port", "8080")

	if !filepath.IsAbs(dbPath) {
		absPath, err := filepath.Abs(dbPath)
		if err == nil {
			dbPath = absPath
		}
	}

	if !filepath.IsAbs(blobsPath) {
		absPath, err := filepath.Abs(blobsPath)
		if err == nil {
			blobsPath = absPath
		}
	}

	if !filepath.IsAbs(screenshotsPath) {
		absPath, err := filepath.Abs(screenshotsPath)
		if err == nil {
			screenshotsPath = absPath
		}
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(blobsPath, 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(screenshotsPath, 0755); err != nil {
		return nil, err
	}

	return &Config{
		DBPath:           dbPath,
		BlobsPath:        blobsPath,
		ScreenshotsPath:  screenshotsPath,
		GowitnessPath:    gowitnessPath,
		GowitnessEnabled: gowitnessEnabled,
		RejectedDays:     rejectedDays,
		Host:             host,
		Port:             port,
	}, nil
}
