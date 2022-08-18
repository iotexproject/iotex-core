// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package util

import (
	"bytes"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common/compiler"
	"github.com/pkg/errors"
)

var versionRegexp = regexp.MustCompile(`([0-9]+)\.([0-9]+)\.([0-9]+)`)

// Solidity contains information about the solidity compiler.
type Solidity struct {
	Path, Version, FullVersion string
	Major, Minor, Patch        int
	ExtraAllowedPath           []string
}

type Contract compiler.Contract

// SolidityVersion runs solc and parses its version output.
func SolidityVersion(solc string) (*Solidity, error) {
	if solc == "" {
		solc = "solc"
	}
	var out bytes.Buffer
	cmd := exec.Command(solc, "--version")
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	matches := versionRegexp.FindStringSubmatch(out.String())
	if len(matches) != 4 {
		return nil, errors.Errorf("can't parse solc version %q", out.String())
	}
	s := &Solidity{Path: cmd.Path, FullVersion: out.String(), Version: matches[0]}
	if s.Major, err = strconv.Atoi(matches[1]); err != nil {
		return nil, err
	}
	if s.Minor, err = strconv.Atoi(matches[2]); err != nil {
		return nil, err
	}
	if s.Patch, err = strconv.Atoi(matches[3]); err != nil {
		return nil, err
	}
	return s, nil
}

// CompileSolidityString builds and returns all the contracts contained within a source string.
func CompileSolidityString(solc, source string) (map[string]*compiler.Contract, error) {
	if len(source) == 0 {
		return nil, errors.New("solc: empty source string")
	}
	s, err := SolidityVersion(solc)
	if err != nil {
		return nil, err
	}
	return s.CompileSource(source)
}

// CompileSolidity compiles all given Solidity source files.
func CompileSolidity(solc string, sourcefiles ...string) (map[string]*compiler.Contract, error) {
	if len(sourcefiles) == 0 {
		return nil, errors.New("solc: no source files")
	}
	s, err := SolidityVersion(solc)
	if err != nil {
		return nil, err
	}

	return s.CompileFiles(sourcefiles...)
}

// CompileSource builds and returns all the contracts contained within a source string.
func (s *Solidity) CompileSource(source string) (map[string]*compiler.Contract, error) {
	args := append(s.makeArgs(), "--")
	cmd := exec.Command(s.Path, append(args, "-")...)
	cmd.Stdin = strings.NewReader(source)
	return s.run(cmd, source)
}

// CompileFiles compiles all given Solidity source files.
func (s *Solidity) CompileFiles(sourcefiles ...string) (map[string]*compiler.Contract, error) {
	source, err := slurpFiles(sourcefiles)
	if err != nil {
		return nil, err
	}
	args := append(s.makeArgs(), "--")
	cmd := exec.Command(s.Path, append(args, sourcefiles...)...)
	return s.run(cmd, source)
}

func (s *Solidity) run(cmd *exec.Cmd, source string) (map[string]*compiler.Contract, error) {
	var stderr, stdout bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, errors.Errorf("solc: %v\n%s", err, stderr.Bytes())
	}
	return compiler.ParseCombinedJSON(stdout.Bytes(), source, s.Version, s.Version, strings.Join(s.makeArgs(), " "))
}

func (s *Solidity) allowedPaths() string {
	paths := []string{".", "./", "../"} // default to support relative paths
	if len(s.ExtraAllowedPath) > 0 {
		paths = append(paths, s.ExtraAllowedPath...)
	}
	return strings.Join(paths, ", ")
}

func (s *Solidity) makeArgs() []string {
	p := []string{
		"--combined-json", "bin,bin-runtime,srcmap,srcmap-runtime,abi,userdoc,devdoc",
		"--optimize", // code optimizer switched on
		"--allow-paths", s.allowedPaths(),
	}
	if s.Major > 0 || s.Minor > 4 || s.Patch > 6 {
		p[1] += ",metadata,hashes"
	}
	return p
}

func slurpFiles(files []string) (string, error) {
	var concat bytes.Buffer
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		concat.Write(content)
	}
	return concat.String(), nil
}
