// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// chokePointExemptions lists the enclosing functions allowed to call
// Candidate.AddVote / Candidate.SubVote directly.
//
// Adding an entry here is a deliberate statement that the function changes a
// candidate's vote total without a corresponding per-(candidate, voter) view
// delta, and that leaving the view behind is correct at that site. Read the
// comment on the function before adding one.
var chokePointExemptions = map[string]string{
	// The choke point itself.
	"addCandidateVotes": "defines the choke point",
	"subCandidateVotes": "defines the choke point",
	// Rebuilds vote totals from scratch at historical revise heights only.
	// See the comment on VoteReviser.calculateVoteWeight.
	"calculateVoteWeight": "revise heights all predate IIP-59 activation",
}

// TestCandidateVoteMutationsUseChokePoint fails when a new call site changes a
// candidate's vote total without going through addCandidateVotes /
// subCandidateVotes.
//
// The IIP-59 VoterWeightView is a second derivation of the same quantity the
// candidate's Votes field holds. A site that updates one and not the other
// leaves them to drift, and nothing detects the drift until an era freeze bakes
// the wrong per-voter weights into a reward snapshot — long after the change
// that caused it. This test is what makes that coupling enforceable rather than
// merely documented.
func TestCandidateVoteMutationsUseChokePoint(t *testing.T) {
	r := require.New(t)

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	r.NoError(err)

	var offenders []string
	for _, pkg := range pkgs {
		for path, file := range pkg.Files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				if _, exempt := chokePointExemptions[fn.Name.Name]; exempt {
					continue
				}
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					sel, ok := call.Fun.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					if sel.Sel.Name != "AddVote" && sel.Sel.Name != "SubVote" {
						return true
					}
					offenders = append(offenders, strings.Join([]string{
						path, fn.Name.Name, sel.Sel.Name,
					}, ":"))
					return true
				})
			}
		}
	}

	r.Emptyf(offenders, "these call sites change a candidate's vote total without updating the "+
		"IIP-59 voter weight view: %v\n\nUse addCandidateVotes / subCandidateVotes instead. If the "+
		"site genuinely must not touch the view, add it to chokePointExemptions with the reason.",
		offenders)
}
