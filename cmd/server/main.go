package main

import (
	"context"
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
	"github.com/corp/btrfs-snapah-pow/internal/storage"
	"google.golang.org/grpc"
)

var appVersion = "dev"

type server struct {
	pb.UnimplementedSnapManagerServer
	config    *config.Config
	db        *storage.DB
	btrfs     *btrfs.Manager
	scheduler *scheduler.Scheduler
}

func newServer(cfg *config.Config, db *storage.DB) *server {
	btrfsMgr := btrfs.NewManager()
	sched := scheduler.NewScheduler(btrfsMgr)

	return &server{
		config:    cfg,
		db:        db,
		btrfs:     btrfsMgr,
		scheduler: sched,
	}
}

func (s *server) RegisterNode(ctx context.Context, req *pb.RegisterNodeRequest) (*pb.Node, error) {
	node := &storage.Node{
		ID:       generateID(),
		Hostname: req.Hostname,
		Address:  req.Address,
		Status:   "online",
		LastSeen: time.Now(),
	}

	if err := s.db.CreateNode(node); err != nil {
		return nil, err
	}

	log.Printf("🖥️  Nodo registrado: %s (%s)", node.Hostname, node.ID)
	return &pb.Node{
		Id:       node.ID,
		Hostname: node.Hostname,
		Address:  node.Address,
		Status:   node.Status,
		LastSeen: node.LastSeen.Format(time.RFC3339),
	}, nil
}

func (s *server) ListNodes(ctx context.Context, req *pb.ListNodesRequest) (*pb.ListNodesResponse, error) {
	nodes, err := s.db.ListNodes()
	if err != nil {
		return nil, err
	}

	var pbNodes []*pb.Node
	for _, n := range nodes {
		pbNodes = append(pbNodes, &pb.Node{
			Id:       n.ID,
			Hostname: n.Hostname,
			Address:  n.Address,
			Status:   n.Status,
			LastSeen: n.LastSeen.Format(time.RFC3339),
		})
	}
	return &pb.ListNodesResponse{Nodes: pbNodes}, nil
}

func (s *server) CreateSnapshot(ctx context.Context, req *pb.CreateSnapshotRequest) (*pb.Snapshot, error) {
	snapPath := btrfs.SnapshotPath(req.SubvolumePath, req.SnapshotName)

	if err := s.btrfs.CreateSnapshot(req.SubvolumePath, snapPath, req.Readonly); err != nil {
		return nil, err
	}

	snap := &storage.Snapshot{
		ID:            generateID(),
		NodeID:        req.NodeId,
		SubvolumePath: req.SubvolumePath,
		SnapshotPath:  snapPath,
		IsReadOnly:    req.Readonly,
		Status:        "active",
	}

	if err := s.db.CreateSnapshot(snap); err != nil {
		return nil, err
	}

	log.Printf("📸 Snapshot creado: %s", snapPath)
	return &pb.Snapshot{
		Id:           snap.ID,
		NodeId:       snap.NodeID,
		SnapshotPath: snap.SnapshotPath,
		SubvolumePath: snap.SubvolumePath,
		CreatedAt:    time.Now().Format(time.RFC3339),
		IsReadonly:   snap.IsReadOnly,
		Status:       snap.Status,
	}, nil
}

func (s *server) ListSnapshots(ctx context.Context, req *pb.ListSnapshotsRequest) (*pb.ListSnapshotsResponse, error) {
	snaps, err := s.db.ListSnapshots(req.NodeId)
	if err != nil {
		return nil, err
	}

	var pbSnaps []*pb.Snapshot
	for _, snap := range snaps {
		pbSnaps = append(pbSnaps, &pb.Snapshot{
			Id:            snap.ID,
			NodeId:        snap.NodeID,
			SubvolumePath: snap.SubvolumePath,
			SnapshotPath:  snap.SnapshotPath,
			CreatedAt:     snap.CreatedAt.Format(time.RFC3339),
			IsReadonly:    snap.IsReadOnly,
			Status:        snap.Status,
		})
	}
	return &pb.ListSnapshotsResponse{Snapshots: pbSnaps}, nil
}

func (s *server) DeleteSnapshot(ctx context.Context, req *pb.DeleteSnapshotRequest) (*pb.DeleteSnapshotResponse, error) {
	snap, err := s.db.GetSnapshot(req.SnapshotId)
	if err != nil {
		return &pb.DeleteSnapshotResponse{Success: false, Message: "snapshot not found"}, nil
	}

	if err := s.btrfs.DeleteSnapshot(snap.SnapshotPath); err != nil {
		return &pb.DeleteSnapshotResponse{Success: false, Message: err.Error()}, nil
	}

	if err := s.db.DeleteSnapshot(req.SnapshotId); err != nil {
		return &pb.DeleteSnapshotResponse{Success: false, Message: err.Error()}, nil
	}

	log.Printf("🗑️  Snapshot eliminado: %s", snap.SnapshotPath)
	return &pb.DeleteSnapshotResponse{Success: true, Message: "deleted"}, nil
}

func (s *server) StreamEvents(req *pb.StreamEventsRequest, stream pb.SnapManager_StreamEventsServer) error {
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

	// Actualizar last_seen del nodo
	if req.NodeId != "" {
		s.db.UpdateNodeStatus(req.NodeId, "online")
	}

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

	// Crear directorio para DB si no existe
	os.MkdirAll("data", 0755)

	// Inicializar base de datos
	db, err := storage.NewDB("data/snapah.db")
	if err != nil {
		log.Fatalf("❌ Database error: %v", err)
	}

	srv := newServer(cfg, db)

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

	httpMux.Handle("/", http.FileServer(http.Dir("web")))
	httpMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","version":"` + appVersion + `"}`))
	})

	httpMux.HandleFunc("/api/nodes", func(w http.ResponseWriter, r *http.Request) {
		nodes, err := db.ListNodes()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"count":%d,"nodes":[]}`, len(nodes))
	})

	httpMux.HandleFunc("/api/snapshots", func(w http.ResponseWriter, r *http.Request) {
		snaps, err := db.ListSnapshots("")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"count":%d,"snapshots":[]}`, len(snaps))
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
