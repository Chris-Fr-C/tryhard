package main

// Here comes da big file cuz im too lazy to do it properly.
// #vibecoded
import (
	"bytes"
	"crypto/sha1"
	"embed"
	"encoding/csv"
	"fmt"
	"html/template"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

//go:embed templates/*
var templatesFS embed.FS

type Handlers struct {
	db     *gorm.DB
	config *Config
}

func NewHandlers(db *gorm.DB, config *Config) *Handlers {
	return &Handlers{db: db, config: config}
}

func (h *Handlers) render(c *gin.Context, page string, data gin.H) {
	contentTmpl := template.Must(template.ParseFS(templatesFS, fmt.Sprintf("templates/%s.html", page)))
	var contentBuf bytes.Buffer
	contentTmpl.ExecuteTemplate(&contentBuf, page, data)
	data["Content"] = template.HTML(contentBuf.String())
	data["CurrentPage"] = page

	baseTmpl := template.Must(template.ParseFS(templatesFS, "templates/base.html"))
	baseTmpl.Execute(c.Writer, data)
}

type DashboardData struct {
	Total        int64
	NotSent      int64
	Submitted    int64
	Interview    int64
	Waiting      int64
	Offer        int64
	Applications []Application
}

func (h *Handlers) Dashboard(c *gin.Context) {
	var total, notSent, submitted, interview, waiting, offer, rejected int64

	h.db.Model(&Application{}).Count(&total)
	h.db.Model(&Application{}).Where("status = ?", StatusNotSubmitted).Count(&notSent)
	h.db.Model(&Application{}).Where("status = ?", StatusSubmitted).Count(&submitted)
	h.db.Model(&Application{}).Where("status = ?", StatusInterview).Count(&interview)
	h.db.Model(&Application{}).Where("status = ?", StatusWaitingAnswer).Count(&waiting)
	h.db.Model(&Application{}).Where("status = ?", StatusOffer).Count(&offer)
	h.db.Model(&Application{}).Where("status = ?", StatusRejected).Count(&rejected)

	var apps []Application
	h.db.Preload("Resume").Preload("Reference").Order("created_at DESC").Limit(20).Find(&apps)

	data := gin.H{
		"Total":        total,
		"NotSent":      notSent,
		"Submitted":    submitted,
		"Interview":    interview,
		"Waiting":      waiting,
		"Offer":        offer,
		"Rejected":     rejected,
		"RejectedDays": h.config.RejectedDays,
		"Applications": apps,
	}

	h.render(c, "dashboard", data)
}

func (h *Handlers) ApplicationsPage(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	perPage := 20

	var total int64
	h.db.Model(&Application{}).Count(&total)

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	var apps []Application
	offset := (page - 1) * perPage
	h.db.Preload("Resume").Preload("Reference").Order("created_at DESC").Offset(offset).Limit(perPage).Find(&apps)

	data := gin.H{
		"Applications": apps,
		"Page":         page,
		"TotalPages":   totalPages,
		"HasPrev":      page > 1,
		"HasNext":      page < totalPages,
		"PrevPage":     page - 1,
		"NextPage":     page + 1,
	}

	h.render(c, "applications", data)
}

func (h *Handlers) ViewApplication(c *gin.Context) {
	id := c.Param("id")
	var app Application
	if err := h.db.Preload("Resume").Preload("Reference").First(&app, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Application not found"})
		return
	}

	data := gin.H{
		"Application": app,
	}

	h.render(c, "view", data)
}

func (h *Handlers) RecordPage(c *gin.Context) {
	var resumes []Resource
	var references []Resource
	h.db.Where("resource_type = ?", "resume").Order("created_at DESC").Find(&resumes)
	h.db.Where("resource_type = ?", "reference").Order("created_at DESC").Find(&references)

	data := gin.H{
		"Resumes":    resumes,
		"References": references,
	}

	h.render(c, "record", data)
}

type CreateApplicationRequest struct {
	JobURL      string `form:"job_url" binding:"required"`
	CompanyName string `form:"company_name"`
	CompanyURL  string `form:"company_url"`
	Status      string `form:"status"`
	ResumeID    string `form:"resume_id"`
	ReferenceID string `form:"reference_id"`
	Notes       string `form:"notes"`
}

func (h *Handlers) CreateApplication(c *gin.Context) {
	var req CreateApplicationRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	screenshotPath := ""
	if req.JobURL != "" && h.config.GowitnessEnabled {
		screenshotPath = captureWithGowitness(req.JobURL, h.config.ScreenshotsPath, h.config.GowitnessPath)
	}

	if req.CompanyName == "" && req.JobURL != "" {
		req.CompanyName = extractCompanyFromURL(req.JobURL)
	}

	app := Application{
		JobURL:         req.JobURL,
		ScreenshotPath: screenshotPath,
		CompanyName:    req.CompanyName,
		CompanyURL:     req.CompanyURL,
		Status:         StatusNotSubmitted,
		Notes:          req.Notes,
	}

	if req.Status != "" {
		app.Status = ApplicationStatus(req.Status)
	}

	if req.ResumeID != "" {
		var resume Resource
		if err := h.db.First(&resume, req.ResumeID).Error; err == nil {
			app.ResumeID = &resume.ID
		}
	}

	if req.ReferenceID != "" {
		var ref Resource
		if err := h.db.First(&ref, req.ReferenceID).Error; err == nil {
			app.ReferenceID = &ref.ID
		}
	}

	if app.ResumeID == nil {
		var latestResume Resource
		if err := h.db.Where("resource_type = ?", "resume").Order("created_at DESC").First(&latestResume).Error; err == nil {
			app.ResumeID = &latestResume.ID
		}
	}

	h.db.Create(&app)

	continueRecording := c.Query("continue")
	if continueRecording == "true" {
		c.Redirect(http.StatusFound, "/record")
	}
	c.Redirect(http.StatusFound, "/")
}

func (h *Handlers) EditApplication(c *gin.Context) {
	id := c.Param("id")
	var app Application
	if err := h.db.First(&app, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Application not found"})
		return
	}

	var resumes []Resource
	var references []Resource
	h.db.Where("resource_type = ?", "resume").Order("created_at DESC").Find(&resumes)
	h.db.Where("resource_type = ?", "reference").Order("created_at DESC").Find(&references)

	data := gin.H{
		"Application": app,
		"Resumes":     resumes,
		"References":  references,
	}

	h.render(c, "edit", data)
}

func (h *Handlers) UpdateApplication(c *gin.Context) {
	id := c.Param("id")
	var app Application
	if err := h.db.First(&app, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Application not found"})
		return
	}

	var req CreateApplicationRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.JobURL != app.JobURL && req.JobURL != "" {
		if app.ScreenshotPath != "" {
			os.Remove(app.ScreenshotPath)
		}
		if h.config.GowitnessEnabled {
			screenshotPath := captureWithGowitness(req.JobURL, h.config.ScreenshotsPath, h.config.GowitnessPath)
			app.ScreenshotPath = screenshotPath
		}
	}

	app.JobURL = req.JobURL
	app.CompanyName = req.CompanyName
	app.CompanyURL = req.CompanyURL
	app.Notes = req.Notes

	if req.Status != "" {
		app.Status = ApplicationStatus(req.Status)
	}

	if req.ResumeID != "" {
		var resume Resource
		if err := h.db.First(&resume, req.ResumeID).Error; err == nil {
			app.ResumeID = &resume.ID
		}
	} else {
		app.ResumeID = nil
	}

	if req.ReferenceID != "" {
		var ref Resource
		if err := h.db.First(&ref, req.ReferenceID).Error; err == nil {
			app.ReferenceID = &ref.ID
		}
	} else {
		app.ReferenceID = nil
	}

	h.db.Save(&app)
	c.Redirect(http.StatusFound, fmt.Sprintf("/applications/%s", id))
}

func (h *Handlers) UpdateStatus(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.db.Model(&Application{}).Where("id = ?", id).Update("status", req.Status)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handlers) DeleteApplication(c *gin.Context) {
	id := c.Param("id")
	var app Application
	if err := h.db.First(&app, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Application not found"})
		return
	}

	if app.ScreenshotPath != "" {
		os.Remove(app.ScreenshotPath)
	}

	h.db.Delete(&app)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handlers) PrefillURL(c *gin.Context) {
	targetURL := c.Query("url")
	if targetURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL required"})
		return
	}

	companyName := extractCompanyFromURL(targetURL)
	companyURL := buildCompanyURL(targetURL)

	var existingApps []Application
	h.db.Where("company_name LIKE ?", "%"+companyName+"%").Or("company_url LIKE ?", "%"+companyURL+"%").Or("job_url = ?", targetURL).Find(&existingApps)

	result := gin.H{
		"company_name":   companyName,
		"company_url":    companyURL,
		"existing_apps":  existingApps,
		"has_existing":   len(existingApps) > 0,
		"existing_count": len(existingApps),
	}

	c.JSON(http.StatusOK, result)
}

