package app

import (
	"context"
	"testing"

	"blogcms/internal/domain"
)

type memSettingsRepo struct {
	m map[string]string
}

func newMemSettingsRepo() *memSettingsRepo {
	return &memSettingsRepo{m: map[string]string{}}
}

func (r *memSettingsRepo) Get(_ context.Context, key string) (string, error) {
	v, ok := r.m[key]
	if !ok {
		return "", domain.ErrNotFound
	}
	return v, nil
}

func (r *memSettingsRepo) Set(_ context.Context, key, value string) error {
	r.m[key] = value
	return nil
}

func TestSettingsService_EnsureDefaults_SeedsMissing(t *testing.T) {
	repo := newMemSettingsRepo()
	svc := NewSettingsService(repo)

	if err := svc.EnsureDefaults(context.Background()); err != nil {
		t.Fatalf("EnsureDefaults: %v", err)
	}

	if got := repo.m[SettingBlogTitle]; got != DefaultBlogTitle {
		t.Fatalf("title: got %q want %q", got, DefaultBlogTitle)
	}
	if got := repo.m[SettingBlogAbout]; got != DefaultBlogAbout {
		t.Fatalf("about: got %q want %q", got, DefaultBlogAbout)
	}
	if got := repo.m[SettingBlogFooter]; got != DefaultBlogFooter {
		t.Fatalf("footer: got %q want %q", got, DefaultBlogFooter)
	}
}

func TestSettingsService_EnsureDefaults_DoesNotOverwriteExisting(t *testing.T) {
	repo := newMemSettingsRepo()
	repo.m[SettingBlogTitle] = "Custom"
	svc := NewSettingsService(repo)

	if err := svc.EnsureDefaults(context.Background()); err != nil {
		t.Fatalf("EnsureDefaults: %v", err)
	}
	if got := repo.m[SettingBlogTitle]; got != "Custom" {
		t.Fatalf("title overwritten: got %q", got)
	}
}

func TestSettingsService_SetBlogTitle_Validates(t *testing.T) {
	repo := newMemSettingsRepo()
	svc := NewSettingsService(repo)

	if err := svc.SetBlogTitle(context.Background(), "   "); err == nil {
		t.Fatalf("expected error")
	}
	if err := svc.SetBlogTitle(context.Background(), "Ok"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
