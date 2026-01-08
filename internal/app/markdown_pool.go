package app

import "github.com/yuin/goldmark"

// markdownPool bounds concurrent markdown conversions and avoids sharing a non-thread-safe renderer.
//
// The pool provides backpressure: if all renderers are busy, Acquire blocks.
// This is deliberate; it prevents runaway CPU usage and GC pressure under load.
type markdownPool struct {
	ch chan goldmark.Markdown
}

func newMarkdownPool(size int, factory func() goldmark.Markdown) *markdownPool {
	if size <= 0 {
		size = 1
	}
	p := &markdownPool{ch: make(chan goldmark.Markdown, size)}
	for i := 0; i < size; i++ {
		p.ch <- factory()
	}
	return p
}

func (p *markdownPool) Acquire() goldmark.Markdown {
	return <-p.ch
}

func (p *markdownPool) Release(md goldmark.Markdown) {
	// Best-effort: in normal operation channel is never full.
	select {
	case p.ch <- md:
	default:
		// If a caller double-releases, avoid blocking forever.
	}
}
