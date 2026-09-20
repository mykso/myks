package myks

import (
	"fmt"
	"maps"
	"math/big"
	"path/filepath"
	"slices"
	"strings"

	"go.starlark.net/syntax"
)

// The ytt computation of a data file is Starlark, and KCL is Starlark-shaped: the same
// literals, the same operators, the same comprehensions. What ytt computes standalone is
// therefore carried over as the derivation it was instead of the value it produced — the
// file's `#@` prelude becomes module-level variables, a `load()` of the repo's ytt library
// becomes an import of that library translated to KCL, and each `key: #@ expr` becomes a KCL
// expression. Every translation is proved by evaluating it (verifyDerivations); an expression
// this package cannot translate, or one KCL does not reproduce, falls back to the literal.

// starSyntax parses ytt's Starlark. The defaults are what ytt itself allows; the `end`
// keyword that closes a ytt code block needs no option of its own.
var starSyntax = &syntax.FileOptions{}

// derivations is the KCL translation of the ytt computation of one or more data files.
type derivations struct {
	// exprs maps a dotted value path (".application.port") to the KCL expression it is
	// written as.
	exprs map[string]string
	// prelude holds the module-level KCL statements those expressions read, in source order.
	prelude []string
	// imports holds the KCL import statements they need.
	imports []string
}

func (d *derivations) has() bool { return d != nil && len(d.exprs) > 0 }

// hasPath reports whether the value at one dotted path is written as a derivation.
func (d *derivations) hasPath(path string) bool {
	if d == nil {
		return false
	}
	_, ok := d.exprs[path]
	return ok
}

// expr returns the KCL expression one dotted path is written as, if any.
func (d *derivations) expr(path string) (string, bool) {
	if d == nil || path == "" {
		return "", false
	}
	expr, ok := d.exprs[path]
	return expr, ok
}

// mergeDerivations merges the translations of the files contributing to one generated file.
// A part whose prelude would redefine a variable another part bound differently is dropped
// whole: its values fall back to the literals ytt resolved.
func mergeDerivations(parts ...*derivations) *derivations {
	out := &derivations{exprs: map[string]string{}}
	bound := map[string][]string{}
	seen := map[string]bool{}
	for _, part := range parts {
		if !part.has() {
			continue
		}
		vars := preludeVars(part.prelude)
		conflict := false
		for name, stmts := range vars {
			if existing, ok := bound[name]; ok && !slices.Equal(existing, stmts) {
				conflict = true
			}
		}
		if conflict {
			continue
		}
		maps.Copy(bound, vars)
		maps.Copy(out.exprs, part.exprs)
		for _, stmt := range part.prelude {
			if !seen[stmt] {
				seen[stmt] = true
				out.prelude = append(out.prelude, stmt)
			}
		}
		for _, imp := range part.imports {
			if !slices.Contains(out.imports, imp) {
				out.imports = append(out.imports, imp)
			}
		}
	}
	slices.Sort(out.imports)
	if len(out.exprs) == 0 {
		return nil
	}
	return out
}

// preludeVars groups prelude statements by the variable they assign. A variable assigned more
// than once (the loop rewrite appends to a list) keeps all of its statements.
func preludeVars(prelude []string) map[string][]string {
	vars := map[string][]string{}
	for _, stmt := range prelude {
		if name, _, ok := strings.Cut(stmt, " = "); ok {
			vars[name] = append(vars[name], stmt)
		}
	}
	return vars
}

// yttLib is the KCL translation of one file of the repo's ytt library: the functions whose
// body is a single expression, which a KCL lambda states as it is.
type yttLib struct {
	name   string          // the file stem, which is also the name of the generated .k file
	funcs  map[string]bool // the functions the translation exports
	source string          // the content of the generated lib file
}

// translateYttLib translates the single-expression functions of one ytt library file. A
// function whose body is more than a `return`, and every other top-level statement, is left
// behind: the values it computes stay literals.
func translateYttLib(path string, content []byte) *yttLib {
	file, err := starSyntax.Parse(path, content, 0)
	if err != nil {
		return nil
	}
	lib := &yttLib{name: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), funcs: map[string]bool{}}
	var b strings.Builder
	for _, stmt := range file.Stmts {
		def, ok := stmt.(*syntax.DefStmt)
		if !ok || len(def.Body) != 1 {
			continue
		}
		ret, ok := def.Body[0].(*syntax.ReturnStmt)
		if !ok || ret.Result == nil {
			continue
		}
		lambda, ok := newStarScope(nil, "", "").lambda(def, ret)
		if !ok {
			continue
		}
		lib.funcs[def.Name.Name] = true
		fmt.Fprintf(&b, "\n%s = %s\n", sanitizeKclIdentifier(def.Name.Name), lambda)
	}
	if len(lib.funcs) == 0 {
		return nil
	}
	lib.source = b.String()
	return lib
}

