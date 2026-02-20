package version

import "fmt"

var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

func Full() string {
	return fmt.Sprintf("%s (commit=%s, build_time=%s)", Version, GitCommit, BuildTime)
}
