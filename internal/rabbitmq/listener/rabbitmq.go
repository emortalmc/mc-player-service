package listener

import (
	"context"
	"github.com/emortalmc/proto-specs/gen/go/message/common"
	"github.com/google/uuid"
	"github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"mc-player-service/internal/repository"
	"mc-player-service/internal/repository/model"
	"time"
)

const (
	queueName = "mc-player:all"

	connectType    = "emortal.message.PlayerConnectMessage"
	disconnectType = "emortal.message.PlayerDisconnectMessage"
)

type rabbitMqListener struct {
	logger *zap.SugaredLogger
	repo   repository.Repository
	chann  *amqp091.Channel
}

func NewRabbitMQListener(logger *zap.SugaredLogger, repo repository.Repository, conn *amqp091.Connection) error {
	channel, err := conn.Channel()
	if err != nil {
		return err
	}

	msgChan, err := channel.Consume(queueName, "", false, false, false, false, amqp091.Table{})
	if err != nil {
		return err
	}

	listener := rabbitMqListener{
		logger: logger,
		repo:   repo,
		chann:  channel,
	}

	// Run as goroutine as it is blocking
	go listener.listen(msgChan)

	return nil
}

func (l *rabbitMqListener) listen(msgChan <-chan amqp091.Delivery) {
	for d := range msgChan {
		success := true

		switch d.Type {
		case connectType:
			msg := &common.PlayerConnectMessage{}
			err := proto.Unmarshal(d.Body, msg)
			if err != nil {
				l.logger.Errorw("error unmarshaling PlayerConnectMessage", err)
			}

			err = l.handlePlayerConnect(msg)
			if err != nil {
				success = false
			}
		case disconnectType:
			msg := &common.PlayerDisconnectMessage{}

			err := proto.Unmarshal(d.Body, msg)
			if err != nil {
				l.logger.Errorw("error unmarshaling PlayerDisconnectMessage", err)
			}

			err = l.handlePlayerDisconnect(msg)
			if err != nil {
				success = false
			}
		default:
			l.logger.Errorw("unknown message type", "type", d.Type)
		}
		if success {
			err := l.chann.Ack(d.DeliveryTag, false)
			if err != nil {
				l.logger.Errorw("error acknowledging message", err)
			}
		}
	}
}

func (l *rabbitMqListener) handlePlayerConnect(message *common.PlayerConnectMessage) error {
	pId, err := uuid.Parse(message.PlayerId)
	if err != nil {
		l.logger.Errorw("error parsing player id", err)
		return err
	}

	p, err := l.repo.GetPlayer(context.TODO(), pId)
	updatedUsername := false

	if err != nil && err == mongo.ErrNoDocuments {
		p = &model.Player{
			Id:              pId,
			CurrentUsername: message.PlayerUsername,
			FirstLogin:      time.Now(),
			LastOnline:      time.Now(),
			TotalPlaytime:   0,
			CurrentlyOnline: true,
		}
		updatedUsername = true
	} else if err != nil {
		l.logger.Errorw("error getting player", err)
		return err
	} else {
		if p.CurrentUsername != message.PlayerUsername {
			p.CurrentUsername = message.PlayerUsername
			updatedUsername = true
		}
		p.CurrentlyOnline = true
	}

	err = l.repo.SavePlayerWithUpsert(context.TODO(), p)
	if err != nil {
		l.logger.Errorw("error saving player", err)
		return err
	}

	session := &model.LoginSession{
		Id:       primitive.NewObjectIDFromTimestamp(message.Timestamp.AsTime()),
		PlayerId: pId,
	}

	err = l.repo.CreateLoginSession(context.TODO(), session)
	if err != nil {
		l.logger.Errorw("error creating login session", err)
		return err
	}

	if updatedUsername {
		dbUsername := &model.PlayerUsername{
			Id:       primitive.NewObjectIDFromTimestamp(message.Timestamp.AsTime()),
			PlayerId: pId,
			Username: message.PlayerUsername,
		}

		err = l.repo.CreatePlayerUsername(context.TODO(), dbUsername)
		if err != nil {
			l.logger.Errorw("error creating player username", err)
			return err
		}
	}

	return nil
}

func (l *rabbitMqListener) handlePlayerDisconnect(message *common.PlayerDisconnectMessage) error {
	pId, err := uuid.Parse(message.PlayerId)
	if err != nil {
		l.logger.Errorw("error parsing player id", err)
		return err
	}

	logoutTime := message.Timestamp.AsTime()

	s, err := l.repo.GetCurrentLoginSession(context.TODO(), pId)
	if err != nil {
		l.logger.Errorw("error getting current login session", err)
		return err
	}

	s.LogoutTime = &logoutTime

	err = l.repo.UpdateLoginSession(context.TODO(), s)
	if err != nil {
		l.logger.Errorw("error updating login session", err)
		return err
	}

	p, err := l.repo.GetPlayer(context.TODO(), pId)
	if err != nil {
		l.logger.Errorw("error getting player", err)
		return err
	}

	p.CurrentlyOnline = false
	p.TotalPlaytime += s.GetDuration()
	p.LastOnline = logoutTime

	err = l.repo.SavePlayerWithUpsert(context.TODO(), p)
	if err != nil {
		l.logger.Errorw("error saving player", err)
		return err
	}

	return nil
}
