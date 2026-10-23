package credential

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strings"

	"github.com/emmyfong/snow/internal/private"
)

// afterLine runs after each line the file write produces. Tests set it to
// crash the write halfway.
var afterLine func()

// readFile reads the credentials file: one "provider = key" per line. A
// missing file holds no keys. Errors name the line, never its text, which
// holds a key.
func readFile(path string) (map[string]string, error) {
	keys := map[string]string{}
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return keys, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if n == 1 {
			// Notepad on Windows starts a UTF-8 file with a byte order mark.
			line = strings.TrimPrefix(line, "\ufeff")
		}
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, value, ok := strings.Cut(line, "=")
		name, value = strings.TrimSpace(name), strings.TrimSpace(value)
		if !ok || name == "" || value == "" {
			return nil, fmt.Errorf("%s line %d: want provider = key", path, n)
		}
		keys[name] = value
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return keys, nil
}

// writeFile replaces the credentials file in one atomic step, in a folder
// only the user can read, with mode 0600.
func writeFile(dir, path string, keys map[string]string) error {
	if err := private.Dir(dir); err != nil {
		return fmt.Errorf("credentials folder: %w", err)
	}
	names := make([]string, 0, len(keys))
	for n := range keys {
		names = append(names, n)
	}
	sort.Strings(names)
	err := private.WriteAtomic(path, func(w io.Writer) error {
		if _, err := io.WriteString(w, "# Snow API keys. Readable only by you.\n"); err != nil {
			return err
		}
		for _, n := range names {
			if _, err := io.WriteString(w, n+" = "+keys[n]+"\n"); err != nil {
				return err
			}
			if afterLine != nil {
				afterLine()
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("save %s: %w", path, err)
	}
	return nil
}
