package repository

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsoncodec"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
	"mc-player-service/internal/config"
	"mc-player-service/internal/repository/registrytypes"
	"sync"
	"time"
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

func NewMongoRepository(ctx context.Context, logger *zap.SugaredLogger, wg *sync.WaitGroup, cfg *config.MongoDBConfig) (Repository, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.URI).SetRegistry(createCodecRegistry()))
	if err != nil {
		return nil, err
	}

	database := client.Database(databaseName)
	repo := &mongoRepository{
		database:           database,
		playerCollection:   database.Collection(playerCollectionName),
		sessionCollection:  database.Collection(sessionCollectionName),
		usernameCollection: database.Collection(usernameCollectionName),
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		if err := client.Disconnect(ctx); err != nil {
			logger.Errorw("failed to disconnect from mongo", err)
		}
	}()

	repo.createIndexes(ctx)
	logger.Infow("created mongo indexes")

	return repo, nil
}

var (
	playerIndexes = []mongo.IndexModel{
		{ // Allows for username search
			Keys:    bson.M{"currentUsername": "text"},
			Options: options.Index().SetName("currentUsername_text"),
		},
		{ // Allows for case-insensitive matching
			Keys: bson.M{"currentUsername": 1},
			Options: options.Index().
				SetCollation(&options.Collation{Strength: 1, Locale: "en"}).
				SetName("currentUsername_ignoreCase"),
		},
		{ // Regular matching
			Keys:    bson.M{"currentUsername": 1},
			Options: options.Index().SetName("currentUsername"),
		},

		// Player tracking
		{
			Keys:    bson.M{"currentServer.serverId": 1},
			Options: options.Index().SetName("currentServer_serverId"),
		},
		{
			Keys:    bson.M{"currentServer.fleetName": 1},
			Options: options.Index().SetName("currentServer_fleetName"),
		},
	}

	sessionIndexes = []mongo.IndexModel{
		{
			Keys:    bson.M{"playerId": 1},
			Options: options.Index().SetName("playerId"),
		},
		{
			Keys:    bson.D{{Key: "playerId", Value: 1}, {Key: "logoutTime", Value: 1}},
			Options: options.Index().SetName("playerId_logoutTime"),
		},
	}

	usernameIndexes = []mongo.IndexModel{
		{
			Keys:    bson.M{"username": 1},
			Options: options.Index().SetName("username"),
		},
		{
			Keys:    bson.M{"playerId": 1},
			Options: options.Index().SetName("playerId"),
		},
	}
)

func (m *mongoRepository) createIndexes(ctx context.Context) {
	collIndexes := map[*mongo.Collection][]mongo.IndexModel{
		m.playerCollection:   playerIndexes,
		m.sessionCollection:  sessionIndexes,
		m.usernameCollection: usernameIndexes,
	}

	wg := sync.WaitGroup{}
	wg.Add(len(collIndexes))

	for coll, indexes := range collIndexes {
		go func(coll *mongo.Collection, indexes []mongo.IndexModel) {
			defer wg.Done()
			_, err := m.createCollIndexes(ctx, coll, indexes)
			if err != nil {
				panic(fmt.Sprintf("failed to create indexes for collection %s: %s", coll.Name(), err))
			}
		}(coll, indexes)
	}

	wg.Wait()
}

func (m *mongoRepository) createCollIndexes(ctx context.Context, coll *mongo.Collection, indexes []mongo.IndexModel) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := coll.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return 0, err
	}

	return len(result), nil
}

func createCodecRegistry() *bsoncodec.Registry {
	return bson.NewRegistryBuilder().
		RegisterTypeEncoder(registrytypes.UUIDType, bsoncodec.ValueEncoderFunc(registrytypes.UuidEncodeValue)).
		RegisterTypeDecoder(registrytypes.UUIDType, bsoncodec.ValueDecoderFunc(registrytypes.UuidDecodeValue)).
		Build()
}
