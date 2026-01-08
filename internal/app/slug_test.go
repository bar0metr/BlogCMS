package app

import "testing"

func TestSlugify(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"Hello World", "hello-world"},
		{"  Go & Postgres  ", "go-postgres"},
		{"", "post"},
		{"---", "post"},
		{"Go__CMS", "go-cms"},
	}

	for _, tt := range tests {
		if got := Slugify(tt.in); got != tt.want {
			t.Fatalf("Slugify(%q)=%q want %q", tt.in, got, tt.want)
		}
	}
}
