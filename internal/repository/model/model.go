package model

import (
	"github.com/emortalmc/proto-specs/gen/go/model/mcplayer"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/protobuf/types/known/timestamppb"
	"time"
)

type Player struct {
	Id              uuid.UUID `bson:"_id"`
	CurrentUsername string    `bson:"currentUsername"`

	FirstLogin time.Time `bson:"firstLogin"`

	// LastOnline this is only updated when the player logs out/in. If the player is CurrentlyOnline, trust that LastOnline is time.Now()
	LastOnline    time.Time     `bson:"lastOnline"`
	TotalPlaytime time.Duration `bson:"totalPlaytime"`

	// Badges IDs of the badges the player has
	Badges []string `bson:"badges"`

	// ActiveBadge ID of the badge the player has currently active (nil if none)
	ActiveBadge *string `bson:"activeBadge,omitempty"`

	CurrentlyOnline bool `bson:"currentlyOnline"`
}

type LoginSession struct {
	Id       primitive.ObjectID `bson:"_id"`
	PlayerId uuid.UUID          `bson:"playerId"`

	LogoutTime *time.Time `bson:"logoutTime,omitempty"`
}

func (s *LoginSession) GetDuration() time.Duration {
	if s.LogoutTime == nil {
		return time.Since(s.Id.Timestamp())
	}

	return s.LogoutTime.Sub(s.Id.Timestamp())
}

func (s *LoginSession) ToProto() *mcplayer.LoginSession {
	proto := &mcplayer.LoginSession{
		SessionId: s.Id.String(),
		LoginTime: timestamppb.New(s.Id.Timestamp()),
	}

	if s.LogoutTime != nil {
		proto.LogoutTime = timestamppb.New(*s.LogoutTime)
	}

	return proto
}

type PlayerUsername struct {
	Id       primitive.ObjectID `bson:"_id"`
	PlayerId uuid.UUID          `bson:"playerId"`
	Username string             `bson:"username"`
}
