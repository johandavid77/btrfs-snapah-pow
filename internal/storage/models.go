package storage

import (
	"time"

	"gorm.io/gorm"
)

// Node representa un nodo agente registrado
type Node struct {
	ID        string         `gorm:"primaryKey" json:"id"`
	Hostname  string         `json:"hostname"`
	Address   string         `json:"address"`
	Status    string         `json:"status"` // online, offline, error
	LastSeen  time.Time      `json:"last_seen"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// Snapshot representa un snapshot BTRFS
type Snapshot struct {
	ID             string         `gorm:"primaryKey" json:"id"`
	NodeID         string         `gorm:"index" json:"node_id"`
	SubvolumePath  string         `json:"subvolume_path"`
	SnapshotPath   string         `gorm:"uniqueIndex" json:"snapshot_path"`
	UUID           string         `json:"uuid"`
	ParentUUID     string         `json:"parent_uuid"`
	IsReadOnly     bool           `json:"is_readonly"`
	SizeBytes      int64          `json:"size_bytes"`
	Status         string         `json:"status"` // active, deleting, error
	ErrorMessage   string         `json:"error_message,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	ExpiresAt      *time.Time     `json:"expires_at,omitempty"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

// Policy representa una política de snapshot programada
type Policy struct {
	ID               string         `gorm:"primaryKey" json:"id"`
	Name             string         `json:"name"`
	NodeID           string         `gorm:"index" json:"node_id"`
	SubvolumePath    string         `json:"subvolume_path"`
	Schedule         string         `json:"schedule"` // cron expression
	RetentionHourly  int            `json:"retention_hourly"`
	RetentionDaily   int            `json:"retention_daily"`
	RetentionWeekly  int            `json:"retention_weekly"`
	RetentionMonthly int            `json:"retention_monthly"`
	ReadOnly         bool           `json:"readonly"`
	Replicate        bool           `json:"replicate"`
	ReplicateTarget  string         `json:"replicate_target"`
	Enabled          bool           `json:"enabled"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}

// Event representa un evento del sistema
type Event struct {
	ID        string         `gorm:"primaryKey" json:"id"`
	Type      string         `json:"type"` // snapshot_created, snapshot_deleted, etc.
	NodeID    string         `gorm:"index" json:"node_id"`
	SnapshotID string        `json:"snapshot_id,omitempty"`
	Message   string         `json:"message"`
	Severity  string         `json:"severity"` // info, warning, error, critical
	Metadata  string         `json:"metadata,omitempty"` // JSON
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// APIKeyRecord persiste las API keys en DB
type APIKeyRecord struct {
	ID          string     `gorm:"primaryKey" json:"id"`
	Name        string     `json:"name"`
	KeyHash     string     `json:"-"`
	Prefix      string     `json:"prefix"`
	Role        string     `json:"role"`
	Active      bool       `json:"active"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
}
