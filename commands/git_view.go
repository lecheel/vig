package commands

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/firstrow/wig"
	"github.com/firstrow/wig/ui"
)

const gitViewMaxBranches = 20

// ── helpers ──────────────────────────────────────────────

func gitRun(args ...string) string {
	cmd := exec.Command("git", args...)
	out, _ := cmd.Output()
	return string(out)
}

func gitIsRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	return cmd.Run() == nil
}

func gitUnquotePath(path string) string {
	if len(path) < 2 || path[0] != '"' {
		return path
	}
	if uq, err := strconv.Unquote(path); err == nil {
		return uq
	}
	return path
}

func gitParsePorcelainPath(entryType byte, rawPath string) string {
	path := rawPath
	if entryType == '2' {
		if idx := strings.Index(path, " "); idx >= 0 {
			path = path[idx+1:]
		}
		if idx := strings.Index(path, "\t"); idx >= 0 {
			path = path[:idx]
		}
	}
	return gitUnquotePath(path)
}

// ── data: git status panel ───────────────────────────────

// GetGitStatusItems builds the full git status panel data.
func GetGitStatusItems() []wig.GitViewItem {
	if !gitIsRepo() {
		return []wig.GitViewItem{
			{Type: "header", Label: "Git Status"},
			{Type: "separator"},
			{Type: "empty", Label: "Not a git repository"},
		}
	}

	var items []wig.GitViewItem

	// Current branch
	curBranchOut, _ := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	curBranch := strings.TrimSpace(string(curBranchOut))

	// Branches
	branchOut, _ := exec.Command("git", "for-each-ref",
		"--sort=-committerdate", "refs/heads/",
		"--format=%(refname:short)|%(committerdate:relative)").Output()
	branchLines := strings.Split(strings.TrimSpace(string(branchOut)), "\n")

	type rawBranch struct {
		name     string
		timeAgo  string
		isActive bool
	}
	var rawBranches []rawBranch
	for _, b := range branchLines {
		if b == "" {
			continue
		}
		parts := strings.SplitN(b, "|", 2)
		if len(parts) < 2 {
			continue
		}
		name := parts[0]
		timeAgo := parts[1]
		rawBranches = append(rawBranches, rawBranch{
			name:     name,
			timeAgo:  timeAgo,
			isActive: name == curBranch,
		})
	}

	// Limit branches, ensuring active is kept
	if len(rawBranches) > gitViewMaxBranches {
		activeIncluded := false
		for i := 0; i < gitViewMaxBranches; i++ {
			if rawBranches[i].isActive {
				activeIncluded = true
				break
			}
		}
		if activeIncluded || curBranch == "" {
			rawBranches = rawBranches[:gitViewMaxBranches]
		} else {
			trimmed := make([]rawBranch, 0, gitViewMaxBranches)
			trimmed = append(trimmed, rawBranches[:gitViewMaxBranches-1]...)
			for _, rb := range rawBranches {
				if rb.isActive {
					trimmed = append(trimmed, rb)
					break
				}
			}
			rawBranches = trimmed
		}
	}

	maxNameLen := 0
	for _, rb := range rawBranches {
		if len(rb.name) > maxNameLen {
			maxNameLen = len(rb.name)
		}
	}

	var branches []wig.GitViewItem
	for _, rb := range rawBranches {
		prefix := "  "
		status := "branch"
		if rb.isActive {
			prefix = "* "
			status = "active_branch"
		}
		branches = append(branches, wig.GitViewItem{
			Type:     "branch",
			Label:    fmt.Sprintf("%s%-*s %s", prefix, maxNameLen, rb.name, rb.timeAgo),
			Status:   status,
			FilePath: rb.name,
			StashRef: rb.name,
		})
	}

	// HEAD hash
	hashOut, _ := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	headHash := strings.TrimSpace(string(hashOut))

	// Stash list
	stashOut, _ := exec.Command("git", "stash", "list").Output()
	stashes := strings.Split(strings.TrimSpace(string(stashOut)), "\n")

	// Status (porcelain v2)
	statusOut, _ := exec.Command("git", "status", "--porcelain=v2").Output()
	statusLines := strings.Split(string(statusOut), "\n")

	var staged, unstaged, untracked []wig.GitViewItem
	for _, line := range statusLines {
		if line == "" {
			continue
		}
		switch line[0] {
		case '1', '2':
			parts := strings.SplitN(line, " ", 9)
			if len(parts) < 9 {
				continue
			}
			xy := parts[1]
			if len(xy) < 2 {
				continue
			}
			x, y := xy[0], xy[1]
			path := gitParsePorcelainPath(line[0], parts[8])
			if x != '.' {
				staged = append(staged, wig.GitViewItem{
					Type: "file", Label: path, Status: "staged",
					FilePath: path, Code: string(x),
				})
			}
			if y != '.' {
				unstaged = append(unstaged, wig.GitViewItem{
					Type: "file", Label: path, Status: "unstaged",
					FilePath: path, Code: string(y),
				})
			}
		case '?':
			path := gitUnquotePath(strings.TrimPrefix(line, "? "))
			untracked = append(untracked, wig.GitViewItem{
				Type: "file", Label: path, Status: "untracked",
				FilePath: path, Code: "?",
			})
		}
	}

	// Last commit files
	lastCommitFiles := getGitLastCommitFiles()

	// ── assemble panel ──

	addSection := func(header string, sectionItems []wig.GitViewItem) {
		items = append(items, wig.GitViewItem{Type: "header", Label: header})
		items = append(items, wig.GitViewItem{Type: "separator"})
		if len(sectionItems) > 0 {
			items = append(items, sectionItems...)
		} else {
			items = append(items, wig.GitViewItem{Type: "empty", Label: "(none)"})
		}
		items = append(items, wig.GitViewItem{Type: "blank"})
	}

	addSection(fmt.Sprintf("Stage Changes (%d)", len(staged)), staged)
	addSection(fmt.Sprintf("Unstage Changes (%d)", len(unstaged)), unstaged)
	addSection(fmt.Sprintf("Untracked Files (%d)", len(untracked)), untracked)
	addSection(fmt.Sprintf("Last Commit [%s] (%d)", headHash, len(lastCommitFiles)), lastCommitFiles)
	addSection("Branches", branches)

	// Stash (no trailing blank — last section)
	items = append(items, wig.GitViewItem{Type: "header", Label: "Stash"})
	items = append(items, wig.GitViewItem{Type: "separator"})
	hasStashes := false
	if len(stashes) > 0 && stashes[0] != "" {
		for _, s := range stashes {
			parts := strings.SplitN(s, ":", 2)
			if len(parts) == 2 {
				items = append(items, wig.GitViewItem{
					Type: "stash", Label: strings.TrimSpace(parts[1]),
					StashRef: parts[0],
				})
				hasStashes = true
			}
		}
	}
	if !hasStashes {
		items = append(items, wig.GitViewItem{Type: "empty", Label: "(none)"})
	}

	return items
}

