package app

import (
	"context"
	"go.uber.org/zap"
	"mc-player-service/internal/app/badge"
	"mc-player-service/internal/app/player"
	"mc-player-service/internal/config"
	"mc-player-service/internal/grpc"
	"mc-player-service/internal/kafka"
	"mc-player-service/internal/repository"
	"os/signal"
	"sync"
	"syscall"
)

func Run(cfg config.Config, log *zap.SugaredLogger) {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	wg := &sync.WaitGroup{}

	badgeCfg, err := config.LoadBadgeConfig()
	if err != nil {
		log.Fatalw("failed to load badge config", err)
	}
	log.Infow("loaded badge config", "badgeCount", len(badgeCfg.Badges))

	repoWg := &sync.WaitGroup{}
	repoCtx, repoCancel := context.WithCancel(ctx)

	repo, err := repository.NewMongoRepository(repoCtx, log, repoWg, cfg.MongoDB)
	if err != nil {
		log.Fatalw("failed to create repository", err)
	}

	badgeSvc := badge.NewService(log, repo, repo, badgeCfg)
	playerSvc := player.NewService(log, repo, cfg)

	kafka.NewConsumer(ctx, wg, cfg, log, repo, badgeSvc, playerSvc)

	grpc.RunServices(ctx, log, wg, cfg, badgeSvc, badgeCfg, repo)

	wg.Wait()
	log.Info("shutting down")

	log.Info("shutting down repository")
	repoCancel()
	repoWg.Wait()
}
