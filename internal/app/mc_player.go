package app

import (
	"context"
	"fmt"
	"github.com/emortalmc/proto-specs/gen/go/grpc/badge"
	"github.com/emortalmc/proto-specs/gen/go/grpc/mcplayer"
	grpczap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"mc-player-service/internal/config"
	"mc-player-service/internal/kafka"
	"mc-player-service/internal/repository"
	"mc-player-service/internal/service"
	"net"
)

func Run(ctx context.Context, cfg *config.Config, logger *zap.SugaredLogger) {
	badgeCfg, err := config.LoadBadgeConfig()
	if err != nil {
		logger.Fatalw("failed to load badge config", err)
	}
	logger.Infow("loaded badge config", "badgeCount", len(badgeCfg.Badges))

	repo, err := repository.NewMongoRepository(ctx, cfg.MongoDB)
	if err != nil {
		logger.Fatalw("failed to create repository", err)
	}

	kafka.NewConsumer(cfg.Kafka, logger, repo, badgeCfg)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		logger.Fatalw("failed to listen", err)
	}

	s := grpc.NewServer(grpc.ChainUnaryInterceptor(
		grpczap.UnaryServerInterceptor(logger.Desugar(), grpczap.WithLevels(func(code codes.Code) zapcore.Level {
			if code != codes.Internal && code != codes.Unavailable && code != codes.Unknown {
				return zapcore.DebugLevel
			} else {
				return zapcore.ErrorLevel
			}
		})),
	))
	mcplayer.RegisterMcPlayerServer(s, service.NewMcPlayerService(repo))
	badge.RegisterBadgeManagerServer(s, service.NewBadgeService(repo, badgeCfg))
	logger.Infow("listening for gRPC requests", "port", cfg.Port)

	err = s.Serve(lis)
	if err != nil {
		logger.Fatalw("failed to serve", err)
	}
}
