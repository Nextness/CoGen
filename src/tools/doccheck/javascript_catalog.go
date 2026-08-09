package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	javascriptFunctionStart = regexp.MustCompile(`^[[:space:]]*(?:export[[:space:]]+)?(?:async[[:space:]]+)?function[[:space:]]*\*?[[:space:]]*[A-Za-z_$][A-Za-z0-9_$]*(?:<[^()]*>)?`)
	javascriptFunction      = regexp.MustCompile(`^[[:space:]]*(?:export[[:space:]]+)?(?:default[[:space:]]+)?(async[[:space:]]+)?function[[:space:]]+([A-Za-z_$][A-Za-z0-9_$]*)(?:[[:space:]]*<[^()]*>)?([[:space:]]*\([^()]*(?:\([^()]*\)[^()]*)*\))`)
	javascriptClassStart    = regexp.MustCompile(`^[[:space:]]*(?:export[[:space:]]+)?class[[:space:]]+[A-Za-z_$][A-Za-z0-9_$]*(?:<[^()]*>)?`)
	javascriptClass         = regexp.MustCompile(`^[[:space:]]*(?:export[[:space:]]+)?class[[:space:]]+([A-Za-z_$][A-Za-z0-9_$]*)(?:[[:space:]]*<[^()]*>)?([[:space:]]+extends[[:space:]]+[A-Za-z_$][A-Za-z0-9_$.]*(?:<[^()]*>)?)?[[:space:]]*\{`)
	javascriptMethod        = regexp.MustCompile(`^[[:space:]]*(?:(public|private|protected)[[:space:]]+)?(static[[:space:]]+)?(async[[:space:]]+)?([A-Za-z_$][A-Za-z0-9_$]*|constructor)(?:[[:space:]]*<[^()]*>)?([[:space:]]*\([^()]*(?:\([^()]*\)[^()]*)*\))(?:[[:space:]]*:[^\{]*)?[[:space:]]*\{`)
	javascriptMethodStart   = regexp.MustCompile(`^[[:space:]]*(?:(?:public|private|protected)[[:space:]]+)?(?:static[[:space:]]+)?(?:async[[:space:]]+)?[A-Za-z_$][A-Za-z0-9_$]*(?:<[^()]*>)?[[:space:]]*\(`)
	javascriptTest          = regexp.MustCompile("^[[:space:]]*(test|it)\\([[:space:]]*([`\"'])(.*)$")
	jsDocLine               = regexp.MustCompile(`^[[:space:]]*\*[[:space:]]?`)
)

// hasPathPart reports whether a path contains an exact part.
func hasPathPart(path, part string) bool {
	for _, value := range strings.Split(filepath.ToSlash(path), "/") {
		if value == part {
			return true
		}
	}
	return false
}