// starScope translates Starlark to KCL against the names a data file's prelude binds.
type starScope struct {
	// names maps a Starlark name to the KCL expression that reads it: a module-level
	// variable, a loop variable, or a function of the translated ytt library.
	names map[string]string
	// prelude collects the module-level KCL statements the translation emits, in order.
	prelude []string
	imports map[string]bool
	libs    map[string]*yttLib
	// libPackage is the KCL package path of the translated ytt library ("lib").
	libPackage string
	// levelVar is the KCL variable holding the environment data of the level a file belongs
	// to, which `@myks:data.lib.yaml`'s `env_data` reads. Empty where no such variable is in
	// scope, which leaves every value computed from the environment untranslated.
	levelVar string
	// taken holds the module-level variable names already claimed.
	taken map[string]bool
}

// kclReservedVars are the module-level names the generated level files bind themselves.
var kclReservedVars = map[string]bool{"_apps": true, "_lvl": true, "_patch": true}

func newStarScope(libs map[string]*yttLib, libPackage, levelVar string) *starScope {
	return &starScope{
		names:      map[string]string{},
		imports:    map[string]bool{},
		libs:       libs,
		libPackage: libPackage,
		levelVar:   levelVar,
		taken:      map[string]bool{},
	}
}

// bind claims a module-level KCL variable for a Starlark name.
func (s *starScope) bind(name string) string {
	kcl := "_" + sanitizeKclIdentifier(name)
	for kclReservedVars[kcl] || (s.taken[kcl] && s.names[name] != kcl) {
		kcl += "_"
	}
	s.taken[kcl] = true
	s.names[name] = kcl
	return kcl
}

// lambda translates a function whose body is a single `return` into a KCL lambda. The
// parameters are bound in a copy of the scope, so they do not leak into what follows.
func (s *starScope) lambda(def *syntax.DefStmt, ret *syntax.ReturnStmt) (string, bool) {
	inner := &starScope{names: maps.Clone(s.names), libs: s.libs, libPackage: s.libPackage, taken: s.taken, imports: s.imports}
	params := make([]string, 0, len(def.Params))
	for _, param := range def.Params {
		ident, ok := param.(*syntax.Ident)
		if !ok {
			// A default, *args or **kwargs: a KCL lambda takes none of them.
			return "", false
		}
		name := sanitizeKclIdentifier(ident.Name)
		params = append(params, name)
		inner.names[ident.Name] = name
	}
	body, err := inner.expr(ret.Result)
	if err != nil {
		return "", false
	}
	return fmt.Sprintf("lambda %s {\n    %s\n}", strings.Join(params, ", "), body), true
}

// run translates the top-level statements of a file's ytt prelude. A statement it cannot
// translate unbinds the names it assigns, so the values reading them stay literals.
func (s *starScope) run(stmts []syntax.Stmt) {
	for _, stmt := range stmts {
		switch typed := stmt.(type) {
		case *syntax.LoadStmt:
			s.load(typed)
		case *syntax.AssignStmt:
			s.assign(typed)
		case *syntax.ForStmt:
			s.appendLoop(typed)
		case *syntax.DefStmt:
			s.def(typed)
		}
	}
}

// load binds what a data file loads to its KCL translation: a function of the repo's ytt
// library, or the environment data myks generates for the file's environment, which is the
// level variable of its KCL package. A load of anything else (a ytt builtin, an untranslated
// function) binds nothing.
func (s *starScope) load(stmt *syntax.LoadStmt) {
	module, ok := stmt.Module.Value.(string)
	if !ok {
		return
	}
	lib := s.libs[strings.TrimSuffix(filepath.Base(module), filepath.Ext(module))]
	for i, from := range stmt.From {
		to := stmt.To[i].Name
		if module == myksDataLibrary && from.Name == "env_data" && s.levelVar != "" {
			s.names[to] = s.levelVar
			continue
		}
		if lib == nil || !lib.funcs[from.Name] || strings.HasPrefix(module, "@") {
			delete(s.names, to)
			continue
		}
		s.names[to] = s.libPackage + "." + sanitizeKclIdentifier(from.Name)
		s.imports["import "+s.libPackage] = true
	}
}