func (h *Handlers) SearchApplications(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query required"})
		return
	}

	var apps []Application
	h.db.Preload("Resume").Preload("Reference").
		Where("company_name LIKE ? OR job_url LIKE ? OR company_url LIKE ? OR notes LIKE ?",
			"%"+query+"%", "%"+query+"%", "%"+query+"%", "%"+query+"%").
		Order("created_at DESC").
		Find(&apps)

	c.JSON(http.StatusOK, gin.H{
		"results": apps,
		"count":   len(apps),
		"query":   query,
	})
}

func (h *Handlers) ResourcesPage(c *gin.Context) {
	var resumes []Resource
	var references []Resource
	h.db.Where("resource_type = ?", "resume").Order("created_at DESC").Find(&resumes)
	h.db.Where("resource_type = ?", "reference").Order("created_at DESC").Find(&references)

	data := gin.H{
		"Resumes":    resumes,
		"References": references,
	}

	h.render(c, "resources", data)
}

func (h *Handlers) UploadResource(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided"})
		return
	}

	resourceType := c.PostForm("resource_type")
	if resourceType != "resume" && resourceType != "reference" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid resource type"})
		return
	}

	name := c.PostForm("name")
	if name == "" {
		name = file.Filename
	}

	savedPath, err := saveUploadedFile(file, h.config.BlobsPath, resourceType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	resource := Resource{
		Name:         name,
		Filename:     file.Filename,
		FilePath:     savedPath,
		MimeType:     file.Header.Get("Content-Type"),
		ResourceType: resourceType,
	}

	h.db.Create(&resource)
	c.Redirect(http.StatusFound, "/resources")
}

