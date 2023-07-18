package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/SlideSolo/chunk-vault/config"

	log "github.com/sirupsen/logrus"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	cfg, err := config.New(ctx)
	if err != nil {
		log.WithError(err).Fatalln("failed to load config")
	}

	logger := config.InitLogrus(cfg)
	logger.Info("starting service")
	logger.Infof("%+v", cfg)

	metrics := config.InitMetrics(cfg.App.PromServeAddr)

	svc := NewService(cfg, logger, metrics)

	svc.Run()
}
