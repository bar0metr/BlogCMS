package web

import (
	"net/http"
	"strconv"
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

	page := 1
	if pv := strings.TrimSpace(r.URL.Query().Get("page")); pv != "" {
		if n, err := strconv.Atoi(pv); err == nil && n > 0 {
			page = n
		}
	}

	perPage := common.HomePostsPerPage
	if perPage <= 0 {
		perPage = 20
	}
	// Fetch one extra row to determine whether there is a next page.
	limit := perPage + 1
	offset := (page - 1) * perPage
	posts, err := s.services.Posts.List(r.Context(), domain.PostListOptions{Limit: limit, Offset: offset})
	if err != nil {
		writeDomainError(w, err)
		return
	}

	hasNext := false
	if len(posts) > perPage {
		hasNext = true
		posts = posts[:perPage]
	}
	hasPrev := page > 1

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
		Page int
		HasPrev bool
		HasNext bool
	}{
		ViewCommon: common,
		Posts:      viewPosts,
		Page: page,
		HasPrev: hasPrev,
		HasNext: hasNext,
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
