package services

import (
	"admin-dashboard/pkg/cache"
	"admin-dashboard/pkg/meta"
	"context"
	"database/sql"
	"fmt"
)

type ProfileService struct {
	db         *sql.DB
	metaClient *meta.Client
	cache      *cache.Cache
}

func NewProfileService(db *sql.DB, metaClient *meta.Client, cache *cache.Cache) *ProfileService {
	return &ProfileService{
		db:         db,
		metaClient: metaClient,
		cache:      cache,
	}
}

func (s *ProfileService) RefreshProfilePicture(ctx context.Context, threadID string) error {
	profile, err := s.metaClient.GetProfile(ctx, threadID)
	if err != nil {
		return fmt.Errorf("get profile: %w", err)
	}

	err = s.updateProfile(ctx, threadID, profile)
	if err != nil {
		return fmt.Errorf("update profile: %w", err)
	}

	s.cache.InvalidateProfile(threadID)
	return nil
}

func (s *ProfileService) updateProfile(ctx context.Context, threadID string, profile *meta.Profile) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE conversations 
		SET profile_picture_url = $1,
		    updated_at = CURRENT_TIMESTAMP
		WHERE thread_id = $2
	`, profile.PictureURL, threadID)
	return err
}
