package cmd

import (
	"fmt"
	"io/fs"
	"os"

	"github.com/common-nighthawk/go-figure"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

var (
	cfgDir      string
	rpcEndpoint string
	profile     string
	embeddedGUI fs.FS
)

var rootCmd = &cobra.Command{
	Use:   "zarkham",
	Short: "Zarkham helps you join the decentralized VPN network.",
	Long:  `A modular CLI to run Zarkham dVPN nodes and manage your Solana-based bandwidth economy.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if err := godotenv.Load(); err != nil {
		}

		if rpcEndpoint == "https://api.devnet.solana.com" {
			rpcEndpoint = "https://api.zarkham.xyz/api/v1/proxy"
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		myFigure := figure.NewFigure("ZARKHAM", "larry3d", true)
		fmt.Println(titleStyle.Render(myFigure.String()))
		fmt.Println(promptStyle.Render("Welcome to Zarkham. Use 'zarkham --help' for available commands."))
	},
}

func Execute(guiFS fs.FS) {
	embeddedGUI = guiFS
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	home, err := os.UserHomeDir()
	defaultCfg := "config"
	if err == nil {
		defaultCfg = fmt.Sprintf("%s/.zarkham/config", home)
	}

	rootCmd.PersistentFlags().StringVar(&cfgDir, "config", defaultCfg, "Configuration directory")
	rootCmd.PersistentFlags().StringVar(&rpcEndpoint, "rpc", "https://api.devnet.solana.com", "Solana RPC endpoint")
	rootCmd.PersistentFlags().StringVarP(&profile, "profile", "p", "default", "Wallet profile name")
}