func (h *Handlers) ViewResource(c *gin.Context) {
	id := c.Param("id")
	var resource Resource
	if err := h.db.First(&resource, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Resource not found"})
		return
	}

	c.Header("Content-Type", resource.MimeType)
	c.File(resource.FilePath)
}

func (h *Handlers) DeleteResource(c *gin.Context) {
	id := c.Param("id")
	var resource Resource
	if err := h.db.First(&resource, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Resource not found"})
		return
	}

	os.Remove(resource.FilePath)
	h.db.Delete(&resource)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *Handlers) ViewScreenshot(c *gin.Context) {
	id := c.Param("id")
	var app Application
	if err := h.db.First(&app, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Application not found"})
		return
	}

	if app.ScreenshotPath == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "No screenshot saved"})
		return
	}

	c.File(app.ScreenshotPath)
}

func (h *Handlers) CaptureScreenshot(c *gin.Context) {
	if !h.config.GowitnessEnabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "Gowitness is not enabled. Set enabled = true in config.ini to enable."})
		return
	}

	id := c.Param("id")
	var app Application
	if err := h.db.First(&app, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Application not found"})
		return
	}

	if app.JobURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No job URL to capture"})
		return
	}

	screenshotPath := captureWithGowitness(app.JobURL, h.config.ScreenshotsPath, h.config.GowitnessPath)

	if screenshotPath == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to capture screenshot. Is gowitness installed?"})
		return
	}

	if app.ScreenshotPath != "" && app.ScreenshotPath != screenshotPath {
		os.Remove(app.ScreenshotPath)
	}

	app.ScreenshotPath = screenshotPath
	h.db.Save(&app)

	c.JSON(http.StatusOK, gin.H{"success": true, "screenshot_path": screenshotPath})
}

