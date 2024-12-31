package log

import (
	"github.com/sirupsen/logrus"
	"os"
)

var logger *logrus.Logger

func init() {
	logger = logrus.New()
	logger.Out = os.Stdout
	logger.SetFormatter(&logrus.TextFormatter{DisableColors: false, FullTimestamp: true})
}

func GetLogger() *logrus.Logger {
	return logger
}
