package scheduler

import (
	"fmt"
	"log"
	"time"

	"github.com/johandavid77/btrfs-snapah-pow/internal/btrfs"
	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	btrfs    *btrfs.Manager
	policies map[string]*Policy
	entryIDs map[string]cron.EntryID
	cron     *cron.Cron
}

type Policy struct {
	ID               string
	Name             string
	NodeID           string
	SubvolumePath    string
	Schedule         string
	RetentionHourly  int
	RetentionDaily   int
	RetentionWeekly  int
	RetentionMonthly int
	ReadOnly         bool
	Replicate        bool
	Enabled          bool
}

func NewScheduler(btrfsMgr *btrfs.Manager) *Scheduler {
	c := cron.New(cron.WithSeconds())
	s := &Scheduler{
		btrfs:    btrfsMgr,
		policies: make(map[string]*Policy),
		entryIDs: make(map[string]cron.EntryID),
		cron:     c,
	}
	c.Start()
	return s
}

func (s *Scheduler) AddPolicy(p *Policy) error {
	s.policies[p.ID] = p

	if !p.Enabled {
		log.Printf("📅 Política '%s' deshabilitada, no programada", p.Name)
		return nil
	}

	schedule := p.Schedule
	if schedule == "" {
		schedule = "0 * * * *" // cada hora por defecto
	}

	entryID, err := s.cron.AddFunc(schedule, func() {
		s.execute(p)
	})
	if err != nil {
		return fmt.Errorf("schedule inválido '%s': %w", schedule, err)
	}

	s.entryIDs[p.ID] = entryID
	log.Printf("📅 Política '%s' programada: %s", p.Name, schedule)
	return nil
}

func (s *Scheduler) RemovePolicy(id string) {
	if entryID, ok := s.entryIDs[id]; ok {
		s.cron.Remove(entryID)
		delete(s.entryIDs, id)
	}
	delete(s.policies, id)
	log.Printf("🗑️  Política %s eliminada del scheduler", id)
}

func (s *Scheduler) UpdatePolicy(p *Policy) error {
	s.RemovePolicy(p.ID)
	return s.AddPolicy(p)
}

func (s *Scheduler) ListPolicies() []*Policy {
	policies := make([]*Policy, 0, len(s.policies))
	for _, p := range s.policies {
		policies = append(policies, p)
	}
	return policies
}

func (s *Scheduler) NextRun(id string) *time.Time {
	entryID, ok := s.entryIDs[id]
	if !ok {
		return nil
	}
	entry := s.cron.Entry(entryID)
	next := entry.Next
	return &next
}

func (s *Scheduler) execute(p *Policy) {
	log.Printf("🔥 Ejecutando snapshot automático: %s -> %s", p.Name, p.SubvolumePath)

	snapPath := btrfs.SnapshotPath(p.SubvolumePath, p.Name)
	if err := s.btrfs.CreateSnapshot(p.SubvolumePath, snapPath, p.ReadOnly); err != nil {
		log.Printf("❌ Error creando snapshot '%s': %v", p.Name, err)
		return
	}

	log.Printf("✅ Snapshot automático creado: %s", snapPath)

	// Aplicar retención
	s.applyRetention(p)
}

func (s *Scheduler) applyRetention(p *Policy) {
	snapshots, err := s.btrfs.ListSnapshots(p.SubvolumePath)
	if err != nil {
		log.Printf("⚠️  No se pudo listar snapshots para retención: %v", err)
		return
	}

	maxToKeep := p.RetentionDaily
	if maxToKeep <= 0 {
		maxToKeep = 7 // default: 7 días
	}

	if len(snapshots) <= maxToKeep {
		return
	}

	// Eliminar los más antiguos que excedan la retención
	toDelete := snapshots[maxToKeep:]
	for _, snap := range toDelete {
		if err := s.btrfs.DeleteSnapshot(snap.Path); err != nil {
			log.Printf("⚠️  No se pudo eliminar snapshot viejo %s: %v", snap.Path, err)
		} else {
			log.Printf("🗑️  Retención: eliminado snapshot viejo %s", snap.Path)
		}
	}
}

func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
	log.Println("⏹️  Scheduler detenido")
}
