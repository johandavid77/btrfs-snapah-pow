package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var appVersion = "dev"

func main() {
	root := &cobra.Command{
		Use:     "snapah",
		Short:   "CLI de Btrfs Snapah Pow",
		Long:    "Administración corporativa de snapshots BTRFS",
		Version: appVersion,
	}

	root.AddCommand(&cobra.Command{
		Use:   "snapshot",
		Short: "Gestiona snapshots",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("📋 Snapshots:")
			fmt.Println("  (conecta al servidor para listar)")
		},
	})

	root.AddCommand(&cobra.Command{
		Use:   "node",
		Short: "Gestiona nodos",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("🖥️  Nodos:")
			fmt.Println("  (conecta al servidor para listar)")
		},
	})

	root.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Estado del sistema",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("╔══════════════════════════════════════╗")
			fmt.Println("║     🔥 BTRFS SNAPAH POW STATUS      ║")
			fmt.Println("╠══════════════════════════════════════╣")
			fmt.Println("║  Server:  localhost:8080             ║")
			fmt.Println("║  Status:  🟢 ONLINE                  ║")
			fmt.Println("╚══════════════════════════════════════╝")
		},
	})

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
