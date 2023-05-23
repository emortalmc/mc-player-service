package app

import (
	"context"
	"go.uber.org/zap"
	"mc-player-service/internal/badge"
	"mc-player-service/internal/config"
	"mc-player-service/internal/kafka"
	"mc-player-service/internal/repository"
	"mc-player-service/internal/service"
	"os/signal"
	"sync"
	"syscall"
)

func Run(cfg *config.Config, logger *zap.SugaredLogger) {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	wg := &sync.WaitGroup{}

	badgeCfg, err := config.LoadBadgeConfig()
	if err != nil {
		logger.Fatalw("failed to load badge config", err)
	}
	logger.Infow("loaded badge config", "badgeCount", len(badgeCfg.Badges))

	repoWg := &sync.WaitGroup{}
	repoCtx, repoCancel := context.WithCancel(ctx)

	repo, err := repository.NewMongoRepository(repoCtx, logger, repoWg, cfg.MongoDB)
	if err != nil {
		logger.Fatalw("failed to create repository", err)
	}

	badgeHandler := badge.NewBadgeHandler(logger, repo, badgeCfg)

	kafka.NewConsumer(ctx, wg, cfg.Kafka, logger, repo, badgeHandler, badgeCfg)

	service.RunServices(ctx, logger, wg, cfg, badgeHandler, badgeCfg, repo)

	wg.Wait()
	logger.Info("shutting down")

	logger.Info("shutting down repository")
	repoCancel()
	repoWg.Wait()
}
