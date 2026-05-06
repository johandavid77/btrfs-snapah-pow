package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "github.com/johandavid77/btrfs-snapah-pow/api/proto"
	"github.com/johandavid77/btrfs-snapah-pow/internal/auth"
	"github.com/johandavid77/btrfs-snapah-pow/internal/btrfs"
	"github.com/johandavid77/btrfs-snapah-pow/internal/config"
	"github.com/johandavid77/btrfs-snapah-pow/internal/ratelimit"
	"github.com/johandavid77/btrfs-snapah-pow/internal/alerts"
	"github.com/johandavid77/btrfs-snapah-pow/internal/apikeys"
	"github.com/johandavid77/btrfs-snapah-pow/internal/replication"
	"github.com/johandavid77/btrfs-snapah-pow/internal/metrics"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/johandavid77/btrfs-snapah-pow/internal/scheduler"
	"github.com/johandavid77/btrfs-snapah-pow/internal/storage"
	"github.com/johandavid77/btrfs-snapah-pow/internal/tlsconfig"
	"github.com/google/uuid"
	"google.golang.org/grpc"
)

var appVersion = "dev"

type server struct {
	pb.UnimplementedSnapManagerServer
	config    *config.Config
	db        *storage.DB
	btrfs     *btrfs.Manager
	scheduler *scheduler.Scheduler
	authMgr   *auth.Manager
	users     *auth.UserStore
	replMgr   *replication.Manager
	alertsMgr *alerts.Manager
	watchdog  *alerts.Watchdog
	apiKeyStore *apikeys.Store
}

func newServer(cfg *config.Config, db *storage.DB) *server {
	btrfsMgr := btrfs.NewManager()
	sched    := scheduler.NewScheduler(btrfsMgr)
	authMgr  := auth.NewManager(cfg.Auth.JWTSecret, cfg.Auth.TokenExpiry)
	users    := auth.NewUserStore()

	adminPass := os.Getenv("SNAPAH_ADMIN_PASSWORD")
	if adminPass == "" {
		adminPass = "admin123"
	}
	users.Add(generateID(), "admin",    adminPass,    "admin")
	users.Add(generateID(), "operator", "operator123", "operator")

	replMgr  := replication.NewManager(btrfsMgr, db)
	alertsMgr := alerts.NewManager(alerts.ConfigFromEnv())
	watchdog  := alerts.NewWatchdog(db, alertsMgr, 60*time.Second)
	watchdog.Start()
	apiKeyStore := apikeys.NewStore()

	return &server{
		config: cfg, db: db, btrfs: btrfsMgr,
		scheduler: sched, authMgr: authMgr, users: users,
		replMgr: replMgr, alertsMgr: alertsMgr, watchdog: watchdog,
		apiKeyStore: apiKeyStore,
	}
}

func generateID() string { return uuid.New().String() }

