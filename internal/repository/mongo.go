package repository

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsoncodec"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"mc-player-service/internal/config"
	"mc-player-service/internal/repository/registrytypes"
)

const (
	databaseName = "mc-player-service"

	playerCollectionName   = "player"
	sessionCollectionName  = "loginSession"
	usernameCollectionName = "playerUsername"
)

type mongoRepository struct {
	database *mongo.Database

	playerCollection   *mongo.Collection
	sessionCollection  *mongo.Collection
	usernameCollection *mongo.Collection
}

func NewMongoRepository(ctx context.Context, cfg *config.MongoDBConfig) (Repository, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.URI).SetRegistry(createCodecRegistry()))
	if err != nil {
		return nil, err
	}

	database := client.Database(databaseName)
	return &mongoRepository{
		database:           database,
		playerCollection:   database.Collection(playerCollectionName),
		sessionCollection:  database.Collection(sessionCollectionName),
		usernameCollection: database.Collection(usernameCollectionName),
	}, nil
}

func createCodecRegistry() *bsoncodec.Registry {
	return bson.NewRegistryBuilder().
		RegisterTypeEncoder(registrytypes.UUIDType, bsoncodec.ValueEncoderFunc(registrytypes.UuidEncodeValue)).
		RegisterTypeDecoder(registrytypes.UUIDType, bsoncodec.ValueDecoderFunc(registrytypes.UuidDecodeValue)).
		Build()
}
