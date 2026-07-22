//go:build targ

package dev

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"pgregory.net/rapid"
)

// TestAnalyzeThinness_AcceptsCorpusShapes runs hermetic copies of the real
// corpus shapes (engram main.go/hugot.go and targ.go, cited in the plan)
// through analyzeThinness and requires zero violations for each.
func TestAnalyzeThinness_AcceptsCorpusShapes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		src  string
	}{
		{
			// engram main.go:40-49
			"engram RunCommand",
			`package p

func RunCommand(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir, cmd.Stdout, cmd.Stderr = dir, os.Stdout, os.Stderr
	return cmd.Run()
}
`,
		},
		{
			// engram main.go:129-140
			"engram StartSignalPulses",
			`package p

func StartSignalPulses(ctx context.Context, path string) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go pulse.Forward(ctx, path, sigs)
}
`,
		},
		{
			// engram main.go:124-128
			"engram OpenDebugFile",
			`package p

func OpenDebugFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
}
`,
		},
		{
			// engram main.go:99-111 — four lock closures with conversions
			"engram lock closures",
			`package p

func lockPrimitives() locks.Primitives {
	return locks.Primitives{
		Flock: func(fd uintptr, how int) error {
			return syscall.Flock(int(fd), how)
		},
		LockSH: func(fd uintptr) error {
			return syscall.Flock(int(fd), int(uint32(syscall.LOCK_SH)))
		},
		LockEX: func(fd uintptr) error {
			return syscall.Flock(int(fd), int(uint32(syscall.LOCK_EX)))
		},
		Unlock: func(fd int) error {
			return syscall.Flock(int(uintptr(fd)), syscall.LOCK_UN)
		},
	}
}
`,
		},
		{
			// engram main.go:56-91 minus WriteFileExcl — composite-literal return
			"engram fsPrimitives composite return",
			`package p

func fsPrimitives() deps.FS {
	return deps.FS{
		ReadFile:  os.ReadFile,
		WriteFile: os.WriteFile,
		MkdirAll:  os.MkdirAll,
		Remove:    os.Remove,
		Stat:      os.Stat,
		Glob:      filepath.Glob,
		TempDir:   os.TempDir,
	}
}
`,
		},
		{
			// engram hugot.go:28-49 — wrapper + type-assert arg + FuncLit-in-return
			// + captured-receiver call
			"engram hugot NewPipeline",
			`package p

func (h hugotRuntime) NewPipeline(session any, name string) (embed.Pipeline, error) {
	pipeline, err := hugotRT.NewPipeline(session.(*hugot.Session), name)
	if err != nil {
		return nil, err
	}
	return func(texts []string) ([][]float32, error) {
		return h.run(pipeline, texts)
	}, nil
}
`,
		},
		{
			// engram main() wiring — nested qualified calls, local calls as field
			// values, local composite literal, spread of a call result
			"engram main wiring",
			`package p

func main() {
	app.Run(cli.New(deps.Deps{
		FS:    fsPrimitives(),
		Locks: lockPrimitives(),
		Embed: hugotRuntime{},
	}), cliArgs()...)
}
`,
		},
		{
			// targ.go Checksum/Watch — closures returning bare local calls
			"targ Checksum and Watch closures",
			`package p

func Checksum(patterns ...string) func() (string, error) {
	return func() (string, error) {
		return Match(patterns...)
	}
}

func Watch(patterns ...string) func() ([]string, error) {
	return func() ([]string, error) {
		return Changed(patterns...)
	}
}
`,
		},
		{
			// targ.go Register — binary-op argument
			"targ Register binary-op arg",
			`package p

func Register(opts ...core.Option) error {
	return core.Register(core.CallerSkipPublicAPI+1, opts...)
}
`,
		},
		{
			// targ.go const/var blocks
			"targ const and var blocks",
			`package p

const (
	KindString = core.KindString
	KindBool   = core.KindBool
)

var (
	ErrHelp = core.ErrHelp
	Default = core.NewDefault()
)
`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			g.Expect(analyzeSrc(t, c.src)).To(BeEmpty())
		})
	}
}

