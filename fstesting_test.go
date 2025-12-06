package metricsfs

import (
	"testing"

	"github.com/absfs/absfs"
	"github.com/absfs/fstesting"
	"github.com/absfs/osfs"
)

// TestMetricsFS_WrapperSuite runs the standard fstesting wrapper suite.
// metricsfs is a transparent wrapper that collects metrics without
// transforming data or metadata.
func TestMetricsFS_WrapperSuite(t *testing.T) {
	// Create a real OS filesystem to wrap
	base, err := osfs.NewFS()
	if err != nil {
		t.Fatalf("failed to create base filesystem: %v", err)
	}

	suite := &fstesting.WrapperSuite{
		Factory: func(baseFS absfs.FileSystem) (absfs.FileSystem, error) {
			return New(baseFS), nil
		},
		BaseFS:         base,
		Name:           "metricsfs",
		TransformsData: false, // metricsfs passes data through unchanged
		TransformsMeta: false, // metricsfs passes metadata through unchanged
		ReadOnly:       false, // metricsfs supports writes
	}

	suite.Run(t)
}