func getGitLastCommitFiles() []wig.GitViewItem {
	out := gitRun("diff-tree", "--no-commit-id", "--name-status", "-r", "HEAD")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	var items []wig.GitViewItem
	for _, l := range lines {
		if l == "" {
			continue
		}
		parts := strings.SplitN(l, "\t", 2)
		if len(parts) < 2 {
			continue
		}
		path := gitUnquotePath(parts[1])
		items = append(items, wig.GitViewItem{
			Type: "file", Label: path, Status: "last_commit",
			FilePath: path, Code: parts[0],
		})
	}
	return items
}

// ── data: git log ────────────────────────────────────────

func GetGitLogItems(n int) []wig.CommitItem {
	if !gitIsRepo() {
		return []wig.CommitItem{{Subject: "Not a git repository"}}
	}
	out := gitRun("log", fmt.Sprintf("-%d", n),
		"--pretty=format:%h\x1f%an\x1f%ar\x1f%s")
	lines := strings.Split(out, "\n")
	var items []wig.CommitItem
	for _, l := range lines {
		if l == "" {
			continue
		}
		parts := strings.SplitN(l, "\x1f", 4)
		if len(parts) < 4 {
			continue
		}
		items = append(items, wig.CommitItem{
			Hash: parts[0], Author: parts[1],
			Date: parts[2], Subject: parts[3],
		})
	}
	return items
}

// ── data: utility functions ──────────────────────────────

func GetGitStatusFiles() []string {
	if !gitIsRepo() {
		return nil
	}
	out := gitRun("status", "--porcelain=v2")
	lines := strings.Split(out, "\n")
	var paths []string
	seen := make(map[string]bool)
	for _, line := range lines {
		if line == "" || (line[0] != '1' && line[0] != '2') {
			continue
		}
		parts := strings.SplitN(line, " ", 9)
		if len(parts) < 9 {
			continue
		}
		path := gitParsePorcelainPath(line[0], parts[8])
		if !seen[path] {
			seen[path] = true
			paths = append(paths, path)
		}
	}
	return paths
}

func GetGitLsFiles() []string {
	if !gitIsRepo() {
		return nil
	}
	out := gitRun("ls-files")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	var paths []string
	for _, l := range lines {
		if l != "" {
			paths = append(paths, gitUnquotePath(l))
		}
	}
	return paths
}

func GetGitFileStatusMap() map[string]string {
	out := gitRun("status", "--porcelain=v2")
	lines := strings.Split(out, "\n")
	statuses := make(map[string]string)
	for _, line := range lines {
		if line == "" {
			continue
		}
		switch line[0] {
		case '1', '2':
			parts := strings.SplitN(line, " ", 9)
			if len(parts) < 9 {
				continue
			}
			xy := parts[1]
			if len(xy) < 2 {
				continue
			}
			path := gitParsePorcelainPath(line[0], parts[8])
			x, y := xy[0], xy[1]
			code := "M"
			switch {
			case x == 'A' || y == 'A':
				code = "A"
			case x == 'D' || y == 'D':
				code = "D"
			case x == 'R' || y == 'R':
				code = "R"
			}
			statuses[path] = code
		case '?':
			path := gitUnquotePath(strings.TrimPrefix(line, "? "))
			statuses[path] = "?"
		}
	}
	return statuses
}

// ── data: remote / repo IP ───────────────────────────────

func GetRemoteHost(s string) (string, bool) {
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") ||
		strings.HasPrefix(s, "ssh://") || strings.HasPrefix(s, "git://") {
		u, err := url.Parse(s)
		if err != nil {
			return "", false
		}
		return u.Hostname(), true
	}
	if idx := strings.Index(s, "@"); idx != -1 {
		rest := s[idx+1:]
		if cIdx := strings.Index(rest, ":"); cIdx != -1 {
			return rest[:cIdx], true
		} else if sIdx := strings.Index(rest, "/"); sIdx != -1 {
			return rest[:sIdx], true
		} else {
			return rest, true
		}
	}
	return s, true
}

func GetRepoIPAddress() string {
	if !gitIsRepo() {
		return "NA"
	}
	out, err := exec.Command("git", "config", "--get", "remote.origin.url").Output()
	if err != nil {
		return "NA"
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return "NA"
	}
	host, ok := GetRemoteHost(s)
	if !ok {
		return "NA"
	}
	if net.ParseIP(host) == nil {
		return "NA"
	}
	return host
}

// ── data: commit helpers ─────────────────────────────────

