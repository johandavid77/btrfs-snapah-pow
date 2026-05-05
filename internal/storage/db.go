package storage

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DB struct {
	gorm *gorm.DB
}

func NewDB(dsn string) (*DB, error) {
	cfg := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}

	var dialector gorm.Dialector

	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		dialector = postgres.Open(dsn)
		log.Println("🐘 Usando PostgreSQL")
	} else {
		dialector = sqlite.Open(dsn)
		log.Println("💾 Usando SQLite:", dsn)
	}

	db, err := gorm.Open(dialector, cfg)
	if err != nil {
		return nil, fmt.Errorf("conectar DB: %w", err)
	}

	if err := db.AutoMigrate(
		&Node{}, &Snapshot{}, &Policy{}, &Event{},
	); err != nil {
		return nil, fmt.Errorf("migrar schema: %w", err)
	}

	return &DB{gorm: db}, nil
}

func (d *DB) CreateNode(n *Node) error         { return d.gorm.Create(n).Error }
func (d *DB) GetNode(id string) (*Node, error) {
	var n Node
	return &n, d.gorm.First(&n, "id = ?", id).Error
}
func (d *DB) ListNodes() ([]Node, error) {
	var nodes []Node
	return nodes, d.gorm.Order("last_seen desc").Find(&nodes).Error
}
func (d *DB) UpdateNodeStatus(id, status string) error {
	return d.gorm.Model(&Node{}).Where("id = ?", id).
		Updates(map[string]interface{}{"status": status, "last_seen": time.Now()}).Error
}

func (d *DB) CreateSnapshot(s *Snapshot) error { return d.gorm.Create(s).Error }
func (d *DB) GetSnapshot(id string) (*Snapshot, error) {
	var s Snapshot
	return &s, d.gorm.First(&s, "id = ?", id).Error
}
func (d *DB) ListSnapshots(nodeID string) ([]Snapshot, error) {
	var snaps []Snapshot
	q := d.gorm.Order("created_at desc")
	if nodeID != "" {
		q = q.Where("node_id = ?", nodeID)
	}
	return snaps, q.Find(&snaps).Error
}
func (d *DB) DeleteSnapshot(id string) error {
	return d.gorm.Delete(&Snapshot{}, "id = ?", id).Error
}

func (d *DB) CreatePolicy(p *Policy) error { return d.gorm.Create(p).Error }
func (d *DB) ListPolicies(nodeID string) ([]Policy, error) {
	var policies []Policy
	q := d.gorm.Order("created_at desc")
	if nodeID != "" {
		q = q.Where("node_id = ?", nodeID)
	}
	return policies, q.Find(&policies).Error
}

func (d *DB) CreateEvent(e *Event) error { return d.gorm.Create(e).Error }
func (d *DB) ListEvents(limit int) ([]Event, error) {
	var events []Event
	return events, d.gorm.Order("created_at desc").Limit(limit).Find(&events).Error
}

func DBFromEnv(fallback string) string {
	if v := os.Getenv("DATABASE_URL"); v != "" {
		return v
	}
	return fallback
}
