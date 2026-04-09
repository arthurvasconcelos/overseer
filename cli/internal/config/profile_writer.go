package config

import (
	"fmt"
	"os"

	"go.yaml.in/yaml/v3"
)

// ReadConfigNode reads the brain config.yaml at path as a yaml.Node document,
// preserving all comments and formatting. If the file does not exist, returns
// an empty document with a root mapping node.
func ReadConfigNode(path string) (*yaml.Node, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return emptyDoc(), nil
	}
	if err != nil {
		return nil, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if len(doc.Content) == 0 {
		doc.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
	}
	return &doc, nil
}

// WriteConfigNode marshals a yaml.Node document and writes it to path.
func WriteConfigNode(path string, doc *yaml.Node) error {
	data, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// GetProfiles decodes all GitProfile entries from the document.
func GetProfiles(doc *yaml.Node) []GitProfile {
	git := mappingFind(docRoot(doc), "git")
	if git == nil {
		return nil
	}
	profiles := mappingFind(git, "profiles")
	if profiles == nil || profiles.Kind != yaml.SequenceNode {
		return nil
	}
	result := make([]GitProfile, 0, len(profiles.Content))
	for _, child := range profiles.Content {
		var p GitProfile
		if err := child.Decode(&p); err == nil {
			result = append(result, p)
		}
	}
	return result
}

// UpsertProfile adds or replaces a profile (matched by name) in the document.
func UpsertProfile(doc *yaml.Node, p GitProfile) {
	git := getOrCreateMapping(docRoot(doc), "git")
	profiles := getOrCreateSequence(git, "profiles")
	node := profileToNode(p)
	for i, child := range profiles.Content {
		if n := mappingFind(child, "name"); n != nil && n.Value == p.Name {
			profiles.Content[i] = node
			return
		}
	}
	profiles.Content = append(profiles.Content, node)
}

// DeleteProfile removes the profile with the given name. Returns true if found.
func DeleteProfile(doc *yaml.Node, name string) bool {
	git := mappingFind(docRoot(doc), "git")
	if git == nil {
		return false
	}
	profiles := mappingFind(git, "profiles")
	if profiles == nil || profiles.Kind != yaml.SequenceNode {
		return false
	}
	for i, child := range profiles.Content {
		if n := mappingFind(child, "name"); n != nil && n.Value == name {
			profiles.Content = append(profiles.Content[:i], profiles.Content[i+1:]...)
			return true
		}
	}
	return false
}

// GetDefaults decodes GitDefaults from the document.
func GetDefaults(doc *yaml.Node) GitDefaults {
	git := mappingFind(docRoot(doc), "git")
	if git == nil {
		return GitDefaults{}
	}
	defaults := mappingFind(git, "defaults")
	if defaults == nil {
		return GitDefaults{}
	}
	var d GitDefaults
	_ = defaults.Decode(&d)
	return d
}

// SetDefaults writes git.defaults into the document, replacing any existing value.
func SetDefaults(doc *yaml.Node, d GitDefaults) {
	git := getOrCreateMapping(docRoot(doc), "git")
	mappingSet(git, "defaults", defaultsToNode(d))
}

// --- yaml.Node helpers ---

func emptyDoc() *yaml.Node {
	return &yaml.Node{
		Kind:    yaml.DocumentNode,
		Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}},
	}
}

// docRoot returns the root mapping node of a document node.
func docRoot(doc *yaml.Node) *yaml.Node {
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		return doc.Content[0]
	}
	return doc
}

// mappingFind returns the value node for key in a mapping node, or nil.
func mappingFind(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// mappingSet sets key=value in a mapping node, replacing the existing value or appending.
func mappingSet(m *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content[i+1] = value
			return
		}
	}
	m.Content = append(m.Content, scalarStr(key), value)
}

// getOrCreateMapping returns the value mapping node for key, creating it if absent.
func getOrCreateMapping(m *yaml.Node, key string) *yaml.Node {
	if v := mappingFind(m, key); v != nil {
		return v
	}
	v := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	mappingSet(m, key, v)
	return v
}

// getOrCreateSequence returns the value sequence node for key, creating it if absent.
func getOrCreateSequence(m *yaml.Node, key string) *yaml.Node {
	if v := mappingFind(m, key); v != nil {
		return v
	}
	v := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	mappingSet(m, key, v)
	return v
}

func scalarStr(val string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: val}
}

func scalarBool(val bool) *yaml.Node {
	v := "false"
	if val {
		v = "true"
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: v}
}

func profileToNode(p GitProfile) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	addStr := func(key, val string) {
		if val == "" {
			return
		}
		m.Content = append(m.Content, scalarStr(key), scalarStr(val))
	}
	addStr("name", p.Name)
	addStr("email", p.Email)
	addStr("user_name", p.UserName)
	addStr("signing_key", p.SigningKey)
	addStr("gpg_format", p.GPGFormat)
	addStr("gpg_ssh_program", p.GPGSSHProgram)
	addStr("op_account", p.OPAccount)
	if p.CommitGPGSign != nil {
		m.Content = append(m.Content, scalarStr("commit_gpgsign"), scalarBool(*p.CommitGPGSign))
	}
	return m
}

func defaultsToNode(d GitDefaults) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	addStr := func(key, val string) {
		if val == "" {
			return
		}
		m.Content = append(m.Content, scalarStr(key), scalarStr(val))
	}
	addStr("user_name", d.UserName)
	addStr("gpg_format", d.GPGFormat)
	addStr("gpg_ssh_program", d.GPGSSHProgram)
	m.Content = append(m.Content, scalarStr("commit_gpgsign"), scalarBool(d.CommitGPGSign))
	return m
}
