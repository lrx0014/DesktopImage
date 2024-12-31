package main

import (
	"context"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/pelletier/go-toml"
	"github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
)

const (
	configFilePath = "/etc/desktopimage/config.toml"
)

type Config struct {
	AppPath     string `toml:"app_path"`
	DesktopPath string `toml:"desktop_path"`
	IconPath    string `toml:"icon_path"`
	Categories  string `toml:"categories"`
}

var (
	config Config
	log    = logrus.New()
)

func ensureConfigDirectoryExists(configDirPath string) error {
	if _, err := os.Stat(configDirPath); os.IsNotExist(err) {
		log.Warnf("Configuration directory %s does not exist. Creating it.", configDirPath)
		if err := os.MkdirAll(configDirPath, 0755); err != nil {
			return fmt.Errorf("failed to create configuration directory: %w", err)
		}
		log.Infof("Configuration directory created at %s.", configDirPath)
	}
	return nil
}

func createDefaultConfig(configFilePath string) error {
	defaultConfig := `# app_path = "/path/to/app_directory"
# desktop_path = "/path/to/desktop_directory"
# icon_path = "/path/to/icon.png"
# categories = "Application"
`
	return os.WriteFile(configFilePath, []byte(defaultConfig), 0644)
}

func isConfigValid(cfg Config) bool {
	return cfg.AppPath != "" && cfg.DesktopPath != "" && cfg.Categories != ""
}

func loadConfig(configFilePath string) error {
	configDirPath := filepath.Dir(configFilePath)
	if err := ensureConfigDirectoryExists(configDirPath); err != nil {
		return err
	}

	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		log.Warnf("Configuration file %s does not exist. Creating default template.", configFilePath)
		if err := createDefaultConfig(configFilePath); err != nil {
			return fmt.Errorf("failed to create default config file: %w", err)
		}
		log.Infof("Default configuration template created at %s. Please edit and uncomment required fields.", configFilePath)
	}

	content, err := os.ReadFile(configFilePath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	err = toml.Unmarshal(content, &config)
	if err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	if !isConfigValid(config) {
		log.Warn("Configuration file is incomplete or invalid. Waiting for user to update it.")
		return nil
	}

	log.Infof("Configuration successfully loaded from %s. with config content = %+v", configFilePath, config)
	return nil
}

func watchConfigFile(ctx context.Context, configFilePath string, reloadConfig chan bool) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("Error initializing config file watcher: %v", err)
	}
	defer watcher.Close()

	if err := watcher.Add(filepath.Dir(configFilePath)); err != nil {
		log.Fatalf("Error adding config directory to watcher: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			log.Info("Stopping config file watcher.")
			return
		case event := <-watcher.Events:
			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 && filepath.Base(event.Name) == filepath.Base(configFilePath) {
				log.Infof("Configuration file %s changed, reloading...", configFilePath)
				reloadConfig <- true
			}
		case err := <-watcher.Errors:
			log.Errorf("Config watcher error: %v", err)
		}
	}
}

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
	log.Out = os.Stdout
	log.SetFormatter(&logrus.TextFormatter{DisableColors: false, FullTimestamp: true})

	checkEnvironment()

	reloadConfig := make(chan bool)
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	// Start watching config file
	wg.Add(1)
	go func() {
		defer wg.Done()
		watchConfigFile(ctx, configFilePath, reloadConfig)
	}()

	if err := loadConfig(configFilePath); err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	log.Info("Starting AppImage watcher...")

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("Error initializing file watcher: %v", err)
	}
	defer watcher.Close()

	if config.AppPath != "" {
		if err := watcher.Add(config.AppPath); err != nil {
			log.Fatalf("Error adding app directory to watcher: %v", err)
		}
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				log.Info("Stopping AppImage watcher.")
				return
			case event := <-watcher.Events:
				if event.Op&fsnotify.Create == fsnotify.Create {
					if strings.HasSuffix(event.Name, ".AppImage") {
						appName := strings.TrimSuffix(filepath.Base(event.Name), ".AppImage")
						desktopFilePath := filepath.Join(config.DesktopPath, appName+".desktop")
						if err := createDesktopFile(appName, desktopFilePath); err != nil {
							log.Errorf("Error creating .desktop file for %s: %v", appName, err)
						} else {
							log.Infof("Created .desktop file for %s", appName)
							updateDesktopDatabase()
						}
					}
				} else if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
					if strings.HasSuffix(event.Name, ".AppImage") {
						appName := strings.TrimSuffix(filepath.Base(event.Name), ".AppImage")
						desktopFilePath := filepath.Join(config.DesktopPath, appName+".desktop")
						if err := os.Remove(desktopFilePath); err != nil {
							log.Errorf("Error removing .desktop file for %s: %v", appName, err)
						} else {
							log.Infof("Removed .desktop file for %s", appName)
							updateDesktopDatabase()
						}
					}
				}
			case <-reloadConfig:
				if err := loadConfig(configFilePath); err != nil {
					log.Errorf("Error reloading configuration: %v", err)
				} else {
					log.Info("Configuration reloaded successfully.")
					if err := watcher.Add(config.AppPath); err != nil {
						log.Errorf("Error adding new app directory to watcher: %v", err)
					}
				}
			}
		}
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
	<-sigs
	log.Info("Shutdown signal received.")
	cancel()
	wg.Wait()
	log.Info("All tasks stopped. Exiting.")
}

func createDesktopFile(appName, desktopFilePath string) error {
	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=%s
Exec=%s/%s
Terminal=false
Categories=%s
`, appName, config.AppPath, appName+".AppImage", config.Categories)

	if config.IconPath != "" {
		content += fmt.Sprintf("Icon=%s\n", config.IconPath)
	}

	return os.WriteFile(desktopFilePath, []byte(content), 0644)
}

func updateDesktopDatabase() {
	if err := exec.Command("update-desktop-database", config.DesktopPath).Run(); err != nil {
		log.Errorf("Error updating desktop database: %v", err)
	} else {
		log.Info("Desktop database updated.")
	}
}
