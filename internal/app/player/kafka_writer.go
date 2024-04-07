package player

import (
	"context"
	"github.com/google/uuid"
	kafkaWriter "mc-player-service/internal/kafka/writer"
)

var (
	_ KafkaWriter = &kafkaWriter.Notifier{}
)

type KafkaWriter interface {
	PlayerExperienceChange(ctx context.Context, playerID uuid.UUID, reason string, oldXP int, newXP int, oldLevel int, newLevel int)
}
