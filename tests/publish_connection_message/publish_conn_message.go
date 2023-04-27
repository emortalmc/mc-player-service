package main

import (
	"context"
	"github.com/emortalmc/proto-specs/gen/go/message/common"
	"github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"
	"log"
	"time"
)

func main() {
	w := &kafka.Writer{
		Addr:      kafka.TCP("localhost:9092"),
		Topic:     "mc-connections",
		BatchSize: 1,
		Async:     false,
	}

	message := &common.PlayerConnectMessage{
		PlayerId:       "8d36737e-1c0a-4a71-87de-9906f577845e",
		PlayerUsername: "Expectational",
		ServerId:       "test-server-wstg",
	}

	bytes, err := proto.Marshal(message)
	if err != nil {
		panic(err)
	}

	err = w.WriteMessages(context.Background(), kafka.Message{
		Key:        nil,
		Value:      bytes,
		Headers:    []kafka.Header{{Key: "X-Proto-Type", Value: []byte(message.ProtoReflect().Descriptor().FullName())}},
		WriterData: nil,
		Time:       time.Time{},
	})

	if err != nil {
		panic(err)
	}
	log.Println("Message sent!")
}
