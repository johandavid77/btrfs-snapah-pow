package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "github.com/corp/btrfs-snapah-pow/api/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var appVersion = "dev"

func main() {
	fmt.Println("🔥 Snapah Pow Agent v" + appVersion)

	serverAddr := os.Getenv("SNAPAH_SERVER")
	if serverAddr == "" {
		serverAddr = "localhost:9090"
	}

	// Conectar al servidor gRPC
	conn, err := grpc.NewClient(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("❌ No pudo conectar al server: %v", err)
	}
	defer conn.Close()

	client := pb.NewSnapManagerClient(conn)
	ctx := context.Background()

	// Registrar nodo
	hostname, _ := os.Hostname()
	regResp, err := client.RegisterNode(ctx, &pb.RegisterNodeRequest{
		Hostname: hostname,
		Address:  serverAddr,
		Token:    "demo-token",
	})
	if err != nil {
		log.Fatalf("❌ Fallo registro: %v", err)
	}
	log.Printf("🖥️  Nodo registrado: %s (ID: %s)", regResp.Hostname, regResp.Id)

	// Heartbeat loop
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			// Enviar heartbeat como evento
			_, err := client.StreamEvents(ctx, &pb.StreamEventsRequest{
				NodeId: regResp.Id,
			})
			if err != nil {
				log.Printf("⚠️  Heartbeat falló: %v", err)
			} else {
				log.Printf("💓 Heartbeat enviado")
			}

		case <-sigChan:
			fmt.Println("\n👋 Agent detenido")
			return
		}
	}
}
