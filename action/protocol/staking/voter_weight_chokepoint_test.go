// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// chokePointExemptions lists the enclosing functions allowed to call
// Candidate.AddVote / Candidate.SubVote directly.
//
// Adding an entry here is a deliberate statement that the function may set a
// candidate's vote total outside the funnel, and that doing so is correct at
// that site. Since IIP-59 freezes Candidate.Votes as the era's TotalWeight
// denominator, an unfunnelled write moves the payout denominator directly.
// Read the comment on the function before adding one.
var chokePointExemptions = map[string]string{
	// The choke point itself.
	"addCandidateVotes": "defines the choke point",
	"subCandidateVotes": "defines the choke point",
	// Rebuilds vote totals from scratch at historical revise heights only.
	// See the comment on VoteReviser.calculateVoteWeight.
	"calculateVoteWeight": "revise heights all predate IIP-59 activation",
}

// votesFieldExemptions lists the sites allowed to touch Candidate.Votes
// directly — by assigning to it, or by calling a mutating big.Int method on it
// — rather than going through AddVote / SubVote and therefore through the
// choke point.
//
// Keyed by "file:function" because the field is touched from several functions
// that share a name with innocent ones elsewhere. Each entry is a claim that
// the site's write to Votes is not a per-voter stake delta at all (a codec, a
// clone, a whole-record move); if that stops being true the entry has to go,
// not the check.
var votesFieldExemptions = map[string]string{
	// The Candidate type's own accessors and codecs: AddVote/SubVote are what
	// the choke point delegates to, and the rest is (de)serialization of a
	// value that has not entered the view yet.
	"candidate.go:AddVote":   "is the primitive the choke point wraps",
	"candidate.go:SubVote":   "is the primitive the choke point wraps",
	"candidate.go:Encode":    "zeroes a clone before serializing, not live state",
	"candidate.go:Decode":    "restores a persisted total; no delta occurred",
	"candidate.go:fromProto": "restores a persisted total; no delta occurred",

	// Ownership transfer moves an entire candidate's votes between records.
	// The per-voter attributions do not change — the same voters keep the same
	// weights — only which record holds them.
	"handler_candidate_transfer_ownership.go:handleCandidateTransferOwnership": "moves totals between candidate records; per-voter weights are unchanged",

	// Rebuilds every total from scratch at historical revise heights, all of
	// which predate IIP-59 activation, so there is no view to keep in step.
	"vote_reviser.go:calculateVoteWeight": "revise heights all predate IIP-59 activation",

	// Adds contract-staking votes onto copies handed to the poll, not onto the
	// stored candidate; the view derives contract weights from its own hooks.
	"protocol.go:ActiveCandidates": "decorates a returned copy; stored candidate is untouched",

	// Test-only bench seeder. Not reachable from block execution.
	"perf_seeder.go:TestOnlySeedPerfBenchState": "test-only state seeder, not a consensus path",
}

// votesMutatingMethods are the big.Int methods that would change a vote total
// in place if called on Candidate.Votes.
var votesMutatingMethods = map[string]bool{
	"Add": true, "Sub": true, "Set": true,
	"SetInt64": true, "SetString": true, "SetUint64": true,
	"Mul": true, "Div": true, "Neg": true,
}

// TestCandidateVoteFieldMutationsUseChokePoint fails when a new site writes
// Candidate.Votes directly instead of going through addCandidateVotes /
// subCandidateVotes.
//
// TestCandidateVoteMutationsUseChokePoint catches the AddVote / SubVote route
// around the choke point; this catches the shorter one. `cand.Votes.Sub(...)`
// and `cand.Votes = x` bypass AddVote entirely, so the sibling test never sees
// them, and they drift the view in exactly the same silent way.
func TestCandidateVoteFieldMutationsUseChokePoint(t *testing.T) {
	r := require.New(t)

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	r.NoError(err)

	// isVotesSelector reports whether e is any expression ending in `.Votes`:
	// cand.Votes, list[i].Votes, candm[key].Votes.
	isVotesSelector := func(e ast.Expr) bool {
		sel, ok := e.(*ast.SelectorExpr)
		return ok && sel.Sel.Name == "Votes"
	}

	var offenders []string
	for _, pkg := range pkgs {
		for path, file := range pkg.Files {
			base := filepath.Base(path)
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				if _, exempt := votesFieldExemptions[base+":"+fn.Name.Name]; exempt {
					continue
				}
				report := func(n ast.Node, what string) {
					offenders = append(offenders, fmt.Sprintf(
						"%s:%d:%s: %s", base, fset.Position(n.Pos()).Line, fn.Name.Name, what,
					))
				}
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					switch node := n.(type) {
					case *ast.AssignStmt:
						for _, lhs := range node.Lhs {
							if isVotesSelector(lhs) {
								report(lhs, "assigns to .Votes")
							}
						}
					case *ast.CallExpr:
						sel, ok := node.Fun.(*ast.SelectorExpr)
						if !ok || !votesMutatingMethods[sel.Sel.Name] {
							return true
						}
						if isVotesSelector(sel.X) {
							report(node, "calls .Votes."+sel.Sel.Name)
						}
					}
					return true
				})
			}
		}
	}

	r.Emptyf(offenders, "these sites mutate Candidate.Votes directly, bypassing the IIP-59 "+
		"vote choke point: %v\n\nUse addCandidateVotes / subCandidateVotes instead. If the site "+
		"genuinely is not a per-voter stake delta, add it to votesFieldExemptions with the reason.",
		offenders)
}

// TestCandidateVoteMutationsUseChokePoint fails when a new call site changes a
// candidate's vote total without going through addCandidateVotes /
// subCandidateVotes.
//
// Under IIP-59 an era boundary freezes Candidate.Votes as that era's
// TotalWeight — the denominator every voter payout is divided by. The drain
// then recomputes each voter's numerator statelessly from the frozen buckets
// (FrozenVoterWeight). A site that moves Votes without a matching bucket
// mutation therefore moves the denominator away from the sum of the numerators,
// and nothing detects it until an era freeze bakes the skew into a reward
// snapshot — long after the change that caused it. Funnelling every stake delta
// through one pair of functions is what keeps that invariant checkable; see
// TestVoterWeightInvariant, which asserts it end to end.
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

	r.Emptyf(offenders, "these call sites change a candidate's vote total outside the IIP-59 "+
		"vote choke point: %v\n\nUse addCandidateVotes / subCandidateVotes instead. If the site "+
		"genuinely must bypass the funnel, add it to chokePointExemptions with the reason.",
		offenders)
}
