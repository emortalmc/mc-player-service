package repository

import (
	"context"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

func (m *mongoRepository) GetPlayerBadges(ctx context.Context, playerId uuid.UUID) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result := struct {
		Badges []string `bson:"badges"`
	}{}
	opts := options.FindOne().SetProjection(bson.M{"badges": 1})
	if err := m.playerCollection.FindOne(ctx, bson.M{"_id": playerId}, opts).Decode(&result); err != nil {
		return nil, err
	}

	return result.Badges, nil
}

func (m *mongoRepository) AddPlayerBadge(ctx context.Context, playerId uuid.UUID, badgeId string) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := m.playerCollection.UpdateOne(ctx, bson.M{"_id": playerId}, bson.M{"$addToSet": bson.M{"badges": badgeId}})
	if err != nil {
		return 0, err
	}

	return result.ModifiedCount, nil
}

func (m *mongoRepository) RemovePlayerBadge(ctx context.Context, playerId uuid.UUID, badgeId string) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := m.playerCollection.UpdateOne(ctx, bson.M{"_id": playerId}, bson.M{"$pull": bson.M{"badges": badgeId}})
	if err != nil {
		return 0, err
	}

	return result.ModifiedCount, nil
}

func (m *mongoRepository) GetActivePlayerBadge(ctx context.Context, playerId uuid.UUID) (*string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result := struct {
		ActiveBadge *string `bson:"activeBadge"`
	}{}

	opts := options.FindOne().SetProjection(bson.M{"activeBadge": 1})
	if err := m.playerCollection.FindOne(ctx, bson.M{"_id": playerId}, opts).Decode(&result); err != nil {
		return nil, err
	}

	return result.ActiveBadge, nil
}

func (m *mongoRepository) SetActivePlayerBadge(ctx context.Context, playerId uuid.UUID, badgeId string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := m.playerCollection.UpdateOne(ctx, bson.M{"_id": playerId}, bson.M{"$set": bson.M{"activeBadge": badgeId}})
	return err
}
