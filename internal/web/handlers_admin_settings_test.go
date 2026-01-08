package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"blogcms/internal/app"
	"blogcms/internal/auth"
	"blogcms/internal/domain"
)

type fakePostsRepo struct{}

func (fakePostsRepo) Create(context.Context, *domain.Post) error { return nil }
func (fakePostsRepo) Update(context.Context, *domain.Post) error { return nil }
func (fakePostsRepo) BySlug(context.Context, string, bool) (domain.Post, error) {
	return domain.Post{}, domain.ErrNotFound
}
func (fakePostsRepo) List(context.Context, domain.PostListOptions) ([]domain.Post, error) {
	return nil, nil
}
func (fakePostsRepo) Delete(context.Context, int64) error                { return nil }
func (fakePostsRepo) SetPostTags(context.Context, int64, []string) error { return nil }

type fakeTagRepo struct{}

func (fakeTagRepo) Cloud(context.Context) ([]domain.Tag, error) { return nil, domain.ErrNotFound }

func (fakeTagRepo) Suggest(context.Context, string, int) ([]domain.Tag, error) {
	return nil, domain.ErrNotFound
}

type fakeUserRepo struct{}

func (fakeUserRepo) ByUsername(context.Context, string) (domain.User, error) {
	return domain.User{}, domain.ErrNotFound
}

type memSettingsRepo struct{ m map[string]string }

func newMemSettingsRepo() *memSettingsRepo { return &memSettingsRepo{m: map[string]string{}} }
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

func TestAdminSettingsGET_RendersForm(t *testing.T) {
	renderer, err := NewTemplateRenderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}

	settingsRepo := newMemSettingsRepo()
	settingsSvc := app.NewSettingsService(settingsRepo)
	if err := settingsSvc.EnsureDefaults(context.Background()); err != nil {
		t.Fatalf("EnsureDefaults: %v", err)
	}

	postSvc := app.NewPostService(fakePostsRepo{}, fakeTagRepo{}, app.RealClock{}, app.PostServiceOptions{MarkdownPoolSize: 1})
	authSvc := app.NewAuthService(fakeUserRepo{}, "test")

	services := Services{
		Posts:    postSvc,
		Auth:     authSvc,
		Settings: settingsSvc,
		Sessions: auth.NewMemoryStore(),
	}

	s := NewServer(":0", ServerOptions{CookieSecure: false}, renderer, services, app.DefaultBlogTitle, app.DefaultBlogAbout, app.DefaultBlogFooter, "", "", 1<<20)

	req := httptest.NewRequest(http.MethodGet, "/admin/settings", nil)
	req = req.WithContext(context.WithValue(req.Context(), ctxUserIDKey, int64(1)))
	req = req.WithContext(context.WithValue(req.Context(), ctxCSRFKey, "csrf-token"))

	rr := httptest.NewRecorder()
	s.handleAdminSettings(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d body=%q", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, "Blog title") {
		t.Fatalf("expected form to contain Blog title, got: %s", body)
	}
	if !strings.Contains(body, "csrf_token") {
		t.Fatalf("expected CSRF field")
	}
}
