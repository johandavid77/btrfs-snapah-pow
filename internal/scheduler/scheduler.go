package scheduler

import (
	"fmt"
	"time"

	"github.com/corp/btrfs-snapah-pow/internal/btrfs"
)

type Scheduler struct {
	btrfs    *btrfs.Manager
	policies map[string]*Policy
	timers   map[string]*time.Timer
}

type Policy struct {
	ID            string
	Name          string
	NodeID        string
	SubvolumePath string
	Schedule      string
	RetentionHourly int
	RetentionDaily  int
	RetentionWeekly int
	RetentionMonthly int
	ReadOnly      bool
	Replicate     bool
	Enabled       bool
}

func NewScheduler(btrfsMgr *btrfs.Manager) *Scheduler {
	return &Scheduler{
		btrfs:    btrfsMgr,
		policies: make(map[string]*Policy),
		timers:   make(map[string]*time.Timer),
	}
}

func (s *Scheduler) AddPolicy(p *Policy) error {
	if !p.Enabled {
		s.policies[p.ID] = p
		return nil
	}

	// Parse simple: cada X minutos para demo
	// En producción: usar github.com/go-co-op/gocron/v2
	interval := 5 * time.Minute
	timer := time.AfterFunc(interval, func() {
		s.execute(p)
	})

	s.policies[p.ID] = p
	s.timers[p.ID] = timer

	fmt.Printf("📅 Política '%s' programada cada %v\n", p.Name, interval)
	return nil
}

func (s *Scheduler) RemovePolicy(id string) {
	if timer, ok := s.timers[id]; ok {
		timer.Stop()
		delete(s.timers, id)
	}
	delete(s.policies, id)
}

func (s *Scheduler) execute(p *Policy) {
	fmt.Printf("🔥 Ejecutando snapshot: %s -> %s\n", p.Name, p.SubvolumePath)

	snapPath := btrfs.SnapshotPath(p.SubvolumePath, p.Name)
	if err := s.btrfs.CreateSnapshot(p.SubvolumePath, snapPath, p.ReadOnly); err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}

	fmt.Printf("✅ Snapshot creado: %s\n", snapPath)

	// Reprogramar
	if p.Enabled {
		s.timers[p.ID] = time.AfterFunc(5*time.Minute, func() {
			s.execute(p)
		})
	}
}

func (s *Scheduler) Stop() {
	for _, timer := range s.timers {
		timer.Stop()
	}
}
