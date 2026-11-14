package wpconfig

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// errComputed marks a value that only PHP could work out: a function call, a
// variable, arithmetic. The reader never guesses one.
var errComputed = errors.New("is computed in PHP")

// unsetEnvError names the environment variable a value was to be read from,
// so the operator knows what to set.
type unsetEnvError struct{ name string }

func (e unsetEnvError) Error() string {
	return fmt.Sprintf("reads environment variable %s, which is not set", e.name)
}

// value is a statically known PHP value.
type value struct {
	str  string
	bool bool
	// isBool marks true/false/null; otherwise str holds the value.
	isBool bool
}

// truthy follows PHP: false, null, "" and "0" are false; anything else true.
func (v value) truthy() bool {
	if v.isBool {
		return v.bool
	}
	return v.str != "" && v.str != "0"
}

// evaluator evaluates the small subset of PHP expressions found in a
// wp-config.php: literals, concatenation, environment lookups, the file's own
// location and constants defined above.
type evaluator struct {
	absPath string
	file    string
	consts  map[string]value
	getenv  func(string) string
}

// eval evaluates expr, or returns errComputed / an unsetEnvError.
func (e *evaluator) eval(expr string) (value, error) {
	var parts []string
	for _, p := range splitConcat(expr) {
		parts = append(parts, strings.TrimSpace(p))
	}

	if len(parts) == 1 {
		return e.term(parts[0])
	}

	var b strings.Builder
	for _, p := range parts {
		v, err := e.term(p)
		if err != nil {
			return value{}, err
		}
		if v.isBool {
			if v.bool {
				b.WriteString("1")
			}
			continue
		}
		b.WriteString(v.str)
	}
	return value{str: b.String()}, nil
}

// term evaluates one operand of a concatenation.
func (e *evaluator) term(t string) (value, error) {
	if t == "" {
		return value{}, errComputed
	}
	if t[0] == '\'' || t[0] == '"' {
		return stringLiteral(t)
	}
	if call, args, ok := callForm(t); ok {
		return e.call(call, args)
	}
	if v, ok, err := e.superglobal(t); ok {
		return v, err
	}
	return e.identifier(t)
}

// identifier evaluates a bare word: a keyword, a number, a magic constant or a
// constant defined earlier in the file.
func (e *evaluator) identifier(t string) (value, error) {
	switch strings.ToLower(t) {
	case "true":
		return value{isBool: true, bool: true}, nil
	case "false", "null":
		return value{isBool: true}, nil
	}

	switch t {
	case "__DIR__":
		return value{str: filepath.Dir(e.file)}, nil
	case "__FILE__":
		return value{str: e.file}, nil
	case "ABSPATH":
		return value{str: strings.TrimSuffix(e.absPath, "/") + "/"}, nil
	}

	if isNumber(t) {
		return value{str: t}, nil
	}
	if v, ok := e.consts[t]; ok {
		return v, nil
	}
	return value{}, errComputed
}

// call evaluates the few functions whose result is known without PHP.
func (e *evaluator) call(name string, args []string) (value, error) {
	switch {
	case name == "getenv" && len(args) == 1:
		return e.env(args[0], nil)
	case name == "getenv_docker" && len(args) == 2:
		return e.env(args[0], &args[1])
	case name == "dirname" && len(args) == 1 && strings.TrimSpace(args[0]) == "__FILE__":
		return value{str: filepath.Dir(e.file)}, nil
	}
	return value{}, errComputed
}

// env reads an environment variable named by a string literal. The official
// Docker image's getenv_docker() prefers <NAME>_FILE: when that is set the real
// value lives in a file this reader does not open, so the fallback default
// would be a wrong guess.
func (e *evaluator) env(nameExpr string, fallback *string) (value, error) {
	name, err := stringLiteral(strings.TrimSpace(nameExpr))
	if err != nil {
		return value{}, errComputed
	}
	if fallback != nil && e.getenv(name.str+"_FILE") != "" {
		return value{}, unsetEnvError{name: name.str + "_FILE (a file this tool does not read)"}
	}
	if v := e.getenv(name.str); v != "" {
		return value{str: v}, nil
	}
	if fallback != nil {
		return e.term(strings.TrimSpace(*fallback))
	}
	return value{}, unsetEnvError{name: name.str}
}

// superglobal evaluates $_ENV['X'] and $_SERVER['X']. ok is false when t is not
// one of them.
func (e *evaluator) superglobal(t string) (value, bool, error) {
	for _, prefix := range []string{"$_ENV[", "$_SERVER["} {
		if !strings.HasPrefix(t, prefix) {
			continue
		}
		if !strings.HasSuffix(t, "]") {
			return value{}, true, errComputed
		}
		v, err := e.env(t[len(prefix):len(t)-1], nil)
		return v, true, err
	}
	return value{}, false, nil
}
