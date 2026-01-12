package cmd

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"zarkham/core"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Zarkham dVPN node",
	Run: func(cmd *cobra.Command, args []string) {
		node, err := core.NewZarkhamNode(core.Config{
			ConfigDir:   cfgDir,
			RpcEndpoint: rpcEndpoint,
		})
		if err != nil {
			log.Fatalf("Failed to initialize node: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if err := node.Start(ctx, profile); err != nil {
			log.Fatalf("Failed to start node: %v", err)
		}

		log.Printf("Zarkham Node is running (Profile: %s)", profile)

		// Wait for interruption
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down...")
		_ = node.Stop()
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
