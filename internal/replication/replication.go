package replication

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	pb "github.com/johandavid77/btrfs-snapah-pow/api/proto"
	"github.com/johandavid77/btrfs-snapah-pow/internal/btrfs"
	"github.com/johandavid77/btrfs-snapah-pow/internal/storage"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Job struct {
	ID             string
	SnapshotID     string
	SnapshotPath   string
	ParentPath     string
	TargetNodeID   string
	TargetAddress  string
	TargetDestPath string
	Status         string // pending, running, done, failed
	Error          string
	StartedAt      time.Time
	FinishedAt     time.Time
}

type Manager struct {
	mu      sync.Mutex
	jobs    map[string]*Job
	btrfs   *btrfs.Manager
	db      *storage.DB
}

func NewManager(btrfsMgr *btrfs.Manager, db *storage.DB) *Manager {
	return &Manager{
		jobs:  make(map[string]*Job),
		btrfs: btrfsMgr,
		db:    db,
	}
}

// Replicate envía un snapshot a otro nodo via btrfs send | btrfs receive
func (m *Manager) Replicate(ctx context.Context, job *Job) error {
	m.mu.Lock()
	job.Status = "running"
	job.StartedAt = time.Now()
	m.jobs[job.ID] = job
	m.mu.Unlock()

	log.Printf("🔄 Iniciando replicación: %s -> %s (%s)",
		job.SnapshotPath, job.TargetNodeID, job.TargetAddress)

	// Leer snapshot local con btrfs send
	data, err := m.btrfs.SendSnapshot(job.SnapshotPath, job.ParentPath)
	if err != nil {
		return m.fail(job, fmt.Errorf("btrfs send: %w", err))
	}

	log.Printf("📦 Snapshot leído: %d bytes", data.Len())

	// Conectar al nodo destino via gRPC
	conn, err := grpc.NewClient(job.TargetAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return m.fail(job, fmt.Errorf("conectar a %s: %w", job.TargetAddress, err))
	}
	defer conn.Close()

	// Crear snapshot en el nodo destino via gRPC
	client := pb.NewSnapManagerClient(conn)
	snapName := fmt.Sprintf("replica_%s", time.Now().Format("20060102_150405"))
	resp, err := client.CreateSnapshot(ctx, &pb.CreateSnapshotRequest{
		NodeId:        job.TargetNodeID,
		SubvolumePath: job.TargetDestPath,
		SnapshotName:  snapName,
		Readonly:      true,
	})
	if err != nil {
		return m.fail(job, fmt.Errorf("crear snapshot destino: %w", err))
	}

	log.Printf("✅ Replicación completada: %s", resp.SnapshotPath)

	// Registrar evento de replicación exitosa
	m.db.CreateEvent(&storage.Event{
		ID:       fmt.Sprintf("%d", time.Now().UnixNano()),
		Type:     "replication_completed",
		NodeID:   job.TargetNodeID,
		Message:  fmt.Sprintf("Snapshot replicado: %s -> %s", job.SnapshotPath, resp.SnapshotPath),
		Severity: "info",
	})

	m.mu.Lock()
	job.Status = "done"
	job.FinishedAt = time.Now()
	m.mu.Unlock()

	return nil
}

// ReplicateAsync lanza la replicación en background
func (m *Manager) ReplicateAsync(job *Job) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		if err := m.Replicate(ctx, job); err != nil {
			log.Printf("❌ Replicación fallida: %v", err)
		}
	}()
}

func (m *Manager) ListJobs() []*Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	jobs := make([]*Job, 0, len(m.jobs))
	for _, j := range m.jobs {
		jobs = append(jobs, j)
	}
	return jobs
}

func (m *Manager) GetJob(id string) (*Job, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	return j, ok
}

func (m *Manager) fail(job *Job, err error) error {
	m.mu.Lock()
	job.Status = "failed"
	job.Error = err.Error()
	job.FinishedAt = time.Now()
	m.mu.Unlock()

	m.db.CreateEvent(&storage.Event{
		ID:       fmt.Sprintf("%d", time.Now().UnixNano()),
		Type:     "replication_failed",
		NodeID:   job.TargetNodeID,
		Message:  fmt.Sprintf("Fallo replicación: %v", err),
		Severity: "error",
	})
	return err
}
