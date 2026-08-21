package wig

import (
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)

type LangConfig struct {
	Languages       []Language                      `toml:"language,omitempty"`
	LanguageServers map[string]LanguageServerConfig `toml:"language-server,omitempty"`
}

type Language struct {
	Name            string `toml:"name"`
	FileTypes       []any  `toml:"file-types"`
	LanguageServers []any  `toml:"language-servers"`
	Indent          Indent `toml:"indent,omitempty"`
}

type Indent struct {
	Unit     string `toml:"unit,omitempty"`
	TabWidth int    `toml:"tab-width,omitempty"`
}

type LanguageServerConfig struct {
	Language Language
	Command  string   `toml:"command"`
	Args     []string `toml:"args"`
}

func (l Language) GetFileTypes() (exts []string, globs []string) {
	for _, entry := range l.FileTypes {
		switch e := entry.(type) {
		case string:
			exts = append(exts, e)
		case map[string]any:
			if g, ok := e["glob"].(string); ok {
				globs = append(globs, g)
			}
		}
	}
	return
}

func (l Language) GetLanguageServers() (servers []string) {
	for _, entry := range l.LanguageServers {
		switch e := entry.(type) {
		case string:
			servers = append(servers, e)
		}
	}
	return
}

func LoadLanguagesConfig() LangConfig {
	colorThemeFile := EditorInst.RuntimeDir(fmt.Sprintf("%s.toml", "languages"))
	data, err := os.ReadFile(colorThemeFile)
	if err != nil {
		// Fail soft: no languages.toml just means no LSP servers and no
		// per-language indent config, not a reason to crash on startup.
		return LangConfig{}
	}
	cfg := LangConfig{}
	err = toml.Unmarshal(data, &cfg)
	if err != nil {
		return LangConfig{}
	}
	return cfg
}
