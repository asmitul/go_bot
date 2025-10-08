package logger

import (
	"os"

	log "github.com/sirupsen/logrus"
)

// Init configures the global logrus logger.
// It is safe to call multiple times; later calls overwrite previous settings.
func Init() {
	log.SetOutput(os.Stdout)
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})

	levelStr := os.Getenv("LOG_LEVEL")
	if levelStr == "" {
		levelStr = "info"
	}
	if lvl, err := log.ParseLevel(levelStr); err == nil {
		log.SetLevel(lvl)
	} else {
		log.SetLevel(log.InfoLevel)
	}
}

// L returns the global logger for convenience.
func L() *log.Logger { return log.StandardLogger() }