func GetHeadCommitMessage() string {
	if !gitIsRepo() {
		return ""
	}
	out, err := exec.Command("git", "log", "-1", "--pretty=%B").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

var (
	commitHeaderRegex = regexp.MustCompile("^\\[(\\S+)(?: \\S+)? ([0-9a-f]+)\\] (.*)$")
	commitStatRegex   = regexp.MustCompile("(\\d+) files? changed(?:, (\\d+) insertions?\\(\\+\\))?(?:, (\\d+) deletions?\\(-\\))?")
)

// FormatCommitSummary turns raw git commit output into a single-line
// status-bar message: hash truncated subject stat counts.
func FormatCommitSummary(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return "Commit complete"
	}
	hash := ""
	subject := lines[0]
	if mm := commitHeaderRegex.FindStringSubmatch(lines[0]); mm != nil {
		hash = mm[2]
		subject = mm[3]
	}
	const maxSubject = 80
	if len(subject) > maxSubject {
		subject = subject[:maxSubject-1] + "..."
	}
	stats := ""
	if len(lines) > 1 {
		if mm := commitStatRegex.FindStringSubmatch(lines[1]); mm != nil {
			var parts []string
			if mm[1] != "" {
				label := "file"
				if mm[1] != "1" {
					label = "files"
				}
				parts = append(parts, mm[1]+" "+label)
			}
			if mm[2] != "" {
				parts = append(parts, "+"+mm[2])
			}
			if mm[3] != "" {
				parts = append(parts, "-"+mm[3])
			}
			stats = strings.Join(parts, " ")
		}
	}
	result := subject
	if hash != "" {
		result = hash + "  " + result
	}
	if stats != "" {
		result += "  -  " + stats
	}
	return result
}

// ── diff preview ─────────────────────────────────────────

// GetGitDiffLines returns diff lines for a file item, suitable for
// rendering with line-by-line styling in the UI widget.
func GetGitDiffLines(item wig.GitViewItem) []string {
	var out string
	switch item.Status {
	case "staged":
		out = gitRun("diff", "--staged", "--", item.FilePath)
	case "unstaged":
		out = gitRun("diff", "--", item.FilePath)
	case "last_commit":
		out = gitRun("diff", "HEAD~1", "HEAD", "--", item.FilePath)
	case "untracked":
		data, err := os.ReadFile(item.FilePath)
		if err != nil {
			return []string{fmt.Sprintf("Cannot read file: %v", err)}
		}
		result := []string{"--- /dev/null", "+++ " + item.FilePath}
		for _, l := range strings.Split(string(data), "\n") {
			result = append(result, "+"+l)
		}
		return result
	default:
		return nil
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return []string{"(no changes)"}
	}
	return strings.Split(out, "\n")
}

// ── command ──────────────────────────────────────────────

// GitIsDirty returns true if there are staged, unstaged, or untracked changes.
func GitIsDirty() bool {
	out := gitRun("status", "--porcelain")
	return strings.TrimSpace(out) != ""
}

// GitSwitchBranch switches to the specified branch.
func GitSwitchBranch(item wig.GitViewItem) error {
	if item.Type != "branch" {
		return nil
	}
	branchName := item.StashRef
	if branchName == "" {
		branchName = item.FilePath
	}
	if branchName == "" {
		return fmt.Errorf("invalid branch name")
	}

	curBranchOut, _ := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	curBranch := strings.TrimSpace(string(curBranchOut))
	if branchName == curBranch {
		return fmt.Errorf("already on branch '%s'", branchName)
	}

	cmd := exec.Command("git", "checkout", branchName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		outStr := strings.TrimSpace(string(out))
		if outStr == "" {
			outStr = err.Error()
		}
		outStr = strings.ReplaceAll(outStr, "\n", " ")
		return fmt.Errorf("%s", outStr)
	}
	return nil
}

type gitStatusLine struct {
	kind     string
	item     wig.GitViewItem
	filePath string
}

// GitAiCommit controls whether git commit automatically uses AI to generate commit message
var GitAiCommit = false

// GitAiTool specifies the command used to generate commit messages (default: "git-ai --tool")
var GitAiTool = "git-ai --tool"

// GitAiTimeout is a safety-net upper bound on how long git-ai --tool is
// allowed to run before wig kills it itself. git-ai already enforces its
// own ~120s internal timeout; this just guards against that mechanism
// failing. Manual cancellation (Esc) is the normal way to stop it early.
var GitAiTimeout = 150 * time.Second

var (
	gitAiMu     sync.Mutex
	gitAiCancel context.CancelFunc
)

func cancelActiveGitAi() bool {
	gitAiMu.Lock()
	defer gitAiMu.Unlock()
	if gitAiCancel != nil {
		gitAiCancel()
		gitAiCancel = nil
		return true
	}
	return false
}

// generateGitAiCommitMessage runs git-ai --tool under the given context, so
// the caller can cancel it early (ctx cancelled) or let it run until
// GitAiTimeout elapses. It must not touch anything on ctx.Buf/ctx.Win since
// it's designed to be called from a background goroutine.
func generateGitAiCommitMessage(cctx context.Context, editor *wig.Editor, rootDir string) string {
	toolCmd := GitAiTool
	if toolCmd == "" {
		toolCmd = "git-ai --tool"
	}
	parts := strings.Fields(toolCmd)
	if len(parts) == 0 {
		parts = []string{"git-ai", "--tool"}
	}

	cmd := exec.CommandContext(cctx, parts[0], parts[1:]...)
	if rootDir != "" {
		cmd.Dir = rootDir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		switch cctx.Err() {
		case context.Canceled:
			// User cancelled manually (e.g. pressed Esc); not an error worth logging.
		case context.DeadlineExceeded:
			editor.LogMessage(fmt.Sprintf("git-ai timed out after %s and was killed", GitAiTimeout))
		default:
			editor.LogMessage("git-ai error: " + err.Error() + " output: " + string(out))
		}
	}

	// 1. Try reading /tmp/commit-edit.txt written by git-ai --tool
	data, err := os.ReadFile("/tmp/commit-edit.txt")
	if err == nil && len(strings.TrimSpace(string(data))) > 0 {
		return strings.TrimSpace(string(data))
	}

	// 2. Fallback to command stdout if any
	outStr := strings.TrimSpace(string(out))
	if len(outStr) > 0 && !strings.Contains(outStr, "error") {
		return outStr
	}

	return ""
}

