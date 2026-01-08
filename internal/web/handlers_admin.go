package web

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"blogcms/internal/app"
	"blogcms/internal/domain"
)

const sessionTTL = 24 * time.Hour

func (s *Server) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		common, err := s.commonViewData(r.Context())
		if err != nil {
			writeDomainError(w, err)
			return
		}
		common.TitleSuffix = "Admin login"
		data := struct {
			ViewCommon
			Error string
		}{ViewCommon: common}
		_ = s.renderer.Render(w, "admin_login.html", data)
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		username := r.FormValue("username")
		password := r.FormValue("password")

		if ok, retry := s.loginLimiter.Allow(r); !ok {
			w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())))
			http.Error(w, "too many login attempts", http.StatusTooManyRequests)
			return
		}

		u, err := s.services.Auth.Authenticate(r.Context(), username, password)
		if err != nil {
			common, _ := s.commonViewData(r.Context())
			common.TitleSuffix = "Admin login"
			data := struct {
				ViewCommon
				Error string
			}{ViewCommon: common, Error: "Invalid credentials"}
			_ = s.renderer.Render(w, "admin_login.html", data)
			return
		}

		ss, err := s.services.Sessions.Create(r.Context(), u.ID, sessionTTL)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		s.setSessionCookie(w, ss.ID, sessionTTL)
		http.Redirect(w, r, "/admin", http.StatusFound)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAdminLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	c, err := r.Cookie("blogcms_session")
	if err == nil && c.Value != "" {
		if sid, err := s.services.Auth.VerifySessionToken(c.Value); err == nil {
			s.services.Sessions.Delete(r.Context(), sid)
		}
	}
	s.clearSessionCookie(w)
	http.Redirect(w, r, "/admin/login", http.StatusFound)
}

