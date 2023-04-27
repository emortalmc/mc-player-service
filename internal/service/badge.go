package service

import (
	"context"
	pb "github.com/emortalmc/proto-specs/gen/go/grpc/badge"
	pbmodel "github.com/emortalmc/proto-specs/gen/go/model/badge"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"mc-player-service/internal/config"
	"mc-player-service/internal/repository"
)

type badgeService struct {
	pb.BadgeManagerServer

	repo     repository.Repository
	badgeCfg *config.BadgeConfig
}

func newBadgeService(repo repository.Repository, badgeCfg *config.BadgeConfig) pb.BadgeManagerServer {
	return &badgeService{
		repo:     repo,
		badgeCfg: badgeCfg,
	}
}

var setActivePlayerBadgeDoesntHaveBadgeErr = panicIfErr(status.New(codes.NotFound, "player does not have this badge").
	WithDetails(&pb.SetActivePlayerBadgeErrorResponse{Reason: pb.SetActivePlayerBadgeErrorResponse_PLAYER_DOESNT_HAVE_BADGE})).Err()

func (s *badgeService) SetActivePlayerBadge(ctx context.Context, request *pb.SetActivePlayerBadgeRequest) (*pb.SetActivePlayerBadgeResponse, error) {
	playerId, err := uuid.Parse(request.PlayerId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid player_id")
	}

	badgeIds, err := s.repo.GetPlayerBadges(ctx, playerId)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get player badges")
	}

	for _, badgeId := range badgeIds {
		if badgeId == request.BadgeId {
			if err := s.repo.SetActivePlayerBadge(ctx, playerId, request.BadgeId); err != nil {
				return nil, status.Error(codes.Internal, "failed to set active badge")
			}

			return &pb.SetActivePlayerBadgeResponse{}, nil
		}
	}

	return nil, setActivePlayerBadgeDoesntHaveBadgeErr
}

var removeBadgeFromPlayerDoesntHaveBadgeErr = panicIfErr(status.New(codes.NotFound, "player does not have this badge").
	WithDetails(&pb.RemoveBadgeFromPlayerErrorResponse{Reason: pb.RemoveBadgeFromPlayerErrorResponse_PLAYER_DOESNT_HAVE_BADGE})).Err()

func (s *badgeService) RemoveBadgeFromPlayer(ctx context.Context, request *pb.RemoveBadgeFromPlayerRequest) (*pb.RemoveBadgeFromPlayerResponse, error) {
	playerId, err := uuid.Parse(request.PlayerId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid player_id")
	}

	changeCount, err := s.repo.RemovePlayerBadge(ctx, playerId, request.BadgeId)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to remove badge from player")
	}

	if changeCount == 0 {
		return nil, removeBadgeFromPlayerDoesntHaveBadgeErr
	}

	// TODO recalculate active badge if necessary

	return &pb.RemoveBadgeFromPlayerResponse{}, nil
}

func (s *badgeService) GetActivePlayerBadge(ctx context.Context, request *pb.GetActivePlayerBadgeRequest) (*pb.GetActivePlayerBadgeResponse, error) {
	playerId, err := uuid.Parse(request.PlayerId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid player_id")
	}

	badgeId, err := s.repo.GetActivePlayerBadge(ctx, playerId)
	if err != nil {

		return nil, status.Error(codes.Internal, "failed to get active badge")
	}
	if badgeId == nil {
		return nil, status.Error(codes.NotFound, "player does not have any badges")
	}

	badge, ok := s.badgeCfg.Badges[*badgeId]
	if !ok {
		return nil, status.Error(codes.Internal, "failed to resolve badgeId to config")
	}

	return &pb.GetActivePlayerBadgeResponse{
		Badge: badge.ToProto(),
	}, nil
}
func (s *badgeService) GetPlayerBadges(ctx context.Context, request *pb.GetPlayerBadgesRequest) (*pb.GetPlayerBadgesResponse, error) {
	playerId, err := uuid.Parse(request.PlayerId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid player_id")
	}

	player, err := s.repo.GetPlayer(ctx, playerId)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get player badges")
	}

	badges := make([]*pbmodel.Badge, len(player.Badges))
	for i, badgeId := range player.Badges {
		badge, ok := s.badgeCfg.Badges[badgeId]
		if !ok {
			return nil, status.Error(codes.Internal, "failed to resolve badgeId to config")
		}

		badges[i] = badge.ToProto()
	}

	return &pb.GetPlayerBadgesResponse{
		Badges:        badges,
		ActiveBadgeId: player.ActiveBadge,
	}, nil
}

var addBadgeToPlayerAlreadyHasBadgeErr = panicIfErr(status.New(codes.AlreadyExists, "player already has this badge").
	WithDetails(&pb.AddBadgeToPlayerErrorResponse{Reason: pb.AddBadgeToPlayerErrorResponse_PLAYER_ALREADY_HAS_BADGE})).Err()

// TODO check for their badge priority and if it changes what badge they have active
func (s *badgeService) AddBadgeToPlayer(ctx context.Context, request *pb.AddBadgeToPlayerRequest) (*pb.AddBadgeToPlayerResponse, error) {
	playerId, err := uuid.Parse(request.PlayerId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid player_id")
	}

	_, ok := s.badgeCfg.Badges[request.BadgeId]
	if !ok {
		return nil, status.Error(codes.InvalidArgument, "invalid badge_id")
	}

	changeCount, err := s.repo.AddPlayerBadge(ctx, playerId, request.BadgeId)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to add badge to player")
	}

	if changeCount == 0 {
		return nil, addBadgeToPlayerAlreadyHasBadgeErr
	}

	return &pb.AddBadgeToPlayerResponse{}, nil
}

func (s *badgeService) GetBadges(context.Context, *pb.GetBadgesRequest) (*pb.GetBadgesResponse, error) {
	badges := make([]*pbmodel.Badge, 0, len(s.badgeCfg.Badges))
	for _, badge := range s.badgeCfg.Badges {
		badges = append(badges, badge.ToProto())
	}

	return &pb.GetBadgesResponse{
		Badges: badges,
	}, nil
}

func panicIfErr[T any](thing T, err error) T {
	if err != nil {
		panic(err)
	}
	return thing
}
