package service

import (
	"context"
	"fmt"

	"go-notes/internal/models"
	"go-notes/internal/repository"
)

type OnboardingStatePayload struct {
	Home         bool `json:"home"`
	HostSettings bool `json:"hostSettings"`
	Networking   bool `json:"networking"`
	GitHub       bool `json:"github"`
}

type OnboardingUpdatePayload struct {
	Home         *bool `json:"home"`
	HostSettings *bool `json:"hostSettings"`
	Networking   *bool `json:"networking"`
	GitHub       *bool `json:"github"`
}

type OnboardingService struct {
	repo repository.OnboardingRepository
}

func NewOnboardingService(repo repository.OnboardingRepository) *OnboardingService {
	return &OnboardingService{repo: repo}
}

func (s *OnboardingService) Get(ctx context.Context, userID uint) (OnboardingStatePayload, error) {
	if userID == 0 {
		return OnboardingStatePayload{}, fmt.Errorf("user id required")
	}
	state, err := s.repo.GetByUser(ctx, userID)
	if err != nil {
		if err == repository.ErrNotFound {
			return OnboardingStatePayload{}, nil
		}
		return OnboardingStatePayload{}, err
	}
	return onboardingFromModel(state), nil
}

func (s *OnboardingService) Update(ctx context.Context, userID uint, input OnboardingUpdatePayload) (OnboardingStatePayload, error) {
	if userID == 0 {
		return OnboardingStatePayload{}, fmt.Errorf("user id required")
	}

	state, err := s.repo.GetByUser(ctx, userID)
	if err != nil {
		if err == repository.ErrNotFound {
			state = &models.OnboardingState{UserID: userID}
		} else {
			return OnboardingStatePayload{}, err
		}
	}

	applyOnboardingUpdate(state, input)

	if err := s.repo.Save(ctx, state); err != nil {
		return OnboardingStatePayload{}, err
	}

	return onboardingFromModel(state), nil
}

func onboardingFromModel(state *models.OnboardingState) OnboardingStatePayload {
	if state == nil {
		return OnboardingStatePayload{}
	}
	return OnboardingStatePayload{
		Home:         state.Home,
		HostSettings: state.HostSettings,
		Networking:   state.Networking,
		GitHub:       state.GitHub,
	}
}

func applyOnboardingUpdate(state *models.OnboardingState, input OnboardingUpdatePayload) {
	if input.Home != nil {
		state.Home = *input.Home
	}
	if input.HostSettings != nil {
		state.HostSettings = *input.HostSettings
	}
	if input.Networking != nil {
		state.Networking = *input.Networking
	}
	if input.GitHub != nil {
		state.GitHub = *input.GitHub
	}
}