// def binds a prelude function whose body is a single `return` as a module-level KCL lambda.
// Anything longer has no KCL counterpart, and unbinds the name.
func (s *starScope) def(stmt *syntax.DefStmt) {
	if len(stmt.Body) == 1 {
		if ret, ok := stmt.Body[0].(*syntax.ReturnStmt); ok && ret.Result != nil {
			if lambda, ok := s.lambda(stmt, ret); ok {
				s.prelude = append(s.prelude, s.bind(stmt.Name.Name)+" = "+lambda)
				return
			}
		}
	}
	delete(s.names, stmt.Name.Name)
}

func (s *starScope) assign(stmt *syntax.AssignStmt) {
	ident, ok := stmt.LHS.(*syntax.Ident)
	if !ok || stmt.Op != syntax.EQ {
		return
	}
	expr, err := s.expr(stmt.RHS)
	if err != nil {
		delete(s.names, ident.Name)
		return
	}
	s.prelude = append(s.prelude, s.bind(ident.Name)+" = "+expr)
}

// appendLoop translates the one loop shape a KCL comprehension states directly: a loop whose
// body only appends to lists bound above it. `for n in ns: xs.append(f(n))` becomes
// `_xs = _xs + [f(n) for n in _ns]`, which is the same list — KCL allows an underscore
// variable to be reassigned.
func (s *starScope) appendLoop(stmt *syntax.ForStmt) {
	loopVar, ok := stmt.Vars.(*syntax.Ident)
	if !ok {
		return
	}
	iterable, err := s.expr(stmt.X)
	if err != nil {
		return
	}

	// The loop variable shadows whatever the name held outside the loop.
	outer, hadOuter := s.names[loopVar.Name]
	local := sanitizeKclIdentifier(loopVar.Name)
	s.names[loopVar.Name] = local
	defer func() {
		if hadOuter {
			s.names[loopVar.Name] = outer
		} else {
			delete(s.names, loopVar.Name)
		}
	}()

	type appended struct{ target, element string }
	appends := make([]appended, 0, len(stmt.Body))
	for _, body := range stmt.Body {
		expr, ok := body.(*syntax.ExprStmt)
		if !ok {
			return
		}
		call, ok := expr.X.(*syntax.CallExpr)
		if !ok || len(call.Args) != 1 {
			return
		}
		dot, ok := call.Fn.(*syntax.DotExpr)
		if !ok || dot.Name.Name != "append" {
			return
		}
		target, ok := dot.X.(*syntax.Ident)
		if !ok {
			return
		}
		kclTarget, bound := s.names[target.Name]
		if !bound || !strings.HasPrefix(kclTarget, "_") {
			// Only a list this prelude built can be appended to.
			return
		}
		element, err := s.expr(call.Args[0])
		if err != nil {
			return
		}
		appends = append(appends, appended{target: kclTarget, element: element})
	}
	for _, a := range appends {
		s.prelude = append(s.prelude, fmt.Sprintf("%s = %s + [%s for %s in %s]", a.target, a.target, a.element, local, iterable))
	}
}

// expr translates one Starlark expression to KCL, or fails when it uses something this
// translation does not cover.
func (s *starScope) expr(e syntax.Expr) (string, error) {
	switch typed := e.(type) {
	case *syntax.Literal:
		return starLiteral(typed)
	case *syntax.Ident:
		return s.ident(typed)
	case *syntax.ParenExpr:
		inner, err := s.expr(typed.X)
		if err != nil {
			return "", err
		}
		return "(" + inner + ")", nil
	case *syntax.UnaryExpr:
		return s.unary(typed)
	case *syntax.BinaryExpr:
		return s.binary(typed)
	case *syntax.ListExpr:
		return s.list(typed.List)
	case *syntax.TupleExpr:
		return s.list(typed.List)
	case *syntax.DictExpr:
		return s.dict(typed)
	case *syntax.IndexExpr:
		return s.index(typed)
	case *syntax.SliceExpr:
		return s.slice(typed)
	case *syntax.CondExpr:
		return s.cond(typed)
	case *syntax.DotExpr:
		return s.dot(typed)
	case *syntax.CallExpr:
		return s.call(typed)
	case *syntax.Comprehension:
		return s.comprehension(typed)
	default:
		return "", fmt.Errorf("%T has no KCL translation", e)
	}
}

