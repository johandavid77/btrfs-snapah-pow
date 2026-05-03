package main

import (
	"context"
	"fmt"
	"os"
	"time"

	pb "github.com/corp/btrfs-snapah-pow/api/proto"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var appVersion = "dev"
var serverAddr string

func getClient() (pb.SnapManagerClient, *grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return nil, nil, fmt.Errorf("no pudo conectar a %s: %w", serverAddr, err)
	}
	return pb.NewSnapManagerClient(conn), conn, nil
}

func main() {
	root := &cobra.Command{
		Use:     "snapah",
		Short:   "CLI de Btrfs Snapah Pow",
		Long:    "Administración corporativa de snapshots BTRFS",
		Version: appVersion,
	}
	root.PersistentFlags().StringVar(&serverAddr, "server", "localhost:9090", "Dirección del servidor gRPC")

	// ─── SNAPSHOT ─────────────────────────────
	snapshotCmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Gestiona snapshots",
	}

	snapshotCmd.AddCommand(&cobra.Command{
		Use:   "create [subvolumen] [nombre]",
		Short: "Crea un snapshot",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			client, conn, err := getClient()
			if err != nil {
				fmt.Fprintf(os.Stderr, "❌ %v\n", err)
				os.Exit(1)
			}
			defer conn.Close()

			resp, err := client.CreateSnapshot(context.Background(), &pb.CreateSnapshotRequest{
				SubvolumePath: args[0],
				SnapshotName:  args[1],
				Readonly:      true,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "❌ Error: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("✅ Snapshot creado:\n")
			fmt.Printf("   ID:   %s\n", resp.Id)
			fmt.Printf("   Path: %s\n", resp.SnapshotPath)
			fmt.Printf("   Time: %s\n", resp.CreatedAt)
		},
	})

	snapshotCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "Lista snapshots",
		Run: func(cmd *cobra.Command, args []string) {
			client, conn, err := getClient()
			if err != nil {
				fmt.Fprintf(os.Stderr, "❌ %v\n", err)
				os.Exit(1)
			}
			defer conn.Close()

			resp, err := client.ListSnapshots(context.Background(), &pb.ListSnapshotsRequest{})
			if err != nil {
				fmt.Fprintf(os.Stderr, "❌ Error: %v\n", err)
				os.Exit(1)
			}

			if len(resp.Snapshots) == 0 {
				fmt.Println("📭 No hay snapshots")
				return
			}

			fmt.Printf("📸 %d snapshot(s):\n", len(resp.Snapshots))
			fmt.Println("────────────────────────────────────────")
			for _, s := range resp.Snapshots {
				ro := "RW"
				if s.IsReadonly {
					ro = "RO"
				}
				fmt.Printf("  %-20s [%s] %s\n", s.Id[:8], ro, s.SnapshotPath)
			}
		},
	})

	snapshotCmd.AddCommand(&cobra.Command{
		Use:   "delete [id]",
		Short: "Elimina un snapshot",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client, conn, err := getClient()
			if err != nil {
				fmt.Fprintf(os.Stderr, "❌ %v\n", err)
				os.Exit(1)
			}
			defer conn.Close()

			resp, err := client.DeleteSnapshot(context.Background(), &pb.DeleteSnapshotRequest{
				SnapshotId: args[0],
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "❌ Error: %v\n", err)
				os.Exit(1)
			}

			if resp.Success {
				fmt.Printf("🗑️  %s\n", resp.Message)
			} else {
				fmt.Printf("❌ %s\n", resp.Message)
			}
		},
	})

	// ─── NODE ─────────────────────────────────
	nodeCmd := &cobra.Command{
		Use:   "node",
		Short: "Gestiona nodos",
	}

	nodeCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "Lista nodos registrados",
		Run: func(cmd *cobra.Command, args []string) {
			client, conn, err := getClient()
			if err != nil {
				fmt.Fprintf(os.Stderr, "❌ %v\n", err)
				os.Exit(1)
			}
			defer conn.Close()

			resp, err := client.ListNodes(context.Background(), &pb.ListNodesRequest{})
			if err != nil {
				fmt.Fprintf(os.Stderr, "❌ Error: %v\n", err)
				os.Exit(1)
			}

			if len(resp.Nodes) == 0 {
				fmt.Println("📭 No hay nodos registrados")
				return
			}

			fmt.Printf("🖥️  %d nodo(s):\n", len(resp.Nodes))
			fmt.Println("────────────────────────────────────────")
			for _, n := range resp.Nodes {
				status := "🟢"
				if n.Status != "online" {
					status = "🔴"
				}
				fmt.Printf("  %s %-15s %-20s %s\n", status, n.Id[:8], n.Hostname, n.Address)
			}
		},
	})

	// ─── STATUS ───────────────────────────────
	root.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Estado del sistema",
		Run: func(cmd *cobra.Command, args []string) {
			client, conn, err := getClient()
			if err != nil {
				fmt.Println("╔══════════════════════════════════════╗")
				fmt.Println("║     🔥 BTRFS SNAPAH POW STATUS      ║")
				fmt.Println("╠══════════════════════════════════════╣")
				fmt.Printf("║  Server:  %-26s ║\n", serverAddr)
				fmt.Println("║  Status:  🔴 OFFLINE                 ║")
				fmt.Println("╚══════════════════════════════════════╝")
				return
			}
			defer conn.Close()

			nodes, _ := client.ListNodes(context.Background(), &pb.ListNodesRequest{})
			snaps, _ := client.ListSnapshots(context.Background(), &pb.ListSnapshotsRequest{})

			fmt.Println("╔══════════════════════════════════════╗")
			fmt.Println("║     🔥 BTRFS SNAPAH POW STATUS      ║")
			fmt.Println("╠══════════════════════════════════════╣")
			fmt.Printf("║  Server:  %-26s ║\n", serverAddr)
			fmt.Println("║  Status:  🟢 ONLINE                  ║")
			fmt.Printf("║  Nodos:   %-26d ║\n", len(nodes.Nodes))
			fmt.Printf("║  Snapshots: %-24d ║\n", len(snaps.Snapshots))
			fmt.Println("╚══════════════════════════════════════╝")
		},
	})

	root.AddCommand(snapshotCmd)
	root.AddCommand(nodeCmd)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
