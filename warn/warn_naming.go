/*
Copyright 2020 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package warn

import (
	"fmt"
	"github.com/bazelbuild/buildtools/build"
	"github.com/bazelbuild/buildtools/bzlenv"
	"strings"
)

var ambiguousNames = map[string]bool{
	"I": true,
	"l": true,
	"O": true,
}

// ambiguousNameCheck checks for the names of idents and functions
func ambiguousNameCheck(f *build.File, expr build.Expr, findings []*LinterFinding) []*LinterFinding {
	var name string
	switch expr := expr.(type) {
	case *build.Ident:
		name = expr.Name
	case *build.DefStmt:
		name = expr.Name
	default:
		return findings
	}

	if ambiguousNames[name] {
		findings = append(findings,
			makeLinterFinding(expr, `Never use 'l', 'I', or 'O' as names (they're too easily confused with 'I', 'l', or '0').`))
	}
	return findings
}

func confusingNameWarning(f *build.File) []*LinterFinding {

	var findings []*LinterFinding
	// check for global variable names
	for _, ident := range collectLocalVariables(f.Stmt) {
		findings = ambiguousNameCheck(f, ident, findings)
	}

	build.Walk(f, func(expr build.Expr, stack []build.Expr) {
		switch expr := expr.(type) {
		case *build.DefStmt:
			findings = ambiguousNameCheck(f, expr, findings)
			for _, param := range getFunctionParams(expr) {
				findings = ambiguousNameCheck(f, param, findings)
			}
			for _, ident := range collectLocalVariables(expr.Body) {
				findings = ambiguousNameCheck(f, ident, findings)
			}
		case *build.Comprehension:
			for _, clause := range expr.Clauses {
				forClause, ok := clause.(*build.ForClause)
				if !ok {
					continue
				}
				for _, ident := range bzlenv.CollectLValues(forClause.Vars) {
					findings = ambiguousNameCheck(f, ident, findings)
				}
			}
		}
	})

	return findings
}

func isUpperCamelCase(name string) bool {
	if strings.HasPrefix(name, "_") {
		// Private providers are allowed
		name = name[1:]
	}
	return !strings.ContainsRune(name, '_') && name == strings.Title(name)
}

func isLowerSnakeCase(name string) bool {
	return name == strings.ToLower(name)
}

func isUpperSnakeCase(name string) bool {
	return name == strings.ToUpper(name)
}

func nameConventionsWarning(f *build.File) []*LinterFinding {
	var findings []*LinterFinding

	build.WalkStatements(f, func(stmt build.Expr, stack []build.Expr) {
		// looking for provider declaration statements: `xxx = provider()`
		// note that the code won't trigger on complex assignments, such as `x, y = foo, provider()`
		binary, ok := stmt.(*build.AssignExpr)
		if !ok {
			return
		}
		for _, ident := range bzlenv.CollectLValues(binary.LHS) {
			if isLowerSnakeCase(ident.Name) || isUpperSnakeCase(ident.Name) {
				continue
			}
			if isUpperCamelCase(ident.Name) && strings.HasSuffix(ident.Name, "Info") {
				continue
			}
			findings = append(findings,
				makeLinterFinding(ident,
					fmt.Sprintf(`Variable name "%s" should be lower_snake_case (for variables), UPPER_SNAKE_CASE (for constants), or UpperCamelCase ending with 'Info' (for providers).`, ident.Name)))
		}
	})

	return findings
}
