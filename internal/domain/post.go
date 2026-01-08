package domain

import "time"

type Post struct {
	ID          int64
	Title       string
	Slug        string
	ContentMD   string
	ContentHTML string
	IsPublished bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
	PublishedAt *time.Time
	Tags        []Tag
}

type Tag struct {
	ID   int64
	Name string
	Slug string
	Used int64 // for tag cloud
}

type User struct {
	ID           int64
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

type Setting struct {
	Key   string
	Value string
}