// projectJavaScriptFiles returns maintained project-authored JavaScript paths.
func projectJavaScriptFiles(root string) ([]string, error) {
	roots := []string{"frontend"}
	allowedSuffixes := map[string]bool{".js": true, ".cjs": true, ".mjs": true, ".ts": true}
	paths := make(map[string]bool)
	for _, relativeRoot := range roots {
		err := filepath.WalkDir(filepath.Join(root, relativeRoot), func(path string, item os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if item.IsDir() {
				if hasPathPart(path, "vendor") || hasPathPart(path, "node_modules") || hasPathPart(path, "dist") || hasPathPart(path, "dist-ts") {
					return filepath.SkipDir
				}
				return nil
			}
			if !item.Type().IsRegular() || !allowedSuffixes[filepath.Ext(path)] || strings.HasSuffix(path, ".d.ts") {
				return nil
			}
			relativePath, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			paths[filepath.ToSlash(relativePath)] = true
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	result := make([]string, 0, len(paths))
	for path := range paths {
		result = append(result, path)
	}
	sort.Strings(result)
	return result, nil
}

// javascriptDoc returns an adjacent JSDoc description and tags without inferring behavior.
func javascriptDoc(lines []string, declaration int) string {
	end := declaration - 1
	if end < 0 || !strings.HasSuffix(strings.TrimSpace(lines[end]), "*/") {
		return noDescription
	}
	start := end
	for start >= 0 && !strings.HasPrefix(strings.TrimSpace(lines[start]), "/**") {
		start--
	}
	if start < 0 {
		return noDescription
	}
	var parts []string
	for _, line := range lines[start : end+1] {
		value := strings.TrimSpace(line)
		value = strings.TrimPrefix(value, "/**")
		value = strings.TrimSuffix(value, "*/")
		value = jsDocLine.ReplaceAllString(value, "")
		value = strings.TrimSpace(value)
		if value != "" {
			parts = append(parts, value)
		}
	}
	if len(parts) == 0 {
		return noDescription
	}
	return strings.Join(parts, " ")
}

// collectJavaScriptDeclarations returns named declarations and test-title entries.
func collectJavaScriptDeclarations(root string) ([]catalogEntry, []catalogEntry, error) {
	paths, err := projectJavaScriptFiles(root)
	if err != nil {
		return nil, nil, err
	}
	var declarations []catalogEntry
	var tests []catalogEntry
	for _, relativePath := range paths {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relativePath)))
		if err != nil {
			return nil, nil, err
		}
		lines := strings.Split(string(data), "\n")
		className := ""
		classDepth := 0
		for index, line := range lines {
			if match := javascriptClass.FindStringSubmatch(line); match != nil {
				className = match[1]
				classDepth = strings.Count(line, "{") - strings.Count(line, "}")
				signature := "class " + className + strings.TrimSpace(match[2])
				declarations = append(declarations, catalogEntry{file: relativePath, kind: "class", name: className, signature: signature, description: javascriptDoc(lines, index), start: index + 1, end: index + 1})
				continue
			}
			if javascriptClassStart.MatchString(line) {
				return nil, nil, fmt.Errorf("unsupported JavaScript class declaration at %s:%d", relativePath, index+1)
			}
			if className != "" && classDepth == 1 {
				if match := javascriptMethod.FindStringSubmatch(line); match != nil {
					parts := []string{match[1], match[2], match[3]}
					var nonEmpty []string
					for _, part := range parts {
						if strings.TrimSpace(part) != "" {
							nonEmpty = append(nonEmpty, strings.TrimSpace(part))
						}
					}
					prefix := strings.Join(nonEmpty, " ")
					if prefix != "" {
						prefix += " "
					}
					name := className + "." + match[4]
					declarations = append(declarations, catalogEntry{file: relativePath, kind: "method", name: name, signature: prefix + match[4] + match[5], description: javascriptDoc(lines, index), start: index + 1, end: index + 1})
				} else if javascriptMethodStart.MatchString(line) {
					return nil, nil, fmt.Errorf("unsupported JavaScript class method declaration at %s:%d", relativePath, index+1)
				}
			}
			if match := javascriptFunction.FindStringSubmatch(line); match != nil {
				prefix := "function "
				if strings.TrimSpace(match[1]) != "" {
					prefix = "async function "
				}
				declarations = append(declarations, catalogEntry{file: relativePath, kind: "function", name: match[2], signature: prefix + match[2] + match[3], description: javascriptDoc(lines, index), start: index + 1, end: index + 1})
			} else if javascriptFunctionStart.MatchString(line) {
				return nil, nil, fmt.Errorf("unsupported JavaScript function declaration at %s:%d", relativePath, index+1)
			}
			if strings.HasPrefix(relativePath, "frontend/tests/") {
				if match := javascriptTest.FindStringSubmatch(line); match != nil {
					quote := match[2]
					if end := strings.Index(match[3], quote); end >= 0 {
						title := match[3][:end]
						tests = append(tests, catalogEntry{file: relativePath, kind: "test", name: title, signature: match[1] + "(" + quote + title + quote + ", callback)", description: title, start: index + 1, end: index + 1})
					}
				}
			}
			if className != "" {
				classDepth += strings.Count(line, "{") - strings.Count(line, "}")
				if classDepth <= 0 {
					className = ""
					classDepth = 0
				}
			}
		}
	}
	return declarations, tests, nil
}
