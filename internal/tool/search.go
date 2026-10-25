package tool

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// Limits of the search tool.
const (
	searchMatches   = 200
	searchBytes     = 64 << 10
	searchLineChars = 300
	searchFileBytes = 4 << 20 // bigger files are skipped
)

type searchTool struct{}

func init() { register(searchTool{}) }

func (searchTool) Def() Def {
	return Def{
		Name: "search",
		Description: "Search the text of files for a regular expression (RE2 syntax). " +
			"Each match is one line: path:line: text. What .gitignore ignores, binary files, " +
			"and files over 4 MB are skipped.",
		Schema: schema(map[string]any{
			"pattern":     prop("string", "the regular expression"),
			"path":        prop("string", "the folder or file to search; default the conversation folder"),
			"glob":        prop("string", "search only files whose name matches, for example *.go"),
			"ignore_case": prop("boolean", "match without regard to case"),
		}, "pattern"),
	}
}

func (searchTool) Run(ctx context.Context, env Env, raw json.RawMessage) (string, error) {
	var in struct {
		Pattern    string `json:"pattern"`
		Path       string `json:"path"`
		Glob       string `json:"glob"`
		IgnoreCase bool   `json:"ignore_case"`
	}
	if err := input(raw, &in); err != nil {
		return "", err
	}
	expr := in.Pattern
	if in.IgnoreCase {
		expr = "(?i)" + expr
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return "", fmt.Errorf("bad pattern: %w", err)
	}
	if _, err := path.Match(in.Glob, ""); err != nil {
		return "", fmt.Errorf("bad glob: %w", err)
	}
	if in.Path == "" {
		in.Path = "."
	}
	p, err := env.Folder.Resolve(in.Path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(p.OS)
	if err != nil {
		return "", fmt.Errorf("search %s: %w", in.Path, err)
	}
	s := searcher{re: re, glob: in.Glob, out: capped{limit: searchBytes}}
	if !info.IsDir() {
		err = s.file(ctx, p.OS, in.Path)
	} else {
		err = walk(ctx, p.OS, func(rel string, d fs.DirEntry) error {
			// Only plain files: a link could lead outside the folder,
			// where reading needs the user's approval.
			if !d.Type().IsRegular() {
				return nil
			}
			return s.file(ctx, filepath.Join(p.OS, filepath.FromSlash(rel)), path.Join(strings.ReplaceAll(in.Path, `\`, "/"), rel))
		})
	}
	if err != nil && !errors.Is(err, errStop) {
		return "", fmt.Errorf("search %s: %w", in.Path, err)
	}
	if s.n == 0 {
		return "no matches\n", nil
	}
	return s.out.String("more matches not shown; narrow the pattern, path, or glob"), nil
}

// searcher collects matches across files.
type searcher struct {
	re   *regexp.Regexp
	glob string
	out  capped
	n    int
}

// file searches one file, shown to the model as name. It returns errStop
// when the output is full.
func (s *searcher) file(ctx context.Context, osPath, name string) error {
	if s.glob != "" {
		if ok, _ := path.Match(s.glob, path.Base(name)); !ok {
			return nil
		}
	}
	f, err := os.Open(osPath)
	if err != nil {
		return nil // a file that cannot be read is skipped, as grep does
	}
	defer func() { _ = f.Close() }()
	if info, err := f.Stat(); err != nil || info.Size() > searchFileBytes {
		return nil
	}
	r := bufio.NewReader(f)
	if head, _ := r.Peek(8000); isBinary(head) {
		return nil
	}
	for n := 1; ; n++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		line, err := r.ReadString('\n')
		if line = strings.TrimRight(line, "\r\n"); s.re.MatchString(line) {
			if s.n >= searchMatches || !s.out.add(fmt.Sprintf("%s:%d: %s", name, n, cutLine(line, searchLineChars))) {
				s.out.full = true
				return errStop
			}
			s.n++
		}
		if err != nil {
			// The end of the file, or a read error: either way the file is
			// done, and a file that fails partway is skipped as grep does.
			return nil
		}
	}
}

// Paths implements Pather.
func (searchTool) Paths(input json.RawMessage) ([]string, error) { return pathField(input, ".") }
