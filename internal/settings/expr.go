package settings

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"

	"cel.dev/cel-go/cel"
	"cel.dev/cel-go/common/cost"
	"cel.dev/cel-go/common/types"
)

// ExprModule is the type id of the CEL module, as listed in `requires`.
const ExprModule = "expr.cel/1"

// Limits of the expr.cel/1 module. They are part of the engine semantics: a
// change that could reject an expression accepted before needs a new engine.
const (
	MaxExprSource     = 512    // code points, checked before parsing
	MaxExprNodes      = 256    // AST nodes, including macro expansion
	MaxExprDepth      = 32     // parser recursion depth
	MaxEstimatedCost  = 10_000 // write-time estimate, upper bound
	MaxRuntimeCost    = 20_000 // evaluation budget
	lineupMaxKeys     = 4      // size hints used by the write-time estimate
	lineupMaxPlayers  = 30
	lineupMaxPlayerKV = 8
	lineupMaxKeyLen   = 32
)

// Rule error codes. A rule error is a result, not a failure: the same input
// always yields the same code, so a recompute reproduces it. The message of the
// underlying error is deliberately dropped, because it may change when cel-go
// is upgraded.
const (
	RuleErrRuntime    = "cel.runtime_error"
	RuleErrCostLimit  = "cel.cost_limit_exceeded"
	RuleErrNotInteger = "cel.result_not_int"
)

// RuleError is the deterministic outcome of an expression that failed at run
// time. Compare Code, never the text.
type RuleError struct{ Code string }

func (e *RuleError) Error() string { return "rule error: " + e.Code }

// Expr is a compiled expr.cel/1 expression, safe for concurrent use.
type Expr struct {
	source string
	prg    cel.Program
	// EstimatedCost is the upper bound of the write-time cost estimate.
	EstimatedCost uint64
}

// celEnvs holds one frozen CEL environment per engine version. The expression
// environment is named by `engine`, so old documents keep their semantics.
var celEnvs = map[string]func() *cel.Env{
	"2026.10": sync.OnceValue(func() *cel.Env {
		env, err := cel.NewEnv(
			// lineup.<group> is a list of players, each a map of integer
			// facts such as voto_puro (hundredths).
			cel.Variable("lineup", cel.MapType(cel.StringType, cel.ListType(cel.MapType(cel.StringType, cel.IntType)))),
			cel.ParserExpressionSizeLimit(MaxExprSource),
			cel.ExpressionNodeLimit(MaxExprNodes),
			cel.ParserRecursionLimit(MaxExprDepth),
		)
		if err != nil {
			panic(err) // a fixed environment: covered by tests
		}
		return env
	}),
}

// CompileExpr checks an expression against the environment of the given engine:
// source length, node count, declared variables, an `int` result, no `double`
// anywhere, and an estimated cost under MaxEstimatedCost.
func CompileExpr(engine, source string) (*Expr, error) {
	newEnv, ok := celEnvs[engine]
	if !ok {
		return nil, fmt.Errorf("%w: no expression environment for engine %q", ErrInvalid, engine)
	}
	// Cap the source before the parser sees it: the limit option alone would
	// still read an arbitrarily long string.
	if n := utf8.RuneCountInString(source); n > MaxExprSource {
		return nil, fmt.Errorf("%w: expression has %d characters, the limit is %d", ErrInvalid, n, MaxExprSource)
	}
	env := newEnv()
	ast, iss := env.Compile(source)
	if iss != nil && iss.Err() != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalid, iss.Err())
	}
	if out := ast.OutputType(); !out.IsExactType(cel.IntType) {
		return nil, fmt.Errorf("%w: expression must produce int, not %s", ErrInvalid, out)
	}
	for _, t := range ast.NativeRep().TypeMap() {
		if t.Kind() == types.DoubleKind {
			return nil, fmt.Errorf("%w: expression must not use double values", ErrInvalid)
		}
	}
	est, err := env.EstimateCost(ast, sizeHints{})
	if err != nil {
		return nil, fmt.Errorf("%w: cost estimate: %w", ErrInvalid, err)
	}
	if est.Max > MaxEstimatedCost {
		return nil, fmt.Errorf("%w: estimated cost %d is over the limit %d", ErrInvalid, est.Max, MaxEstimatedCost)
	}
	prg, err := env.Program(ast, cel.CostLimit(MaxRuntimeCost))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	return &Expr{source: source, prg: prg, EstimatedCost: est.Max}, nil
}

// Eval runs the expression. The error, if any, is always a *RuleError.
func (e *Expr) Eval(vars map[string]any) (int64, error) {
	out, _, err := e.prg.Eval(vars)
	if err != nil {
		if strings.Contains(err.Error(), "cost limit") {
			return 0, &RuleError{Code: RuleErrCostLimit}
		}
		return 0, &RuleError{Code: RuleErrRuntime}
	}
	n, ok := out.Value().(int64)
	if !ok { // unreachable after the compile-time type check; kept as a guard
		return 0, &RuleError{Code: RuleErrNotInteger}
	}
	return n, nil
}

// IsRuleError reports whether err is a rule error, and returns it.
func IsRuleError(err error) (*RuleError, bool) {
	var re *RuleError
	return re, errors.As(err, &re)
}

// sizeHints tells the cost estimator how big the declared variables can be.
// Group names are dynamic map keys, so paths are matched by shape:
//
//	[lineup]                       the groups
//	[lineup, g]                    one group: a list of players
//	[lineup, g, @items]            one player: a map of facts
//	[lineup, g, @items, @keys]     the name of a fact
type sizeHints struct{}

func (sizeHints) EstimateSize(n cost.AstNode) *cost.SizeEstimate {
	path := n.Path()
	if len(path) == 0 || path[0] != "lineup" {
		return nil
	}
	var r cost.SizeEstimate
	switch {
	case len(path) == 1:
		r = cost.RangedSizeEstimate(0, lineupMaxKeys)
	case len(path) == 2:
		r = cost.RangedSizeEstimate(0, lineupMaxPlayers)
	case len(path) == 3 && path[2] == "@items":
		r = cost.RangedSizeEstimate(0, lineupMaxPlayerKV)
	case len(path) == 4 && path[2] == "@items" && path[3] == "@keys":
		r = cost.RangedSizeEstimate(0, lineupMaxKeyLen)
	default:
		return nil
	}
	return &r
}

func (sizeHints) EstimateCallCost(string, string, *cost.AstNode, []cost.AstNode) *cost.CallEstimate {
	return nil
}