// runGitAiWithSpinner runs git-ai --tool in the background and animates a
// spinner with elapsed seconds in the status bar echo area. Esc or q cancels
// the async process early. When generation finishes, onSuccess is invoked.
func runGitAiWithSpinner(ctx wig.Context, onSuccess func(msg string)) {
	cancelActiveGitAi()

	rootDir := ctx.Editor.Projects.GetRoot()
	cctx, cancel := context.WithTimeout(context.Background(), GitAiTimeout)

	gitAiMu.Lock()
	gitAiCancel = cancel
	gitAiMu.Unlock()

	done := make(chan struct{})

	go startSpinner(ctx.Editor, "Generating AI commit message... (Esc to cancel)", done)

	go func() {
		msg := generateGitAiCommitMessage(cctx, ctx.Editor, rootDir)
		cancelled := cctx.Err() == context.Canceled

		gitAiMu.Lock()
		gitAiCancel = nil
		gitAiMu.Unlock()
		cancel()
		close(done)

		if cancelled {
			ctx.Editor.EchoMessage("AI commit message generation cancelled")
			ctx.Editor.Redraw()
			return
		}

		if msg == "" {
			ctx.Editor.EchoMessage("git-ai did not return a message; please enter manually")
		} else {
			ctx.Editor.EchoMessage("AI commit message generated")
		}
		onSuccess(msg)
		ctx.Editor.Redraw()
	}()
}

// GitStatusHighlighter provides syntax coloring for the buffer-based git status panel.
type GitStatusHighlighter struct {
	Buf     *wig.Buffer
	LineMap map[int]gitStatusLine
}

func (h *GitStatusHighlighter) Build()                          {}
func (h *GitStatusHighlighter) TextChanged(wig.EventTextChange) {}

func (h *GitStatusHighlighter) HighlightLine(lineNum int) []wig.Span {
	if h.Buf == nil || h.LineMap == nil {
		return nil
	}
	entry, ok := h.LineMap[lineNum]
	if !ok {
		return nil
	}
	line := wig.CursorLineByNum(h.Buf, lineNum)
	if line == nil {
		return nil
	}
	text := line.Value.String()
	runes := []rune(text)
	lineLen := uint16(len(runes))
	if lineLen == 0 {
		return nil
	}

	var spans []wig.Span
	switch entry.kind {
	case "shortcut":
		inBracket := false
		lastIdx := 0
		for idx, r := range runes {
			if r == '[' {
				if idx > lastIdx {
					spans = append(spans, wig.Span{
						StartCol: uint16(lastIdx),
						EndCol:   uint16(idx),
						Style:    wig.Color("comment"),
					})
				}
				inBracket = true
				lastIdx = idx
			} else if r == ']' && inBracket {
				spans = append(spans, wig.Span{
					StartCol: uint16(lastIdx),
					EndCol:   uint16(idx + 1),
					Style:    wig.Color("constant"),
				})
				inBracket = false
				lastIdx = idx + 1
			}
		}
		if lastIdx < len(runes) {
			spans = append(spans, wig.Span{
				StartCol: uint16(lastIdx),
				EndCol:   uint16(len(runes)),
				Style:    wig.Color("comment"),
			})
		}
	case "header":
		spans = append(spans, wig.Span{
			StartCol: 0,
			EndCol:   lineLen,
			Style:    wig.Color("ui.statusline"),
		})
	case "file":
		codeStyle := "ui.text"
		switch entry.item.Code {
		case "M", "R":
			codeStyle = "diff.delta"
		case "A":
			codeStyle = "diff.plus"
		case "D":
			codeStyle = "diff.minus"
		case "?":
			codeStyle = "ui.linenr"
		}
		if lineLen >= 3 {
			spans = append(spans, wig.Span{
				StartCol: 2,
				EndCol:   3,
				Style:    wig.Color(codeStyle),
			})
		}
		if lineLen >= 5 {
			spans = append(spans, wig.Span{
				StartCol: 5,
				EndCol:   lineLen,
				Style:    wig.Color("ui.text"),
			})
		}
	case "branch":
		if entry.item.Status == "active_branch" {
			cut := uint16(min(4, int(lineLen)))
			spans = append(spans, wig.Span{
				StartCol: 0,
				EndCol:   cut,
				Style:    wig.Color("diff.plus"),
			})
			spans = append(spans, wig.Span{
				StartCol: cut,
				EndCol:   lineLen,
				Style:    wig.Color("ui.linenr.selected"),
			})
		} else {
			spans = append(spans, wig.Span{
				StartCol: 0,
				EndCol:   lineLen,
				Style:    wig.Color("ui.text"),
			})
		}
	case "stash":
		colonIdx := strings.Index(text, ":")
		if colonIdx > 0 {
			spans = append(spans, wig.Span{
				StartCol: 2,
				EndCol:   uint16(colonIdx),
				Style:    wig.Color("constant"),
			})
			spans = append(spans, wig.Span{
				StartCol: uint16(colonIdx),
				EndCol:   lineLen,
				Style:    wig.Color("ui.text"),
			})
		}
	case "empty":
		spans = append(spans, wig.Span{
			StartCol: 0,
			EndCol:   lineLen,
			Style:    wig.Color("ui.linenr"),
		})
	}

	return spans
}

