package filedao

import (
	"context"
	"sort"
)

type (
	fileV2Index struct {
		start, end uint64
		fd         *fileDAOv2
	}

	// FileV2Manager manages collection of v2 files
	FileV2Manager []*fileV2Index
)

// NewFileV2Manager creates an instance of FileV2Manager
func NewFileV2Manager(fds []*fileDAOv2) FileV2Manager {
	fm := make(FileV2Manager, len(fds))
	for i := range fds {
		fm[i] = &fileV2Index{fd: fds[i]}
	}
	return fm
}

// Start starts the FileV2Manager
func (fm FileV2Manager) Start(ctx context.Context) error {
	for i := range fm {
		fd := fm[i].fd
		if err := fd.Start(ctx); err != nil {
			return err
		}

		// check start/end height
		start, err := fd.Bottom()
		if err != nil {
			return err
		}

		end, err := fd.Height()
		if err != nil {
			return err
		}
		fm[i].start = start
		fm[i].end = end
	}
	sort.Slice(fm, func(i, j int) bool { return fm[i].start < fm[j].start })
	return nil
}

// Stop stops the FileV2Manager
func (fm FileV2Manager) Stop(ctx context.Context) error {
	for i := range fm {
		if err := fm[i].fd.Stop(ctx); err != nil {
			return err
		}
	}
	return nil
}

// FileDAOByHeight returns FileDAO for the given height
func (fm FileV2Manager) FileDAOByHeight(height uint64) FileDAO {
	top := len(fm) - 1
	if height >= fm[top].start {
		return fm[top].fd
	}

	left := 0
	right := len(fm) - 1
	for left <= right {
		mid := (left + right) / 2
		v := fm[mid]
		if v.start <= height && height <= v.end {
			return v.fd
		}
		if height < v.start {
			right = mid - 1
		} else {
			left = mid + 1
		}
	}
	return nil
}

// TopFd returns the top (with maximum height) v2 file
func (fm FileV2Manager) TopFd() FileDAO {
	return fm[len(fm)-1].fd
}
