package repository

import (
	"context"
	"github.com/emortalmc/proto-specs/gen/go/model/common"
	"github.com/google/uuid"
	"mc-player-service/internal/repository/model"
	"time"
)

type Repository interface {
	GetPlayer(ctx context.Context, id uuid.UUID) (*model.Player, error)
	GetPlayers(ctx context.Context, ids []uuid.UUID) ([]*model.Player, error)
	SavePlayerWithUpsert(ctx context.Context, player *model.Player) error
	GetPlayerByUsername(ctx context.Context, username string, ignoreCase bool) (*model.Player, error)
	SearchPlayersByUsername(ctx context.Context, username string, pageable *common.Pageable, filter UsernameSearchFilter) ([]*model.Player, *common.PageData, error)

	CreateLoginSession(ctx context.Context, session *model.LoginSession) error
	SetLoginSessionLogoutTime(ctx context.Context, playerId uuid.UUID, logoutTime time.Time) error
	GetCurrentLoginSession(ctx context.Context, playerId uuid.UUID) (*model.LoginSession, error)
	GetLoginSessions(ctx context.Context, playerId uuid.UUID, pageable *common.Pageable) ([]*model.LoginSession, error)

	CreatePlayerUsername(ctx context.Context, username *model.PlayerUsername) error

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