func jsonResp(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func httpErr(w http.ResponseWriter, err error, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func forbidden(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// ── gRPC handlers ────────────────────────────────────────

func (s *server) RegisterNode(ctx context.Context, req *pb.RegisterNodeRequest) (*pb.Node, error) {
	node := &storage.Node{
		ID: generateID(), Hostname: req.Hostname,
		Address: req.Address, Status: "online", LastSeen: time.Now(),
	}
	if err := s.db.CreateNode(node); err != nil {
		return nil, err
	}
	log.Printf("🖥️  Nodo registrado: %s (%s)", node.Hostname, node.ID)
	return &pb.Node{
		Id: node.ID, Hostname: node.Hostname,
		Address: node.Address, Status: node.Status,
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
			Id: n.ID, Hostname: n.Hostname,
			Address: n.Address, Status: n.Status,
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
		ID: generateID(), NodeID: req.NodeId,
		SubvolumePath: req.SubvolumePath, SnapshotPath: snapPath,
		IsReadOnly: req.Readonly, Status: "active",
	}
	if err := s.db.CreateSnapshot(snap); err != nil {
		return nil, err
	}
	log.Printf("📸 Snapshot creado: %s", snapPath)
	metrics.SnapshotCreatedTotal.Inc()
	return &pb.Snapshot{
		Id: snap.ID, NodeId: snap.NodeID,
		SnapshotPath: snap.SnapshotPath, SubvolumePath: snap.SubvolumePath,
		CreatedAt: time.Now().Format(time.RFC3339),
		IsReadonly: snap.IsReadOnly, Status: snap.Status,
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
			Id: snap.ID, NodeId: snap.NodeID,
			SubvolumePath: snap.SubvolumePath, SnapshotPath: snap.SnapshotPath,
			CreatedAt: snap.CreatedAt.Format(time.RFC3339),
			IsReadonly: snap.IsReadOnly, Status: snap.Status,
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
	s.db.DeleteSnapshot(req.SnapshotId)
	log.Printf("🗑️  Snapshot eliminado: %s", snap.SnapshotPath)
	metrics.SnapshotDeletedTotal.Inc()
	return &pb.DeleteSnapshotResponse{Success: true, Message: "deleted"}, nil
}

func (s *server) StreamEvents(req *pb.StreamEventsRequest, stream pb.SnapManager_StreamEventsServer) error {
	stream.Send(&pb.Event{
		Id: generateID(), Type: "connection", NodeId: req.NodeId,
		Message: "Stream iniciado", Timestamp: time.Now().Format(time.RFC3339), Severity: "info",
	})
	if req.NodeId != "" {
		s.db.UpdateNodeStatus(req.NodeId, "online")
	}
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case <-time.After(10 * time.Second):
			stream.Send(&pb.Event{
				Id: generateID(), Type: "heartbeat", NodeId: req.NodeId,
				Message: "ping", Timestamp: time.Now().Format(time.RFC3339), Severity: "info",
			})
		}
	}
}

// ── HTTP handlers ─────────────────────────────────────────

func (s *server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpErr(w, err, http.StatusBadRequest)
		return
	}
	user, err := s.users.Authenticate(req.Username, req.Password)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "credenciales invalidas"})
		return
	}
	token, err := s.authMgr.Generate(user.ID, user.Username, user.Role)
	if err != nil {
		httpErr(w, err, http.StatusInternalServerError)
		return
	}
	jsonResp(w, map[string]string{"token": token, "username": user.Username, "role": user.Role})
}

func (s *server) handleNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := s.db.ListNodes()
	if err != nil {
		httpErr(w, err, http.StatusInternalServerError)
		return
	}
	jsonResp(w, map[string]interface{}{"count": len(nodes), "nodes": nodes})
}

func (s *server) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		nodeID := r.URL.Query().Get("node_id")
		snaps, err := s.db.ListSnapshots(nodeID)
		if err != nil {
			httpErr(w, err, http.StatusInternalServerError)
			return
		}
		jsonResp(w, map[string]interface{}{"count": len(snaps), "snapshots": snaps})

	case http.MethodPost:
		var req struct {
			NodeID        string `json:"node_id"`
			SubvolumePath string `json:"subvolume_path"`
			SnapshotName  string `json:"snapshot_name"`
			Readonly      bool   `json:"readonly"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpErr(w, err, http.StatusBadRequest)
			return
		}
		resp, err := s.CreateSnapshot(r.Context(), &pb.CreateSnapshotRequest{
			NodeId: req.NodeID, SubvolumePath: req.SubvolumePath,
			SnapshotName: req.SnapshotName, Readonly: req.Readonly,
		})
		if err != nil {
			httpErr(w, err, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		jsonResp(w, resp)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *server) handleDeleteSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	// RBAC: solo admin u operator
	role := r.Header.Get("X-User-Role")
	if role != "admin" && role != "operator" {
		forbidden(w, "permisos insuficientes para eliminar snapshots")
		return
	}
	var req struct {
		SnapshotID string `json:"snapshot_id"`
		Force      bool   `json:"force"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpErr(w, err, http.StatusBadRequest)
		return
	}
	resp, err := s.DeleteSnapshot(r.Context(), &pb.DeleteSnapshotRequest{
		SnapshotId: req.SnapshotID, Force: req.Force,
	})
	if err != nil {
		httpErr(w, err, http.StatusInternalServerError)
		return
	}
	jsonResp(w, resp)
}

func (s *server) handleEvents(w http.ResponseWriter, r *http.Request) {
	events, err := s.db.ListEvents(50)
	if err != nil {
		httpErr(w, err, http.StatusInternalServerError)
		return
	}
	jsonResp(w, map[string]interface{}{"count": len(events), "events": events})
}