// TestAnalyzeThinness_CompositeLitForLoop verifies a composite-literal return
// with an embedded FuncLit containing a for-loop yields exactly that FuncLit
// violation.
func TestAnalyzeThinness_CompositeLitForLoop(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	src := `package p

func hooks() pkg.Hooks {
	return pkg.Hooks{
		OnTick: func() {
			for { // marker
			}
		},
	}
}
`

	violations := analyzeSrc(t, src)
	g.Expect(violations).To(HaveLen(1))
	g.Expect(violations[0].Line).To(Equal(fixtureLine(t, src, "// marker")))
	g.Expect(violations[0].Name).To(HaveSuffix("(closure)"))
	g.Expect(violations[0].Reason).To(ContainSubstring("for statement"))
}

// TestAnalyzeThinness_MultiResultReturn verifies every result of a
// multi-result return is validated: "return pkg.F(), m[k]" fails on the
// second result.
func TestAnalyzeThinness_MultiResultReturn(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	src := `package p

func f() (int, int) {
	return pkg.F(), m[k] // marker
}
`

	violations := analyzeSrc(t, src)
	g.Expect(violations).To(HaveLen(1))
	g.Expect(violations[0].Line).To(Equal(fixtureLine(t, src, "// marker")))
	g.Expect(violations[0].Name).To(Equal("f"))
	g.Expect(violations[0].Reason).To(ContainSubstring("index expression"))
}

// TestAnalyzeThinness_VarFuncLit verifies top-level "var X = func() {...}"
// both ways: a linear closure body passes; a loop inside fails with the
// "var X (closure)" naming convention.
func TestAnalyzeThinness_VarFuncLit(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	pass := `package p

var X = func() { pkg.F() }
`
	g.Expect(analyzeSrc(t, pass)).To(BeEmpty())

	fail := `package p

var X = func() {
	for { // marker
	}
}
`

	violations := analyzeSrc(t, fail)
	g.Expect(violations).To(HaveLen(1))
	g.Expect(violations[0].Line).To(Equal(fixtureLine(t, fail, "// marker")))
	g.Expect(violations[0].Name).To(Equal("var X (closure)"))
	g.Expect(violations[0].Reason).To(ContainSubstring("for statement"))
}

// TestAnalyzeThinness_WriteFileExcl runs the issue's must-FAIL corpus shape
// (engram main.go:69-89) and requires exactly one violation: at the
// compound-condition if, named as a closure, with a reason naming the guard
// shape.
func TestAnalyzeThinness_WriteFileExcl(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	src := `package p

func writers() deps.Writers {
	return deps.Writers{
		WriteFileExcl: func(path string, data []byte) error {
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
			if err != nil {
				return err
			}
			_, err = f.Write(data)
			closeErr := f.Close()
			if closeErr != nil && err == nil { // marker
				err = closeErr
			}
			return err
		},
	}
}
`

	violations := analyzeSrc(t, src)
	g.Expect(violations).To(HaveLen(1))
	g.Expect(violations[0].Line).To(Equal(fixtureLine(t, src, "// marker")))
	g.Expect(violations[0].Name).To(HaveSuffix("(closure)"))
	g.Expect(violations[0].Reason).To(ContainSubstring("compound condition"))
}

// TestCheckLinearBody_ClosureNaming verifies the violation model's closure
// flag: violations found inside a FuncLit body are flagged so callers can
// append " (closure)" to the enclosing declaration's name; top-level
// violations are not flagged.
func TestCheckLinearBody_ClosureNaming(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	violation := checkLinearBody(parseBodyStmts(t, `for {}`))
	g.Expect(violation).NotTo(BeNil())
	g.Expect(violation.closure).To(BeFalse(), "top-level violation must not carry the closure flag")

	violation = checkLinearBody(parseBodyStmts(t, `x := func() { for {} }`))
	g.Expect(violation).NotTo(BeNil())
	g.Expect(violation.closure).To(BeTrue(), "violation inside a FuncLit must carry the closure flag")

	violation = checkLinearBody(parseBodyStmts(t, `x := func() { y := func() { for {} }; _ = y }`))
	g.Expect(violation).NotTo(BeNil())
	g.Expect(violation.closure).To(BeTrue(), "violation inside a nested FuncLit must carry the closure flag")
}

