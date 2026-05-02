package main

import (
	"context"
	"sync"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "github.com/corp/btrfs-snapah-pow/api/proto"
	"github.com/corp/btrfs-snapah-pow/internal/btrfs"
	"github.com/corp/btrfs-snapah-pow/internal/config"
	"github.com/corp/btrfs-snapah-pow/internal/scheduler"
	"google.golang.org/grpc"
)

var appVersion = "dev"

type server struct {
	pb.UnimplementedSnapManagerServer
	config    *config.Config
	btrfs     *btrfs.Manager
	scheduler *scheduler.Scheduler
	nodes     map[string]*NodeInfo
	snapshots map[string]*SnapshotInfo
	mu        sync.RWMutex
}

type NodeInfo struct {
	ID       string
	Hostname string
	Address  string
	LastSeen time.Time
	Status   string
}

type SnapshotInfo struct {
	ID       string
	NodeID   string
	Path     string
	Created  time.Time
	ReadOnly bool
}

func newServer(cfg *config.Config) *server {
	btrfsMgr := btrfs.NewManager()
	sched := scheduler.NewScheduler(btrfsMgr)

	return &server{
		config:    cfg,
		btrfs:     btrfsMgr,
		scheduler: sched,
		nodes:     make(map[string]*NodeInfo),
		snapshots: make(map[string]*SnapshotInfo),
	}
}

func (s *server) RegisterNode(ctx context.Context, req *pb.RegisterNodeRequest) (*pb.Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	node := &NodeInfo{
		ID:       generateID(),
		Hostname: req.Hostname,
		Address:  req.Address,
		LastSeen: time.Now(),
		Status:   "online",
	}
	s.nodes[node.ID] = node

	log.Printf("🖥️  Nodo registrado: %s (%s)", node.Hostname, node.ID)
	return &pb.Node{
		Id:       node.ID,
		Hostname: node.Hostname,
		Address:  node.Address,
		Status:   node.Status,
		LastSeen: time.Now().Format(time.RFC3339),
	}, nil
}

func (s *server) ListNodes(ctx context.Context, req *pb.ListNodesRequest) (*pb.ListNodesResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var nodes []*pb.Node
	for _, n := range s.nodes {
		nodes = append(nodes, &pb.Node{
			Id:       n.ID,
			Hostname: n.Hostname,
			Address:  n.Address,
			Status:   n.Status,
			LastSeen: n.LastSeen.Format(time.RFC3339),
		})
	}
	return &pb.ListNodesResponse{Nodes: nodes}, nil
}

func (s *server) CreateSnapshot(ctx context.Context, req *pb.CreateSnapshotRequest) (*pb.Snapshot, error) {
	snapPath := btrfs.SnapshotPath(req.SubvolumePath, req.SnapshotName)

	if err := s.btrfs.CreateSnapshot(req.SubvolumePath, snapPath, req.Readonly); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	snap := &SnapshotInfo{
		ID:       generateID(),
		NodeID:   req.NodeId,
		Path:     snapPath,
		Created:  time.Now(),
		ReadOnly: req.Readonly,
	}
	s.snapshots[snap.ID] = snap

	log.Printf("📸 Snapshot creado: %s", snapPath)
	return &pb.Snapshot{
		Id:           snap.ID,
		NodeId:       snap.NodeID,
		SnapshotPath: snap.Path,
		CreatedAt:    snap.Created.Format(time.RFC3339),
		IsReadonly:   snap.ReadOnly,
		Status:       "active",
	}, nil
}

func (s *server) ListSnapshots(ctx context.Context, req *pb.ListSnapshotsRequest) (*pb.ListSnapshotsResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var snaps []*pb.Snapshot
	for _, snap := range s.snapshots {
		if req.NodeId != "" && snap.NodeID != req.NodeId {
			continue
		}
		snaps = append(snaps, &pb.Snapshot{
			Id:           snap.ID,
			NodeId:       snap.NodeID,
			SnapshotPath: snap.Path,
			CreatedAt:    snap.Created.Format(time.RFC3339),
			IsReadonly:   snap.ReadOnly,
			Status:       "active",
		})
	}
	return &pb.ListSnapshotsResponse{Snapshots: snaps}, nil
}

func (s *server) DeleteSnapshot(ctx context.Context, req *pb.DeleteSnapshotRequest) (*pb.DeleteSnapshotResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	snap, ok := s.snapshots[req.SnapshotId]
	if !ok {
		return &pb.DeleteSnapshotResponse{Success: false, Message: "snapshot not found"}, nil
	}

	if err := s.btrfs.DeleteSnapshot(snap.Path); err != nil {
		return &pb.DeleteSnapshotResponse{Success: false, Message: err.Error()}, nil
	}

	delete(s.snapshots, req.SnapshotId)
	log.Printf("🗑️  Snapshot eliminado: %s", snap.Path)

	return &pb.DeleteSnapshotResponse{Success: true, Message: "deleted"}, nil
}

func (s *server) StreamEvents(req *pb.StreamEventsRequest, stream pb.SnapManager_StreamEventsServer) error {
	// Enviar evento de bienvenida
	event := &pb.Event{
		Id:        generateID(),
		Type:      "connection",
		NodeId:    req.NodeId,
		Message:   "Stream de eventos iniciado",
		Timestamp: time.Now().Format(time.RFC3339),
		Severity:  "info",
	}
	if err := stream.Send(event); err != nil {
		return err
	}

	// Mantener stream abierto
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case <-time.After(10 * time.Second):
			event := &pb.Event{
				Id:        generateID(),
				Type:      "heartbeat",
				NodeId:    req.NodeId,
				Message:   "ping",
				Timestamp: time.Now().Format(time.RFC3339),
				Severity:  "info",
			}
			if err := stream.Send(event); err != nil {
				return err
			}
		}
	}
}

func main() {
	fmt.Println("🔥 Snapah Pow Server v" + appVersion)

	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("❌ Config error: %v", err)
	}

	srv := newServer(cfg)
	_ = srv

	// gRPC server
	grpcAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.GRPCPort)
	grpcLis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("❌ gRPC listen failed: %v", err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterSnapManagerServer(grpcServer, srv)

	go func() {
		log.Printf("🚀 gRPC server en %s", grpcAddr)
		if err := grpcServer.Serve(grpcLis); err != nil {
			log.Printf("gRPC error: %v", err)
		}
	}()

	// HTTP server
	httpAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	httpMux := http.NewServeMux()

	httpMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","version":"` + appVersion + `"}`))
	})

	httpServer := &http.Server{
		Addr:    httpAddr,
		Handler: httpMux,
	}

	go func() {
		log.Printf("🌐 HTTP API en http://%s", httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP error: %v", err)
		}
	}()

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("👋 Shutdown iniciado...")
	grpcServer.GracefulStop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	httpServer.Shutdown(shutdownCtx)

	log.Println("👋 Server detenido")
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
