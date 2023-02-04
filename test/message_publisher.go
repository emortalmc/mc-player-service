package main

import (
	"context"
	"flag"
	"github.com/emortalmc/proto-specs/gen/go/message/common"
	"github.com/google/uuid"
	"github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"log"
	"math/rand"
	"mc-player-service/internal/config"
	"mc-player-service/internal/rabbitmq"
	"time"
)

const (
	connectMessageFlag    = "connect"
	disconnectMessageFlag = "disconnect"

	queueName = "mc-player:all"
)

var (
	connect = flag.Bool(connectMessageFlag, false, "publish a connect message")

	disconnect      = flag.Bool(disconnectMessageFlag, false, "publish a disconnect message")
	disconnectDelay = flag.Duration("disconnect-delay", 5*time.Second, "delay between connect and disconnect")

	period = flag.Duration("period", 1*time.Second, "period to publish messages")
	rate   = flag.Int("rate", 1, "number of messages to publish per period")
)

func main() {
	unsugared, err := zap.NewDevelopment()
	if err != nil {
		log.Fatal(err)
	}
	logger := unsugared.Sugar()

	cfg, err := config.LoadGlobalConfig()
	if err != nil {
		logger.Fatal("failed to load config", err)
	}

	conn, err := rabbitmq.NewConnection(cfg.RabbitMQ)
	if err != nil {
		logger.Fatal("failed to connect to rabbitmq", err)
	}
	defer func(conn *amqp091.Connection) {
		err := conn.Close()
		if err != nil {
			logger.Error("failed to close rabbitmq connection", err)
		}
	}(conn)

	ch, err := conn.Channel()
	if err != nil {
		logger.Fatal("failed to open channel", err)
	}

	flag.Parse()

	if !*connect && !*disconnect {
		logger.Fatal("must specify either ", connectMessageFlag, " or ", disconnectMessageFlag)
	}

	for tick := range time.Tick(*period) {
		for i := 0; i < *rate; i++ {
			pId := uuid.New().String()
			name := randomName()

			if *connect {
				err := publishConnectMessage(ch, pId, name)
				if err != nil {
					logger.Fatal("failed to publish connect message", err)
				}
			}

			if *disconnect {
				go func() {
					if *disconnectDelay > 0 {
						time.Sleep(*disconnectDelay)
					}

					err := publishDisconnectMessage(ch, pId)
					if err != nil {
						logger.Fatal("failed to publish disconnect message", err)
					}
				}()
			}
		}
		logger.Info("published ", *rate, " messages at ", tick)
	}

	select {}
}

func publishConnectMessage(ch *amqp091.Channel, pId string, username string) error {
	msg := &common.PlayerConnectMessage{
		PlayerId:       pId,
		ServerId:       "test-server-id",
		PlayerUsername: username,
	}

	body, err := proto.Marshal(msg)
	if err != nil {
		return err
	}

	err = ch.PublishWithContext(context.Background(), "", queueName, false, false, amqp091.Publishing{
		ContentType: "application/x-protobuf",
		Type:        string(msg.ProtoReflect().Descriptor().FullName()),
		Timestamp:   time.Now(),
		Body:        body,
	})
	return err
}

func publishDisconnectMessage(ch *amqp091.Channel, pId string) error {
	msg := &common.PlayerDisconnectMessage{
		PlayerId: pId,
	}

	body, err := proto.Marshal(msg)
	if err != nil {
		return err
	}

	err = ch.PublishWithContext(context.Background(), "", queueName, false, false, amqp091.Publishing{
		ContentType: "application/x-protobuf",
		Type:        string(msg.ProtoReflect().Descriptor().FullName()),
		Timestamp:   time.Now(),
		Body:        body,
	})
	return err
}

// randomName generated a random 10 char string, A-Z, a-z, 0-9
func randomName() string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]rune, 10)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