func getGitStatusLineMap(buf *wig.Buffer) map[int]gitStatusLine {
	if buf != nil && buf.Highlighter != nil {
		if hl, ok := buf.Highlighter.(*GitStatusHighlighter); ok {
			return hl.LineMap
		}
	}
	return nil
}

func populateGitStatusBuffer(buf *wig.Buffer) (map[int]gitStatusLine, int) {
	items := GetGitStatusItems()
	lineMap := make(map[int]gitStatusLine)
	lines := make([]string, 0, len(items)+2)

	// Top shortcuts guide bar
	lines = append(lines, "  [Enter] Open/Stash  [s] Stage  [d] Diff  [c] Commit  [a] AI Commit  [p] Push  [z] Stash  [r] Refresh  [Esc] Close")
	lineMap[0] = gitStatusLine{kind: "shortcut"}
	lines = append(lines, "")
	lineMap[1] = gitStatusLine{kind: "none"}

	firstSelectable := 2
	foundSelectable := false

	for _, it := range items {
		var lineText string
		kind := it.Type
		switch it.Type {
		case "header":
			lineText = fmt.Sprintf("── %s ", it.Label)
			pad := 50 - len([]rune(lineText))
			if pad > 0 {
				lineText += strings.Repeat("─", pad)
			}
		case "separator", "blank":
			lineText = ""
		case "empty":
			lineText = fmt.Sprintf("   %s", it.Label)
		case "file":
			lineText = fmt.Sprintf("  %s  %s", it.Code, it.FilePath)
			if !foundSelectable {
				firstSelectable = len(lines)
				foundSelectable = true
			}
		case "branch":
			lineText = fmt.Sprintf("  %s", it.Label)
			if !foundSelectable {
				firstSelectable = len(lines)
				foundSelectable = true
			}
		case "stash":
			lineText = fmt.Sprintf("  %s: %s", it.StashRef, it.Label)
			if !foundSelectable {
				firstSelectable = len(lines)
				foundSelectable = true
			}
		default:
			lineText = it.Label
		}

		lineIdx := len(lines)
		lineMap[lineIdx] = gitStatusLine{
			kind:     kind,
			item:     it,
			filePath: it.FilePath,
		}
		lines = append(lines, lineText)
	}

	buf.ResetLines()
	for _, l := range lines {
		buf.Append(l)
	}

	buf.Highlighter = &GitStatusHighlighter{
		Buf:     buf,
		LineMap: lineMap,
	}

	return lineMap, firstSelectable
}

// CmdGitView opens or toggles the buffer-based git status panel.
func CmdGitView(ctx wig.Context) {
	if !gitIsRepo() {
		ctx.Editor.EchoMessage("Not a git repository")
		return
	}

	gitBuf := ctx.Editor.BufferFindByFilePath("[git]", false)
	if gitBuf != nil && ctx.Editor.ActiveBuffer() == gitBuf {
		// Toggle off: close split or kill buffer
		if len(ctx.Editor.Windows) > 1 {
			wig.CmdWindowCloseAndKillBuffer(ctx)
		} else {
			wig.CmdKillBuffer(ctx)
		}
		return
	}

	if gitBuf == nil {
		gitBuf = wig.NewBuffer()
		gitBuf.FilePath = "[git]"
		ctx.Editor.Buffers = append(ctx.Editor.Buffers, gitBuf)
	}

	_, firstLine := populateGitStatusBuffer(gitBuf)
	setupGitStatusKeyHandler(gitBuf)

	useSplit := ctx.Editor.Config.GitStatusView != "full"

	if useSplit && len(ctx.Editor.Windows) == 1 {
		wig.CmdWindowVSplit(ctx)
		wig.CmdWindowNext(ctx)
	}

	ctx.Buf = gitBuf
	ctx.Editor.ActiveWindow().VisitBuffer(ctx, wig.Cursor{Line: firstLine, Char: 0})
	wig.CmdCursorCenter(ctx)
}

