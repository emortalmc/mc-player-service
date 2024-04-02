package main

import (
	"fmt"
	"go.uber.org/zap"
	"log"
	"mc-player-service/internal/app"
	"mc-player-service/internal/config"
)

func main() {
	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		panic(fmt.Sprintf("failed to load config: %v", err))
	}

	unsugared, err := createLogger(cfg)
	if err != nil {
		log.Fatal(err)
	}
	log := unsugared.Sugar()

	app.Run(cfg, log)
}

func createLogger(cfg config.Config) (log *zap.Logger, err error) {
	if cfg.Development {
		log, err = zap.NewDevelopment()
	} else {
		log, err = zap.NewProduction()
	}
	return
}
