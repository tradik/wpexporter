package localdb

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

// socketPaths are where MySQL and MariaDB put their socket on common systems,
// in the order PHP-based installs most often use them.
var socketPaths = []string{
	"/run/mysqld/mysqld.sock",
	"/var/run/mysqld/mysqld.sock",
	"/var/lib/mysql/mysql.sock",
	"/tmp/mysql.sock",
}

// endpoint is where the database listens: a network ("tcp" or "unix") and an
// address.
type endpoint struct {
	network string
	addr    string
}

// resolver looks a host name up; swapped in tests.
type resolver func(host string) error

// lookupHost is the default resolver.
func lookupHost(host string) error {
	_, err := net.LookupHost(host)
	return err
}

// socketExists reports whether a socket file is present; swapped in tests.
var socketExists = func(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// parseEndpoint reads DB_HOST the way WordPress hands it to mysqli:
//
//	host, host:3307, [::1]:3306       TCP, port 3306 unless given
//	localhost:/run/mysqld/mysqld.sock UNIX socket (also ":/path")
//	localhost                         PHP means the socket: the usual paths
//	                                  are tried, then 127.0.0.1:3306
//
// A name that does not resolve here — typically a Docker service name such as
// "db" seen from the host — is an error that says what to do about it.
func parseEndpoint(dbHost string, resolve resolver) (endpoint, error) {
	h := strings.TrimSpace(dbHost)
	if i := strings.Index(h, ":/"); i >= 0 {
		return endpoint{network: "unix", addr: h[i+1:]}, nil
	}

	host, port := h, "3306"
	if strings.HasPrefix(h, "[") {
		if end := strings.Index(h, "]"); end > 0 {
			host = h[1:end]
			if rest := h[end+1:]; strings.HasPrefix(rest, ":") {
				port = rest[1:]
			}
		}
	} else if i := strings.LastIndex(h, ":"); i >= 0 && strings.Count(h, ":") == 1 {
		host, port = h[:i], h[i+1:]
	}
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		return endpoint{}, fmt.Errorf("DB_HOST %q: port must be a number between 1 and 65535", dbHost)
	}

	if host == "" || strings.EqualFold(host, "localhost") {
		if !strings.Contains(h, ":") {
			for _, path := range socketPaths {
				if socketExists(path) {
					return endpoint{network: "unix", addr: path}, nil
				}
			}
		}
		return endpoint{network: "tcp", addr: net.JoinHostPort("127.0.0.1", port)}, nil
	}

	if net.ParseIP(host) == nil {
		if err := resolve(host); err != nil {
			return endpoint{}, fmt.Errorf("DB_HOST %q does not resolve from here (a Docker service name?) — "+
				"give --db-host with an address this machine can reach, or run wpexporter in the "+
				"database's network (docker run --network container:<wordpress>)", dbHost)
		}
	}
	return endpoint{network: "tcp", addr: net.JoinHostPort(host, port)}, nil
}
