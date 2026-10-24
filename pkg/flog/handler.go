package flog

import (
	"context"
	"log/slog"
	"strings"
)

type FilterHandler struct {
	slog.Handler
	attrs           []slog.Attr
	allowedPrefixes []string
}

func NewFilterHandler(handler slog.Handler, allowedPrefixes string) *FilterHandler {
	return &FilterHandler{
		Handler:         handler,
		attrs:           make([]slog.Attr, 0, 10),
		allowedPrefixes: strings.Split(allowedPrefixes, ";"),
	}
}

func (h *FilterHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h2 := *h
	h2.Handler = h.Handler.WithAttrs(attrs)
	h2.attrs = append(h2.attrs, attrs...)
	return &h2
}

func (h *FilterHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level > slog.LevelDebug {
		return h.Handler.Handle(ctx, r)
	}

	var op slog.Attr

	if h.attrs != nil {
		for _, attr := range h.attrs {
			if attr.Key == "op" {
				op = attr
				break
			}
		}
	}

	if op.Key == "" {
		r.Attrs(func(a slog.Attr) bool {
			if a.Key == "op" {
				op = a
				return false
			}
			return true
		})

	}

	if op.Key == "" {
		return nil
	}

	matched := false
	for _, prefix := range h.allowedPrefixes {
		if matchPattern(prefix, op.Value.String()) {
			matched = true
			break
		}
	}

	if !matched {
		return nil
	}

	return h.Handler.Handle(ctx, r)
}

func matchPattern(pattern, value string) bool {
	if pattern == "" {
		return false
	}

	if pattern == "*" {
		return true
	}

	startsWithWildcard := strings.HasPrefix(pattern, "*")
	endsWithWildcard := strings.HasSuffix(pattern, "*")

	switch {
	case startsWithWildcard && endsWithWildcard:
		// *contains* - check if value contains the middle part
		middle := strings.TrimSuffix(strings.TrimPrefix(pattern, "*"), "*")
		if middle == "" {
			return true // Pattern is just "*" or "**", matches everything
		}
		return strings.Contains(value, middle)

	case endsWithWildcard:
		// prefix* - check if value starts with prefix
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(value, prefix)

	case startsWithWildcard:
		// *suffix - check if value ends with suffix
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(value, suffix)

	default:
		// Exact match
		return pattern == value
	}
}
