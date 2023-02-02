package repository

import (
	"context"
	"github.com/emortalmc/proto-specs/gen/go/model/common"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsoncodec"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"math"
	"mc-player-service/internal/config"
	"mc-player-service/internal/repository/model"
	"mc-player-service/internal/repository/registrytypes"
	"time"
)

const (
	databaseName = "mc-player-service"

	playerCollectionName   = "player"
	sessionCollectionName  = "loginSession"
	usernameCollectionName = "playerUsername"
)

type mongoRepository struct {
	Repository
	database *mongo.Database

	playerCollection   *mongo.Collection
	sessionCollection  *mongo.Collection
	usernameCollection *mongo.Collection
}

func NewMongoRepository(ctx context.Context, cfg config.MongoDBConfig) (Repository, error) {
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

func (m *mongoRepository) GetPlayer(ctx context.Context, playerId uuid.UUID) (*model.Player, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var mongoResult model.Player
	err := m.playerCollection.FindOne(ctx, bson.M{"_id": playerId}).Decode(&mongoResult)
	if err != nil {
		return nil, err
	}

	return &mongoResult, nil
}

func (m *mongoRepository) GetPlayers(ctx context.Context, pIds []uuid.UUID) ([]*model.Player, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cursor, err := m.playerCollection.Find(ctx, bson.M{"_id": bson.M{"$in": pIds}})
	if err != nil {
		return nil, err
	}

	var mongoResult []*model.Player
	err = cursor.All(ctx, &mongoResult)
	if err != nil {
		return nil, err
	}

	return mongoResult, nil
}

func (m *mongoRepository) SavePlayerWithUpsert(ctx context.Context, player *model.Player) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := m.playerCollection.UpdateOne(ctx, bson.M{"_id": player.Id}, bson.M{"$set": player}, options.Update().SetUpsert(true))
	return err
}

func (m *mongoRepository) GetPlayerByUsername(ctx context.Context, username string, ignoreCase bool) (*model.Player, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var query bson.M
	if ignoreCase {
		query = bson.M{"currentUsername": primitive.Regex{
			Pattern: "^" + username,
			Options: "i",
		}}
	} else {
		query = bson.M{"currentUsername": username}
	}

	var mongoResult *model.Player
	err := m.playerCollection.FindOne(ctx, query).Decode(&mongoResult)
	if err != nil {
		return nil, err
	}
	return mongoResult, nil
}

func (m *mongoRepository) SearchPlayersByUsername(ctx context.Context, username string, pageable *common.Pageable, filter UsernameSearchFilter) ([]*model.Player, *common.PageData, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var queries []bson.M

	queries = append(queries, bson.M{"currentUsername": bson.M{"$regex": primitive.Regex{
		Pattern: "^" + username,
		Options: "i",
	}}})

	if filter.OnlineOnly {
		queries = append(queries, bson.M{"currentlyOnline": true})
	}
	query := bson.M{"$and": queries}

	// todo friend filters

	page := int64(pageable.Page)
	skip := (page - 1) * int64(pageable.Size)

	var mongoResult []*model.Player
	cursor, err := m.playerCollection.Find(ctx, query, options.Find().SetSkip(skip).SetLimit(int64(pageable.Size)))

	if err != nil {
		return nil, nil, err
	}

	err = cursor.All(ctx, &mongoResult)
	if err != nil {
		return nil, nil, err
	}

	total, err := m.playerCollection.CountDocuments(ctx, query)
	if err != nil {
		return nil, nil, err
	}

	pageCount := uint64(math.Ceil(float64(total) / float64(pageable.Size)))

	pageData := &common.PageData{
		Page:          uint64(page),
		Size:          uint64(len(mongoResult)),
		TotalElements: uint64(total),
		TotalPages:    pageCount,
	}

	return mongoResult, pageData, nil
}

func (m *mongoRepository) CreateLoginSession(ctx context.Context, session *model.LoginSession) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := m.sessionCollection.InsertOne(ctx, session)
	return err
}

func (m *mongoRepository) UpdateLoginSession(ctx context.Context, session *model.LoginSession) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := m.sessionCollection.UpdateByID(ctx, session.Id, bson.M{"$set": session})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (m *mongoRepository) GetCurrentLoginSession(ctx context.Context, playerId uuid.UUID) (*model.LoginSession, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var mongoResult model.LoginSession
	err := m.sessionCollection.FindOne(ctx, bson.M{"$and": []bson.M{
		{"playerId": playerId}, {"logoutTime": bson.M{"$exists": false}},
	}}).Decode(&mongoResult)
	if err != nil {
		return nil, err
	}
	return &mongoResult, nil
}

func (m *mongoRepository) GetLoginSessions(ctx context.Context, playerId uuid.UUID, pageable *common.Pageable) ([]*model.LoginSession, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	page := int64(pageable.Page)
	skip := (page - 1) * int64(pageable.Size)

	cursor, err := m.sessionCollection.Find(ctx, bson.M{"playerId": playerId}, options.Find().SetSkip(skip).SetLimit(int64(pageable.Size)))
	if err != nil {
		return nil, err
	}

	var mongoResult []*model.LoginSession
	err = cursor.All(ctx, &mongoResult)
	if err != nil {
		return nil, err
	}

	return mongoResult, nil
}

func (m *mongoRepository) CreatePlayerUsername(ctx context.Context, username *model.PlayerUsername) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := m.usernameCollection.InsertOne(ctx, username)
	return err
}

func createCodecRegistry() *bsoncodec.Registry {
	return bson.NewRegistryBuilder().
		RegisterTypeEncoder(registrytypes.UUIDType, bsoncodec.ValueEncoderFunc(registrytypes.UuidEncodeValue)).
		RegisterTypeDecoder(registrytypes.UUIDType, bsoncodec.ValueDecoderFunc(registrytypes.UuidDecodeValue)).
		Build()
}
