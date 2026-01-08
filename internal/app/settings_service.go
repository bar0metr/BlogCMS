package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"blogcms/internal/domain"
)

const (
	SettingBlogTitle  = "blog.title"
	SettingBlogAbout  = "blog.about"
	SettingBlogFooter = "blog.footer"
	SettingHomePostsPerPage = "home.posts_per_page"

	DefaultBlogTitle  = "My Blog"
	DefaultBlogAbout  = "Go + PostgreSQL. HTML templates. Minimal dependencies."
	DefaultBlogFooter = ""
	DefaultHomePostsPerPage = 20
)

type SettingsService struct {
	repo domain.SettingsRepository
}

func NewSettingsService(repo domain.SettingsRepository) *SettingsService {
	return &SettingsService{repo: repo}
}

func (s *SettingsService) BlogTitle(ctx context.Context, fallback string) (string, error) {
	v, err := s.repo.Get(ctx, SettingBlogTitle)
	if err == nil && strings.TrimSpace(v) != "" {
		return v, nil
	}
	if err != nil && err != domain.ErrNotFound {
		return "", fmt.Errorf("get blog title: %w", err)
	}
	return fallback, nil
}

func (s *SettingsService) SetBlogTitle(ctx context.Context, title string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return fmt.Errorf("%w: title is required", domain.ErrInvalidArgument)
	}
	if err := s.repo.Set(ctx, SettingBlogTitle, title); err != nil {
		return fmt.Errorf("set blog title: %w", err)
	}
	return nil
}

func (s *SettingsService) BlogAbout(ctx context.Context, fallback string) (string, error) {
	v, err := s.repo.Get(ctx, SettingBlogAbout)
	if err == nil && strings.TrimSpace(v) != "" {
		return v, nil
	}
	if err != nil && err != domain.ErrNotFound {
		return "", fmt.Errorf("get blog about: %w", err)
	}
	return fallback, nil
}

func (s *SettingsService) SetBlogAbout(ctx context.Context, about string) error {
	about = strings.TrimSpace(about)
	if about == "" {
		return fmt.Errorf("%w: about is required", domain.ErrInvalidArgument)
	}
	if err := s.repo.Set(ctx, SettingBlogAbout, about); err != nil {
		return fmt.Errorf("set blog about: %w", err)
	}
	return nil
}

func (s *SettingsService) BlogFooter(ctx context.Context, fallback string) (string, error) {
	v, err := s.repo.Get(ctx, SettingBlogFooter)
	if err == nil {
		// Footer is optional; empty is allowed.
		return strings.TrimSpace(v), nil
	}
	if err != nil && err != domain.ErrNotFound {
		return "", fmt.Errorf("get blog footer: %w", err)
	}
	return fallback, nil
}

func (s *SettingsService) SetBlogFooter(ctx context.Context, footer string) error {
	footer = strings.TrimSpace(footer)
	if err := s.repo.Set(ctx, SettingBlogFooter, footer); err != nil {
		return fmt.Errorf("set blog footer: %w", err)
	}
	return nil
}

func (s *SettingsService) HomePostsPerPage(ctx context.Context, fallback int) (int, error) {
	v, err := s.repo.Get(ctx, SettingHomePostsPerPage)
	if err == nil {
		n, perr := strconv.Atoi(strings.TrimSpace(v))
		if perr == nil && n > 0 {
			return n, nil
		}
	}
	if err != nil && err != domain.ErrNotFound {
		return 0, fmt.Errorf("get home posts per page: %w", err)
	}
	if fallback > 0 {
		return fallback, nil
	}
	return DefaultHomePostsPerPage, nil
}

func (s *SettingsService) SetHomePostsPerPage(ctx context.Context, n int) error {
	if n < 1 || n > 200 {
		return fmt.Errorf("%w: posts per page must be between 1 and 200", domain.ErrInvalidArgument)
	}
	if err := s.repo.Set(ctx, SettingHomePostsPerPage, strconv.Itoa(n)); err != nil {
		return fmt.Errorf("set home posts per page: %w", err)
	}
	return nil
}

// EnsureDefaults seeds required settings into persistent storage if they are missing.
// This makes a fresh install usable without relying on static config for UI text.
func (s *SettingsService) EnsureDefaults(ctx context.Context) error {
	if err := s.ensure(ctx, SettingBlogTitle, DefaultBlogTitle); err != nil {
		return err
	}
	if err := s.ensure(ctx, SettingBlogAbout, DefaultBlogAbout); err != nil {
		return err
	}
	if err := s.ensure(ctx, SettingBlogFooter, DefaultBlogFooter); err != nil {
		return err
	}
	if err := s.ensure(ctx, SettingHomePostsPerPage, strconv.Itoa(DefaultHomePostsPerPage)); err != nil {
		return err
	}
	return nil
}

func (s *SettingsService) ensure(ctx context.Context, key, value string) error {
	_, err := s.repo.Get(ctx, key)
	if err == nil {
		return nil
	}
	if err != domain.ErrNotFound {
		return fmt.Errorf("get setting %s: %w", key, err)
	}
	if err := s.repo.Set(ctx, key, value); err != nil {
		return fmt.Errorf("seed setting %s: %w", key, err)
	}
	return nil
}
