package service

import (
	"context"
	"errors"
	"fmt"
	pb "github.com/emortalmc/proto-specs/gen/go/grpc/mcplayer"
	"github.com/emortalmc/proto-specs/gen/go/model/common"
	"github.com/emortalmc/proto-specs/gen/go/model/mcplayer"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"mc-player-service/internal/repository"
	"mc-player-service/internal/repository/model"
	"mc-player-service/internal/utils"
)

type mcPlayerService struct {
	pb.McPlayerServer

	repo repository.Repository
}

func newMcPlayerService(repo repository.Repository) pb.McPlayerServer {
	return &mcPlayerService{
		repo: repo,
	}
}

func (s *mcPlayerService) GetPlayer(ctx context.Context, req *pb.GetPlayerRequest) (*pb.GetPlayerResponse, error) {
	pId, err := uuid.Parse(req.PlayerId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid player id %s", req.PlayerId))
	}

	mcPlayer, err := s.getOrCreateMcPlayer(ctx, pId)
	if err != nil {
		return nil, err
	}

	return &pb.GetPlayerResponse{
		Player: mcPlayer,
	}, nil
}

func (s *mcPlayerService) GetPlayers(ctx context.Context, req *pb.GetPlayersRequest) (*pb.GetPlayersResponse, error) {
	var ids []uuid.UUID
	for _, id := range req.PlayerIds {
		pId, err := uuid.Parse(id)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid player id %s", id))
		}
		ids = append(ids, pId)
	}

	players, err := s.repo.GetPlayers(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to get players: %w", err)
	}

	var protoPlayers = make([]*mcplayer.McPlayer, len(players))
	for i, p := range players {
		protoPlayers[i], err = s.createMcPlayerFromPlayer(ctx, p)
		if err != nil {
			return nil, fmt.Errorf("error creating player proto: %w", err)
		}
	}

	return &pb.GetPlayersResponse{
		Players: protoPlayers,
	}, nil
}

func (s *mcPlayerService) GetPlayerByUsername(ctx context.Context, req *pb.PlayerUsernameRequest) (*pb.GetPlayerByUsernameResponse, error) {
	p, err := s.repo.GetPlayerByUsername(ctx, req.Username, true)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, status.Error(codes.NotFound, fmt.Sprintf("player with username %s not found", req.Username))
		}
		return nil, err
	}

	mcPlayer, err := s.createMcPlayerFromPlayer(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("error creating player proto: %w", err)
	}

	return &pb.GetPlayerByUsernameResponse{
		Player: mcPlayer,
	}, nil
}

func (s *mcPlayerService) SearchPlayersByUsername(ctx context.Context, req *pb.SearchPlayersByUsernameRequest) (*pb.SearchPlayersByUsernameResponse, error) {
	filter := &repository.UsernameSearchFilter{
		OnlineOnly: req.FilterMethod == pb.SearchPlayersByUsernameRequest_ONLINE,
		Friends:    req.FilterMethod == pb.SearchPlayersByUsernameRequest_FRIENDS,
	}

	if req.Pageable == nil {
		req.Pageable = &common.Pageable{
			Page: 0,
			Size: utils.PointerOf(uint64(20)),
		}
	} else if req.Pageable.Size == nil || *req.Pageable.Size == 0 {
		req.Pageable.Size = utils.PointerOf(uint64(20))
	}

	var excludedPlayerIds []uuid.UUID
	if req.ExcludedPlayerIds != nil {
		excludedPlayerIds = make([]uuid.UUID, 0, len(req.ExcludedPlayerIds))

		for _, id := range req.ExcludedPlayerIds {
			pId, err := uuid.Parse(id)
			if err != nil {
				return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid player id %s", id))
			}
			excludedPlayerIds = append(excludedPlayerIds, pId)
		}
	}

	players, pageData, err := s.repo.SearchPlayersByUsername(ctx, req.SearchUsername, req.Pageable, filter, excludedPlayerIds)
	if err != nil {
		return nil, fmt.Errorf("error searching for players: %w", err)
	}

	var protoPlayers = make([]*mcplayer.McPlayer, len(players))
	for i, p := range players {
		protoPlayers[i], err = s.createMcPlayerFromPlayer(ctx, p)
		if err != nil {
			return nil, fmt.Errorf("error creating player proto: %w", err)
		}
	}

	return &pb.SearchPlayersByUsernameResponse{
		Players:  protoPlayers,
		PageData: pageData,
	}, nil
}

func (s *mcPlayerService) GetLoginSessions(ctx context.Context, req *pb.GetLoginSessionsRequest) (*pb.LoginSessionsResponse, error) {
	pId, err := uuid.Parse(req.PlayerId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid player id %s", req.PlayerId))
	}

	sessions, err := s.repo.GetLoginSessions(ctx, pId, req.Pageable)
	if err != nil {
		return nil, fmt.Errorf("error getting login sessions: %w", err)
	}

	var protoSessions = make([]*mcplayer.LoginSession, len(sessions))
	for i, s := range sessions {
		protoSessions[i] = s.ToProto()
	}

	return &pb.LoginSessionsResponse{
		Sessions: protoSessions,
	}, nil
}

func (s *mcPlayerService) GetStatTotalUniquePlayers(ctx context.Context, _ *pb.GetStatTotalUniquePlayersRequest) (*pb.GetStatTotalUniquePlayersResponse, error) {
	count, err := s.repo.GetTotalUniquePlayers(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting total unique players: %w", err)
	}

	return &pb.GetStatTotalUniquePlayersResponse{Count: count}, nil
}

func (s *mcPlayerService) GetStatTotalPlaytime(ctx context.Context, _ *pb.GetStatTotalPlaytimeRequest) (*pb.GetStatTotalPlaytimeResponse, error) {
	count, err := s.repo.GetTotalPlaytimeHours(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting total playtime: %w", err)
	}

	return &pb.GetStatTotalPlaytimeResponse{PlaytimeHours: count}, nil
}

func (s *mcPlayerService) getOrCreateMcPlayer(ctx context.Context, pId uuid.UUID) (*mcplayer.McPlayer, error) {
	p, err := s.repo.GetPlayer(ctx, pId)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, status.Error(codes.NotFound, fmt.Sprintf("player with id %s not found", pId.String()))
		}
		return nil, err
	}

	return s.createMcPlayerFromPlayer(ctx, p)
}

func (s *mcPlayerService) createMcPlayerFromPlayer(ctx context.Context, p *model.Player) (*mcplayer.McPlayer, error) {
	var session *model.LoginSession
	if p.CurrentServer != nil {
		var err error
		session, err = s.repo.GetCurrentLoginSession(ctx, p.Id)
		if err != nil {
			return nil, fmt.Errorf("error getting current login session (id: %s): %w", p.Id, err)
		}
	}

	return p.ToProto(session), nil
}
