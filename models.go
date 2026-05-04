package main

import (
	"time"
)

type ApplicationStatus string

const (
	StatusNotSubmitted  ApplicationStatus = "not_submitted"
	StatusSubmitted     ApplicationStatus = "submitted"
	StatusInterview     ApplicationStatus = "interview"
	StatusWaitingAnswer ApplicationStatus = "waiting_for_answer"
	StatusOffer         ApplicationStatus = "offer"
	StatusRejected      ApplicationStatus = "rejected"
	StatusNoResponse    ApplicationStatus = "rejected_no_response"
)

type Application struct {
	ID              uint              `gorm:"primaryKey" json:"id"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
	StatusUpdatedAt time.Time         `json:"status_updated_at"`
	JobURL          string            `gorm:"size:2048" json:"job_url"`
	ScreenshotPath  string            `gorm:"size:1024" json:"screenshot_path"`
	CompanyName     string            `gorm:"size:512" json:"company_name"`
	CompanyURL      string            `gorm:"size:2048" json:"company_url"`
	Status          ApplicationStatus `gorm:"size:64;default:'submitted'" json:"status"`
	Notes           string            `gorm:"type:text" json:"notes"`
	ResumeID        *uint             `json:"resume_id"`
	Resume          *Resource         `gorm:"foreignKey:ResumeID" json:"resume,omitempty"`
	ReferenceID     *uint             `json:"reference_id"`
	Reference       *Resource         `gorm:"foreignKey:ReferenceID" json:"reference,omitempty"`
}

type Resource struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Name         string    `gorm:"size:512" json:"name"`
	Filename     string    `gorm:"size:512" json:"filename"`
	FilePath     string    `gorm:"size:1024" json:"file_path"`
	MimeType     string    `gorm:"size:128" json:"mime_type"`
	ResourceType string    `gorm:"size:64" json:"resource_type"`
}
