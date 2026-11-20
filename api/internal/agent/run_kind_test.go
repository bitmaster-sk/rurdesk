package agent

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runKindConstants returns every RunKind* constant declared in the constants package, so a kind
// added there without an implementation fails the registry test.
func runKindConstants(t *testing.T) map[string]string {
	t.Helper()
	paths, err := filepath.Glob("../constants/*.go")
	require.NoError(t, err)
	kinds := map[string]string{}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		require.NoError(t, err)
		ast.Inspect(file, func(node ast.Node) bool {
			spec, isValue := node.(*ast.ValueSpec)
			if !isValue {
				return true
			}
			for i, name := range spec.Names {
				if !strings.HasPrefix(name.Name, "RunKind") || i >= len(spec.Values) {
					continue
				}
				literal, isLiteral := spec.Values[i].(*ast.BasicLit)
				if !isLiteral || literal.Kind != token.STRING {
					continue
				}
				value, err := strconv.Unquote(literal.Value)
				require.NoError(t, err)
				kinds[name.Name] = value
			}
			return true
		})
	}
	require.NotEmpty(t, kinds)
	return kinds
}

func TestRunKinds_EveryKindHasAnImplementation(t *testing.T) {
	kinds := NewRunKinds(RunKindDeps{})
	for name, value := range runKindConstants(t) {
		t.Run(name, func(t *testing.T) {
			assert.NotPanics(t, func() { kinds.For(&model.AgentRun{Kind: value}) })
		})
	}
}

func TestRunKinds_UnknownKindPanics(t *testing.T) {
	assert.Panics(t, func() { NewRunKinds(RunKindDeps{}).For(&model.AgentRun{IdRun: 7, Kind: "review"}) })
}

// runKindRuleExceptions are the places outside the RunKind implementations allowed to tell kinds
// of run apart, with the reason.
var runKindRuleExceptions = map[string]string{
	"constants/agent.go":               "declares the kinds",
	"repository/":                      "SQL filters by kind, creates runs of a known kind, and mirrors every phase write onto the issue state",
	"controller/issue_controller.go":   "plans a run of a known kind",
	"service/release_service.go":       "plans a run of a known kind",
	"agent/notify_run.go":              "the run snapshot's parked flag compares with the issue's assignee",
	"controller/tracker_controller.go": "IdIssue of a time track filter, not of a run",
	"service/wiki_proposal_service.go": "IdIssue of a wiki proposal, not of a run",
	"model/attachment.go":              "IdIssue of an attachment scope, not of a run",
	"agent/run_kind.go":                "the registry",
	"agent/run_kind_chat.go":           "the chat implementation",
	"agent/run_kind_issue.go":          "the issue implementation",
	"agent/run_kind_release_notes.go":  "the release notes implementation",
	"agent/run_kind_planned.go":        "steps the issue and release notes implementations share",
	"agent/stage_payload_builder.go":   "payload steps the issue and release notes implementations share",
	"agent/agent_run_git_repo.go":      "repository step the implementations share",
	"agent/chat_payload_builder.go":    "the chat implementation's payload",
}

func isRunKindRuleException(path string) bool {
	for prefix := range runKindRuleExceptions {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// TestRunKindRule keeps run kind decisions inside the RunKind implementations: elsewhere no code
// names a run kind constant or tells runs apart by a nil IdIssue or IdRelease.
func TestRunKindRule(t *testing.T) {
	kindConstants := runKindConstants(t)
	root := ".."
	var violations []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if isRunKindRuleException(relative) {
			return nil
		}
		fileSet := token.NewFileSet()
		file, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			if reason := runKindDecision(node, kindConstants); reason != "" {
				violations = append(violations, relative+":"+strconv.Itoa(fileSet.Position(node.Pos()).Line)+" "+reason)
			}
			return true
		})
		return nil
	})
	require.NoError(t, err)
	assert.Empty(t, violations, "decide by run kind inside a RunKind implementation (internal/agent/run_kind*.go)")
}

func runKindDecision(node ast.Node, kindConstants map[string]string) string {
	switch expr := node.(type) {
	case *ast.SelectorExpr:
		if _, isKind := kindConstants[expr.Sel.Name]; isKind && isIdent(expr.X, "constants") {
			return "names constants." + expr.Sel.Name
		}
	case *ast.BinaryExpr:
		if expr.Op != token.EQL && expr.Op != token.NEQ {
			return ""
		}
		for _, side := range [][2]ast.Expr{{expr.X, expr.Y}, {expr.Y, expr.X}} {
			selector, isSelector := side[0].(*ast.SelectorExpr)
			if isSelector && (selector.Sel.Name == "IdIssue" || selector.Sel.Name == "IdRelease") && isIdent(side[1], "nil") {
				return "tests " + selector.Sel.Name + " against nil"
			}
		}
	}
	return ""
}

func isIdent(expr ast.Expr, name string) bool {
	ident, isIdentifier := expr.(*ast.Ident)
	return isIdentifier && ident.Name == name
}
