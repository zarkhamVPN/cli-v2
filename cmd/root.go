package cmd

import (
	"fmt"
	"os"

	figure "github.com/common-nighthawk/go-figure"
	"github.com/spf13/cobra"
)

var (
	cfgDir      string
	rpcEndpoint string
	profile     string
)

var rootCmd = &cobra.Command{
	Use:   "zarkham",
	Short: "Zarkham helps you join the decentralized VPN network.",
	Long:  `A modular CLI to run Zarkham dVPN nodes and manage your Solana-based bandwidth economy.`,
	Run: func(cmd *cobra.Command, args []string) {
		myFigure := figure.NewFigure("ZARKHAM", "larry3d", true)
		fmt.Println(titleStyle.Render(myFigure.String()))
		fmt.Println(promptStyle.Render("Welcome to Zarkham. Use 'zarkham --help' for available commands."))
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgDir, "config", "config", "Configuration directory")
	rootCmd.PersistentFlags().StringVar(&rpcEndpoint, "rpc", "https://api.devnet.solana.com", "Solana RPC endpoint")
	rootCmd.PersistentFlags().StringVarP(&profile, "profile", "p", "default", "Wallet profile name")
}
