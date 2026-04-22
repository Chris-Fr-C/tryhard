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

	r.Static("/static", "./static")

	r.GET("/", handlers.Dashboard)
	r.GET("/applications", handlers.ApplicationsPage)
	r.GET("/applications/:id", handlers.ViewApplication)
	r.GET("/applications/:id/edit", handlers.EditApplication)
	r.POST("/applications", handlers.CreateApplication)
	r.POST("/applications/:id", handlers.UpdateApplication)
	r.PUT("/api/applications/:id/status", handlers.UpdateStatus)
	r.DELETE("/api/applications/:id", handlers.DeleteApplication)

	r.GET("/record", handlers.RecordPage)

	r.GET("/resources", handlers.ResourcesPage)
	r.POST("/resources", handlers.UploadResource)
	r.GET("/resources/:id", handlers.ViewResource)
	r.DELETE("/api/resources/:id", handlers.DeleteResource)

	r.GET("/screenshot/:id", handlers.ViewScreenshot)
	r.POST("/api/applications/:id/screenshot", handlers.CaptureScreenshot)

	r.GET("/api/prefill", handlers.PrefillURL)
	r.GET("/api/search", handlers.SearchApplications)
	r.GET("/api/export/csv", handlers.ExportCSV)
	r.GET("/api/gowitness/status", handlers.GowitnessStatus)
	r.POST("/api/applications/update-old", handlers.UpdateOldApplications)

	addr := fmt.Sprintf("%s:%s", config.Host, config.Port)
	log.Printf("Starting server on %s", addr)
	if err := r.Run(addr); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Failed to start server: %v", err)
	}
}
