package kafka

import (
	"context"
	"fmt"
	"github.com/emortalmc/proto-specs/gen/go/message/common"
	permissionmsg "github.com/emortalmc/proto-specs/gen/go/message/permission"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"mc-player-service/internal/config"
	"mc-player-service/internal/repository"
	"mc-player-service/internal/repository/model"
)

const connectionsTopic = "mc-connections"
const permissionsTopic = "permissions"

type consumer struct {
	logger *zap.SugaredLogger
	repo   repository.Repository

	badges map[string]*config.Badge

	connectionsReader *kafka.Reader
	permissionsReader *kafka.Reader
}

func NewConsumer(config *config.KafkaConfig, logger *zap.SugaredLogger, repo repository.Repository,
	badgeCfg *config.BadgeConfig) {

	connectionsReader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{fmt.Sprintf("%s:%d", config.Host, config.Port)},
		GroupID: "matchmaker",
		Topic:   connectionsTopic,
	})

	permissionsReader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{fmt.Sprintf("%s:%d", config.Host, config.Port)},
		GroupID: "matchmaker",
		Topic:   permissionsTopic,
	})

	c := &consumer{
		logger: logger,
		repo:   repo,

		badges: badgeCfg.Badges,

		connectionsReader: connectionsReader,
		permissionsReader: permissionsReader,
	}

	logger.Infow("listening for connections", "topic", connectionsTopic)
	go c.consumeConnections()
	logger.Infow("listening for permission changes", "topic", connectionsTopic)
	go c.consumerPermissions()
}

func (c *consumer) consumeConnections() {
	for {
		m, err := c.connectionsReader.ReadMessage(context.TODO())
		if err != nil {
			c.logger.Errorw("failed to read message", "error", err)
			continue
		}

		protoType, err := parseProtoType(m.Headers)
		if err != nil {
			c.logger.Errorw("failed to parse proto type", "error", err, "offset", m.Offset, "headers", m.Headers)
			continue
		}

		switch protoType {
		case string((&common.PlayerConnectMessage{}).ProtoReflect().Descriptor().FullName()):
			parsedMsg := &common.PlayerConnectMessage{}

			err = proto.Unmarshal(m.Value, parsedMsg)
			if err != nil {
				err = fmt.Errorf("failed to unmarshal message: %w", err)
				break
			}

			err = c.handlePlayerConnectMessage(&m, parsedMsg)
		case string((&common.PlayerDisconnectMessage{}).ProtoReflect().Descriptor().FullName()):
			parsedMsg := &common.PlayerDisconnectMessage{}

			err = proto.Unmarshal(m.Value, parsedMsg)
			if err != nil {
				err = fmt.Errorf("failed to unmarshal message: %w", err)
				break
			}

			err = c.handlePlayerDisconnectMessage(&m, parsedMsg)
		default:
			return
		}

		if err != nil {
			c.logger.Errorw("failed to handle message",
				"error", err,
				"type", protoType,
				"offset", m.Offset,
				"headers", m.Headers,
			)
		}
	}
}

func (c *consumer) handlePlayerConnectMessage(kafkaM *kafka.Message, m *common.PlayerConnectMessage) error {
	pId, err := uuid.Parse(m.PlayerId)
	if err != nil {
		return fmt.Errorf("error parsing player id: %w", err)
	}

	p, err := c.repo.GetPlayer(context.TODO(), pId)
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
		return fmt.Errorf("error getting player: %w", err)
	} else {
		if p.CurrentUsername != m.PlayerUsername {
			p.CurrentUsername = m.PlayerUsername
			updatedUsername = true
		}
		p.CurrentlyOnline = true
	}

	err = c.repo.SavePlayerWithUpsert(context.TODO(), p)
	if err != nil {
		return fmt.Errorf("error saving player: %w", err)
	}

	session := &model.LoginSession{
		Id:       primitive.NewObjectIDFromTimestamp(kafkaM.Time),
		PlayerId: pId,
	}

	err = c.repo.CreateLoginSession(context.TODO(), session)
	if err != nil {
		return fmt.Errorf("error creating login session: %w", err)
	}

	if updatedUsername {
		dbUsername := &model.PlayerUsername{
			Id:       primitive.NewObjectIDFromTimestamp(kafkaM.Time),
			PlayerId: pId,
			Username: m.PlayerUsername,
		}

		err = c.repo.CreatePlayerUsername(context.TODO(), dbUsername)
		if err != nil {
			return fmt.Errorf("error creating player username: %w", err)
		}
	}

	return nil
}

