package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func main() {
	config, err := LoadConfig("config.ini")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := gorm.Open(sqlite.Open(config.DBPath), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	if err := db.AutoMigrate(&Application{}, &Resource{}); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	handlers := NewHandlers(db, config)

	r := gin.Default()

	// #vibecoded
	// Group all routes under config.Prefix (e.g. "/jobsearch") so the app can be
	// hosted behind a reverse proxy on a sub-path. Empty prefix keeps old behavior.
	group := r.Group(config.Prefix)

	group.Static("/static", "./static")

	group.GET("/", handlers.Dashboard)
	group.GET("/applications", handlers.ApplicationsPage)
	group.GET("/applications/:id", handlers.ViewApplication)
	group.GET("/applications/:id/edit", handlers.EditApplication)
	group.POST("/applications", handlers.CreateApplication)
	group.POST("/applications/:id", handlers.UpdateApplication)
	group.PUT("/api/applications/:id/status", handlers.UpdateStatus)
	group.DELETE("/api/applications/:id", handlers.DeleteApplication)

	group.GET("/record", handlers.RecordPage)

	group.GET("/resources", handlers.ResourcesPage)
	group.POST("/resources", handlers.UploadResource)
	group.GET("/resources/:id", handlers.ViewResource)
	group.DELETE("/api/resources/:id", handlers.DeleteResource)

	group.GET("/screenshot/:id", handlers.ViewScreenshot)
	group.POST("/api/applications/:id/screenshot", handlers.CaptureScreenshot)

	group.GET("/api/prefill", handlers.PrefillURL)
	group.GET("/api/search", handlers.SearchApplications)
	group.GET("/api/export/csv", handlers.ExportCSV)
	group.GET("/api/gowitness/status", handlers.GowitnessStatus)
	group.POST("/api/applications/update-old", handlers.UpdateOldApplications)

	addr := fmt.Sprintf("%s:%s", config.Host, config.Port)
	log.Printf("Starting server on %s", addr)
	if err := r.Run(addr); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Failed to start server: %v", err)
	}
}
