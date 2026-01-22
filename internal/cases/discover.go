package cases

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/matias/regrada/internal/config"
)

type discoveredCase struct {
	Path string
	Case Case
}

func DiscoverCases(cfg *config.ProjectConfig) ([]Case, error) {
	var casePaths []string
	for _, root := range cfg.Cases.Roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}

			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			rel = filepath.ToSlash(rel)

			if !matchesAny(rel, cfg.Cases.Include) {
				return nil
			}
			if matchesAny(rel, cfg.Cases.Exclude) {
				return nil
			}

			casePaths = append(casePaths, path)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("discover cases: %w", err)
		}
	}

	sort.Strings(casePaths)
	seenIDs := make(map[string]string)
	casesOut := make([]Case, 0, len(casePaths))
	for _, path := range casePaths {
		c, err := LoadCase(path)
		if err != nil {
			return nil, err
		}
		if prev, ok := seenIDs[c.ID]; ok {
			return nil, fmt.Errorf("duplicate case id %q in %s (also in %s)", c.ID, path, prev)
		}
		seenIDs[c.ID] = path
		casesOut = append(casesOut, c)
	}

	return casesOut, nil
}

func matchesAny(path string, globs []string) bool {
	if len(globs) == 0 {
		return false
	}
	for _, glob := range globs {
		if matchGlob(glob, path) {
			return true
		}
	}
	return false
}

func matchGlob(pattern, path string) bool {
	regex, err := globToRegex(pattern)
	if err != nil {
		return false
	}
	return regex.MatchString(path)
}

func globToRegex(pattern string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	escape := func(r rune) {
		if strings.ContainsRune(".+()|[]{}^$\\", r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}

	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				if i+2 < len(pattern) && pattern[i+2] == '/' {
					b.WriteString("(?:.*/)?")
					i += 2
				} else {
					b.WriteString(".*")
					i++
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		case '/':
			b.WriteString("/")
		default:
			escape(rune(pattern[i]))
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}
