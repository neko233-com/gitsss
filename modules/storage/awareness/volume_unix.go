//go:build !windows

package awareness

import (
	"errors"
	"math"

	"golang.org/x/sys/unix"
)

type statfsInteger interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

func statfsUint64[T statfsInteger](value T) (uint64, bool) {
	if value < 0 {
		return 0, false
	}
	return uint64(value), true
}

func volumeStats(path string) (VolumeStats, error) {
	var stats unix.Statfs_t
	if err := unix.Statfs(path, &stats); err != nil {
		return VolumeStats{}, err
	}
	blockSize, blockSizeOK := statfsUint64(stats.Bsize)
	blocks, blocksOK := statfsUint64(stats.Blocks)
	availableBlocks, availableBlocksOK := statfsUint64(stats.Bavail)
	if !blockSizeOK || !blocksOK || !availableBlocksOK || blockSize == 0 {
		return VolumeStats{}, errors.New("invalid volume statistics")
	}
	if blocks > math.MaxInt64/blockSize || availableBlocks > math.MaxInt64/blockSize {
		return VolumeStats{}, errors.New("volume statistics overflow")
	}
	return VolumeStats{
		CapacityBytes: int64(blocks * blockSize), AvailableBytes: int64(availableBlocks * blockSize),
	}, nil
}
