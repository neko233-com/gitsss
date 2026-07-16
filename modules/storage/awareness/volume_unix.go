//go:build !windows

package awareness

import (
	"errors"
	"math"

	"golang.org/x/sys/unix"
)

func volumeStats(path string) (VolumeStats, error) {
	var stats unix.Statfs_t
	if err := unix.Statfs(path, &stats); err != nil {
		return VolumeStats{}, err
	}
	blockSize := uint64(stats.Bsize)
	blocks := stats.Blocks
	availableBlocks := stats.Bavail
	if blockSize == 0 || blocks > math.MaxInt64/blockSize || availableBlocks > math.MaxInt64/blockSize {
		return VolumeStats{}, errors.New("volume statistics overflow")
	}
	return VolumeStats{
		CapacityBytes: int64(blocks * blockSize), AvailableBytes: int64(availableBlocks * blockSize),
	}, nil
}
