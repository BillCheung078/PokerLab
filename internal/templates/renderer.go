package templates

import (
	"fmt"
	"html/template"
	"io/fs"
	stdhttp "net/http"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// Renderer wraps parsed HTML templates and a single render method.
type Renderer struct {
	templates *template.Template
}

// New parses the supplied template patterns relative to the project root.
func New(patterns ...string) (*Renderer, error) {
	if len(patterns) == 0 {
		patterns = []string{"web/templates/**/*.gohtml"}
	}

	files, err := expandPatterns(patterns...)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no templates matched: %v", patterns)
	}

	tmpl, err := template.ParseFiles(files...)
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	return &Renderer{templates: tmpl}, nil
}

// Render executes a named template into the response writer.
func (r *Renderer) Render(w stdhttp.ResponseWriter, name string, data any) error {
	return r.templates.ExecuteTemplate(w, name, data)
}

func expandPatterns(patterns ...string) ([]string, error) {
	var files []string

	for _, pattern := range patterns {
		absolutePattern := resolveProjectPath(pattern)

		if strings.Contains(absolutePattern, string(filepath.Separator)+"**"+string(filepath.Separator)) {
			matches, err := walkRecursivePattern(absolutePattern)
			if err != nil {
				return nil, err
			}
			files = append(files, matches...)
			continue
		}

		matches, err := filepath.Glob(absolutePattern)
		if err != nil {
			return nil, fmt.Errorf("glob %q: %w", pattern, err)
		}
		files = append(files, matches...)
	}

	sort.Strings(files)
	return unique(files), nil
}

func walkRecursivePattern(pattern string) ([]string, error) {
	needle := string(filepath.Separator) + "**" + string(filepath.Separator)
	parts := strings.Split(pattern, needle)
	if len(parts) != 2 {
		return nil, fmt.Errorf("unsupported recursive pattern: %q", pattern)
	}

	root := parts[0]
	filePattern := parts[1]
	if filePattern == "" {
		filePattern = "*"
	}

	var matches []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		ok, matchErr := filepath.Match(filePattern, filepath.Base(path))
		if matchErr != nil {
			return matchErr
		}
		if ok {
			matches = append(matches, path)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %q: %w", root, err)
	}

	return matches, nil
}

func resolveProjectPath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}

	return filepath.Join(projectRoot(), filepath.FromSlash(path))
}

func projectRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}

	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func unique(values []string) []string {
	if len(values) == 0 {
		return values
	}

	result := []string{values[0]}
	for _, value := range values[1:] {
		if value == result[len(result)-1] {
			continue
		}
		result = append(result, value)
	}

	return result
}
