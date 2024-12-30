package main

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/pelletier/go-toml"
	"github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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

const (
	configFilePath = "/etc/desktopimage/config.toml"
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
		os.Exit(0)
	}

	content, err := os.ReadFile(configFilePath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	err = toml.Unmarshal(content, &config)
	if err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	return nil
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

	if err := loadConfig(configFilePath); err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	log.Info("Starting AppImage watcher...")

	// Initialize the watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("Error initializing watcher: %v", err)
	}
	defer watcher.Close()

	// Add the app directory to the watcher
	if err := watcher.Add(config.AppPath); err != nil {
		log.Fatalf("Error adding directory to watcher: %v", err)
	}

	// Process events
	for {
		select {
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
			} else if event.Op&fsnotify.Remove == fsnotify.Remove {
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
		case err := <-watcher.Errors:
			log.Errorf("Watcher error: %v", err)
		}
	}
}

func createDesktopFile(appName, desktopFilePath string) error {
	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=%s
Exec=%s/%s
Icon=%s
Terminal=false
Categories=%s
`, appName, config.AppPath, appName+".AppImage", config.IconPath, config.Categories)

	return os.WriteFile(desktopFilePath, []byte(content), 0644)
}

func updateDesktopDatabase() {
	if err := exec.Command("update-desktop-database", config.DesktopPath).Run(); err != nil {
		log.Errorf("Error updating desktop database: %v", err)
	} else {
		log.Info("Desktop database updated.")
	}
}
