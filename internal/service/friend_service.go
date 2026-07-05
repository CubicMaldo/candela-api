package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/tuusuario/candela-api/internal/domain"
	"github.com/tuusuario/candela-api/internal/repo"
)

var (
	ErrCannotFriendYourself       = errors.New("No puedes enviarte una solicitud a ti mismo.")
	ErrFriendRequestAlreadyExists = errors.New("Ya hay una solicitud pendiente con esta persona.")
	ErrAlreadyFriends             = errors.New("Ya son amigos.")
	ErrNotRecipient               = errors.New("Esta solicitud no es para ti.")
	ErrNotRequester               = errors.New("Esta solicitud no es tuya.")
)

type FriendService struct {
	friendshipRepo *repo.FriendshipRepository
	redis          *redis.Client
}

func NewFriendService(fr *repo.FriendshipRepository, rdb *redis.Client) *FriendService {
	return &FriendService{friendshipRepo: fr, redis: rdb}
}

func (s *FriendService) invalidateFriendCaches(ctx context.Context, a, b uuid.UUID) {
	s.redis.Del(ctx, fmt.Sprintf(friendRankingKey, a.String()))
	s.redis.Del(ctx, fmt.Sprintf(friendRankingKey, b.String()))
}

func (s *FriendService) SendRequest(ctx context.Context, senderID, recipientID uuid.UUID) error {
	if senderID == recipientID {
		return ErrCannotFriendYourself
	}

	blocked, err := s.friendshipRepo.IsBlocked(ctx, senderID, recipientID)
	if err != nil {
		return err
	}
	if blocked {
		return ErrUserNotVisible
	}

	status, _, err := s.friendshipRepo.GetFriendship(ctx, senderID, recipientID)
	if err != nil && !errors.Is(err, repo.ErrFriendRequestNotFound) {
		return err
	}
	if err == nil {
		switch status {
		case "pending":
			return ErrFriendRequestAlreadyExists
		case "accepted":
			return ErrAlreadyFriends
		case "blocked":
			return ErrUserNotVisible
		}
	}

	return s.friendshipRepo.SendRequest(ctx, senderID, recipientID)
}

func (s *FriendService) AcceptRequest(ctx context.Context, recipientID, senderID uuid.UUID) error {
	status, requesterID, err := s.friendshipRepo.GetFriendship(ctx, recipientID, senderID)
	if err != nil {
		return err
	}
	if status != "pending" {
		return repo.ErrFriendRequestNotFound
	}
	if requesterID != senderID {
		return ErrNotRecipient
	}

	if err := s.friendshipRepo.AcceptRequest(ctx, recipientID, senderID); err != nil {
		return err
	}
	s.invalidateFriendCaches(ctx, recipientID, senderID)
	return nil
}

func (s *FriendService) RejectRequest(ctx context.Context, recipientID, senderID uuid.UUID) error {
	status, requesterID, err := s.friendshipRepo.GetFriendship(ctx, recipientID, senderID)
	if err != nil {
		return err
	}
	if status != "pending" {
		return repo.ErrFriendRequestNotFound
	}
	if requesterID != senderID {
		return ErrNotRecipient
	}

	return s.friendshipRepo.RejectRequest(ctx, recipientID, senderID)
}

func (s *FriendService) CancelRequest(ctx context.Context, senderID, recipientID uuid.UUID) error {
	status, requesterID, err := s.friendshipRepo.GetFriendship(ctx, senderID, recipientID)
	if err != nil {
		return err
	}
	if status != "pending" {
		return repo.ErrFriendRequestNotFound
	}
	if requesterID != senderID {
		return ErrNotRequester
	}

	return s.friendshipRepo.CancelRequest(ctx, senderID, recipientID)
}

func (s *FriendService) RemoveFriend(ctx context.Context, userID, friendID uuid.UUID) error {
	if err := s.friendshipRepo.RemoveFriend(ctx, userID, friendID); err != nil {
		return err
	}
	s.invalidateFriendCaches(ctx, userID, friendID)
	return nil
}

func (s *FriendService) GetIncomingRequests(ctx context.Context, userID uuid.UUID) ([]domain.FriendRequestEntry, error) {
	return s.friendshipRepo.GetIncomingRequests(ctx, userID)
}

func (s *FriendService) GetOutgoingRequests(ctx context.Context, userID uuid.UUID) ([]domain.FriendRequestEntry, error) {
	return s.friendshipRepo.GetOutgoingRequests(ctx, userID)
}

func (s *FriendService) GetFriends(ctx context.Context, userID uuid.UUID) ([]domain.FriendEntry, error) {
	return s.friendshipRepo.GetFriendsList(ctx, userID)
}