func (h *Handlers) GowitnessStatus(c *gin.Context) {
	available := true
	message := "Gowitness is available"

	if _, err := os.Stat(h.config.GowitnessPath); os.IsNotExist(err) {
		available = false
		message = fmt.Sprintf("Gowitness not found at: %s", h.config.GowitnessPath)
	}

	c.JSON(http.StatusOK, gin.H{
		"available": available,
		"enabled":   h.config.GowitnessEnabled,
		"message":   message,
		"path":      h.config.GowitnessPath,
	})
}

func (h *Handlers) UpdateOldApplications(c *gin.Context) {
	days := h.config.RejectedDays
	var oldApps []Application

	h.db.Where("status = ? AND updated_at < ?", StatusWaitingAnswer, time.Now().AddDate(0, 0, -days)).Find(&oldApps)

	count := 0
	for i := range oldApps {
		oldApps[i].Status = StatusNoResponse
		h.db.Save(&oldApps[i])
		count++
	}

	c.JSON(http.StatusOK, gin.H{
		"updated": count,
		"days":    days,
	})
}

func (h *Handlers) ExportCSV(c *gin.Context) {
	var apps []Application
	h.db.Preload("Resume").Preload("Reference").Order("created_at DESC").Find(&apps)

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=job_applications_%s.csv", time.Now().Format("2006-01-02")))

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	writer.Write([]string{
		"ID",
		"Company Name",
		"Job URL",
		"Company URL",
		"Status",
		"Resume",
		"Reference Letter",
		"Notes",
		"Created At",
	})

	for _, app := range apps {
		resumeName := ""
		if app.Resume != nil {
			resumeName = app.Resume.Name
		}
		refName := ""
		if app.Reference != nil {
			refName = app.Reference.Name
		}

		writer.Write([]string{
			strconv.FormatUint(uint64(app.ID), 10),
			app.CompanyName,
			app.JobURL,
			app.CompanyURL,
			string(app.Status),
			resumeName,
			refName,
			app.Notes,
			app.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
}

func captureWithGowitness(targetURL, screenshotsPath, gowitnessPath string) string {
	if _, err := os.Stat(gowitnessPath); os.IsNotExist(err) {
		log.Printf("Gowitness not found at: %s", gowitnessPath)
		return ""
	}

	uuidStr := uuid.New().String()
	tempDir := filepath.Join(screenshotsPath, uuidStr)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		log.Printf("Failed to create temp directory: %v", err)
		return ""
	}

	log.Printf("Capturing screenshot for: %s", targetURL)
	cmd := exec.Command(gowitnessPath, "scan", "single", "--url", targetURL, "-s", tempDir, "--screenshot-fullpage")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Gowitness failed: %v\nOutput: %s", err, string(output))
		os.RemoveAll(tempDir)
		return ""
	}

	entries, err := os.ReadDir(tempDir)
	if err != nil || len(entries) == 0 {
		log.Printf("No screenshot file found in: %s", tempDir)
		os.RemoveAll(tempDir)
		return ""
	}

	savedPath := filepath.Join(tempDir, entries[0].Name())
	log.Printf("Screenshot saved: %s", savedPath)
	return savedPath
}

func extractCompanyFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	host := u.Host
	parts := strings.Split(host, ".")
	if len(parts) >= 3 {
		return strings.Title(parts[len(parts)-2])
	}
	return strings.Title(parts[0])
}

func buildCompanyURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	if strings.Contains(u.Host, "linkedin.com") {
		return ""
	}
	return fmt.Sprintf("%s://%s/", u.Scheme, u.Host)
}

func saveUploadedFile(file *multipart.FileHeader, basePath, resourceType string) (string, error) {
	ext := filepath.Ext(file.Filename)
	hash := sha1.Sum([]byte(fmt.Sprintf("%s-%d", file.Filename, file.Size)))
	filename := fmt.Sprintf("%s-%x%s", resourceType, hash, ext)

	subdir := filepath.Join(basePath, resourceType+"s")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		return "", err
	}

	fullPath := filepath.Join(subdir, filename)

	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	dst, err := os.Create(fullPath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", err
	}

	return fullPath, nil
}
