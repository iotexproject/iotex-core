// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/util/fileutil"
)

// SimpleFileSystem defines a Blockchain db based on file system
type SimpleFileSystem struct {
	rootDir   string
	subDirs   []string
	lifecycle lifecycle.Lifecycle
}

// NewSimpleFileSystem returns a file system with write and read feature
func NewSimpleFileSystem(rootDir string, subDirs ...string) *SimpleFileSystem {
	return &SimpleFileSystem{rootDir: rootDir, subDirs: subDirs}
}

// Start starts the blockchain
func (s *SimpleFileSystem) Start(ctx context.Context) error {
	if err := s.lifecycle.OnStart(ctx); err != nil {
		return err
	}
	if err := mkdir(s.rootDir); err != nil {
		return errors.Wrapf(
			err,
			"failed to create the root directory %s",
			s.rootDir,
		)
	}
	for _, subDir := range s.subDirs {
		if err := mkdir(s.fullPath(subDir)); err != nil {
			return errors.Wrapf(
				err,
				"failed to create sub directory %s",
				subDir,
			)
		}
	}

	return nil
}

// Stop stops the blockchain
func (s *SimpleFileSystem) Stop(ctx context.Context) error {
	return s.lifecycle.OnStop(ctx)
}

// Read returns the data stored in file under subDir
func (s *SimpleFileSystem) Read(reletiveFilePath string) ([]byte, error) {
	// TODO: retry on failure
	return ioutil.ReadFile(s.fullPath(reletiveFilePath))
}

// Write writes data into file
func (s *SimpleFileSystem) Write(reletiveFilePath string, data []byte) error {
	filepath := s.fullPath(reletiveFilePath)
	if fileutil.FileExists(filepath) {
		return errors.Errorf("file already exist", filepath)
	}

	return s.write(filepath, data)
}

// Overwrite overwrites data into file if the file already exists
func (s *SimpleFileSystem) Overwrite(reletiveFilePath string, data []byte) error {
	return s.write(s.fullPath(reletiveFilePath), data)
}

func (s *SimpleFileSystem) fullPath(filepath string) string {
	return path.Join(s.rootDir, filepath)
}

func (s *SimpleFileSystem) write(filepath string, data []byte) error {
	// TODO: retry on failure
	tmpFile, err := ioutil.TempFile(path.Dir(filepath), path.Base(filepath)+".tmp")
	if err != nil {
		return errors.Wrap(err, "failed to create tmp file")
	}
	n, err := tmpFile.Write(data)
	if err == nil && n < len(data) {
		err = io.ErrShortWrite
	}
	if err1 := tmpFile.Close(); err == nil {
		err = err1
	}
	if err != nil {
		return errors.Wrap(err, "failed to write block to tmp file")
	}
	if err := os.Rename(tmpFile.Name(), filepath); err != nil {
		return errors.Wrap(err, "failed to rename tmp block file")
	}

	return nil
}

func mkdir(dir string) error {
	stat, err := os.Stat(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := os.MkdirAll(dir, 0700); err != nil && !os.IsExist(err) {
			return err
		}
		if stat, err = os.Stat(dir); err != nil {
			return err
		}
	}
	if !stat.IsDir() {
		return errors.Errorf("%s is not a directory", dir)
	}
	return nil
}
