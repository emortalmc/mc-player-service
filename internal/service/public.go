package service

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
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	badgeh "mc-player-service/internal/badge"
	"mc-player-service/internal/config"
	"mc-player-service/internal/healthprovider"
	"mc-player-service/internal/repository"
	"net"
	"sync"
)

func RunServices(ctx context.Context, logger *zap.SugaredLogger, wg *sync.WaitGroup, cfg *config.Config,
	badgeH badgeh.Handler, badgeCfg *config.BadgeConfig, repo repository.Repository) {

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

	if cfg.Development {
		reflection.Register(s)
	}

	healthSrv := healthprovider.Create(ctx, repo)

	grpc_health_v1.RegisterHealthServer(s, healthSrv)
	mcplayer.RegisterMcPlayerServer(s, newMcPlayerService(repo))
	badge.RegisterBadgeManagerServer(s, newBadgeService(repo, badgeH, badgeCfg))
	mcplayer.RegisterPlayerTrackerServer(s, newPlayerTrackerService(repo))
	logger.Infow("listening for gRPC requests", "port", cfg.Port)

	go func() {
		if err := s.Serve(lis); err != nil {
			logger.Fatalw("failed to serve", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		s.GracefulStop()
	}()
}