func starLiteral(lit *syntax.Literal) (string, error) {
	switch value := lit.Value.(type) {
	case string:
		return quoteKclString(value), nil
	case int64:
		return fmt.Sprintf("%d", value), nil
	case *big.Int:
		return value.String(), nil
	case float64:
		return kclScalar(value)
	default:
		return "", fmt.Errorf("literal %v has no KCL translation", lit.Value)
	}
}

func (s *starScope) ident(ident *syntax.Ident) (string, error) {
	switch ident.Name {
	case "True", "False", "None":
		return ident.Name, nil
	}
	if kcl, ok := s.names[ident.Name]; ok {
		return kcl, nil
	}
	return "", fmt.Errorf("%s is not bound to a KCL expression", ident.Name)
}

func (s *starScope) list(items []syntax.Expr) (string, error) {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		part, err := s.expr(item)
		if err != nil {
			return "", err
		}
		parts = append(parts, part)
	}
	return "[" + strings.Join(parts, ", ") + "]", nil
}

func (s *starScope) dict(d *syntax.DictExpr) (string, error) {
	parts := make([]string, 0, len(d.List))
	for _, raw := range d.List {
		entry, ok := raw.(*syntax.DictEntry)
		if !ok {
			return "", fmt.Errorf("%T has no KCL translation", raw)
		}
		pair, err := s.dictEntry(entry)
		if err != nil {
			return "", err
		}
		parts = append(parts, pair)
	}
	return "{" + strings.Join(parts, ", ") + "}", nil
}

func (s *starScope) dictEntry(entry *syntax.DictEntry) (string, error) {
	key, err := s.expr(entry.Key)
	if err != nil {
		return "", err
	}
	value, err := s.expr(entry.Value)
	if err != nil {
		return "", err
	}
	return key + ": " + value, nil
}

// dot translates attribute access. A key that is no KCL identifier is read by subscript,
// which is how KCL reads any other key of a dict.
func (s *starScope) dot(d *syntax.DotExpr) (string, error) {
	receiver, err := s.operand(d.X, precPrimary, false)
	if err != nil {
		return "", err
	}
	if isKclIdentifier(d.Name.Name) {
		return receiver + "." + d.Name.Name, nil
	}
	return receiver + "[" + quoteKclString(d.Name.Name) + "]", nil
}

func (s *starScope) index(idx *syntax.IndexExpr) (string, error) {
	target, err := s.operand(idx.X, precPrimary, false)
	if err != nil {
		return "", err
	}
	index, err := s.expr(idx.Y)
	if err != nil {
		return "", err
	}
	return target + "[" + index + "]", nil
}

func (s *starScope) slice(sl *syntax.SliceExpr) (string, error) {
	target, err := s.operand(sl.X, precPrimary, false)
	if err != nil {
		return "", err
	}
	bounds := make([]string, 0, 3)
	for _, bound := range []syntax.Expr{sl.Lo, sl.Hi, sl.Step} {
		if bound == nil {
			bounds = append(bounds, "")
			continue
		}
		part, err := s.expr(bound)
		if err != nil {
			return "", err
		}
		bounds = append(bounds, part)
	}
	if bounds[2] == "" {
		bounds = bounds[:2]
	}
	return target + "[" + strings.Join(bounds, ":") + "]", nil
}

func (s *starScope) cond(c *syntax.CondExpr) (string, error) {
	parts := make([]string, 0, 3)
	for _, part := range []syntax.Expr{c.True, c.Cond, c.False} {
		translated, err := s.operand(part, precCond+1, false)
		if err != nil {
			return "", err
		}
		parts = append(parts, translated)
	}
	return parts[0] + " if " + parts[1] + " else " + parts[2], nil
}

