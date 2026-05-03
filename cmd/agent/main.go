package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "github.com/johandavid77/btrfs-snapah-pow/api/proto"
	"github.com/johandavid77/btrfs-snapah-pow/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var appVersion = "dev"

func main() {
	fmt.Println("🔥 Snapah Pow Agent v" + appVersion)

	cfgPath := os.Getenv("SNAPAH_CONFIG")
	if cfgPath == "" {
		cfgPath = "config.yaml"
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Printf("⚠️  Config no encontrada, usando defaults: %v", err)
	}

	serverAddr := os.Getenv("SNAPAH_SERVER")
	if serverAddr == "" && cfg != nil {
		serverAddr = cfg.Agent.ServerAddress
	}
	if serverAddr == "" {
		serverAddr = "localhost:9090"
	}

	token := os.Getenv("SNAPAH_TOKEN")
	if token == "" && cfg != nil {
		token = cfg.Agent.Token
	}
	if token == "" {
		token = "demo-token"
	}

	log.Printf("🔌 Conectando a servidor: %s", serverAddr)

	conn, err := grpc.NewClient(serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("❌ No pudo conectar: %v", err)
	}
	defer conn.Close()

	client := pb.NewSnapManagerClient(conn)

	// Registrar nodo
	hostname, _ := os.Hostname()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	regResp, err := client.RegisterNode(ctx, &pb.RegisterNodeRequest{
		Hostname: hostname,
		Address:  serverAddr,
		Token:    token,
	})
	cancel()
	if err != nil {
		log.Fatalf("❌ Fallo registro: %v", err)
	}
	log.Printf("🖥️  Nodo registrado: %s (ID: %s)", regResp.Hostname, regResp.Id)

	nodeID := regResp.Id

	// Heartbeat loop — simple ping al servidor cada N segundos
	heartbeatSec := 30
	if cfg != nil && cfg.Agent.HeartbeatSec > 0 {
		heartbeatSec = cfg.Agent.HeartbeatSec
	}

	ticker := time.NewTicker(time.Duration(heartbeatSec) * time.Second)
	defer ticker.Stop()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("💓 Heartbeat cada %ds. Ctrl+C para detener.", heartbeatSec)

	for {
		select {
		case <-ticker.C:
			// Heartbeat: listar snapshots como ping liviano
			hbCtx, hbCancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, err := client.ListSnapshots(hbCtx, &pb.ListSnapshotsRequest{NodeId: nodeID})
			hbCancel()
			if err != nil {
				log.Printf("⚠️  Heartbeat falló: %v", err)
			} else {
				log.Printf("💓 Heartbeat OK [%s]", time.Now().Format("15:04:05"))
			}

		case sig := <-sigChan:
			fmt.Printf("\n👋 Agent detenido (%v)\n", sig)
			return
		}
	}
}
