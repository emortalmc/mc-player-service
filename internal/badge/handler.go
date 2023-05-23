// Package badge handles common methods interacting with badges
// where both the Kafka consumer and service need to make
// common calls
package badge

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"mc-player-service/internal/config"
	"mc-player-service/internal/repository"
)

// todo
type Handler interface {
	// AddBadgeToPlayer adds a badge to a player and updates
	// their active badge if necessary
	AddBadgeToPlayer(ctx context.Context, playerId uuid.UUID, badgeId string) error

	// RemoveBadgeFromPlayer removes a player's badge and updates
	// their active badge if necessary
	RemoveBadgeFromPlayer(ctx context.Context, playerId uuid.UUID, badgeId string) error

	// UpdateActiveBadge updates a player's active badge based on the badges
	// they own and their priority. If the player no longer owns their active badge
	// and none are required, the active badge will be set to nil
	UpdateActiveBadge(ctx context.Context, playerId uuid.UUID) error
}

type handlerImpl struct {
	logger *zap.SugaredLogger

	repo     repository.Repository
	badgeCfg *config.BadgeConfig
}

func NewBadgeHandler(logger *zap.SugaredLogger, repo repository.Repository, badgeCfg *config.BadgeConfig) Handler {
	return &handlerImpl{
		logger: logger,

		repo:     repo,
		badgeCfg: badgeCfg,
	}
}

var (
	AlreadyHasBadgeErr = errors.New("player already has this badge")
	DoesntExistErr     = errors.New("badge does not exist")
)

func (h *handlerImpl) AddBadgeToPlayer(ctx context.Context, playerId uuid.UUID, badgeId string) error {
	badge, ok := h.badgeCfg.Badges[badgeId]
	if !ok {
		return DoesntExistErr
	}

	modCount, err := h.repo.AddPlayerBadge(ctx, playerId, badgeId)
	if err != nil {
		return fmt.Errorf("failed to add player badge: %w", err)
	}

	if modCount == 0 {
		return AlreadyHasBadgeErr
	}

	if badge.Required {
		if err := h.UpdateActiveBadge(ctx, playerId); err != nil {
			return fmt.Errorf("failed to update active badge: %w", err)
		}
	}

	return nil
}

var (
	DoesntHaveBadgeErr = errors.New("player does not have this badge")
)

func (h *handlerImpl) RemoveBadgeFromPlayer(ctx context.Context, playerId uuid.UUID, badgeId string) error {
	player, err := h.repo.GetPlayer(ctx, playerId)
	if err != nil {
		return fmt.Errorf("failed to get player: %w", err)
	}

	removed := false
	for i, playerBadgeId := range player.Badges {
		if playerBadgeId == badgeId {
			player.Badges = append(player.Badges[:i], player.Badges[i+1:]...)
			removed = true
			break
		}
	}
	if !removed {
		return DoesntHaveBadgeErr
	}

	h.logger.Debugw("checking active badge", "activeBadge", player.ActiveBadge, "badgeId", badgeId)
	if player.ActiveBadge != nil && *player.ActiveBadge == badgeId {
		player.ActiveBadge = h.calculateActiveBadge(player.Badges)
	}

	h.logger.Debugw("Updating badges", "playerId", playerId, "badges", player.Badges, "activeBadge", player.ActiveBadge)
	if err := h.repo.UpdatePlayerBadgesAndActive(ctx, playerId, player.Badges, player.ActiveBadge); err != nil {
		return fmt.Errorf("failed to update player: %w", err)
	}

	return nil
}

func (h *handlerImpl) UpdateActiveBadge(ctx context.Context, playerId uuid.UUID) error {
	badgeIds, err := h.repo.GetPlayerBadges(ctx, playerId)
	if err != nil {
		return fmt.Errorf("failed to get player badges: %w", err)
	}

	highestBadgeId := h.calculateActiveBadge(badgeIds)

	if err := h.repo.SetActivePlayerBadge(ctx, playerId, highestBadgeId); err != nil {
		return fmt.Errorf("failed to set active badge: %w", err)
	}

	return nil
}

func (h *handlerImpl) calculateActiveBadge(badgeIds []string) *string {
	var highestBadgeId *string
	var highestBadgePriority int

	for _, badgeId := range badgeIds {
		badge, ok := h.badgeCfg.Badges[badgeId]
		if !ok {
			h.logger.Warnw("player has badge that does not exist", "badgeId", badgeId)
			continue
		}

		if badge.Priority > highestBadgePriority {
			highestBadgeId = &badgeId
			highestBadgePriority = badge.Priority
		}
	}

	h.logger.Debugw("Calculated active badge", "badgeIds", badgeIds, "activeBadge", highestBadgeId)

	return highestBadgeId
}
