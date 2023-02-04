package app

import (
	"context"
	"fmt"
	"github.com/emortalmc/proto-specs/gen/go/grpc/mcplayer"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"mc-player-service/internal/config"
	"mc-player-service/internal/rabbitmq"
	"mc-player-service/internal/rabbitmq/listener"
	"mc-player-service/internal/repository"
	"mc-player-service/internal/service"
	"net"
)

func Run(ctx context.Context, cfg *config.Config, logger *zap.SugaredLogger) {
	repo, err := repository.NewMongoRepository(ctx, cfg.MongoDB)
	if err != nil {
		logger.Fatalw("failed to create repository", "error", err)
	}

	// NOTE: We can share a RabbitMQ connection, but it is not recommended to share a channel
	rabbitConn, err := rabbitmq.NewConnection(cfg.RabbitMQ)
	if err != nil {
		logger.Fatalw("failed to create rabbitmq connection", "error", err)
	}

	err = listener.NewRabbitMQListener(logger, repo, rabbitConn)
	if err != nil {
		logger.Fatalw("failed to create rabbitmq listener", "error", err)
	}
	logger.Infow("connected to RabbitMQ", "host", cfg.RabbitMQ.Host)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		logger.Fatalw("failed to listen", "error", err)
	}

	s := grpc.NewServer()
	mcplayer.RegisterMcPlayerServer(s, service.NewMcPlayerService(repo))
	logger.Infow("listening for gRPC requests", "port", cfg.Port)

	err = s.Serve(lis)
	if err != nil {
		logger.Fatalw("failed to serve", "error", err)
	}
}
