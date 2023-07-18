package config

import (
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

func InitLogrus(cfg *Config) *logrus.Entry {
	logger := logrus.New()

	if cfg.Log.Format == "json" {
		logger.SetFormatter(&logrus.JSONFormatter{})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{})
	}

	level, err := logrus.ParseLevel(cfg.Log.Level)
	if err == nil {
		logger.SetLevel(level)
	}

	if cfg.Log.FilePath != "" {
		logger.SetOutput(&lumberjack.Logger{
			Filename:   cfg.Log.FilePath,
			MaxSize:    500,
			MaxAge:     28,
			Compress:   false,
			MaxBackups: 3,
		})
	}

	return logger.WithField("service", cfg.App.ServiceName)
}
