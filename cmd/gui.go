package cmd

import (
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"
)

var guiCmd = &cobra.Command{
	Use:   "gui",
	Short: "Open the Zarkham Graphical Interface",
	Run: func(cmd *cobra.Command, args []string) {
		url := "http://localhost:8088"
		
		fmt.Printf(titleStyle.Render("Launching Zarkham GUI at %s\n"), url)

		// Open browser in background after a short delay
		go func() {
			time.Sleep(2 * time.Second)
			var err error
			switch runtime.GOOS {
			case "linux":
				err = exec.Command("xdg-open", url).Start()
			case "darwin":
				err = exec.Command("open", url).Start()
			case "windows":
				err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
			}
			if err != nil {
				log.Printf("Note: Could not auto-open browser: %v", err)
			}
		}()

		// Call the existing start command logic
		startCmd.Run(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(guiCmd)
}