func (s *starScope) comprehension(c *syntax.Comprehension) (string, error) {
	// A comprehension binds its own variables; they are restored afterwards so the names of
	// the enclosing scope survive.
	outer := maps.Clone(s.names)
	defer func() { s.names = outer }()

	clauses := make([]string, 0, len(c.Clauses))
	for _, raw := range c.Clauses {
		switch clause := raw.(type) {
		case *syntax.ForClause:
			vars, err := s.comprehensionVars(clause.Vars)
			if err != nil {
				return "", err
			}
			iterable, err := s.expr(clause.X)
			if err != nil {
				return "", err
			}
			clauses = append(clauses, "for "+vars+" in "+iterable)
		case *syntax.IfClause:
			cond, err := s.expr(clause.Cond)
			if err != nil {
				return "", err
			}
			clauses = append(clauses, "if "+cond)
		default:
			return "", fmt.Errorf("%T has no KCL translation", raw)
		}
	}

	open, closing := "[", "]"
	body := ""
	if entry, ok := c.Body.(*syntax.DictEntry); ok {
		open, closing = "{", "}"
		pair, err := s.dictEntry(entry)
		if err != nil {
			return "", err
		}
		body = pair
	} else {
		translated, err := s.expr(c.Body)
		if err != nil {
			return "", err
		}
		body = translated
	}
	return open + body + " " + strings.Join(clauses, " ") + closing, nil
}

// comprehensionVars binds the loop variables of one comprehension clause to themselves.
func (s *starScope) comprehensionVars(vars syntax.Expr) (string, error) {
	switch typed := vars.(type) {
	case *syntax.Ident:
		name := sanitizeKclIdentifier(typed.Name)
		s.names[typed.Name] = name
		return name, nil
	case *syntax.TupleExpr:
		names := make([]string, 0, len(typed.List))
		for _, item := range typed.List {
			name, err := s.comprehensionVars(item)
			if err != nil {
				return "", err
			}
			names = append(names, name)
		}
		return strings.Join(names, ", "), nil
	default:
		return "", fmt.Errorf("%T is no comprehension variable", vars)
	}
}

// Operator precedence, shared by Starlark and KCL. A translated operand is parenthesized
// only where the nesting would otherwise re-associate it.
const (
	precCond = iota + 1
	precOr
	precAnd
	precNot
	precCompare
	precBitOr
	precBitXor
	precBitAnd
	precShift
	precAdd
	precMul
	precUnary
	precPower
	precPrimary
)

// starBinaryOps are the binary operators KCL spells the same way as Starlark, with their
// precedence. Everything absent here (`%` on a string is Starlark's printf, which KCL has
// no counterpart for) fails the translation.
var starBinaryOps = map[syntax.Token]struct {
	text string
	prec int
}{
	syntax.OR:         {"or", precOr},
	syntax.AND:        {"and", precAnd},
	syntax.EQL:        {"==", precCompare},
	syntax.NEQ:        {"!=", precCompare},
	syntax.LT:         {"<", precCompare},
	syntax.GT:         {">", precCompare},
	syntax.LE:         {"<=", precCompare},
	syntax.GE:         {">=", precCompare},
	syntax.IN:         {"in", precCompare},
	syntax.NOT_IN:     {"not in", precCompare},
	syntax.PIPE:       {"|", precBitOr},
	syntax.CIRCUMFLEX: {"^", precBitXor},
	syntax.AMP:        {"&", precBitAnd},
	syntax.LTLT:       {"<<", precShift},
	syntax.GTGT:       {">>", precShift},
	syntax.PLUS:       {"+", precAdd},
	syntax.MINUS:      {"-", precAdd},
	syntax.STAR:       {"*", precMul},
	syntax.SLASH:      {"/", precMul},
	syntax.SLASHSLASH: {"//", precMul},
	syntax.PERCENT:    {"%", precMul},
	syntax.STARSTAR:   {"**", precPower},
}

func exprPrec(e syntax.Expr) int {
	switch typed := e.(type) {
	case *syntax.BinaryExpr:
		if op, ok := starBinaryOps[typed.Op]; ok {
			return op.prec
		}
		return precCond
	case *syntax.UnaryExpr:
		if typed.Op == syntax.NOT {
			return precNot
		}
		return precUnary
	case *syntax.CondExpr:
		return precCond
	case *syntax.LambdaExpr:
		return precCond
	default:
		return precPrimary
	}
}