func setupGitStatusKeyHandler(gitBuf *wig.Buffer) {
	var pendingStash *wig.GitViewItem

	refresh := func(ctx wig.Context, preferredPath string) {
		pendingStash = nil
		cur := wig.ContextCursorGet(ctx)
		lineIdx := 0
		if cur != nil {
			lineIdx = cur.Line
		}

		lineMap, _ := populateGitStatusBuffer(gitBuf)

		if preferredPath != "" {
			for l, entry := range lineMap {
				if entry.filePath == preferredPath {
					lineIdx = l
					break
				}
			}
		}

		if lineIdx >= gitBuf.Lines.Len {
			lineIdx = gitBuf.Lines.Len - 1
		}
		if lineIdx < 0 {
			lineIdx = 0
		}

		ctx.Buf = gitBuf
		ctx.Editor.ActiveWindow().VisitBuffer(ctx, wig.Cursor{Line: lineIdx, Char: 0})
	}

	gitBuf.KeyHandler = wig.DefaultKeyHandler(wig.ModeKeyMap{
		wig.MODE_NORMAL: wig.KeyMap{
			"l": func(ctx wig.Context) {
				cur := wig.ContextCursorGet(ctx)
				lineMap := getGitStatusLineMap(gitBuf)
				if lineMap == nil {
					return
				}
				for i := cur.Line + 1; i < gitBuf.Lines.Len; i++ {
					if entry, ok := lineMap[i]; ok && entry.kind == "header" {
						// Found next header, now find first selectable item inside it
						for j := i + 1; j < gitBuf.Lines.Len; j++ {
							if nextEntry, ok := lineMap[j]; ok {
								if nextEntry.kind == "file" || nextEntry.kind == "branch" || nextEntry.kind == "stash" {
									cur.Line = j
									cur.Char = 0
									wig.CmdCursorCenter(ctx)
									return
								}
								// If we hit a blank or another header, the section is empty
								if nextEntry.kind == "blank" || nextEntry.kind == "header" {
									break
								}
							}
						}
						// If no selectable item found in this section, just land on the header
						cur.Line = i
						cur.Char = 0
						wig.CmdCursorCenter(ctx)
						return
					}
				}
			},
			"L": func(ctx wig.Context) {
				cur := wig.ContextCursorGet(ctx)
				lineMap := getGitStatusLineMap(gitBuf)
				if lineMap == nil {
					return
				}

				// 1. Skip the current session's header to find the PREVIOUS session.
				foundCurrentHeader := false
				for i := cur.Line - 1; i >= 0; i-- {
					if entry, ok := lineMap[i]; ok && entry.kind == "header" {
						if !foundCurrentHeader {
							// This is the header of the section we are currently in. Skip it.
							foundCurrentHeader = true
							continue
						}

						// 2. Found the previous session's header at line i.
						// Now find the first selectable item inside it.
						for j := i + 1; j < gitBuf.Lines.Len; j++ {
							if nextEntry, ok := lineMap[j]; ok {
								if nextEntry.kind == "file" || nextEntry.kind == "branch" || nextEntry.kind == "stash" {
									cur.Line = j
									cur.Char = 0
									wig.CmdCursorCenter(ctx)
									return
								}
								// If we hit a blank or another header, the section is empty
								if nextEntry.kind == "blank" || nextEntry.kind == "header" {
									break
								}
							}
						}
						// If no selectable item found in this previous section, land on its header
						cur.Line = i
						cur.Char = 0
						wig.CmdCursorCenter(ctx)
						return
					}
				}
			},
			"Enter": func(ctx wig.Context) {
				cur := wig.ContextCursorGet(ctx)
				if cur == nil {
					return
				}
				lineMap := getGitStatusLineMap(gitBuf)
				if lineMap == nil {
					return
				}
				entry, ok := lineMap[cur.Line]
				if !ok {
					return
				}

				switch entry.kind {
				case "file":
					pendingStash = nil
					filePath := entry.filePath
					if !filepath.IsAbs(filePath) {
						rootDir := ctx.Editor.Projects.GetRoot()
						filePath = filepath.Join(rootDir, filePath)
					}
					buf, err := ctx.Editor.OpenFile(filePath)
					if err != nil {
						ctx.Editor.EchoMessage("Cannot open: " + err.Error())
						return
					}
					ctx.Buf = buf
					wig.VisitAtLine(ctx, gitBuf, wig.VisitOptions{})
				case "branch":
					pendingStash = nil
					err := GitSwitchBranch(entry.item)
					branchName := entry.item.StashRef
					if branchName == "" {
						branchName = entry.item.FilePath
					}
					if err != nil {
						ctx.Editor.EchoMessage(err.Error())
					} else {
						for _, b := range ctx.Editor.Buffers {
							if b.FilePath != "" && !strings.HasPrefix(b.FilePath, "[") {
								_ = wig.BufferReloadFile(b)
								if b.Highlighter != nil {
									b.Highlighter.Build()
								}
							}
						}
						refresh(ctx, "")
						ctx.Editor.EchoMessage(fmt.Sprintf("Switched to branch '%s'", branchName))
					}
				case "stash":
					stashItem := entry.item
					prompt := fmt.Sprintf("Stash %s: Pop stash? (y: pop / n: drop / c: cancel)", stashItem.StashRef)
					ui.ConfirmInit(ctx, prompt, func() {
						GitStashAction(stashItem, "pop")
						refresh(ctx, "")
						ctx.Editor.EchoMessage("Stash popped: " + stashItem.StashRef)
					}, func() {
						GitStashAction(stashItem, "drop")
						refresh(ctx, "")
						ctx.Editor.EchoMessage("Stash dropped: " + stashItem.StashRef)
					}, func() {
						ctx.Editor.EchoMessage("Stash cancelled")
					})
				}
			},
			"p": func(ctx wig.Context) {
				curBranchOut, _ := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
				curBranch := strings.TrimSpace(string(curBranchOut))
				if curBranch == "" {
					curBranch = "HEAD"
				}
				prompt := fmt.Sprintf("Push '%s' to origin? (y/n/c)", curBranch)
				ui.ConfirmInit(ctx, prompt, func() {
					ctx.Editor.EchoMessage(fmt.Sprintf("pushing '%s' to origin...", curBranch))
					ctx.Editor.Redraw()
					cmd := exec.Command("git", "push", "origin", "HEAD")
					out, err := cmd.CombinedOutput()
					if err != nil {
						outStr := strings.TrimSpace(string(out))
						if outStr == "" {
							outStr = err.Error()
						}
						outStr = strings.ReplaceAll(outStr, "\n", " ")
						ctx.Editor.EchoMessage("Push failed: " + outStr)
					} else {
						ctx.Editor.EchoMessage("Push complete: origin/" + curBranch)
					}
				}, func() {
					ctx.Editor.EchoMessage("Push cancelled")
				}, func() {
					ctx.Editor.EchoMessage("Push cancelled")
				})
			},
			"S": func(ctx wig.Context) {
				if pendingStash != nil {
					stashRef := pendingStash.StashRef
					GitStashAction(*pendingStash, "pop")
					pendingStash = nil
					refresh(ctx, "")
					ctx.Editor.EchoMessage("Stash popped: " + stashRef)
					return
				}
				wig.CmdYankPut(ctx)
			},
			"s": func(ctx wig.Context) {
				pendingStash = nil
				cur := wig.ContextCursorGet(ctx)
				if cur == nil {
					return
				}
				lineMap := getGitStatusLineMap(gitBuf)
				if lineMap == nil {
					return
				}
				entry, ok := lineMap[cur.Line]
				if !ok || entry.kind != "file" {
					return
				}
				oldPath := entry.filePath
				GitStageItem(entry.item)
				refresh(ctx, oldPath)
				ctx.Editor.EchoMessage("Toggled stage for: " + oldPath)
			},
			"d": func(ctx wig.Context) {
				if pendingStash != nil {
					stashRef := pendingStash.StashRef
					GitStashAction(*pendingStash, "drop")
					pendingStash = nil
					refresh(ctx, "")
					ctx.Editor.EchoMessage("Stash dropped: " + stashRef)
					return
				}

				cur := wig.ContextCursorGet(ctx)
				if cur == nil {
					return
				}
				lineMap := getGitStatusLineMap(gitBuf)
				if lineMap == nil {
					return
				}
				entry, ok := lineMap[cur.Line]
				if !ok {
					return
				}

				if entry.kind == "stash" {
					diffOut := gitRun("stash", "show", "-p", entry.item.StashRef)
					if strings.TrimSpace(diffOut) == "" {
						ctx.Editor.EchoMessage("No diff for " + entry.item.StashRef)
						return
					}
					diffBufName := fmt.Sprintf("[diff: %s]", entry.item.StashRef)
					dBuf := ctx.Editor.BufferFindByFilePath(diffBufName, true)
					dBuf.ResetLines()
					for _, l := range strings.Split(diffOut, "\n") {
						dBuf.Append(l)
					}
					dBuf.Highlighter = &DiffHighlighter{Buf: dBuf}

					dBuf.KeyHandler = wig.DefaultKeyHandler(wig.ModeKeyMap{
						wig.MODE_NORMAL: wig.KeyMap{
							"d":   wig.CmdKillBuffer,
							"q":   wig.CmdKillBuffer,
							"Esc": wig.CmdKillBuffer,
						},
					})

					ctx.Buf = dBuf
					ctx.Editor.ActiveWindow().VisitBuffer(ctx, wig.Cursor{Line: 0, Char: 0})
					return
				}

				if entry.kind != "file" {
					return
				}

				diffLines := GetGitDiffLines(entry.item)
				if len(diffLines) == 0 {
					ctx.Editor.EchoMessage("No diff")
					return
				}

				diffBufName := fmt.Sprintf("[diff: %s]", entry.filePath)
				dBuf := ctx.Editor.BufferFindByFilePath(diffBufName, true)
				dBuf.ResetLines()
				for _, l := range diffLines {
					dBuf.Append(l)
				}
				dBuf.Highlighter = &DiffHighlighter{Buf: dBuf}

				dBuf.KeyHandler = wig.DefaultKeyHandler(wig.ModeKeyMap{
					wig.MODE_NORMAL: wig.KeyMap{
						"d":   wig.CmdKillBuffer,
						"q":   wig.CmdKillBuffer,
						"Esc": wig.CmdKillBuffer,
					},
				})

				ctx.Buf = dBuf
				ctx.Editor.ActiveWindow().VisitBuffer(ctx, wig.Cursor{Line: 0, Char: 0})
			},
			"z": func(ctx wig.Context) {
				pendingStash = nil
				GitStashUnstaged()
				refresh(ctx, "")
				ctx.Editor.EchoMessage("Stashed unstaged changes")
			},
			"r": func(ctx wig.Context) {
				pendingStash = nil
				refresh(ctx, "")
				ctx.Editor.EchoMessage("Git status refreshed")
			},
			"a": func(ctx wig.Context) {
				GitStageAll()
				refresh(ctx, "")
				GitShowCommitBuffer(ctx, true)
			},
			"c": func(ctx wig.Context) {
				GitStageAll()
				refresh(ctx, "")
				GitShowCommitBuffer(ctx)
			},
			"q": func(ctx wig.Context) {
				if cancelActiveGitAi() {
					return
				}
				pendingStash = nil
				if len(ctx.Editor.Windows) > 1 {
					wig.CmdWindowCloseAndKillBuffer(ctx)
				} else {
					wig.CmdKillBuffer(ctx)
				}
			},
			"Esc": func(ctx wig.Context) {
				if cancelActiveGitAi() {
					return
				}
				if pendingStash != nil {
					pendingStash = nil
					ctx.Editor.EchoMessage("Cancelled")
					return
				}
				if len(ctx.Editor.Windows) > 1 {
					wig.CmdWindowCloseAndKillBuffer(ctx)
				} else {
					wig.CmdKillBuffer(ctx)
				}
			},
		},
	})
}

