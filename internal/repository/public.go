package repository

import (
	"context"
	"github.com/emortalmc/proto-specs/gen/go/model/common"
	"github.com/google/uuid"
	"mc-player-service/internal/repository/model"
	"time"
)

type Repository interface {
	Ping(ctx context.Context) error

	GetPlayer(ctx context.Context, id uuid.UUID) (*model.Player, error)
	GetPlayers(ctx context.Context, ids []uuid.UUID) ([]*model.Player, error)
	SavePlayer(ctx context.Context, player *model.Player, upsert bool) error
	GetPlayerByUsername(ctx context.Context, username string, ignoreCase bool) (*model.Player, error)
	SearchPlayersByUsername(ctx context.Context, username string, pageable *common.Pageable, filter *UsernameSearchFilter, ignoredPlayerIds []uuid.UUID) ([]*model.Player, *common.PageData, error)
	PlayerLogout(ctx context.Context, playerId uuid.UUID, lastOnline time.Time, addedPlaytime time.Duration) error

	CreateLoginSession(ctx context.Context, session *model.LoginSession) error
	SetLoginSessionLogoutTime(ctx context.Context, playerId uuid.UUID, logoutTime time.Time) error
	GetCurrentLoginSession(ctx context.Context, playerId uuid.UUID) (*model.LoginSession, error)
	GetLoginSessions(ctx context.Context, playerId uuid.UUID, pageable *common.Pageable) ([]*model.LoginSession, error)

	CreatePlayerUsername(ctx context.Context, username *model.PlayerUsername) error

	// Stats

	GetTotalUniquePlayers(ctx context.Context) (int64, error)
	GetTotalPlaytimeHours(ctx context.Context) (int64, error)

	// Player Tracker

	SetPlayerServerAndFleet(ctx context.Context, playerId uuid.UUID, serverId string, fleet string) error

	GetPlayerServers(ctx context.Context, playerId []uuid.UUID) (map[uuid.UUID]*model.CurrentServer, error)
	GetServerPlayers(ctx context.Context, serverId string) ([]*model.OnlinePlayer, error)

	// GetPlayerCount returns the number of players on:
	// 1. the given server if present
	// 2. the given fleets if present
	// 3. globally if neither are present
	GetPlayerCount(ctx context.Context, serverId *string, fleetNames []string) (int64, error)

	// GetOnlinePlayers functions the same as GetPlayerCount
	GetOnlinePlayers(ctx context.Context, serverId *string, fleetNames []string) ([]*model.OnlinePlayer, error)

	GetFleetPlayerCounts(ctx context.Context, fleetNames []string) (map[string]int64, error)

	// Badges

	UpdatePlayerBadgesAndActive(ctx context.Context, playerId uuid.UUID, badges []string, activeBadge *string) error
	GetPlayerBadges(ctx context.Context, playerId uuid.UUID) ([]string, error)
	AddPlayerBadge(ctx context.Context, playerId uuid.UUID, badgeId string) (int64, error)
	RemovePlayerBadge(ctx context.Context, playerId uuid.UUID, badgeId string) (int64, error)

	GetActivePlayerBadge(ctx context.Context, playerId uuid.UUID) (*string, error)
	SetActivePlayerBadge(ctx context.Context, playerId uuid.UUID, badgeId *string) error
}

type UsernameSearchFilter struct {
	OnlineOnly bool
	Friends    bool
}
