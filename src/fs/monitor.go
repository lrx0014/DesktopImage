package fs

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/lrx0014/DesktopImage/src/config"
	logging "github.com/lrx0014/DesktopImage/src/log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

import (
	"context"
	"sync"
)

var (
	log = logging.GetLogger()
)

type FManager struct {
	currentWatchers map[string]context.CancelFunc
	mutex           sync.Mutex
	cm              *config.ConfManager
	closeCh         chan struct{}
}

func NewFsManager(cm *config.ConfManager) (fm *FManager, err error) {
	fm = &FManager{
		cm: cm,
	}

	fm.cm.AddCallbacks(func(conf *config.Config) {
		err = fm.reloadWatchers(conf)
		if err != nil {
			return
		}
	})

	return
}

func (fm *FManager) StartWatchers(ctx context.Context) {
	cf := fm.cm.GetConfig()
	if cf == nil {
		return
	}

	wg := sync.WaitGroup{}

	for _, watcher := range cf.Watcher {
		wg.Add(1)
		_watcher := watcher
		go func() {
			defer wg.Done()
			fm.startWatching(ctx, _watcher)
		}()
	}

	wg.Wait()
	log.Info("App file watcher stopped")
}

func (fm *FManager) Close() {
	if fm.closeCh != nil {
		<-fm.closeCh
		log.Info("FS Manager closed")
	}
	return
}

func (fm *FManager) startWatching(ctx context.Context, watcherConfig config.Watcher) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("Error initializing file watcher: %v", err)
	}
	defer watcher.Close()

	if err := watcher.Add(watcherConfig.AppPath); err != nil {
		log.Fatalf("Error adding app directory to watcher: %v", err)
	}

	log.Infof("Starting file watcher on: %s => %s", watcherConfig.AppPath, watcherConfig.DesktopPath)

	fm.closeCh = make(chan struct{})

	for {
		select {
		case <-ctx.Done():
			log.Info("Stopping AppImage watcher.")
			fm.closeCh <- struct{}{}
			return
		case event := <-watcher.Events:
			if event.Op&fsnotify.Create == fsnotify.Create {
				if strings.HasSuffix(event.Name, ".AppImage") {
					appName := strings.TrimSuffix(filepath.Base(event.Name), ".AppImage")
					if err := fm.createDesktopFile(appName, watcherConfig); err != nil {
						log.Errorf("Error creating .desktop file for %s: %v", appName, err)
					} else {
						log.Infof("Created .desktop file for %s on %s", appName, watcherConfig.DesktopPath)
						fm.updateDesktopDatabase(watcherConfig)
					}
				}
			} else if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
				if strings.HasSuffix(event.Name, ".AppImage") {
					appName := strings.TrimSuffix(filepath.Base(event.Name), ".AppImage")
					desktopFilePath := filepath.Join(watcherConfig.DesktopPath, appName+".desktop")
					if err := os.Remove(desktopFilePath); err != nil {
						log.Errorf("Error removing .desktop file for %s: %v", appName, err)
					} else {
						log.Infof("Removed .desktop file for %s", appName)
						fm.updateDesktopDatabase(watcherConfig)
					}
				}
			}
		}
	}
}

func (fm *FManager) reloadWatchers(newConf *config.Config) (err error) {
	for _, cancel := range fm.currentWatchers {
		cancel()
	}
	fm.currentWatchers = make(map[string]context.CancelFunc)

	for _, watcher := range newConf.Watcher {
		ctx, cancel := context.WithCancel(context.Background())
		fm.currentWatchers[watcher.AppPath] = cancel
		go fm.startWatching(ctx, watcher)
	}
	return
}

func (fm *FManager) createDesktopFile(appName string, watcherConfig config.Watcher) error {
	desktopFilePath := filepath.Join(watcherConfig.DesktopPath, appName+".desktop")
	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=%s
Exec=%s/%s
Terminal=false
Categories=%s
`, appName, watcherConfig.AppPath, appName+".AppImage", watcherConfig.Categories)

	if watcherConfig.IconPath != "" {
		content += fmt.Sprintf("Icon=%s\n", watcherConfig.IconPath)
	}

	return os.WriteFile(desktopFilePath, []byte(content), 0644)
}

func (fm *FManager) updateDesktopDatabase(watcherConfig config.Watcher) {
	if err := exec.Command("update-desktop-database", watcherConfig.DesktopPath).Run(); err != nil {
		log.Errorf("Error updating desktop database: %v", err)
	} else {
		log.Info("Desktop database updated.")
	}
}
