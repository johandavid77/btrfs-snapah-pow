package storage

import (
	"fmt"
	"log"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB maneja la conexión a la base de datos
type DB struct {
	*gorm.DB
}

// NewDB crea una nueva conexión SQLite
func NewDB(path string) (*DB, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Migrar schemas
	if err := db.AutoMigrate(&Node{}, &Snapshot{}, &Policy{}, &Event{}); err != nil {
		return nil, fmt.Errorf("failed to migrate: %w", err)
	}

	log.Printf("💾 Base de datos SQLite lista: %s", path)
	return &DB{db}, nil
}

// CreateNode registra un nuevo nodo
func (db *DB) CreateNode(node *Node) error {
	return db.DB.Create(node).Error
}

// GetNode busca un nodo por ID
func (db *DB) GetNode(id string) (*Node, error) {
	var node Node
	if err := db.DB.First(&node, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &node, nil
}

// ListNodes lista todos los nodos
func (db *DB) ListNodes() ([]Node, error) {
	var nodes []Node
	err := db.DB.Find(&nodes).Error
	return nodes, err
}

// UpdateNodeStatus actualiza el estado de un nodo
func (db *DB) UpdateNodeStatus(id, status string) error {
	return db.DB.Model(&Node{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":    status,
		"last_seen": gorm.Expr("CURRENT_TIMESTAMP"),
	}).Error
}

// CreateSnapshot registra un snapshot
func (db *DB) CreateSnapshot(snap *Snapshot) error {
	return db.DB.Create(snap).Error
}

// GetSnapshot busca un snapshot por ID
func (db *DB) GetSnapshot(id string) (*Snapshot, error) {
	var snap Snapshot
	if err := db.DB.First(&snap, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &snap, nil
}

// ListSnapshots lista snapshots, opcionalmente filtrados por nodo
func (db *DB) ListSnapshots(nodeID string) ([]Snapshot, error) {
	var snaps []Snapshot
	query := db.DB
	if nodeID != "" {
		query = query.Where("node_id = ?", nodeID)
	}
	err := query.Order("created_at DESC").Find(&snaps).Error
	return snaps, err
}

// DeleteSnapshot marca un snapshot como eliminado
func (db *DB) DeleteSnapshot(id string) error {
	return db.DB.Delete(&Snapshot{}, "id = ?", id).Error
}

// CreatePolicy crea una política
func (db *DB) CreatePolicy(policy *Policy) error {
	return db.DB.Create(policy).Error
}

// ListPolicies lista políticas
func (db *DB) ListPolicies(nodeID string) ([]Policy, error) {
	var policies []Policy
	query := db.DB
	if nodeID != "" {
		query = query.Where("node_id = ?", nodeID)
	}
	err := query.Where("enabled = ?", true).Find(&policies).Error
	return policies, err
}

// CreateEvent registra un evento
func (db *DB) CreateEvent(event *Event) error {
	return db.DB.Create(event).Error
}

// ListEvents lista eventos recientes
func (db *DB) ListEvents(limit int) ([]Event, error) {
	var events []Event
	err := db.DB.Order("created_at DESC").Limit(limit).Find(&events).Error
	return events, err
}