func exitModeOrClose(ctx wig.Context) {
	if cancelActiveGitAi() {
		return
	}

	if ctx.Buf.Mode() != wig.MODE_NORMAL {
		wig.CmdNormalMode(ctx)
		return
	}

	wig.CmdKillBuffer(ctx)
}

// openGitCommitEditor opens (or refreshes) the "[git: edit commit message]"
// buffer with aiMsg pre-filled (or blank if empty) and wires up its keys.
func openGitCommitEditor(ctx wig.Context, aiMsg string) {
	contents := gitRun("status", "-v")
	diffBufName := "[git: edit commit message]"
	dBuf := ctx.Editor.BufferFindByFilePath(diffBufName, true)

	writeTemplate := func(bodyLines []string) {
		dBuf.ResetLines()
		if len(bodyLines) == 0 {
			dBuf.Append("")
			dBuf.Append("")
		} else {
			for _, l := range bodyLines {
				dBuf.Append(l)
			}
		}
		dBuf.Append("")
		dBuf.Append("# Please enter your commit message and press ctrl+c to commit, 'a' to regenerate AI msg, or Esc to exit.")
		dBuf.Append("")
		for _, l := range strings.Split(contents, "\n") {
			dBuf.Append(fmt.Sprintf("# %s", l))
		}
		dBuf.Highlighter = &DiffHighlighter{Buf: dBuf}
	}

	if aiMsg != "" {
		writeTemplate(strings.Split(aiMsg, "\n"))
	} else {
		writeTemplate(nil)
	}

	dBuf.KeyHandler = wig.DefaultKeyHandler(wig.ModeKeyMap{
		wig.MODE_NORMAL: wig.KeyMap{
			"Esc":    exitModeOrClose,
			"q":      exitModeOrClose,
			"ctrl+c": gitCommitFinish,
			"a": func(c wig.Context) {
				runGitAiWithSpinner(c, func(msg string) {
					openGitCommitEditor(c, msg)
				})
			},
		},
		wig.MODE_INSERT: wig.KeyMap{
			"Esc":    exitModeOrClose,
			"ctrl+c": gitCommitFinish,
		},
	})

	ctx.Buf = dBuf
	wig.CmdEnterInsertMode(ctx)
	ctx.Editor.ActiveWindow().VisitBuffer(ctx, wig.Cursor{Line: 0, Char: 0})
}

