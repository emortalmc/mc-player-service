package repository

import (
	"context"
	"github.com/emortalmc/proto-specs/gen/go/model/common"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"math"
	"mc-player-service/internal/repository/model"
	"time"
)

func (m *mongoRepository) PlayerLogout(ctx context.Context, playerId uuid.UUID, lastOnline time.Time, addedPlaytime time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	res, err := m.playerCollection.UpdateByID(ctx, playerId, bson.M{
		"$unset": bson.M{"currentServer": ""},
		"$set":   bson.M{"lastOnline": lastOnline},
		"$inc":   bson.M{"totalPlaytime": addedPlaytime.Milliseconds()},
	})
	if err != nil {
		return err
	}

	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}

	return nil
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

func (m *mongoRepository) SavePlayer(ctx context.Context, player *model.Player, upsert bool) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := m.playerCollection.UpdateOne(ctx, bson.M{"_id": player.Id}, bson.M{"$set": player}, options.Update().SetUpsert(upsert))
	return err
}

func (m *mongoRepository) GetPlayerByUsername(ctx context.Context, username string, ignoreCase bool) (*model.Player, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := bson.M{"currentUsername": username}

	opts := options.FindOne()
	if ignoreCase {
		opts.SetCollation(&options.Collation{
			Locale:   "en",
			Strength: 1,
		})
	}

	var mongoResult *model.Player
	err := m.playerCollection.FindOne(ctx, query, opts).Decode(&mongoResult)
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
		queries = append(queries, bson.M{"currentServer": bson.M{"$exists": true}})
	}
	query := bson.M{"$and": queries}

	// todo friend filters

	page := int64(pageable.Page)
	skip := page * int64(*pageable.Size)

	var mongoResult []*model.Player
	cursor, err := m.playerCollection.Find(ctx, query, options.Find().SetSkip(skip).SetLimit(int64(*pageable.Size)))

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

	pageCount := uint64(math.Ceil(float64(total) / float64(*pageable.Size)))

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

func (m *mongoRepository) SetLoginSessionLogoutTime(ctx context.Context, playerId uuid.UUID, logoutTime time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := m.sessionCollection.UpdateOne(ctx, bson.M{"$and": []bson.M{
		{"playerId": playerId}, {"logoutTime": bson.M{"$exists": false}},
	}}, bson.M{"$set": bson.M{"logoutTime": logoutTime}})

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
	skip := (page - 1) * int64(*pageable.Size)

	cursor, err := m.sessionCollection.Find(ctx, bson.M{"playerId": playerId}, options.Find().SetSkip(skip).SetLimit(int64(*pageable.Size)))
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

func (m *mongoRepository) GetTotalUniquePlayers(ctx context.Context) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return m.playerCollection.CountDocuments(ctx, bson.M{})
}

var MillisInHour = time.Hour.Milliseconds()

func (m *mongoRepository) GetTotalPlaytimeHours(ctx context.Context) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	pipeline := []bson.M{
		{
			"$group": bson.M{
				"_id":   nil,
				"total": bson.M{"$sum": "$totalPlaytime"},
			},
		},
	}

	cursor, err := m.playerCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, err
	}

	var mongoResult []bson.M
	err = cursor.All(ctx, &mongoResult)
	if err != nil {
		return 0, err
	}

	if len(mongoResult) == 0 {
		return 0, nil
	}

	return mongoResult[0]["total"].(int64) / MillisInHour, nil
}
