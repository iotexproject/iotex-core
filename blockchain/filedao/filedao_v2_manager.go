package filedao

import (
	"context"
	"sort"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/db"
)

type (
	fileV2Index struct {
		start, end uint64
		fd         *fileDAOv2
	}

	// FileV2Manager manages collection of v2 files
	FileV2Manager struct {
		Indices []*fileV2Index
	}
)

// newFileV2Manager creates an instance of FileV2Manager
func newFileV2Manager(fds []*fileDAOv2) (*FileV2Manager, error) {
	if len(fds) == 0 {
		return nil, ErrNotSupported
	}

	fm := FileV2Manager{
		Indices: make([]*fileV2Index, len(fds)),
	}
	for i := range fds {
		fm.Indices[i] = &fileV2Index{fd: fds[i]}
	}
	return &fm, nil
}

// Start starts the FileV2Manager
func (fm *FileV2Manager) Start(ctx context.Context) error {
	for i := range fm.Indices {
		fd := fm.Indices[i].fd
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
		fm.Indices[i].start = start
		fm.Indices[i].end = end
	}
	sort.Slice(fm.Indices, func(i, j int) bool { return fm.Indices[i].start < fm.Indices[j].start })
	return nil
}

// Stop stops the FileV2Manager
func (fm *FileV2Manager) Stop(ctx context.Context) error {
	for i := range fm.Indices {
		if err := fm.Indices[i].fd.Stop(ctx); err != nil {
			return err
		}
	}
	return nil
}

// FileDAOByHeight returns FileDAO for the given height
func (fm *FileV2Manager) FileDAOByHeight(height uint64) BaseFileDAO {
	if height == 0 {
		return fm.Indices[0].fd
	}
	right := len(fm.Indices) - 1
	if height >= fm.Indices[right].start {
		return fm.Indices[right].fd
	}

	left := 0
	for left <= right {
		mid := (left + right) / 2
		v := fm.Indices[mid]
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

// GetBlockHeight returns height by hash
func (fm *FileV2Manager) GetBlockHeight(hash hash.Hash256) (uint64, error) {
	for _, file := range fm.Indices {
		if height, err := file.fd.GetBlockHeight(hash); err == nil {
			return height, nil
		}
	}
	return 0, db.ErrNotExist
}

// GetBlock returns block by hash
func (fm *FileV2Manager) GetBlock(hash hash.Hash256) (*block.Block, error) {
	for _, file := range fm.Indices {
		if blk, err := file.fd.GetBlock(hash); err == nil {
			return blk, nil
		}
	}
	return nil, db.ErrNotExist
}

// AddFileDAO add a new v2 file
func (fm *FileV2Manager) AddFileDAO(fd *fileDAOv2, start uint64) error {
	// update current top's end
	top := fm.Indices[len(fm.Indices)-1]
	end, err := top.fd.Height()
	if err != nil {
		return err
	}
	if start != end+1 {
		// new file's first block != current tip height
		return ErrInvalidTipHeight
	}

	top.end = end
	fm.Indices = append(fm.Indices, &fileV2Index{fd: fd, start: start, end: end})
	return nil
}

// TopFd returns the top (with maximum height) v2 file
func (fm *FileV2Manager) TopFd() (BaseFileDAO, uint64) {
	top := fm.Indices[len(fm.Indices)-1]
	return top.fd, top.start
}