// TestCheckLinearBody_Grammar covers every statement rule S1-S6 of the linear
// thin-body grammar, every forbidden statement kind, and the error-guard
// tightening cases. wantReason "" means the body is allowed (nil violation);
// otherwise the violation's reason must contain wantReason and its node must
// carry a valid position (T3 derives violation lines from it).
func TestCheckLinearBody_Grammar(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		body       string
		wantReason string
	}{
		// S1: assignments
		{"define", `x := 1`, ""},
		{"assign qualified call", `x = pkg.F()`, ""},
		{"tuple define", `a, b := pkg.F()`, ""},
		{"blank assign", `_ = pkg.F()`, ""},
		{"field assign", `cmd.Dir = d`, ""},
		{"tuple field assign", `cmd.Dir, cmd.Stdout, cmd.Stderr = dir, os.Stdout, os.Stderr`, ""},
		{"index assign target", `m[k] = v`, "not allowed as assignment target"},
		{"deref assign target", `*p = v`, "not allowed as assignment target"},
		{"compound assign operator", `x += 1`, "assignment operator"},
		{"assign rhs index", `x := m[k]`, "index expression"},
		// S2: var/const declarations
		{"var decl", `var x = pkg.F()`, ""},
		{"var decl no value", `var x pkg.T`, ""},
		{"const decl", `const c = 1`, ""},
		{"var decl bad value", `var x = m[k]`, "index expression"},
		{"type decl", `type T int`, "type declaration"},
		// S3: expression statements
		{"call statement", `pkg.F(x)`, ""},
		{"local call statement", `localF()`, ""},
		{"iife statement", `func() {}()`, "function literal call"},
		{"receive statement", `<-ch`, "must be a call"},
		{"bare ident statement", `x`, "must be a call"},
		// S4: go statements — qualified heads only
		{"go qualified call", `go pkg.F(x)`, ""},
		{"go method call", `go recv.M()`, ""},
		{"go generic qualified call", `go pkg.F[int](x)`, ""},
		{"go local call", `go localFunc()`, "go statement must launch a qualified call"},
		{"go func literal", `go func() {}()`, "go statement must launch a qualified call"},
		{"go generic local call", `go localF[int](x)`, "go statement must launch a qualified call"},
		{"go chained selector head", `go a.b.C()`, "call head must qualify a plain identifier"},
		{"go bad arg", `go pkg.F(m[k])`, "index expression"},
		// S5: error guards
		{"error guard", "if err != nil {\nreturn err\n}", ""},
		{"error guard selector", "if out.Err != nil {\nreturn out.Err\n}", ""},
		{"error guard reversed operands", "if nil != err {\nreturn err\n}", ""},
		{"error guard wrapped return", "if err != nil {\nreturn fmt.Errorf(\"x: %w\", err)\n}", ""},
		{"guard compound condition", "if closeErr != nil && err == nil {\nerr = closeErr\n}", "compound condition"},
		{"guard body not lone return", "if err != nil {\nlog()\nreturn err\n}", "lone return"},
		{"guard body assignment", "if err != nil {\nerr = closeErr\n}", "lone return"},
		{"guard empty body", "if err != nil {\n}", "lone return"},
		{"guard nil against nil", "if nil != nil {\nreturn err\n}", "nil comparison"},
		{"guard non-nil comparison", "if a != b {\nreturn x\n}", "nil comparison"},
		{"guard equality comparison", "if err == nil {\nreturn x\n}", "nil comparison"},
		{"guard non-binary condition", "if ok {\nreturn x\n}", "nil comparison"},
		{"guard with else", "if err != nil {\nreturn err\n} else {\nreturn nil\n}", "else clause"},
		{"guard with init", "if err := pkg.F(); err != nil {\nreturn err\n}", "init clause"},
		{"guard operand recursion", "if m[k].err != nil {\nreturn err\n}", "index expression"},
		{"guard return bad result", "if err != nil {\nreturn m[k]\n}", "index expression"},
		// S6: returns
		{"lone return", `return pkg.F()`, ""},
		{"empty return", `return`, ""},
		{"multi result return", `return pkg.F(), nil`, ""},
		{"return bad result", `return m[k]`, "index expression"},
		{"return second result bad", `return pkg.F(), m[k]`, "index expression"},
		{"mid-body return", "x := pkg.F()\nreturn x\npkg.G()", "mid-body return"},
		// Multi-statement linear bodies
		{"empty body", ``, ""},
		{
			"run command shape",
			"cmd := exec.Command(name, args...)\ncmd.Dir, cmd.Stdout, cmd.Stderr = dir, os.Stdout, os.Stderr\nreturn cmd.Run()",
			"",
		},
		{
			"assign guard return shape",
			"result, err := pkg.F(x)\nif err != nil {\nreturn nil, err\n}\nreturn result, nil",
			"",
		},
		// Forbidden statement kinds
		{"for statement", `for {}`, "for statement"},
		{"range statement", "for range xs {\n}", "range statement"},
		{"switch statement", "switch x {\n}", "switch statement"},
		{"type switch statement", "switch x.(type) {\n}", "switch statement"},
		{"select statement", `select {}`, "select statement"},
		{"defer statement", `defer pkg.F()`, "defer statement"},
		{"channel send statement", `ch <- v`, "channel send statement"},
		{"increment statement", `x++`, "increment/decrement statement"},
		{"decrement statement", `x--`, "increment/decrement statement"},
		{"labeled statement", "L:\npkg.F(x)", "labeled statement"},
		{"branch statement", `goto L`, "branch statement"},
		{"bare block statement", "{\nx := 1\n}", "BlockStmt"},
		// FuncLit recursion: closure bodies are walked by this same grammar
		{"funclit linear body", `x := func() { pkg.F(y) }`, ""},
		{"funclit with loop", `x := func() { for {} }`, "for statement"},
		{"funclit in return", `return func() { for {} }`, "for statement"},
		{"funclit in call arg", `pkg.F(func() { for {} })`, "for statement"},
		{"funclit mid-body return", `x := func() { return; pkg.F(y) }`, "mid-body return"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			violation := checkLinearBody(parseBodyStmts(t, c.body))
			if c.wantReason == "" {
				g.Expect(violation).To(BeNil())

				return
			}

			g.Expect(violation).NotTo(BeNil())
			g.Expect(violation.reason).To(ContainSubstring(c.wantReason))
			g.Expect(violation.node).NotTo(BeNil())
			g.Expect(violation.node.Pos().IsValid()).To(BeTrue(), "violation node must carry a position")
		})
	}
}

