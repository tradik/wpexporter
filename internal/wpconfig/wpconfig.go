// Package wpconfig reads the database settings out of a WordPress
// wp-config.php without running it.
//
// The file is PHP, and executing it is out of the question: it can do anything,
// and it is not ours. So it is read as text. Comments are removed first — a
// commented-out define() must never win — and the define() calls and the
// $table_prefix assignment are evaluated over a small, safe subset of PHP:
// string literals, concatenation, getenv()/getenv_docker()/$_ENV/$_SERVER,
// __DIR__, ABSPATH and constants defined above. Anything else is "computed in
// PHP", and a value that is needed but computed is an error naming the flag or
// environment variable that supplies it. Nothing is guessed.
package wpconfig

import (
	"errors"
	"fmt"
	"regexp"
)

// PasswordEnv is the environment variable that supplies the database password
// when wp-config.php cannot. There is deliberately no flag: a flag is visible to
// every user of the machine in the process list.
const PasswordEnv = "WPEXPORTER_DB_PASSWORD" // #nosec G101 -- the name of a variable, not a credential

// defaultCharset is what WordPress connects with when DB_CHARSET is not set.
const defaultCharset = "utf8mb4"

// prefixPattern is what a table prefix may contain. It is the only value that
// ever reaches an SQL statement as text, so it is checked here, once.
var prefixPattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// Settings is what an export needs from wp-config.php.
type Settings struct {
	DBName string
	DBUser string
	// DBPassword is never printed: see String.
	DBPassword  string `json:"-"`
	DBHost      string
	DBCharset   string
	TablePrefix string
	Multisite   bool
	// Home and SiteURL are WP_HOME and WP_SITEURL, which override the options
	// stored in the database. Empty when not defined.
	Home    string
	SiteURL string
	// ContentDir is WP_CONTENT_DIR and Uploads is UPLOADS (relative to ABSPATH,
	// as WordPress reads it), when they could be worked out statically.
	ContentDir string
	Uploads    string
	// ABSPath is the WordPress directory and File the wp-config.php read.
	ABSPath string
	File    string
	// Notes are things that were skipped, worded for the operator.
	Notes []string
}

// Overrides are the --db-* flags and the password from the environment. A
// non-empty field replaces what the file says.
type Overrides struct {
	DBHost      string
	DBName      string
	DBUser      string
	DBPassword  string `json:"-"`
	TablePrefix string
}

// String describes the settings with the password redacted, so a %v in a log
// line or an error cannot leak it.
func (s Settings) String() string {
	return fmt.Sprintf("wp-config %s: db=%s user=%s host=%s charset=%s prefix=%s multisite=%t password=[redacted]",
		s.File, s.DBName, s.DBUser, s.DBHost, s.DBCharset, s.TablePrefix, s.Multisite)
}

// GoString is String, so %#v redacts the password too.
func (s Settings) GoString() string { return s.String() }

// required lists the values an export cannot start without, with what supplies
// each one when the file cannot.
var required = []struct{ name, remedy string }{
	{"DB_NAME", "give --db-name"},
	{"DB_USER", "give --db-user"},
	{"DB_PASSWORD", "set " + PasswordEnv},
	{"DB_HOST", "give --db-host"},
}

// parse evaluates src, the contents of file, for the WordPress in absPath.
func parse(src []byte, absPath, file string, ov Overrides, getenv func(string) string) (*Settings, error) {
	code := stripComments(string(src))
	ev := &evaluator{absPath: absPath, file: file, consts: map[string]value{}, getenv: getenv}
	problems := map[string]error{}

	for _, st := range findDefines(code) {
		if _, seen := ev.consts[st.name]; seen {
			continue // define() on a defined constant fails in PHP: first wins
		}
		if _, failed := problems[st.name]; failed {
			continue
		}
		v, err := ev.eval(st.expr)
		if err != nil {
			problems[st.name] = err
			continue
		}
		ev.consts[st.name] = v
	}

	s := &Settings{ABSPath: absPath, File: file}
	var errs []error
	fields := map[string]*string{"DB_NAME": &s.DBName, "DB_USER": &s.DBUser, "DB_PASSWORD": &s.DBPassword, "DB_HOST": &s.DBHost}
	given := map[string]string{"DB_NAME": ov.DBName, "DB_USER": ov.DBUser, "DB_PASSWORD": ov.DBPassword, "DB_HOST": ov.DBHost}

	for _, r := range required {
		if given[r.name] != "" {
			*fields[r.name] = given[r.name]
			continue
		}
		if v, ok := ev.consts[r.name]; ok {
			*fields[r.name] = v.str
			continue
		}
		reason := "is not defined in " + file
		if err := problems[r.name]; err != nil {
			reason = err.Error()
		}
		errs = append(errs, fmt.Errorf("%s %s — %s", r.name, reason, r.remedy))
	}

	prefix, err := tablePrefix(code, ev, ov.TablePrefix)
	if err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	s.TablePrefix = prefix

	s.DBCharset = ev.consts["DB_CHARSET"].str
	if s.DBCharset == "" {
		s.DBCharset = defaultCharset
	}
	s.Multisite = ev.consts["MULTISITE"].truthy()
	s.Home = ev.consts["WP_HOME"].str
	s.SiteURL = ev.consts["WP_SITEURL"].str
	s.ContentDir = ev.consts["WP_CONTENT_DIR"].str
	s.Uploads = ev.consts["UPLOADS"].str

	for _, name := range []string{"WP_HOME", "WP_SITEURL", "WP_CONTENT_DIR", "UPLOADS"} {
		if problems[name] != nil {
			s.Notes = append(s.Notes, fmt.Sprintf("%s in wp-config.php %s; using the database's value or the default instead",
				name, problems[name]))
		}
	}

	return s, nil
}

// tablePrefix returns the validated table prefix: the override, else the
// file's $table_prefix.
func tablePrefix(code string, ev *evaluator, override string) (string, error) {
	prefix := override
	if prefix == "" {
		expr, ok := findTablePrefix(code)
		if !ok {
			return "", fmt.Errorf("$table_prefix is not set in %s — give --db-prefix", ev.file)
		}
		v, err := ev.eval(expr)
		if err != nil {
			return "", fmt.Errorf("$table_prefix %s — give --db-prefix", err.Error())
		}
		prefix = v.str
	}
	if !prefixPattern.MatchString(prefix) {
		return "", fmt.Errorf("table_prefix %q may contain only letters, digits and underscores", prefix)
	}
	return prefix, nil
}
