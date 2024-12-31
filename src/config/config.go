package config

import (
	"context"
	"fmt"
	"github.com/fsnotify/fsnotify"
	logging "github.com/lrx0014/DesktopImage/src/log"
	"github.com/pelletier/go-toml"
	"os"
	"path/filepath"
	"sync"
)

const (
	configPath = "/etc/desktopimage/config.toml"
)

var (
	mutex sync.Mutex
	log   = logging.GetLogger()
)

type ConfManager struct {
	onChangeCallbacks []func(*Config)
	conf              *Config
	closeCh           chan struct{}
}

type Config struct {
	AutoGrantExecutable bool      `toml:"auto_grant_executable"`
	Watcher             []Watcher `toml:"Watcher"`
}

type Watcher struct {
	AppPath     string `toml:"app_path"`
	DesktopPath string `toml:"desktop_path"`
	IconPath    string `toml:"icon_path,omitempty"`
	Categories  string `toml:"categories"`
}

func NewConfigManager(ctx context.Context) (cm *ConfManager, err error) {
	cm = &ConfManager{}
	configDirPath := filepath.Dir(configPath)
	if err = ensureConfigDirectoryExists(configDirPath); err != nil {
		return
	}

	if _, err = os.Stat(configPath); os.IsNotExist(err) {
		log.Warnf("Configuration file %s does not exist. Creating default template.", configPath)
		if err = createDefaultConfig(configPath); err != nil {
			return
		}
		log.Infof("Default configuration template created at %s. Please edit and uncomment required fields.", configPath)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		return
	}

	config := &Config{}
	err = toml.Unmarshal(content, config)
	if err != nil {
		return
	}

	if !valid(config) {
		log.Warn("Configuration file is incomplete or invalid. Waiting for user to update it.")
		return
	}

	cm.conf = config

	go cm.watch(ctx)

	log.Infof("Configuration successfully loaded from %s. with config content = %+v", configPath, config)
	return
}

func (cm *ConfManager) GetConfig() *Config {
	return cm.conf
}

func (cm *ConfManager) AddCallbacks(callbacks ...func(config2 *Config)) {
	mutex.Lock()
	defer mutex.Unlock()
	cm.onChangeCallbacks = append(cm.onChangeCallbacks, callbacks...)
}

func (cm *ConfManager) Close() {
	if cm.closeCh != nil {
		<-cm.closeCh
		log.Info("Config Manager closed")
	}
	return
}

// watch start/reload Watcher on the configured paths
func (cm *ConfManager) watch(ctx context.Context) (err error) {
	log.Info("Starting config watcher...")
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("Error initializing config file watcher: %v", err)
		return
	}
	defer watcher.Close()

	if err = watcher.Add(filepath.Dir(configPath)); err != nil {
		log.Fatalf("Error adding config directory to watcher: %v", err)
		return
	}

	var wg sync.WaitGroup
	cm.closeCh = make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Info("Started config file watcher...")
		for {
			select {
			case <-ctx.Done():
				log.Info("Stopping config file watcher.")
				cm.closeCh <- struct{}{}
				return
			case event := <-watcher.Events:
				if event.Op&(fsnotify.Write|fsnotify.Create) != 0 && filepath.Base(event.Name) == filepath.Base(configPath) {
					log.Infof("Configuration file %s changed, reloading...", configPath)
					// reload config
					if err = cm.reload(); err != nil {
						log.Errorf("Error reloading configuration: %v", err)
					} else {
						log.Infof("Configuration reloaded successfully. new config: %+v", cm.conf)
						for _, callback := range cm.onChangeCallbacks {
							go callback(cm.conf)
						}
					}
				}
			case err := <-watcher.Errors:
				log.Errorf("Config watcher error: %v", err)
			}
		}
	}()

	wg.Wait()
	log.Info("Config file watcher stopped")

	return
}

func (cm *ConfManager) reload() (err error) {
	rf, err := os.ReadFile(configPath)
	if err != nil {
		return
	}
	newConf := Config{}
	if err = toml.Unmarshal(rf, &newConf); err != nil {
		return
	}

	cm.conf.AutoGrantExecutable = newConf.AutoGrantExecutable
	cm.conf.Watcher = newConf.Watcher

	return
}

func valid(conf *Config) bool {
	if conf == nil {
		return false
	}

	if len(conf.Watcher) == 0 {
		return false
	}

	return conf.Watcher[0].AppPath != "" &&
		conf.Watcher[0].DesktopPath != "" &&
		conf.Watcher[0].Categories != ""
}

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
	defaultConfig := `# auto_grant_executable = false

# [[Watcher]]
# app_path = "/path/to/app_directory"
# desktop_path = "/path/to/desktop_directory"
# icon_path = "/path/to/icon.png"
# categories = "Application"
`
	return os.WriteFile(configFilePath, []byte(defaultConfig), 0644)
}