func (s *server) handlePolicies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		nodeID := r.URL.Query().Get("node_id")
		policies, err := s.db.ListPolicies(nodeID)
		if err != nil {
			httpErr(w, err, http.StatusInternalServerError)
			return
		}
		jsonResp(w, map[string]interface{}{"count": len(policies), "policies": policies})

	case http.MethodPost:
		// RBAC: solo admin
		if r.Header.Get("X-User-Role") != "admin" {
			forbidden(w, "solo admin puede crear politicas")
			return
		}
		var p storage.Policy
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			httpErr(w, err, http.StatusBadRequest)
			return
		}
		p.ID = generateID()
		if err := s.db.CreatePolicy(&p); err != nil {
			httpErr(w, err, http.StatusInternalServerError)
			return
		}
		s.scheduler.AddPolicy(&scheduler.Policy{
			ID: p.ID, Name: p.Name, NodeID: p.NodeID,
			SubvolumePath: p.SubvolumePath, Schedule: p.Schedule,
			RetentionHourly: p.RetentionHourly, RetentionDaily: p.RetentionDaily,
			RetentionWeekly: p.RetentionWeekly, RetentionMonthly: p.RetentionMonthly,
			ReadOnly: p.ReadOnly, Replicate: p.Replicate, Enabled: p.Enabled,
		})
		w.WriteHeader(http.StatusCreated)
		jsonResp(w, p)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}



func (s *server) handleRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	role := r.Header.Get("X-User-Role")
	if role != "admin" && role != "operator" {
		forbidden(w, "solo admin u operator puede restaurar snapshots")
		return
	}

	var req struct {
		SnapshotID  string `json:"snapshot_id"`
		RestorePath string `json:"restore_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpErr(w, err, http.StatusBadRequest)
		return
	}
	if req.SnapshotID == "" || req.RestorePath == "" {
		httpErr(w, fmt.Errorf("snapshot_id y restore_path son requeridos"), http.StatusBadRequest)
		return
	}

	snap, err := s.db.GetSnapshot(req.SnapshotID)
	if err != nil {
		httpErr(w, fmt.Errorf("snapshot no encontrado: %w", err), http.StatusNotFound)
		return
	}

	log.Printf("🔄 Restaurando snapshot %s -> %s", snap.SnapshotPath, req.RestorePath)

	// btrfs subvolume snapshot <snap_path> <restore_path>
	// Crea un snapshot writable del snapshot readonly como punto de restauracion
	if err := s.btrfs.CreateSnapshot(snap.SnapshotPath, req.RestorePath, false); err != nil {
		httpErr(w, fmt.Errorf("error restaurando: %w", err), http.StatusInternalServerError)
		return
	}

	// Registrar evento
	s.db.CreateEvent(&storage.Event{
		ID:       generateID(),
		Type:     "snapshot_restored",
		NodeID:   snap.NodeID,
		Message:  fmt.Sprintf("Snapshot restaurado: %s -> %s", snap.SnapshotPath, req.RestorePath),
		Severity: "info",
	})

	log.Printf("✅ Restore completado: %s", req.RestorePath)
	jsonResp(w, map[string]interface{}{
		"success":      true,
		"snapshot_path": snap.SnapshotPath,
		"restore_path":  req.RestorePath,
		"message":       fmt.Sprintf("Snapshot restaurado exitosamente en %s", req.RestorePath),
	})
}



func (s *server) handleAPIKeys(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if r.Header.Get("X-User-Role") != "admin" {
			forbidden(w, "solo admin puede listar API keys")
			return
		}
		keys := s.apiKeyStore.List()
		safe := make([]map[string]interface{}, 0, len(keys))
		for _, k := range keys {
			m := map[string]interface{}{
				"id": k.ID, "name": k.Name, "prefix": k.Prefix,
				"role": k.Role, "active": k.Active,
				"created_at": k.CreatedAt,
			}
			if k.ExpiresAt != nil { m["expires_at"] = k.ExpiresAt }
			if k.LastUsedAt != nil { m["last_used_at"] = k.LastUsedAt }
			safe = append(safe, m)
		}
		jsonResp(w, map[string]interface{}{"count": len(safe), "keys": safe})

	case http.MethodPost:
		if r.Header.Get("X-User-Role") != "admin" {
			forbidden(w, "solo admin puede crear API keys")
			return
		}
		var req struct {
			Name        string `json:"name"`
			Role        string `json:"role"`
			ExpiresDays int    `json:"expires_days"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpErr(w, err, http.StatusBadRequest)
			return
		}
		if req.Name == "" { req.Name = "api-key-" + generateID()[:6] }
		if req.Role == "" { req.Role = "operator" }

		plaintext, key, err := s.apiKeyStore.Generate(req.Name, req.Role, req.ExpiresDays)
		if err != nil {
			httpErr(w, err, http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		jsonResp(w, map[string]interface{}{
			"id": key.ID, "name": key.Name, "role": key.Role,
			"prefix": key.Prefix, "active": key.Active,
			"created_at": key.CreatedAt,
			"key": plaintext,
			"warning": "Guarda esta key ahora. No se puede recuperar despues.",
		})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *server) handleRevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if r.Header.Get("X-User-Role") != "admin" {
		forbidden(w, "solo admin puede revocar API keys")
		return
	}
	var req struct { ID string `json:"id"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpErr(w, err, http.StatusBadRequest)
		return
	}
	ok := s.apiKeyStore.Revoke(req.ID)
	jsonResp(w, map[string]interface{}{"success": ok, "id": req.ID})
}

func (s *server) handleAlertTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if r.Header.Get("X-User-Role") != "admin" {
		forbidden(w, "solo admin puede enviar alertas de prueba")
		return
	}
	var req struct {
		Level   string `json:"level"`
		Title   string `json:"title"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpErr(w, err, http.StatusBadRequest)
		return
	}
	if req.Level == "" { req.Level = "info" }
	if req.Title == "" { req.Title = "Alerta de prueba" }
	if req.Message == "" { req.Message = "Test desde snapah-pow" }

	s.alertsMgr.Send(alerts.Alert{
		Level:   req.Level,
		Title:   req.Title,
		Message: req.Message,
		NodeID:  "manual",
	})
	jsonResp(w, map[string]string{"status": "alerta enviada", "level": req.Level})
}

