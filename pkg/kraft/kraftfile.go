package kraft

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Kraftfile represents the parsed structure of a Unikraft Kraftfile (spec v0.6+).
// We only model the fields needed for the local dvr developer experience
// (especially target discovery for drun-001).
type Kraftfile struct {
	Spec        string   `yaml:"spec,omitempty"`
	Specification string `yaml:"specification,omitempty"` // legacy alias
	Name        string   `yaml:"name,omitempty"`
	Outdir      string   `yaml:"outdir,omitempty"`
	Unikraft    Component `yaml:"unikraft,omitempty"`
	Runtime     string    `yaml:"runtime,omitempty"`
	Template    string    `yaml:"template,omitempty"`
	Targets     Targets   `yaml:"targets,omitempty"`
	Libraries   map[string]Component `yaml:"libraries,omitempty"`
	// We ignore volumes, cmd, env, rootfs, etc. for now — they are passed through to kraft.
	Raw map[string]interface{} `yaml:",inline"`
}

// Component models unikraft, library, template entries (can be string or object).
// Short form:  unikraft: stable
// Long form:   unikraft: { version: stable, source: ..., kconfig: ... }
type Component struct {
	Source  string            `yaml:"source,omitempty"`
	Version string            `yaml:"version,omitempty"`
	KConfig map[string]string `yaml:"kconfig,omitempty"`
}

// UnmarshalYAML supports both string shorthand and full object form for Component fields.
func (c *Component) UnmarshalYAML(value *yaml.Node) error {
	// Try string first (the very common "unikraft: stable" case)
	var s string
	if err := value.Decode(&s); err == nil {
		c.Version = s // treat the string as the version/channel
		return nil
	}

	// Otherwise decode as the full object
	type alias Component
	var a alias
	if err := value.Decode(&a); err != nil {
		return err
	}
	*c = Component(a)
	return nil
}

// Targets is a custom type to handle both short-form strings ("qemu/x86_64")
// and long-form objects in the YAML list.
type Targets []Target

type Target struct {
	Name string `yaml:"name,omitempty"`
	Plat string `yaml:"plat,omitempty"`
	Arch string `yaml:"arch,omitempty"`
	// Full object form may also contain kconfig, etc.
	KConfig map[string]string `yaml:"kconfig,omitempty"`
}

// UnmarshalYAML implements custom unmarshaling so we accept both:
//   targets:
//     - qemu/x86_64
//     - plat: qemu
//       arch: arm64
//       name: my-target
func (ts *Targets) UnmarshalYAML(value *yaml.Node) error {
	var raw []interface{}
	if err := value.Decode(&raw); err != nil {
		return err
	}

	for _, item := range raw {
		t := Target{}
		switch v := item.(type) {
		case string:
			// Short form: "qemu/x86_64" or "fc/x86_64"
			parts := strings.SplitN(v, "/", 2)
			if len(parts) == 2 {
				t.Plat = parts[0]
				t.Arch = parts[1]
			} else {
				t.Name = v
			}
		case map[string]interface{}:
			if plat, ok := v["plat"].(string); ok {
				t.Plat = plat
			}
			if arch, ok := v["arch"].(string); ok {
				t.Arch = arch
			}
			if platform, ok := v["platform"].(string); ok && t.Plat == "" {
				t.Plat = platform
			}
			if architecture, ok := v["architecture"].(string); ok && t.Arch == "" {
				t.Arch = architecture
			}
			if name, ok := v["name"].(string); ok {
				t.Name = name
			}
			if kcfg, ok := v["kconfig"].(map[string]interface{}); ok {
				t.KConfig = make(map[string]string)
				for k, val := range kcfg {
					if s, ok := val.(string); ok {
						t.KConfig[k] = s
					}
				}
			}
		default:
			// ignore unknown forms
		}
		if t.Plat != "" || t.Arch != "" || t.Name != "" {
			*ts = append(*ts, t)
		}
	}
	return nil
}

// String returns a canonical "plat/arch" or name representation for the target.
func (t Target) String() string {
	if t.Name != "" && (t.Plat == "" || t.Arch == "") {
		return t.Name
	}
	if t.Plat != "" && t.Arch != "" {
		return fmt.Sprintf("%s/%s", t.Plat, t.Arch)
	}
	if t.Plat != "" {
		return t.Plat
	}
	return t.Name
}

// ParseKraftfile reads and parses a Kraftfile from the given path.
// It searches common locations if a directory is passed.
func ParseKraftfile(path string) (*Kraftfile, error) {
	if path == "" {
		path = "."
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("cannot access %s: %w", path, err)
	}

	var kfPath string
	if info.IsDir() {
		for _, candidate := range []string{"Kraftfile", "kraft.yaml", "kraft.yml"} {
			cand := filepath.Join(path, candidate)
			if _, err := os.Stat(cand); err == nil {
				kfPath = cand
				break
			}
		}
		if kfPath == "" {
			return nil, fmt.Errorf("no Kraftfile found in %s (looked for Kraftfile, kraft.yaml, kraft.yml)", path)
		}
	} else {
		kfPath = path
	}

	data, err := os.ReadFile(kfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Kraftfile %s: %w", kfPath, err)
	}

	var kf Kraftfile
	if err := yaml.Unmarshal(data, &kf); err != nil {
		return nil, fmt.Errorf("failed to parse YAML in %s: %w", kfPath, err)
	}

	// Normalize spec
	if kf.Spec == "" {
		kf.Spec = kf.Specification
	}

	return &kf, nil
}

// ListTargetStrings returns human-readable target identifiers (e.g. ["qemu/x86_64", "fc/x86_64"]).
func (kf *Kraftfile) ListTargetStrings() []string {
	if kf == nil {
		return nil
	}
	out := make([]string, 0, len(kf.Targets))
	for _, t := range kf.Targets {
		out = append(out, t.String())
	}
	return out
}

// HasTarget checks whether a specific target string is declared.
func (kf *Kraftfile) HasTarget(target string) bool {
	for _, t := range kf.ListTargetStrings() {
		if t == target {
			return true
		}
	}
	return false
}
