package player

import (
	"context"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"mc-player-service/internal/config"
	"mc-player-service/internal/repository"
	"mc-player-service/internal/repository/model"
	"mc-player-service/internal/webhook"
	"time"
)

type Service interface {
	HandlePlayerConnect(ctx context.Context, time time.Time, playerID uuid.UUID, playerUsername string,
		proxyID string,	playerSkin model.PlayerSkin, player model.Player)
	HandlePlayerDisconnect(ctx context.Context, time time.Time, playerID uuid.UUID, playerUsername string)
	HandlePlayerServerSwitch(ctx context.Context, pID uuid.UUID, newServerID string)
}

type serviceImpl struct {
	log *zap.SugaredLogger

	repo repository.PlayerReadWriter
	webhook webhook.Webhook
}

func NewService(log *zap.SugaredLogger, repo repository.PlayerReadWriter, cfg config.Config) Service {
	return &serviceImpl{
		log: log,
		repo: repo,
		webhook: webhook.NewWebhook(cfg.DiscordWebhookUrl, log),
	}
}