func (s *server) handleAlertConfig(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-User-Role") != "admin" {
		forbidden(w, "solo admin")
		return
	}
	cfg := alerts.ConfigFromEnv()
	jsonResp(w, map[string]interface{}{
		"email_enabled":   cfg.EmailEnabled,
		"webhook_enabled": cfg.WebhookEnabled,
		"smtp_host":       cfg.SMTPHost,
		"smtp_from":       cfg.SMTPFrom,
	})
}

func (s *server) handleReplicate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	// Solo admin u operator
	role := r.Header.Get("X-User-Role")
	if role != "admin" && role != "operator" {
		forbidden(w, "permisos insuficientes")
		return
	}

	var req struct {
		SnapshotID     string `json:"snapshot_id"`
		TargetNodeID   string `json:"target_node_id"`
		TargetAddress  string `json:"target_address"`
		TargetDestPath string `json:"target_dest_path"`
		ParentPath     string `json:"parent_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpErr(w, err, http.StatusBadRequest)
		return
	}

	snap, err := s.db.GetSnapshot(req.SnapshotID)
	if err != nil {
		httpErr(w, fmt.Errorf("snapshot no encontrado: %w", err), http.StatusNotFound)
		return
	}

	job := &replication.Job{
		ID:             generateID(),
		SnapshotID:     snap.ID,
		SnapshotPath:   snap.SnapshotPath,
		ParentPath:     req.ParentPath,
		TargetNodeID:   req.TargetNodeID,
		TargetAddress:  req.TargetAddress,
		TargetDestPath: req.TargetDestPath,
		Status:         "pending",
	}

	s.replMgr.ReplicateAsync(job)
	w.WriteHeader(http.StatusAccepted)
	jsonResp(w, map[string]string{"job_id": job.ID, "status": "started"})
}

func (s *server) handleReplicationJobs(w http.ResponseWriter, r *http.Request) {
	jobs := s.replMgr.ListJobs()
	jsonResp(w, map[string]interface{}{"count": len(jobs), "jobs": jobs})
}

// ── main ─────────────────────────────────────────────────

func main() {
	fmt.Println("🔥 Snapah Pow Server v" + appVersion)

	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("❌ Config error: %v", err)
	}

	os.MkdirAll("data", 0755)
	db, err := storage.NewDB(storage.DBFromEnv("data/snapah.db"))
	if err != nil {
		log.Fatalf("❌ Database error: %v", err)
	}

	srv := newServer(cfg, db)

	// ── Rate limiters ─────────────────────────────────────
	loginLimiter := ratelimit.New(10, time.Minute)
	apiLimiter   := ratelimit.New(200, time.Minute)

	// Actualizar metricas cada 30s
	go func() {
		for {
			if nodes, err := db.ListNodes(); err == nil {
				online := 0
				for _, n := range nodes {
					if n.Status == "online" { online++ }
				}
				metrics.NodesOnline.Set(float64(online))
			}
			if snaps, err := db.ListSnapshots(""); err == nil {
				metrics.SnapshotsTotal.Set(float64(len(snaps)))
			}
			if pols, err := db.ListPolicies(""); err == nil {
				metrics.PoliciesActive.Set(float64(len(pols)))
			}
			time.Sleep(30 * time.Second)
		}
	}()

	// ── gRPC ─────────────────────────────────────────────
	grpcAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.GRPCPort)
	grpcLis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("❌ gRPC listen: %v", err)
	}

	var grpcServer *grpc.Server
	if cfg.Server.TLSEnabled && tlsconfig.CertsExist(cfg.Server.TLSCert, cfg.Server.TLSKey, cfg.Server.TLSCACert) {
		creds, err := tlsconfig.ServerTLS(cfg.Server.TLSCert, cfg.Server.TLSKey, cfg.Server.TLSCACert)
		if err != nil {
			log.Fatalf("❌ mTLS: %v", err)
		}
		grpcServer = grpc.NewServer(grpc.Creds(creds))
		log.Println("🔐 gRPC con mTLS")
	} else {
		grpcServer = grpc.NewServer()
		log.Println("⚠️  gRPC sin TLS (desarrollo)")
	}

	pb.RegisterSnapManagerServer(grpcServer, srv)
	go func() {
		log.Printf("🚀 gRPC en %s", grpcAddr)
		grpcServer.Serve(grpcLis)
	}()

	// ── HTTP ─────────────────────────────────────────────
	mux := http.NewServeMux()

	// Metricas Prometheus — público
	mux.Handle("/", http.FileServer(http.Dir("web")))

	// Público
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		jsonResp(w, map[string]string{"status": "ok", "version": appVersion})
	})

	// Auth — rate limit estricto
	mux.HandleFunc("/api/auth/login", loginLimiter.Middleware(srv.handleLogin))

	// API — rate limit + JWT
	mux.HandleFunc("/api/nodes",             apiLimiter.Middleware(srv.authMgr.Middleware(srv.handleNodes)))
	mux.HandleFunc("/api/snapshots",         apiLimiter.Middleware(srv.authMgr.Middleware(srv.handleSnapshots)))
	mux.HandleFunc("/api/snapshots/delete",  apiLimiter.Middleware(srv.authMgr.Middleware(srv.handleDeleteSnapshot)))
	mux.HandleFunc("/api/events",            apiLimiter.Middleware(srv.authMgr.Middleware(srv.handleEvents)))
	mux.HandleFunc("/api/policies",          apiLimiter.Middleware(srv.authMgr.Middleware(srv.handlePolicies)))
	mux.HandleFunc("/api/restore",          apiLimiter.Middleware(srv.authMgr.Middleware(srv.handleRestore)))
	mux.HandleFunc("/api/keys",        apiLimiter.Middleware(srv.authMgr.Middleware(srv.handleAPIKeys)))
	mux.HandleFunc("/api/keys/revoke",  apiLimiter.Middleware(srv.authMgr.Middleware(srv.handleRevokeAPIKey)))
	mux.HandleFunc("/api/alerts/test",   apiLimiter.Middleware(srv.authMgr.Middleware(srv.handleAlertTest)))
	mux.HandleFunc("/api/alerts/config", apiLimiter.Middleware(srv.authMgr.Middleware(srv.handleAlertConfig)))
	mux.HandleFunc("/api/replicate",         apiLimiter.Middleware(srv.authMgr.Middleware(srv.handleReplicate)))
	mux.HandleFunc("/api/replication/jobs",  apiLimiter.Middleware(srv.authMgr.Middleware(srv.handleReplicationJobs)))

	// Servidor de métricas en puerto separado (no interfiere con FileServer)
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())
	metricsServer := &http.Server{Addr: "0.0.0.0:9093", Handler: metricsMux}
	go func() {
		log.Printf("📊 Metrics en http://0.0.0.0:9093/metrics")
		metricsServer.ListenAndServe()
	}()

	httpAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	httpServer := &http.Server{Addr: httpAddr, Handler: mux}
	go func() {
		log.Printf("🌐 HTTP en http://%s", httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP error: %v", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("👋 Shutdown...")
	grpcServer.GracefulStop()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	httpServer.Shutdown(ctx)
	log.Println("✅ Server detenido")
}
