package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"blogcms/internal/app"
	"blogcms/internal/auth"
	"blogcms/internal/domain"
)

type Services struct {
	Posts    *app.PostService
	Auth     *app.AuthService
	Settings *app.SettingsService
	Sessions auth.Store
}

type Server struct {
	httpServer        *http.Server
	renderer          Renderer
	services          Services
	cookieSecure      bool
	loginLimiter      *loginLimiter
	defaultBlogTitle  string
	defaultBlogAbout  string
	defaultBlogFooter string
	uploadDir         string
	uploadBase        string
	maxUploadBytes    int64

	// Small caches to reduce DB load on hot endpoints.
	settingsTTL time.Duration
	tagCloudTTL time.Duration
	cacheTitle  ttlCache[string]
	cacheAbout  ttlCache[string]
	cacheFooter ttlCache[string]
	cacheTags   ttlCache[[]domain.Tag]
}

type ServerOptions struct {
	CookieSecure      bool
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
	MaxHeaderBytes    int
	RequestTimeout    time.Duration

	SettingsCacheTTL time.Duration
	TagCloudCacheTTL time.Duration

	LoginLimiterMaxAttempts int
	LoginLimiterWindow      time.Duration
}

// StartBackground starts optional background maintenance goroutines.
// It is safe to call once during process init.
func (s *Server) StartBackground(ctx context.Context) {
	// Session store janitor (memory store)
	type janitor interface {
		StartJanitor(ctx context.Context, interval time.Duration)
	}
	if j, ok := s.services.Sessions.(janitor); ok {
		j.StartJanitor(ctx, 1*time.Minute)
	}

	// Login limiter janitor
	if s.loginLimiter != nil {
		s.loginLimiter.StartJanitor(ctx, 5*time.Minute)
	}
}

func NewServer(addr string, opt ServerOptions, renderer Renderer, services Services, defaultBlogTitle, defaultBlogAbout, defaultBlogFooter, uploadDir, uploadBase string, maxUploadBytes int64) *Server {
	mux := http.NewServeMux()
	if opt.ReadHeaderTimeout <= 0 {
		opt.ReadHeaderTimeout = 5 * time.Second
	}
	if opt.ReadTimeout <= 0 {
		opt.ReadTimeout = 15 * time.Second
	}
	if opt.WriteTimeout <= 0 {
		opt.WriteTimeout = 30 * time.Second
	}
	if opt.IdleTimeout <= 0 {
		opt.IdleTimeout = 2 * time.Minute
	}
	if opt.ShutdownTimeout <= 0 {
		opt.ShutdownTimeout = 10 * time.Second
	}
	if opt.MaxHeaderBytes <= 0 {
		opt.MaxHeaderBytes = 1 << 20
	}
	if opt.RequestTimeout <= 0 {
		opt.RequestTimeout = 10 * time.Second
	}
	if opt.SettingsCacheTTL <= 0 {
		opt.SettingsCacheTTL = 30 * time.Second
	}
	if opt.TagCloudCacheTTL <= 0 {
		opt.TagCloudCacheTTL = 30 * time.Second
	}
	if opt.LoginLimiterMaxAttempts <= 0 {
		opt.LoginLimiterMaxAttempts = 5
	}
	if opt.LoginLimiterWindow <= 0 {
		opt.LoginLimiterWindow = 5 * time.Minute
	}

	s := &Server{
		renderer:          renderer,
		services:          services,
		cookieSecure:      opt.CookieSecure,
		loginLimiter:      newLoginLimiter(opt.LoginLimiterMaxAttempts, opt.LoginLimiterWindow),
		defaultBlogTitle:  defaultBlogTitle,
		defaultBlogAbout:  defaultBlogAbout,
		defaultBlogFooter: defaultBlogFooter,
		uploadDir:         uploadDir,
		uploadBase:        uploadBase,
		maxUploadBytes:    maxUploadBytes,
		settingsTTL:       opt.SettingsCacheTTL,
		tagCloudTTL:       opt.TagCloudCacheTTL,
	}

	s.registerRoutes(mux)

	h := withRecovery(withLogging(withRequestID(withSecurityHeaders(opt.CookieSecure)(withTimeout(opt.RequestTimeout)(s.withAuthContext(mux))))))

	s.httpServer = &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: opt.ReadHeaderTimeout,
		ReadTimeout:       opt.ReadTimeout,
		WriteTimeout:      opt.WriteTimeout,
		IdleTimeout:       opt.IdleTimeout,
		MaxHeaderBytes:    opt.MaxHeaderBytes,
	}
	return s
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	// Public
	mux.HandleFunc("/", s.handleHome)
	mux.HandleFunc("/post/", s.handlePost)
	mux.HandleFunc("/tag/", s.handleTag)
	// Public uploads (served from local filesystem)
	if s.uploadBase != "" && s.uploadDir != "" {
		fs := http.FileServer(http.Dir(s.uploadDir))
		mux.Handle(s.uploadBase, http.StripPrefix(s.uploadBase, fs))
	}

	// Admin
	mux.HandleFunc("/admin/login", s.handleAdminLogin)
	mux.Handle("/admin/logout", requireAuth(requireCSRF(http.HandlerFunc(s.handleAdminLogout))))

	mux.Handle("/admin", requireAuth(http.HandlerFunc(s.handleAdminIndex)))
	mux.Handle("/admin/posts/new", requireAuth(http.HandlerFunc(s.handleAdminNewPost)))
	mux.Handle("/admin/posts/create", requireAuth(requireCSRF(http.HandlerFunc(s.handleAdminCreatePost))))
	mux.Handle("/admin/posts/edit/", requireAuth(http.HandlerFunc(s.handleAdminEditPost)))
	mux.Handle("/admin/posts/update", requireAuth(requireCSRF(http.HandlerFunc(s.handleAdminUpdatePost))))
	mux.Handle("/admin/settings", requireAuth(requireCSRF(http.HandlerFunc(s.handleAdminSettings))))
	mux.Handle("/admin/upload", requireAuth(requireCSRF(http.HandlerFunc(s.handleAdminUpload))))
}