func (s *Server) handleAdminIndex(w http.ResponseWriter, r *http.Request) {
	common, err := s.commonViewData(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	common.TitleSuffix = "Admin"

	posts, err := s.services.Posts.List(r.Context(), domain.PostListOptions{
		Limit:              100,
		IncludeUnpublished: true,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}

	data := struct {
		ViewCommon
		Posts []domain.Post
	}{
		ViewCommon: common,
		Posts:      posts,
	}
	_ = s.renderer.Render(w, "admin_index.html", data)
}

func (s *Server) handleAdminNewPost(w http.ResponseWriter, r *http.Request) {
	common, err := s.commonViewData(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	common.TitleSuffix = "New post"

	data := struct {
		ViewCommon
		Post  domain.Post
		Tags  string
		Error string
	}{ViewCommon: common}

	_ = s.renderer.Render(w, "admin_edit.html", data)
}

func (s *Server) handleAdminCreatePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	in := app.CreatePostInput{
		Title:       r.FormValue("title"),
		ContentMD:   r.FormValue("content_md"),
		TagsCSV:     r.FormValue("tags"),
		IsPublished: r.FormValue("is_published") == "on",
	}

	p, err := s.services.Posts.Create(r.Context(), in)
	if err != nil {
		common, _ := s.commonViewData(r.Context())
		common.TitleSuffix = "New post"
		data := struct {
			ViewCommon
			Post  domain.Post
			Tags  string
			Error string
		}{
			ViewCommon: common,
			Post: domain.Post{
				Title:     in.Title,
				ContentMD: in.ContentMD,
			},
			Tags:  in.TagsCSV,
			Error: err.Error(),
		}
		_ = s.renderer.Render(w, "admin_edit.html", data)
		return
	}

	http.Redirect(w, r, "/admin/posts/edit/"+p.Slug, http.StatusFound)
}

func (s *Server) handleAdminEditPost(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimPrefix(r.URL.Path, "/admin/posts/edit/")
	slug = strings.Trim(slug, "/")
	if slug == "" {
		http.NotFound(w, r)
		return
	}

	common, err := s.commonViewData(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	common.TitleSuffix = "Edit post"

	p, err := s.services.Posts.GetBySlug(r.Context(), slug, true)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	data := struct {
		ViewCommon
		Post  domain.Post
		Tags  string
		Error string
	}{
		ViewCommon: common,
		Post:       p,
		Tags:       joinTags(p.Tags),
	}
	_ = s.renderer.Render(w, "admin_edit.html", data)
}

func (s *Server) handleAdminUpdatePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	in := app.UpdatePostInput{
		ID:          id,
		Title:       r.FormValue("title"),
		ContentMD:   r.FormValue("content_md"),
		TagsCSV:     r.FormValue("tags"),
		IsPublished: r.FormValue("is_published") == "on",
	}

	p, err := s.services.Posts.Update(r.Context(), in)
	if err != nil {
		common, _ := s.commonViewData(r.Context())
		common.TitleSuffix = "Edit post"
		data := struct {
			ViewCommon
			Post  domain.Post
			Tags  string
			Error string
		}{
			ViewCommon: common,
			Post: domain.Post{
				ID:          in.ID,
				Title:       in.Title,
				ContentMD:   in.ContentMD,
				IsPublished: in.IsPublished,
			},
			Tags:  in.TagsCSV,
			Error: err.Error(),
		}
		_ = s.renderer.Render(w, "admin_edit.html", data)
		return
	}
	http.Redirect(w, r, "/admin/posts/edit/"+p.Slug, http.StatusFound)
}

func (s *Server) handleAdminSettings(w http.ResponseWriter, r *http.Request) {
	common, err := s.commonViewData(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	common.TitleSuffix = "Settings"

	type pageData struct {
		ViewCommon
		Message string
		Error   string
	}

	switch r.Method {
	case http.MethodGet:
		data := pageData{ViewCommon: common}
		if err := s.renderer.Render(w, "admin_settings.html", data); err != nil {
			http.Error(w, "template error", http.StatusInternalServerError)
			return
		}
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		title := r.FormValue("blog_title")
		about := r.FormValue("blog_about")
		footer := r.FormValue("blog_footer")
		if err := s.services.Settings.SetBlogTitle(r.Context(), title); err != nil {
			data := pageData{ViewCommon: common, Error: err.Error()}
			_ = s.renderer.Render(w, "admin_settings.html", data)
			return
		}
		if err := s.services.Settings.SetBlogAbout(r.Context(), about); err != nil {
			data := pageData{ViewCommon: common, Error: err.Error()}
			_ = s.renderer.Render(w, "admin_settings.html", data)
			return
		}
		if err := s.services.Settings.SetBlogFooter(r.Context(), footer); err != nil {
			data := pageData{ViewCommon: common, Error: err.Error()}
			_ = s.renderer.Render(w, "admin_settings.html", data)
			return
		}

		// Invalidate caches so public pages reflect changes quickly.
		s.cacheTitle.Invalidate()
		s.cacheAbout.Invalidate()
		s.cacheFooter.Invalidate()

		// Refresh common view data so the form reflects saved values.
		common, _ = s.commonViewData(r.Context())
		common.TitleSuffix = "Settings"
		data := pageData{ViewCommon: common, Message: "Saved"}
		_ = s.renderer.Render(w, "admin_settings.html", data)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func joinTags(tags []domain.Tag) string {
	if len(tags) == 0 {
		return ""
	}
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		out = append(out, t.Name)
	}
	return strings.Join(out, ", ")
}

func (s *Server) handleAdminUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.uploadDir == "" || s.uploadBase == "" {
		http.Error(w, "uploads not configured", http.StatusServiceUnavailable)
		return
	}

	// Hard limit request body size.
	r.Body = http.MaxBytesReader(w, r.Body, s.maxUploadBytes)

	if err := r.ParseMultipartForm(s.maxUploadBytes); err != nil {
		http.Error(w, "bad upload", http.StatusBadRequest)
		return
	}

	f, hdr, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file required", http.StatusBadRequest)
		return
	}
	defer f.Close()

	// Read a small header to detect content type. We'll write these bytes first
	// to avoid relying on seek/reopen behavior of multipart.File.
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	contentType := http.DetectContentType(buf[:n])
	if !isAllowedUploadContentType(contentType) {
		http.Error(w, "unsupported file type", http.StatusUnsupportedMediaType)
		return
	}

	ext := strings.ToLower(filepath.Ext(hdr.Filename))
	if ext == "" {
		ext = extFromContentType(contentType)
	}
	if ext == ".svg" {
		http.Error(w, "unsupported file type", http.StatusUnsupportedMediaType)
		return
	}

	if err := os.MkdirAll(s.uploadDir, 0o750); err != nil {
		http.Error(w, "cannot create upload dir", http.StatusInternalServerError)
		return
	}

	name, err := randomHex(16)
	if err != nil {
		http.Error(w, "cannot generate name", http.StatusInternalServerError)
		return
	}
	filename := name + ext
	path := filepath.Join(s.uploadDir, filename)

	out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o640)
	if err != nil {
		http.Error(w, "cannot save file", http.StatusInternalServerError)
		return
	}
	defer out.Close()

	if n > 0 {
		if _, err := out.Write(buf[:n]); err != nil {
			http.Error(w, "cannot save file", http.StatusInternalServerError)
			return
		}
	}
	if _, err := io.Copy(out, f); err != nil {
		http.Error(w, "cannot save file", http.StatusInternalServerError)
		return
	}

	url := s.uploadBase + filename
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"url": url, "content_type": contentType, "original": hdr.Filename})
}

func randomHex(nbytes int) (string, error) {
	b := make([]byte, nbytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func isAllowedUploadContentType(ct string) bool {
	if strings.HasPrefix(ct, "image/") {
		// SVG is rejected separately.
		return true
	}
	switch ct {
	case "application/pdf", "text/plain; charset=utf-8", "text/plain", "application/zip", "application/x-zip-compressed":
		return true
	default:
		return false
	}
}

func extFromContentType(ct string) string {
	switch ct {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "application/pdf":
		return ".pdf"
	case "text/plain", "text/plain; charset=utf-8":
		return ".txt"
	case "application/zip", "application/x-zip-compressed":
		return ".zip"
	default:
		return ""
	}
}
