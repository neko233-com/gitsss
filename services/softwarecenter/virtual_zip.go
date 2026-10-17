// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package softwarecenter

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type zipSegment struct {
	offset int64
	size   int64
	data   []byte
	path   string
}
type virtualZIP struct {
	segments []zipSegment
	size     int64
	offset   int64
	file     *os.File
	filePath string
}

// ZIP64 Store headers are virtual; only the requested existing blob ranges are read.
func newVirtualZIP(orgID int64, entries []ArchiveEntry) (*virtualZIP, error) {
	if len(entries) > MaxEntries {
		return nil, ErrQuota
	}
	v := &virtualZIP{}
	nameBytes := 0
	central := []byte{}
	appendBytes := func(data []byte) {
		v.segments = append(v.segments, zipSegment{offset: v.size, size: int64(len(data)), data: data})
		v.size += int64(len(data))
	}
	for _, entry := range entries {
		nameBytes += len(entry.Path)
		if nameBytes > 4*1024*1024 {
			return nil, ErrQuota
		}
		clean := strings.TrimSuffix(entry.Path, "/")
		if clean == "" || strings.ContainsAny(clean, "\\:\x00") || strings.HasPrefix(clean, "/") {
			return nil, ErrInvalid
		}
		for part := range strings.SplitSeq(clean, "/") {
			if part == "" || part == "." || part == ".." {
				return nil, ErrInvalid
			}
		}
		if len(entry.Path) > 65535 || entry.Size < 0 || (!entry.Directory && !hexDigest(entry.Digest)) {
			return nil, ErrInvalid
		}
		name := []byte(entry.Path)
		start := v.size
		stamp := time.Unix(entry.Modified, 0).UTC()
		year := max(1980, min(2107, stamp.Year()))
		dosTime := uint16(stamp.Hour()<<11 | stamp.Minute()<<5 | stamp.Second()/2)
		dosDate := uint16((year-1980)<<9 | int(stamp.Month())<<5 | stamp.Day())
		header := make([]byte, 30+len(name)+20)
		binary.LittleEndian.PutUint32(header, 0x04034b50)
		binary.LittleEndian.PutUint16(header[4:], 45)
		binary.LittleEndian.PutUint16(header[6:], 0x800)
		binary.LittleEndian.PutUint16(header[10:], dosTime)
		binary.LittleEndian.PutUint16(header[12:], dosDate)
		binary.LittleEndian.PutUint32(header[14:], entry.CRC32)
		binary.LittleEndian.PutUint32(header[18:], math.MaxUint32)
		binary.LittleEndian.PutUint32(header[22:], math.MaxUint32)
		binary.LittleEndian.PutUint16(header[26:], uint16(len(name)))
		binary.LittleEndian.PutUint16(header[28:], 20)
		copy(header[30:], name)
		extra := header[30+len(name):]
		binary.LittleEndian.PutUint16(extra, 1)
		binary.LittleEndian.PutUint16(extra[2:], 16)
		binary.LittleEndian.PutUint64(extra[4:], uint64(entry.Size))
		binary.LittleEndian.PutUint64(extra[12:], uint64(entry.Size))
		appendBytes(header)
		if !entry.Directory && entry.Size > 0 {
			v.segments = append(v.segments, zipSegment{offset: v.size, size: entry.Size, path: filepath.Join(root(orgID), "blobs", entry.Digest)})
			v.size += entry.Size
		}
		row := make([]byte, 46+len(name)+28)
		binary.LittleEndian.PutUint32(row, 0x02014b50)
		binary.LittleEndian.PutUint16(row[4:], 3<<8|45)
		binary.LittleEndian.PutUint16(row[6:], 45)
		binary.LittleEndian.PutUint16(row[8:], 0x800)
		binary.LittleEndian.PutUint16(row[12:], dosTime)
		binary.LittleEndian.PutUint16(row[14:], dosDate)
		binary.LittleEndian.PutUint32(row[16:], entry.CRC32)
		binary.LittleEndian.PutUint32(row[20:], math.MaxUint32)
		binary.LittleEndian.PutUint32(row[24:], math.MaxUint32)
		binary.LittleEndian.PutUint16(row[28:], uint16(len(name)))
		binary.LittleEndian.PutUint16(row[30:], 28)
		mode := uint32(0o100644 << 16)
		if entry.Directory {
			mode = 0o040755<<16 | 0x10
		}
		binary.LittleEndian.PutUint32(row[38:], mode)
		binary.LittleEndian.PutUint32(row[42:], math.MaxUint32)
		copy(row[46:], name)
		extra = row[46+len(name):]
		binary.LittleEndian.PutUint16(extra, 1)
		binary.LittleEndian.PutUint16(extra[2:], 24)
		binary.LittleEndian.PutUint64(extra[4:], uint64(entry.Size))
		binary.LittleEndian.PutUint64(extra[12:], uint64(entry.Size))
		binary.LittleEndian.PutUint64(extra[20:], uint64(start))
		central = append(central, row...)
	}
	start := v.size
	appendBytes(central)
	end64Offset := v.size
	end := make([]byte, 56+20+22)
	binary.LittleEndian.PutUint32(end, 0x06064b50)
	binary.LittleEndian.PutUint64(end[4:], 44)
	binary.LittleEndian.PutUint16(end[12:], 45)
	binary.LittleEndian.PutUint16(end[14:], 45)
	binary.LittleEndian.PutUint64(end[24:], uint64(len(entries)))
	binary.LittleEndian.PutUint64(end[32:], uint64(len(entries)))
	binary.LittleEndian.PutUint64(end[40:], uint64(len(central)))
	binary.LittleEndian.PutUint64(end[48:], uint64(start))
	binary.LittleEndian.PutUint32(end[56:], 0x07064b50)
	binary.LittleEndian.PutUint64(end[64:], uint64(end64Offset))
	binary.LittleEndian.PutUint32(end[72:], 1)
	binary.LittleEndian.PutUint32(end[76:], 0x06054b50)
	binary.LittleEndian.PutUint16(end[84:], math.MaxUint16)
	binary.LittleEndian.PutUint16(end[86:], math.MaxUint16)
	binary.LittleEndian.PutUint32(end[88:], math.MaxUint32)
	binary.LittleEndian.PutUint32(end[92:], math.MaxUint32)
	appendBytes(end)
	return v, nil
}

