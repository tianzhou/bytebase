//go:build !release

package cmd

import (
	"time"

	"github.com/bytebase/bytebase/backend/common"
	"github.com/bytebase/bytebase/backend/component/config"
)

func activeProfile(dataDir string) *config.Profile {
	p := getBaseProfile(dataDir)
	p.RuntimeDebug.Store(p.Debug)
	p.Mode = common.ReleaseModeDev
	p.PgUser = "bb"
	p.BackupRunnerInterval = 10 * time.Second
	p.AppRunnerInterval = 30 * time.Second
	p.EnableMetric = false
	// Metric collection is disabled in dev mode.
	// p.MetricConnectionKey = "3zcZLeX3ahvlueEJqNyJysGfVAErsjjT"
	return p
}
