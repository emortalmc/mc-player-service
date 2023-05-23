package kafka

import (
	"context"
	"fmt"
	"github.com/emortalmc/proto-specs/gen/go/message/common"
	permmsg "github.com/emortalmc/proto-specs/gen/go/message/permission"
	"github.com/emortalmc/proto-specs/gen/go/nongenerated/kafkautils"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	badgeh "mc-player-service/internal/badge"
	"mc-player-service/internal/config"
	"mc-player-service/internal/repository"
	"mc-player-service/internal/repository/model"
	"sync"
	"time"
)

const connectionsTopic = "mc-connections"
const permissionsTopic = "permissions"

type consumer struct {
	logger *zap.SugaredLogger
	repo   repository.Repository
	badgeH badgeh.Handler

	badges map[string]*config.Badge

	reader *kafka.Reader
}

func NewConsumer(ctx context.Context, wg *sync.WaitGroup, config *config.KafkaConfig, logger *zap.SugaredLogger, repo repository.Repository,
	badgeH badgeh.Handler, badgeCfg *config.BadgeConfig) {

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{fmt.Sprintf("%s:%d", config.Host, config.Port)},
		GroupID:     "mc-player-service",
		GroupTopics: []string{connectionsTopic, permissionsTopic},

		Logger: kafka.LoggerFunc(func(format string, args ...interface{}) {
			logger.Infow(fmt.Sprintf(format, args...))
		}),
		ErrorLogger: kafka.LoggerFunc(func(format string, args ...interface{}) {
			logger.Errorw(fmt.Sprintf(format, args...))
		}),

		MaxWait: 5 * time.Second,
	})

	c := &consumer{
		logger: logger,
		repo:   repo,
		badgeH: badgeH,

		badges: badgeCfg.Badges,

		reader: reader,
	}

	handler := kafkautils.NewConsumerHandler(logger, reader)
	handler.RegisterHandler(&common.PlayerConnectMessage{}, c.handlePlayerConnectMessage)
	handler.RegisterHandler(&common.PlayerDisconnectMessage{}, c.handlePlayerDisconnectMessage)
	handler.RegisterHandler(&permmsg.PlayerRolesUpdateMessage{}, c.handlePlayerRolesUpdateMessage)

	logger.Infow("starting listening for kafka messages", "topics", reader.Config().GroupTopics)

	wg.Add(1)
	go func() {
		defer wg.Done()
		handler.Run(ctx) // Run is blocking until the context is cancelled
		if err := reader.Close(); err != nil {
			logger.Errorw("error closing kafka reader", "error", err)
		}
	}()
}

func (c *consumer) handlePlayerConnectMessage(ctx context.Context, kafkaM *kafka.Message, uncastMsg proto.Message) {
	m := uncastMsg.(*common.PlayerConnectMessage)

	pId, err := uuid.Parse(m.PlayerId)
	if err != nil {
		c.logger.Errorw("error parsing player id", "error", err)
		return
	}

	p, err := c.repo.GetPlayer(ctx, pId)
	updatedUsername := false

	if err != nil && err == mongo.ErrNoDocuments {
		p = &model.Player{
			Id:              pId,
			CurrentUsername: m.PlayerUsername,
			FirstLogin:      kafkaM.Time,
			LastOnline:      kafkaM.Time,
			TotalPlaytime:   0,
			CurrentlyOnline: true,
		}
		updatedUsername = true
	} else if err != nil {
		c.logger.Errorw("error getting player", "error", err)
		return
	} else {
		if p.CurrentUsername != m.PlayerUsername {
			p.CurrentUsername = m.PlayerUsername
			updatedUsername = true
		}
		p.CurrentlyOnline = true
	}

	err = c.repo.SavePlayerWithUpsert(ctx, p)
	if err != nil {
		c.logger.Errorw("error saving player", "error", err)
		return
	}

	session := &model.LoginSession{
		Id:       primitive.NewObjectIDFromTimestamp(kafkaM.Time),
		PlayerId: pId,
	}

	err = c.repo.CreateLoginSession(ctx, session)
	if err != nil {
		c.logger.Errorw("error creating login session", "error", err)
	}

	if updatedUsername {
		dbUsername := &model.PlayerUsername{
			Id:       primitive.NewObjectIDFromTimestamp(kafkaM.Time),
			PlayerId: pId,
			Username: m.PlayerUsername,
		}

		err = c.repo.CreatePlayerUsername(ctx, dbUsername)
		if err != nil {
			c.logger.Errorw("error creating player username", "error", err)
		}
	}
}

func (c *consumer) handlePlayerDisconnectMessage(ctx context.Context, kafkaMsg *kafka.Message, uncastMsg proto.Message) {
	m := uncastMsg.(*common.PlayerDisconnectMessage)

	pId, err := uuid.Parse(m.PlayerId)
	if err != nil {
		c.logger.Errorw("error parsing player id", "error", err)
		return
	}

	s, err := c.repo.GetCurrentLoginSession(ctx, pId)
	if err != nil {
		c.logger.Errorw("error getting current login session", "error", err)
		return
	}

	err = c.repo.SetLoginSessionLogoutTime(ctx, pId, kafkaMsg.Time)
	if err != nil {
		c.logger.Errorw("error setting logout time", "error", err)
		return
	}

	p, err := c.repo.GetPlayer(ctx, pId)
	if err != nil {
		c.logger.Errorw("error getting player", "error", err)
		return
	}

	p.CurrentlyOnline = false
	p.TotalPlaytime += s.GetDuration()
	p.LastOnline = kafkaMsg.Time

	if err := c.repo.SavePlayerWithUpsert(ctx, p); err != nil {
		c.logger.Errorw("error saving player", "error", err)
		return
	}
}

func (c *consumer) handlePlayerRolesUpdateMessage(ctx context.Context, _ *kafka.Message, uncastMsg proto.Message) {
	m := uncastMsg.(*permmsg.PlayerRolesUpdateMessage)
	roleId := m.RoleId

	var badge *config.Badge
	for id, b := range c.badges {
		if b.AutomaticGrants == nil || b.AutomaticGrants.PermissionRole == nil {
			continue
		}

		if *b.AutomaticGrants.PermissionRole == roleId {
			badge = c.badges[id]
			break
		}
	}

	playerId, err := uuid.Parse(m.PlayerId)
	if err != nil {
		c.logger.Errorw("error parsing player id", "error", err)
	}

	switch m.ChangeType {
	case permmsg.PlayerRolesUpdateMessage_ADD:
		err = c.badgeH.AddBadgeToPlayer(ctx, playerId, badge.Id)
	case permmsg.PlayerRolesUpdateMessage_REMOVE:
		err = c.badgeH.RemoveBadgeFromPlayer(ctx, playerId, badge.Id)
	}
	if err != nil {
		c.logger.Errorw("error updating player badges", "error", err)
	}
}