// operand translates a subexpression, parenthesizing it where the enclosing operator binds
// at least as tightly. isRight marks the right-hand operand of a left-associative operator,
// which needs parentheses at equal precedence.
func (s *starScope) operand(e syntax.Expr, parent int, isRight bool) (string, error) {
	translated, err := s.expr(e)
	if err != nil {
		return "", err
	}
	prec := exprPrec(e)
	if prec < parent || (prec == parent && isRight) {
		return "(" + translated + ")", nil
	}
	return translated, nil
}

func (s *starScope) unary(u *syntax.UnaryExpr) (string, error) {
	if u.X == nil {
		// A bare `*` in an argument list.
		return "", fmt.Errorf("%s has no KCL translation", u.Op)
	}
	text := ""
	switch u.Op {
	case syntax.MINUS:
		text = "-"
	case syntax.PLUS:
		text = "+"
	case syntax.TILDE:
		text = "~"
	case syntax.NOT:
		text = "not "
	default:
		return "", fmt.Errorf("unary %s has no KCL translation", u.Op)
	}
	operand, err := s.operand(u.X, exprPrec(u), false)
	if err != nil {
		return "", err
	}
	return text + operand, nil
}

func (s *starScope) binary(b *syntax.BinaryExpr) (string, error) {
	op, ok := starBinaryOps[b.Op]
	if !ok {
		return "", fmt.Errorf("operator %s has no KCL translation", b.Op)
	}
	if op.text == "%" && isStarString(b.X) {
		// Starlark formats a string with `%`; KCL only divides with it.
		return "", fmt.Errorf("string %% formatting has no KCL translation")
	}
	// `**` is right-associative; every other operator here is left-associative.
	leftEqualWraps := b.Op == syntax.STARSTAR
	left, err := s.operand(b.X, op.prec, leftEqualWraps)
	if err != nil {
		return "", err
	}
	right, err := s.operand(b.Y, op.prec, !leftEqualWraps)
	if err != nil {
		return "", err
	}
	return left + " " + op.text + " " + right, nil
}

func isStarString(e syntax.Expr) bool {
	lit, ok := e.(*syntax.Literal)
	if !ok {
		return false
	}
	_, isString := lit.Value.(string)
	return isString
}

// starMethods are the methods KCL provides under the same name and meaning as Starlark.
var starMethods = map[string]bool{
	"capitalize": true, "count": true, "endswith": true, "find": true, "format": true,
	"index": true, "isalpha": true, "isdigit": true, "islower": true, "isupper": true,
	"items": true, "join": true, "keys": true, "lower": true, "lstrip": true,
	"replace": true, "rstrip": true, "split": true, "splitlines": true, "startswith": true,
	"strip": true, "title": true, "upper": true, "values": true,
}

// starBuiltins are the Starlark builtins KCL provides under the same name and meaning.
var starBuiltins = map[string]bool{
	"abs": true, "all": true, "any": true, "bool": true, "float": true, "int": true,
	"len": true, "max": true, "min": true, "sorted": true, "str": true, "sum": true,
}

func (s *starScope) call(call *syntax.CallExpr) (string, error) {
	args := make([]string, 0, len(call.Args))
	for _, raw := range call.Args {
		if binary, ok := raw.(*syntax.BinaryExpr); ok && binary.Op == syntax.EQ {
			// A keyword argument: KCL lambdas take positional arguments only.
			return "", fmt.Errorf("a keyword argument has no KCL translation")
		}
		arg, err := s.expr(raw)
		if err != nil {
			return "", err
		}
		args = append(args, arg)
	}
	joined := "(" + strings.Join(args, ", ") + ")"

	switch fn := call.Fn.(type) {
	case *syntax.DotExpr:
		if !starMethods[fn.Name.Name] {
			return "", fmt.Errorf("method %s has no KCL translation", fn.Name.Name)
		}
		receiver, err := s.operand(fn.X, precPrimary, false)
		if err != nil {
			return "", err
		}
		return receiver + "." + fn.Name.Name + joined, nil
	case *syntax.Ident:
		if bound, ok := s.names[fn.Name]; ok {
			return bound + joined, nil
		}
		if starBuiltins[fn.Name] {
			return fn.Name + joined, nil
		}
		return "", fmt.Errorf("%s is not bound to a KCL function", fn.Name)
	default:
		return "", fmt.Errorf("%T is no callable KCL expression", call.Fn)
	}
}

