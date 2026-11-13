package cli

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not launch a browser: %w", err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func envTokenSet() bool { return os.Getenv("IBKR_ACCESS_TOKEN") != "" }