// GitShowCommitBuffer opens the commit message editor. If useAI (or
// GitAiCommit) is set, it animates a spinner in the status bar echo area while
// running git-ai --tool in the background; the commit editor opens once generation finishes.
func GitShowCommitBuffer(ctx wig.Context, useAI ...bool) {
	withAI := GitAiCommit
	if len(useAI) > 0 {
		withAI = useAI[0]
	}

	if !withAI {
		openGitCommitEditor(ctx, "")
		return
	}

	runGitAiWithSpinner(ctx, func(msg string) {
		openGitCommitEditor(ctx, msg)
	})
}

var gitSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// startSpinner animates a spinner with elapsed seconds in the echo area
// until the given done channel is closed. It does not block the caller;
// run it with `go startSpinner(...)` from a goroutine that is not the
// input-handling goroutine, so keypresses (like Esc to cancel) keep working
// while it runs.
func startSpinner(editor *wig.Editor, label string, done <-chan struct{}) {
	start := time.Now()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	frame := 0
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			elapsed := time.Since(start).Seconds()
			editor.EchoMessage(fmt.Sprintf("%s %s (%.1fs)", gitSpinnerFrames[frame%len(gitSpinnerFrames)], label, elapsed))
			editor.Redraw()
			frame++
		}
	}
}

// runWithSpinner blocks the calling goroutine, animating a spinner until the
// given done channel is closed. Used for work that intentionally isn't
// cancellable mid-flight (e.g. the final `git commit` call).
func runWithSpinner(editor *wig.Editor, label string, done <-chan struct{}) {
	go startSpinner(editor, label, done)
	<-done
}

func gitCommitFinish(ctx wig.Context) {
	cBuf := ctx.Buf
	if cBuf == nil || cBuf.FilePath != "[git: edit commit message]" {
		cBuf = ctx.Editor.BufferFindByFilePath("[git: edit commit message]", false)
	}
	if cBuf == nil {
		ctx.Editor.EchoMessage("No commit buffer found")
		return
	}

	cBuf.FilePath = "/tmp/commit_msg.txt"
	if err := cBuf.Save(); err != nil {
		ctx.Editor.EchoMessage("Failed to save commit message: " + err.Error())
		return
	}

	rootDir := ctx.Editor.Projects.GetRoot()

	var outStr string
	var cmdErr error
	done := make(chan struct{})

	go func() {
		cmd := exec.Command("git", "commit", "-F", "/tmp/commit_msg.txt", "--cleanup=strip")
		if rootDir != "" {
			cmd.Dir = rootDir
		}
		out, err := cmd.CombinedOutput()
		outStr = strings.TrimSpace(string(out))
		cmdErr = err
		close(done)
	}()

	runWithSpinner(ctx.Editor, "Committing...", done)

	if cmdErr != nil {
		if outStr == "" {
			outStr = cmdErr.Error()
		}
		outStr = strings.ReplaceAll(outStr, "\n", " ")
		ctx.Editor.EchoMessage("Commit failed: " + outStr)
		return
	}

	wig.CmdKillBuffer(ctx)

	gitBuf := ctx.Editor.BufferFindByFilePath("[git]", false)
	if gitBuf != nil {
		populateGitStatusBuffer(gitBuf)
	}
	msg := FormatCommitSummary(outStr)
	if msg == "" {
		msg = "commit done"
	}
	wig.EditorInst.EchoMessage(msg)
}

// GitStageItem stages or unstages a file.
func GitStageItem(item wig.GitViewItem) {
	if item.Type != "file" {
		return
	}
	if item.Status == "unstaged" || item.Status == "untracked" {
		gitRun("add", item.FilePath)
	} else if item.Status == "staged" {
		gitRun("restore", "--staged", item.FilePath)
	}
}

// GitStashUnstaged stashes unstaged changes, keeping staged changes and untracked files.
func GitStashUnstaged() {
	gitRun("stash", "push", "--keep-index", "-m", "Stashed unstaged changes")
}

// GitStageAll stages all unstaged changes for already-tracked files (modified/deleted),
// equivalent to `git add -u`. Untracked files are left alone.
func GitStageAll() {
	gitRun("add", "-u")
}

// GitStashAction drops or pops a stash.
func GitStashAction(item wig.GitViewItem, action string) {
	if item.Type != "stash" {
		return
	}
	gitRun("stash", action, item.StashRef)
}
