package config

var (
	GitTag      string // set at compile time with -ldflags
	GitRevision string // set at compile time with -ldflags
	GitSummary  string // set at compile time with -ldflags
	Bundled     string // set at compile time with -ldflags
)
