package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gookit/config/v2"
	"github.com/gookit/config/v2/ini"
)

type Config struct {
	DBPath          string
	BlobsPath       string // That one is for the resume and stuff.
	ScreenshotsPath string // If you wanna record the website since some companies just delete the add.
	GowitnessPath   string  // Link to da binary if you wanna screenshot. Usually $HOME/go/bin
	Host            string
	Port            string
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

func LoadConfig(configPath string) (*Config, error) {
	config.AddDriver(ini.Driver)
	err := config.LoadFiles(configPath)
	if err != nil {
		return nil, err
	}

	dbPath := config.String("database.db_path", "./data/jobtracker.db")
	blobsPath := config.String("storage.blobs_path", "./blobs")
	screenshotsPath := config.String("storage.screenshots_path", "./screenshots")
	gowitnessPath := config.String("gowitness.path", "/home/christophe/go/bin/gowitness")

	gowitnessPath = expandPath(gowitnessPath)
	host := config.String("app.host", "0.0.0.0")
	port := config.String("app.port", "8080")

	gowitnessPath = expandPath(gowitnessPath)

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
		DBPath:          dbPath,
		BlobsPath:       blobsPath,
		ScreenshotsPath: screenshotsPath,
		GowitnessPath:   gowitnessPath,
		Host:            host,
		Port:            port,
	}, nil
}