// myksDataLibrary is the ytt library myks generates per application, through which a legacy
// data file reads the data values of its environment.
const myksDataLibrary = "@myks:data.lib.yaml"

// yttDerivations translates the ytt computation of one data file into KCL: the prelude into
// module-level variables, and every `key: #@ expr` the file states into an expression for
// that value's path. A value whose expression does not translate is left out, to be written
// as the literal ytt resolved.
func yttDerivations(file string, content []byte, libs map[string]*yttLib, libPackage, levelVar string) *derivations {
	split, err := splitYttFile(content)
	if err != nil || len(split.exprs) == 0 {
		return nil
	}

	scope := newStarScope(libs, libPackage, levelVar)
	prelude, err := starSyntax.Parse(file, yttPreludeSource(split.lines, false), 0)
	if err != nil {
		// A ytt template function — a `def` whose body is YAML rather than Starlark — is no
		// Starlark program. Dropping those blocks leaves the rest of the prelude readable.
		prelude, err = starSyntax.Parse(file, yttPreludeSource(split.lines, true), 0)
	}
	if err == nil {
		scope.run(prelude.Stmts)
	}

	d := &derivations{exprs: map[string]string{}}
	for _, path := range slices.Sorted(maps.Keys(split.exprs)) {
		parsed, err := starSyntax.ParseExpr(file, split.exprs[path], 0)
		if err != nil {
			continue
		}
		translated, err := scope.expr(parsed)
		if err != nil {
			continue
		}
		d.exprs[path] = translated
	}
	if len(d.exprs) == 0 {
		return nil
	}
	d.prelude = prunePrelude(scope.prelude, d.exprs)
	for imp := range scope.imports {
		d.imports = append(d.imports, imp)
	}
	slices.Sort(d.imports)
	return d
}

// yttPreludeSource returns the Starlark of a ytt file's top-level code: the `#@` lines at
// zero indentation that carry code rather than an annotation. Code indented into the YAML
// body belongs to a template construct, which this translation does not cover. With
// dropDefs, function blocks are left out with their bodies — which is what a ytt template
// function needs, its body being YAML the `#@` lines do not carry.
func yttPreludeSource(lines []string, dropDefs bool) string {
	code := make([]string, 0, len(lines))
	depth := 0
	for _, line := range lines {
		if !strings.HasPrefix(line, "#@") {
			continue
		}
		if match := yttAnnotationRe.FindStringSubmatch(line); len(match) > 1 && match[1] != "" {
			continue
		}
		statement := strings.TrimPrefix(line[2:], " ")
		trimmed := strings.TrimSpace(statement)
		switch {
		case depth > 0:
			if strings.HasSuffix(trimmed, ":") {
				depth++
			} else if trimmed == "end" {
				depth--
			}
			continue
		case dropDefs && strings.HasPrefix(trimmed, "def "):
			depth = 1
			continue
		}
		code = append(code, statement)
	}
	return strings.Join(code, "\n") + "\n"
}

// prunePrelude keeps the statements the translated expressions reach, directly or through
// another statement. A prelude variable no expression reads would be dead code in the
// generated file.
func prunePrelude(prelude []string, exprs map[string]string) []string {
	used := map[string]bool{}
	names := preludeVars(prelude)
	reads := func(text string) {
		for name := range names {
			if readsName(text, name) {
				used[name] = true
			}
		}
	}
	for _, expr := range exprs {
		reads(expr)
	}
	var kept []string
	// Backwards: a statement is reached by the ones below it, never by the ones above.
	for i := len(prelude) - 1; i >= 0; i-- {
		name, rhs, _ := strings.Cut(prelude[i], " = ")
		if !used[name] {
			continue
		}
		reads(rhs)
		kept = append(kept, prelude[i])
	}
	slices.Reverse(kept)
	return kept
}

// readsName reports whether text reads name as a whole identifier rather than as part of a
// longer one.
func readsName(text, name string) bool {
	for i := 0; i+len(name) <= len(text); {
		idx := strings.Index(text[i:], name)
		if idx < 0 {
			return false
		}
		start, end := i+idx, i+idx+len(name)
		if (start == 0 || !isIdentifierChar(text[start-1])) && (end == len(text) || !isIdentifierChar(text[end])) {
			return true
		}
		i = end
	}
	return false
}

func isIdentifierChar(c byte) bool {
	return c == '_' || ('0' <= c && c <= '9') || ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}