func (s *Server) withAuthContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("blogcms_session")
		if err != nil || c.Value == "" {
			next.ServeHTTP(w, r)
			return
		}

		sessionID, err := s.services.Auth.VerifySessionToken(c.Value)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		ss, ok := s.services.Sessions.Get(r.Context(), sessionID)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		ctx := context.WithValue(r.Context(), ctxUserIDKey, ss.UserID)
		ctx = context.WithValue(ctx, ctxCSRFKey, s.services.Auth.CSRFToken(sessionID))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) setSessionCookie(w http.ResponseWriter, sessionID string, ttl time.Duration) {
	token := s.services.Auth.SignSessionToken(sessionID)
	http.SetCookie(w, &http.Cookie{
		Name:     "blogcms_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(ttl),
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "blogcms_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func (s *Server) commonViewData(ctx context.Context) (ViewCommon, error) {
	// These are independent DB calls. Fetch them concurrently to reduce request latency.
	var (
		title  string
		about  string
		footer string
		cloud  []domain.Tag
	)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		wg       sync.WaitGroup
		errOnce  sync.Once
		firstErr error
	)
	setErr := func(err error) {
		if err == nil {
			return
		}
		errOnce.Do(func() {
			firstErr = err
			cancel()
		})
	}

	wg.Add(4)
	go func() {
		defer wg.Done()
		v, err := s.cacheTitle.GetOrLoad(ctx, s.settingsTTL, func(c context.Context) (string, error) {
			return s.services.Settings.BlogTitle(c, s.defaultBlogTitle)
		})
		if err != nil {
			setErr(err)
			return
		}
		title = strings.TrimSpace(v)
		if title == "" {
			title = s.defaultBlogTitle
		}
	}()
	go func() {
		defer wg.Done()
		v, err := s.cacheAbout.GetOrLoad(ctx, s.settingsTTL, func(c context.Context) (string, error) {
			return s.services.Settings.BlogAbout(c, s.defaultBlogAbout)
		})
		if err != nil {
			setErr(err)
			return
		}
		about = strings.TrimSpace(v)
		if about == "" {
			about = s.defaultBlogAbout
		}
	}()
	go func() {
		defer wg.Done()
		v, err := s.cacheFooter.GetOrLoad(ctx, s.settingsTTL, func(c context.Context) (string, error) {
			return s.services.Settings.BlogFooter(c, s.defaultBlogFooter)
		})
		if err != nil {
			setErr(err)
			return
		}
		footer = v
	}()
	go func() {
		defer wg.Done()
		t, err := s.cacheTags.GetOrLoad(ctx, s.tagCloudTTL, func(c context.Context) ([]domain.Tag, error) {
			return s.services.Posts.TagCloud(c)
		})
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				cloud = nil
				return
			}
			setErr(err)
			return
		}
		cloud = t
	}()

	wg.Wait()
	if firstErr != nil {
		return ViewCommon{}, firstErr
	}

	common := ViewCommon{
		BlogTitle:  title,
		BlogAbout:  about,
		BlogFooter: footer,
		TagCloud:   cloud,
	}

	if _, ok := userIDFromContext(ctx); ok {
		common.IsAuthed = true
	}
	if t, ok := csrfFromContext(ctx); ok {
		common.CSRFToken = t
	}

	return common, nil
}

type ViewCommon struct {
	BlogTitle   string
	BlogAbout   string
	BlogFooter  string
	TagCloud    []domain.Tag
	CSRFToken   string
	IsAuthed    bool
	TitleSuffix string
}

func (c ViewCommon) PageTitle(suffix string) string {
	if suffix == "" {
		return c.BlogTitle
	}
	return fmt.Sprintf("%s — %s", suffix, c.BlogTitle)
}