func (v *virtualZIP) Size() int64 { return v.size }
func (v *virtualZIP) Close() error {
	if v.file != nil {
		err := v.file.Close()
		v.file = nil
		v.filePath = ""
		return err
	}
	return nil
}

func (v *virtualZIP) Seek(offset int64, whence int) (int64, error) {
	base := int64(0)
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		base = v.offset
	case io.SeekEnd:
		base = v.size
	default:
		return 0, ErrInvalid
	}
	if offset < -base || offset > math.MaxInt64-base {
		return 0, ErrInvalid
	}
	v.offset = base + offset
	return v.offset, nil
}

func (v *virtualZIP) Read(p []byte) (int, error) {
	n, err := v.ReadAt(p, v.offset)
	v.offset += int64(n)
	return n, err
}

func (v *virtualZIP) ReadAt(p []byte, offset int64) (int, error) {
	if offset < 0 {
		return 0, ErrInvalid
	}
	if len(p) == 0 {
		return 0, nil
	}
	if offset >= v.size {
		return 0, io.EOF
	}
	total := 0
	for len(p) > 0 && offset < v.size {
		i := sort.Search(len(v.segments), func(i int) bool { return v.segments[i].offset+v.segments[i].size > offset })
		if i == len(v.segments) {
			return total, io.ErrUnexpectedEOF
		}
		seg := v.segments[i]
		within := offset - seg.offset
		length := min(int64(len(p)), seg.size-within)
		if seg.data != nil {
			copy(p[:length], seg.data[within:within+length])
		} else {
			if v.filePath != seg.path {
				if err := v.Close(); err != nil {
					return total, err
				}
				f, err := os.Open(seg.path)
				if err != nil {
					return total, err
				}
				v.file = f
				v.filePath = seg.path
				info, err := f.Stat()
				if err != nil {
					return total, err
				}
				if info.Size() != seg.size {
					return total, ErrConflict
				}
			}
			n, err := v.file.ReadAt(p[:length], within)
			if err != nil && !errors.Is(err, io.EOF) {
				return total, err
			}
			if int64(n) != length {
				return total, io.ErrUnexpectedEOF
			}
		}
		total += int(length)
		offset += length
		p = p[length:]
	}
	if len(p) > 0 {
		return total, io.EOF
	}
	return total, nil
}