// TestCheckLinearBody_NilBody verifies nil statement lists (interface
// methods have nil bodies) are thin.
func TestCheckLinearBody_NilBody(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	g.Expect(checkLinearBody(nil)).To(BeNil())
}

// TestCheckLinearExpr_Grammar covers every expression rule E1-E10 of the
// linear thin-body grammar and every explicitly forbidden expression kind.
// wantReason "" means the expression is allowed (nil violation); otherwise
// the violation's reason must contain wantReason and its node must carry a
// valid position (T3 derives violation lines from it).
func TestCheckLinearExpr_Grammar(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		src        string
		wantReason string
	}{
		// E1: Ident, BasicLit
		{"ident", `x`, ""},
		{"nil ident", `nil`, ""},
		{"bool ident", `true`, ""},
		{"int literal", `42`, ""},
		{"string literal", `"hello"`, ""},
		// E2: SelectorExpr chains
		{"selector", `out.Embeddings`, ""},
		{"selector chain", `a.b.c`, ""},
		// E3a: qualified call heads
		{"qualified call", `pkg.F(x)`, ""},
		{"method call", `recv.M(1, "s")`, ""},
		// E3b: bare-Ident call heads
		{"builtin conversion", `int(fd)`, ""},
		{"local call", `fsPrimitives()`, ""},
		// E3c: generic instantiation heads (index args are type positions)
		{"generic call", `pkg.F[int](x)`, ""},
		{"generic call two type args", `pkg.F[int, string](x)`, ""},
		{"generic local call", `localF[pkg.T](x)`, ""},
		// E3: argument recursion, ellipsis spread, make's type argument
		{"nested qualified calls", `pkg.F(other.G(y), z)`, ""},
		{"ellipsis spread", `pkg.F(xs...)`, ""},
		{"make chan", `make(chan os.Signal, 1)`, ""},
		{"make map", `make(map[string]int)`, ""},
		{"make slice", `make([]byte, n)`, ""},
		// E4: composite literals
		{"composite literal", `pkg.T{A: 1, B: pkg.F(x)}`, ""},
		{"composite nested elided type", `[]pkg.T{{A: 1}}`, ""},
		{"map literal", `map[string]int{"a": 1}`, ""},
		{"array literal ellipsis length", `[...]int{1, 2}`, ""},
		// E5: FuncLit (body walked by the statement grammar)
		{"funclit walked", `func() { x := 1 }`, ""}, // body delegated to checkLinearBody
		// E6: allowed unary operators
		{"address of", `&x`, ""},
		{"unary plus", `+1`, ""},
		{"unary minus", `-1`, ""},
		{"unary xor", `^x`, ""},
		{"unary not", `!ok`, ""},
		{"address of composite", `&pkg.T{A: 1}`, ""},
		// E7: binary expressions, any operator
		{"flag or", `os.O_APPEND | os.O_CREATE`, ""},
		{"caller skip add", `core.CallerSkipPublicAPI + 1`, ""},
		{"comparison", `a == b`, ""},
		// E8: ParenExpr
		{"paren", `(x)`, ""},
		// E9: type assertions (asserted type is a type position)
		{"type assert pointer", `session.(*hugot.Session)`, ""},
		{"type assert interface", `x.(interface{ M() })`, ""},
		{"type assert func type", `x.(func(int) error)`, ""},
		{"type assert struct type", `x.(struct{ A int })`, ""},
		{"type assert map type", `x.(map[string]pkg.T)`, ""},

		// Forbidden: index and slice expressions in value position
		{"index expr", `m[k]`, "index expression"},
		{"slice expr", `s[i:j]`, "slice expression"},
		{"full slice expr", `s[i:j:k]`, "slice expression"},
		// Forbidden: value dereference
		{"value deref", `*p`, "dereference"},
		// Forbidden: receive and any other unary operator outside & + - ^ !
		{"receive", `<-ch`, "unary operator"},
		// Forbidden: IIFE (FuncLit call head)
		{"iife", `func() {}()`, "function literal call"},
		// Forbidden: call heads outside E3a-c
		{"chained selector call head", `a.b.C(x)`, "call head"},
		{"nested generic call head", `f[T][U](x)`, "call head"},
		// Forbidden: type nodes in value position (E10 is type-position only),
		// each named human-readably in the violation reason
		{"bare map type in value position", `map[string]int`, "map type not allowed"},
		{"bare chan type in value position", `chan int`, "channel type not allowed"},
		{"bare array type in value position", `[]int`, "array type not allowed"},
		{"bare func type in value position", `func(int) error`, "function type not allowed"},
		{"bare interface type in value position", `interface{ M() }`, "interface type not allowed"},
		{"bare struct type in value position", `struct{ A int }`, "struct type not allowed"},
		// Forbidden: non-type nodes in type positions (E10)
		{"call in make type position", `make(f(), 1)`, "type position"},
		{"index in make type position", `make(pkg.List[int], 1)`, "type position"},
		// Forbidden kinds nested inside allowed shapes (recursion)
		{"index in call arg", `pkg.F(m[k])`, "index expression"},
		{"index in composite value", `pkg.T{A: m[k]}`, "index expression"},
		{"index in binary operand", `x + m[k]`, "index expression"},
		{"index under address of", `&m[k]`, "index expression"},
		{"index under selector", `m[k].Field`, "index expression"},
		{"index in type assert operand", `m[k].(pkg.T)`, "index expression"},
		{"index in paren", `(m[k])`, "index expression"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			expr, err := parser.ParseExpr(c.src)
			g.Expect(err).NotTo(HaveOccurred(), "fixture %q must parse", c.src)

			violation := checkLinearExpr(expr)
			if c.wantReason == "" {
				g.Expect(violation).To(BeNil())

				return
			}

			g.Expect(violation).NotTo(BeNil())
			g.Expect(violation.reason).To(ContainSubstring(c.wantReason))
			g.Expect(violation.node).NotTo(BeNil())
			g.Expect(violation.node.Pos().IsValid()).To(BeTrue(), "violation node must carry a position")
		})
	}
}