func (c *consumer) handlePlayerDisconnectMessage(kafkaM *kafka.Message, m *common.PlayerDisconnectMessage) error {
	ctx := context.TODO()

	pId, err := uuid.Parse(m.PlayerId)
	if err != nil {
		return fmt.Errorf("error parsing player id: %w", err)
	}

	s, err := c.repo.GetCurrentLoginSession(ctx, pId)
	if err != nil {
		return fmt.Errorf("error getting current login session: %w", err)
	}

	err = c.repo.SetLoginSessionLogoutTime(ctx, pId, kafkaM.Time)
	if err != nil {
		return fmt.Errorf("error updating login session: %w", err)
	}

	p, err := c.repo.GetPlayer(ctx, pId)
	if err != nil {
		return fmt.Errorf("error getting player: %w", err)
	}

	p.CurrentlyOnline = false
	p.TotalPlaytime += s.GetDuration()
	p.LastOnline = kafkaM.Time

	err = c.repo.SavePlayerWithUpsert(ctx, p)
	if err != nil {
		return fmt.Errorf("error saving player: %w", err)
	}

	return nil
}

func (c *consumer) consumerPermissions() {
	for {
		m, err := c.permissionsReader.ReadMessage(context.TODO())
		if err != nil {
			c.logger.Errorw("failed to read message", "error", err)
			continue
		}

		protoType, err := parseProtoType(m.Headers)
		if err != nil {
			c.logger.Errorw("failed to parse proto type", "error", err, "offset", m.Offset, "headers", m.Headers)
			continue
		}

		switch protoType {
		case string((&permissionmsg.PlayerRolesUpdateMessage{}).ProtoReflect().Descriptor().FullName()):
			parsedMsg := &permissionmsg.PlayerRolesUpdateMessage{}

			err = proto.Unmarshal(m.Value, parsedMsg)
			if err != nil {
				err = fmt.Errorf("failed to unmarshal message: %w", err)
				break
			}

			err = c.handlePlayerRolesUpdateMessage(parsedMsg)
		default:
			return
		}

		if err != nil {
			c.logger.Errorw("failed to handle message",
				"error", err,
				"type", protoType,
				"offset", m.Offset,
				"headers", m.Headers,
			)
		}
	}
}

// TODO handle a change if the role priority changes
func (c *consumer) handlePlayerRolesUpdateMessage(m *permissionmsg.PlayerRolesUpdateMessage) error {
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
		return fmt.Errorf("error parsing player id: %w", err)
	}

	switch m.ChangeType {
	case permissionmsg.PlayerRolesUpdateMessage_ADD:
		_, err = c.repo.AddPlayerBadge(context.TODO(), playerId, badge.Id)
	case permissionmsg.PlayerRolesUpdateMessage_REMOVE:
		_, err = c.repo.RemovePlayerBadge(context.TODO(), playerId, badge.Id)
	}
	if err != nil {
		return fmt.Errorf("error updating player badges (action: %s): %w", m.ChangeType, err)
	}

	return nil
}

func parseProtoType(headers []kafka.Header) (string, error) {
	for _, header := range headers {
		if header.Key == "X-Proto-Type" {
			return string(header.Value), nil
		}
	}
	return "", fmt.Errorf("no proto type found in message headers")
}
