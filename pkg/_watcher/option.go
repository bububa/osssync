package watcher

type Option = func(w *Watcher)

// WithMaxEvents controls the maximum amount of events that are sent on
// the Event channel per watching cycle. If max events is less than 1, there is
// no limit, which is the default.
func WithMaxEvents(maxEvents int) Option {
	return func(w *Watcher) {
		w.maxEvents = maxEvents
	}
}

// WithFilterHook
func WithFilterHook(f FilterFileHookFunc) Option {
	return func(w *Watcher) {
		w.ffh = append(w.ffh, f)
	}
}

// WithIgnoreHiddenFiles sets the watcher to ignore any file or directory
// that starts with a dot.
func WithIgnoreHiddenFiles(ignore bool) Option {
	return func(w *Watcher) {
		w.ignoreHidden = ignore
	}
}

// WithFilterOps filters which event op types should be returned
// when an event occurs.
func WithFilterOp(op Op) Option {
	return func(w *Watcher) {
		w.ops[op] = struct{}{}
	}
}