// TestProperty_ThinGrammar is the plan's T4 property test: any body composed
// from allowed-statement templates (S1-S6 pass shapes) yields zero
// violations, and injecting one forbidden statement at a random position
// yields at least one violation whose Line matches the injected statement's
// first line.
func TestProperty_ThinGrammar(t *testing.T) {
	t.Parallel()

	thin := []string{
		`x := pkg.F(a)`,                        // S1 define
		`cmd.Dir, cmd.Stdout = dir, os.Stdout`, // S1 tuple field assign
		`var v = pkg.G(b)`,                     // S2 var decl
		`pkg.F(x)`,                             // S3 qualified call
		`localF()`,                             // S3 local call
		`go pkg.F(x)`,                          // S4 qualified go
		"if err != nil {\n\treturn err\n}",     // S5 error guard
		`h := func() { pkg.F(y) }`,             // S1 rhs FuncLit (E5)
	}
	forbidden := []string{
		"for {\n}",
		`defer pkg.F()`,
		"if closeErr != nil && err == nil {\n\terr = closeErr\n}",
		`ch <- v`,
		`x++`,
		`return`, // mid-body once injected before the final statement
	}

	// Capture what needs the real *testing.T once, before rapid shadows t;
	// each iteration overwrites the same fixture file in this dir.
	path := filepath.Join(t.TempDir(), "fixture.go")

	rapid.Check(t, func(t *rapid.T) {
		g := NewWithT(t)

		body := rapid.SliceOfN(rapid.SampledFrom(thin), 1, 5).Draw(t, "body")
		if rapid.Bool().Draw(t, "trailingReturn") {
			body = append(body, `return pkg.F()`)
		}

		g.Expect(os.WriteFile(path, []byte(fixtureSrc(body)), 0o600)).To(Succeed())
		violations, err := analyzeThinness(path)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(violations).To(BeEmpty(),
			"a body of allowed templates must yield zero violations")

		pos := rapid.IntRange(0, len(body)-1).Draw(t, "pos")
		bad := rapid.SampledFrom(forbidden).Draw(t, "bad")
		injected := slices.Insert(slices.Clone(body), pos, bad)

		g.Expect(os.WriteFile(path, []byte(fixtureSrc(injected)), 0o600)).To(Succeed())
		violations, err = analyzeThinness(path)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(violations).To(
			ContainElement(HaveField("Line", fixtureStmtLine(injected, pos))),
			"a violation must land on the injected statement's first line")
	})
}

