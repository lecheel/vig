package wig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	CommentToken    string `toml:"comment-token,omitempty"`
}

var languageCommentTokens = map[string]string{}

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

// GetCommentToken returns the single-line comment prefix for the given buffer.
func GetCommentToken(buf *Buffer) string {
	if buf == nil {
		return "//"
	}
	return CommentTokenForFile(buf.FilePath)
}

// CommentTokenForFile determines the appropriate comment token by file extension,
// filename, or user configuration in languages.toml.
func CommentTokenForFile(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	base := strings.ToLower(filepath.Base(filePath))

	if token, ok := languageCommentTokens[ext]; ok && token != "" {
		return token
	}

	switch base {
	case "dockerfile", "makefile", "cmakelists.txt", ".gitignore", ".gitconfig", ".env", ".bashrc", ".zshrc", ".profile":
		return "#"
	}

	switch ext {
	case ".toml", ".yaml", ".yml", ".py", ".sh", ".bash", ".zsh", ".fish", ".rb", ".pl", ".pm", ".r",
		".env", ".ini", ".conf", ".cfg", ".properties", ".tf", ".hcl", ".nim":
		return "#"
	case ".lua", ".sql", ".hs", ".lhs", ".ada", ".elm", ".vhdl":
		return "--"
	case ".clj", ".cljs", ".cljc", ".edn", ".lisp", ".lsp", ".el", ".scm", ".ss", ".asm", ".s":
		return ";"
	case ".vim":
		return "\""
	case ".tex", ".latex", ".erl", ".hrl":
		return "%"
	case ".go", ".rs", ".c", ".cpp", ".cc", ".cxx", ".h", ".hpp", ".hh", ".odin", ".zig",
		".js", ".mjs", ".cjs", ".jsx", ".ts", ".tsx", ".java", ".kt", ".kts", ".scala",
		".cs", ".swift", ".php", ".dart", ".proto", ".jsonc", ".vert", ".frag", ".glsl", ".hlsl":
		return "//"
	default:
		return "//"
	}
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
