package main

import (
	"context"
	"github.com/lrx0014/DesktopImage/src/fs"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/lrx0014/DesktopImage/src/config"
	logging "github.com/lrx0014/DesktopImage/src/log"
)

var (
	log = logging.GetLogger()
)

func checkEnvironment() {
	if runtime.GOOS != "linux" {
		log.Fatalf("Unsupported operating system: %s. This program can only run on Linux.", runtime.GOOS)
	}

	if _, err := exec.LookPath("update-desktop-database"); err != nil {
		log.Fatalf("Required desktop utility 'update-desktop-database' is not installed or not in PATH.")
	}

	log.Info("Environment check passed: Linux system with desktop utilities available.")
}

func main() {

	checkEnvironment()

	ctx, cancel := context.WithCancel(context.Background())

	log.Info("creating configManager...")
	configManager, err := config.NewConfigManager(ctx)
	if err != nil {
		log.Fatalf("failed to start config manager: %+v", err)
		return
	}

	log.Info("creating fsManager...")
	fsManager, err := fs.NewFsManager(configManager)
	if err != nil {
		log.Fatalf("failed to start fs manager: %+v", err)
		return
	}

	fsManager.StartWatchers(ctx)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	<-sigs
	log.Info("Shutdown signal received.")
	cancel()
	configManager.Close()
	fsManager.Close()
	log.Info("All tasks stopped. Exiting.")
}