// TestSortedViolationFiles verifies checkThinAPI's cross-file report order is
// deterministic: the helper returns the by-file grouping's keys in sorted
// (lexical path) order regardless of map-iteration order, and leaves the
// per-file violation slices untouched (source order preserved).
func TestSortedViolationFiles(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	byFile := map[string][]thinViolation{
		"z.go":     {{File: "z.go", Line: 1}},
		"a/sub.go": {{File: "a/sub.go", Line: 3}, {File: "a/sub.go", Line: 7}},
		"m.go":     {{File: "m.go", Line: 2}},
	}

	g.Expect(sortedViolationFiles(byFile)).To(Equal([]string{"a/sub.go", "m.go", "z.go"}))
	g.Expect(byFile["a/sub.go"]).To(Equal([]thinViolation{
		{File: "a/sub.go", Line: 3},
		{File: "a/sub.go", Line: 7},
	}), "per-file violations must keep their source order")
}

// analyzeSrc writes the fixture source to a temp .go file, runs
// analyzeThinness on it, and returns the violations — the plan's T3 helper.
func analyzeSrc(t *testing.T, src string) []thinViolation {
	t.Helper()
	g := NewWithT(t)

	path := filepath.Join(t.TempDir(), "fixture.go")
	g.Expect(os.WriteFile(path, []byte(src), 0o600)).To(Succeed())

	violations, err := analyzeThinness(path)
	g.Expect(err).NotTo(HaveOccurred())

	return violations
}

// fixtureLine returns the 1-based line number of the first fixture line
// containing marker, so tests assert violation lines without hardcoding.
func fixtureLine(t *testing.T, src, marker string) int {
	t.Helper()

	for i, line := range strings.Split(src, "\n") {
		if strings.Contains(line, marker) {
			return i + 1
		}
	}

	t.Fatalf("marker %q not found in fixture", marker)

	return 0
}

// fixtureSrc wraps body statement templates in the property fixture's
// function shell; the first statement lands on line 4 (see fixtureStmtLine).
func fixtureSrc(stmts []string) string {
	return "package p\n\nfunc f() {\n" + strings.Join(stmts, "\n") + "\n}\n"
}

// fixtureStmtLine returns the 1-based source line where statement i of a
// fixtureSrc body begins: 3 shell header lines, then each preceding
// (possibly multi-line) template.
func fixtureStmtLine(stmts []string, i int) int {
	line := 4
	for _, s := range stmts[:i] {
		line += strings.Count(s, "\n") + 1
	}

	return line
}

// parseBodyStmts parses the given statements as the body of a fixture
// function ("package p\nfunc f() { ... }") and returns the body's statement
// list — the plan's T2 fixture pattern.
func parseBodyStmts(t *testing.T, body string) []ast.Stmt {
	t.Helper()
	g := NewWithT(t)

	src := "package p\nfunc f() {\n" + body + "\n}"

	file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", src, 0)
	g.Expect(err).NotTo(HaveOccurred(), "fixture %q must parse", body)

	fn, ok := file.Decls[0].(*ast.FuncDecl)
	g.Expect(ok).To(BeTrue(), "fixture must contain a FuncDecl")

	return fn.Body.List
}
