// Package skills loads SKILL.md files from a project repository following the
// skills.sh convention (https://skills.sh). Skills are markdown capability
// bundles installed into the repo via `npx skills add <owner>/<repo>`.
//
// Discovery rules:
//   - any path matching `**/SKILL.md` or `**/skills/**/SKILL.md` under the
//     project root is a candidate.
//   - `node_modules`, `.git`, `dist`, and `target` directories are skipped.
//   - the front-matter (YAML between leading `---` lines) is parsed loosely
//     to extract `name` and `description`; missing values fall back to the
//     enclosing directory name and the first non-empty body line.
//   - file size is capped at maxSkillBytes to avoid prompt explosions.
package skills

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxSkillBytes = 16 * 1024
	maxSkills     = 64
)

// Skill is a loaded SKILL.md bundle ready for prompt injection.
type Skill struct {
	Name        string
	Description string
	Path        string
	Body        string
}

// Load discovers SKILL.md files under projectRoot and returns them.
// Returns an empty slice (not an error) when no skills are present.
func Load(projectRoot string) ([]Skill, error) {
	if projectRoot == "" {
		return nil, errors.New("projectRoot is required")
	}
	info, err := os.Stat(projectRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, nil
	}

	out := make([]Skill, 0, 8)
	walkErr := filepath.WalkDir(projectRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // ignore permission errors and continue
		}
		if d.IsDir() {
			base := d.Name()
			if base == "node_modules" || base == ".git" || base == "dist" || base == "target" || base == "build" {
				return fs.SkipDir
			}
			return nil
		}
		if d.Name() != "SKILL.md" {
			return nil
		}
		if len(out) >= maxSkills {
			return fs.SkipAll
		}
		skill, loaded := loadFile(path)
		if loaded {
			out = append(out, skill)
		}
		return nil
	})
	if walkErr != nil && !errors.Is(walkErr, fs.SkipAll) {
		return out, walkErr
	}
	return out, nil
}

func loadFile(path string) (Skill, bool) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return Skill{}, false
	}
	if info.Size() > maxSkillBytes {
		return Skill{
			Name:        skillNameFromPath(path),
			Description: "(skipped: SKILL.md exceeds 16KB cap)",
			Path:        path,
		}, true
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Skill{}, false
	}
	body := string(data)
	name, description, body := parseFrontMatter(body)
	if name == "" {
		name = skillNameFromPath(path)
	}
	if description == "" {
		description = firstNonEmptyLine(body)
	}
	return Skill{
		Name:        name,
		Description: description,
		Path:        path,
		Body:        strings.TrimSpace(body),
	}, true
}

// skillNameFromPath returns the parent-directory name when a SKILL.md sits in
// `.../skills/<name>/SKILL.md`.
func skillNameFromPath(path string) string {
	parent := filepath.Base(filepath.Dir(path))
	if parent == "" || parent == "." || parent == "/" {
		return "skill"
	}
	return parent
}

// parseFrontMatter reads a leading YAML-style block delimited by `---` lines
// and extracts only the `name` and `description` keys. The remaining body is
// returned with the front-matter stripped. This is intentionally tolerant:
// anything we don't understand is ignored.
func parseFrontMatter(content string) (string, string, string) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Buffer(make([]byte, 0, 8192), maxSkillBytes)
	if !scanner.Scan() {
		return "", "", content
	}
	if strings.TrimSpace(scanner.Text()) != "---" {
		return "", "", content
	}
	var name, description string
	var bodyLines []string
	inFront := true
	for scanner.Scan() {
		line := scanner.Text()
		if inFront {
			if strings.TrimSpace(line) == "---" {
				inFront = false
				continue
			}
			if k, v, ok := splitKV(line); ok {
				switch strings.ToLower(k) {
				case "name":
					name = strings.Trim(v, "\"' ")
				case "description":
					description = strings.Trim(v, "\"' ")
				}
			}
			continue
		}
		bodyLines = append(bodyLines, line)
	}
	body := strings.Join(bodyLines, "\n")
	if !inFront {
		return name, description, body
	}
	// No closing fence - treat whole thing as body.
	return "", "", content
}

func splitKV(line string) (string, string, bool) {
	idx := strings.Index(line, ":")
	if idx <= 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx+1:]), true
}

func firstNonEmptyLine(body string) string {
	for _, raw := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		// Skip markdown headings to grab the first descriptive line.
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if len(trimmed) > 200 {
			return trimmed[:200] + "…"
		}
		return trimmed
	}
	return ""
}

// FormatPromptSection renders skills into a compact prompt section. Returns an
// empty string when no skills are present so the caller can skip the heading.
func FormatPromptSection(skills []Skill) string {
	if len(skills) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Installed Skills (skills.sh)\n")
	b.WriteString("These project-scoped capability bundles are available. Reference them by name when relevant.\n\n")
	for _, skill := range skills {
		b.WriteString("### ")
		b.WriteString(skill.Name)
		b.WriteString("\n")
		if skill.Description != "" {
			b.WriteString(skill.Description)
			b.WriteString("\n")
		}
		if skill.Body != "" {
			b.WriteString(truncate(skill.Body, 1200))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max] + "…"
}
