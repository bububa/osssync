package pkg

var (
	GitTag      string // set at compile time with -ldflags
	GitRevision string // set at compile time with -ldflags
	GitSummary  string // set at compile time with -ldflags
	AppName     string // set at compile time with -ldflags
	AppIdentity string // set at compile time with -ldflags
)
