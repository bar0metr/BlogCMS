package web

import (
	"net/http"
	"strings"

	"blogcms/internal/domain"
)

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	common, err := s.commonViewData(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	common.TitleSuffix = "Home"

	posts, err := s.services.Posts.List(r.Context(), domain.PostListOptions{Limit: 20})
	if err != nil {
		writeDomainError(w, err)
		return
	}

	type homePost struct {
		domain.Post
		Excerpt string
	}
	viewPosts := make([]homePost, 0, len(posts))
	for _, p := range posts {
		viewPosts = append(viewPosts, homePost{Post: p, Excerpt: excerptFromHTML(p.ContentHTML)})
	}

	data := struct {
		ViewCommon
		Posts []homePost
	}{
		ViewCommon: common,
		Posts:      viewPosts,
	}

	if err := s.renderer.Render(w, "home.html", data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
}

func (s *Server) handlePost(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimPrefix(r.URL.Path, "/post/")
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

	p, err := s.services.Posts.GetBySlug(r.Context(), slug, false)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	common.TitleSuffix = p.Title

	data := struct {
		ViewCommon
		Post domain.Post
	}{
		ViewCommon: common,
		Post:       p,
	}

	if err := s.renderer.Render(w, "post.html", data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleTag(w http.ResponseWriter, r *http.Request) {
	tag := strings.TrimPrefix(r.URL.Path, "/tag/")
	tag = strings.Trim(tag, "/")
	if tag == "" {
		http.NotFound(w, r)
		return
	}

	common, err := s.commonViewData(r.Context())
	if err != nil {
		writeDomainError(w, err)
		return
	}

	common.TitleSuffix = "Tag: " + tag
	posts, err := s.services.Posts.List(r.Context(), domain.PostListOptions{
		Limit:   50,
		TagSlug: tag,
	})
	if err != nil {
		writeDomainError(w, err)
		return
	}

	data := struct {
		ViewCommon
		TagSlug string
		Posts   []domain.Post
	}{
		ViewCommon: common,
		TagSlug:    tag,
		Posts:      posts,
	}

	if err := s.renderer.Render(w, "tag.html", data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
}
