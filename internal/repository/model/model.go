package model

import (
	commonmodel "github.com/emortalmc/proto-specs/gen/go/model/common"
	"github.com/emortalmc/proto-specs/gen/go/model/mcplayer"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"time"
)

type Player struct {
	Id              uuid.UUID   `bson:"_id"`
	CurrentUsername string      `bson:"currentUsername"`
	CurrentSkin     *PlayerSkin `bson:"currentSkin,omitempty"` // Only null as it didn't use to be stored

	FirstLogin time.Time `bson:"firstLogin"`

	// LastOnline this is only updated when the player logs out/in. If the player is CurrentlyOnline, trust that LastOnline is time.Now()
	LastOnline    time.Time     `bson:"lastOnline"`
	TotalPlaytime time.Duration `bson:"totalPlaytime"`

	// Badges IDs of the badges the player has
	Badges []string `bson:"badges,omitempty"`

	// ActiveBadge ID of the badge the player has currently active (nil if none)
	ActiveBadge *string `bson:"activeBadge,omitempty"`

	CurrentServer *CurrentServer `bson:"currentServer,omitempty"`
}

func (p *Player) ToProto(session *LoginSession) *mcplayer.McPlayer {
	return &mcplayer.McPlayer{
		Id:               p.Id.String(),
		CurrentUsername:  p.CurrentUsername,
		FirstLogin:       timestamppb.New(p.FirstLogin),
		LastOnline:       timestamppb.New(p.LastOnline),
		CurrentlyOnline:  p.CurrentServer != nil,
		CurrentSession:   session.ToProto(),
		HistoricPlayTime: durationpb.New(p.TotalPlaytime),
		CurrentServer:    p.CurrentServer.ToProto(),
		CurrentSkin:      p.CurrentSkin.ToProto(),
	}
}

var OnlinePlayerProjection = map[string]interface{}{
	"_id":             1,
	"currentUsername": 1,
	"currentServer":   1,
}

// OnlinePlayer a partial player object that is used
// by the PlayerTracker to track online players
type OnlinePlayer struct {
	Id              uuid.UUID `bson:"_id"`
	CurrentUsername string    `bson:"currentUsername"`

	CurrentServer *CurrentServer `bson:"currentServer,omitempty"`
}

func (p *OnlinePlayer) ToProto() *mcplayer.OnlinePlayer {
	return &mcplayer.OnlinePlayer{
		PlayerId: p.Id.String(),
		Username: p.CurrentUsername,
		Server:   p.CurrentServer.ToProto(),
	}
}

type CurrentServer struct {
	ServerId  string `bson:"serverId"`
	ProxyId   string `bson:"proxyId"`
	FleetName string `bson:"fleetName"`
}

func (s *CurrentServer) ToProto() *mcplayer.CurrentServer {
	if s == nil {
		return nil
	}

	return &mcplayer.CurrentServer{
		ServerId: s.ServerId,
		ProxyId:  s.ProxyId,
	}
}

type PlayerSkin struct {
	Texture   string `bson:"texture"`
	Signature string `bson:"signature"`
}

func PlayerSkinFromProto(s *commonmodel.PlayerSkin) *PlayerSkin {
	return &PlayerSkin{
		Texture:   s.Texture,
		Signature: s.Signature,
	}
}

func (s *PlayerSkin) ToProto() *commonmodel.PlayerSkin {
	if s == nil {
		return nil
	}

	return &commonmodel.PlayerSkin{
		Texture:   s.Texture,
		Signature: s.Signature,
	}
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
	if s == nil {
		return nil
	}

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
