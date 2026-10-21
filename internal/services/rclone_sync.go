package services

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"ssh2rclone/internal/helper"
	"ssh2rclone/internal/model"
)

const (
	BlockBeginTag = "# --- BEGIN ssh2rclone GENERATED BLOCK (DO NOT EDIT MANUALLY) ---"
	BlockEndTag   = "# --- END ssh2rclone GENERATED BLOCK ---"
)

var sectionRegex = regexp.MustCompile(`^\s*\[([^\]]+)\]\s*$`)

// SyncResult summarizes the synchronization operation.
type SyncResult struct {
	AddedRemotes     []string
	SkippedConflicts []string
	BackupPath       string
	RenderedBlock    string
}

// BuildRemotes converts SSHEntry models to RcloneRemote models.
func BuildRemotes(entries []model.SSHEntry) []model.RcloneRemote {
	var remotes []model.RcloneRemote
	for _, e := range entries {
		remote := model.RcloneRemote{
			Name: e.Alias,
			Type: "sftp",
		}
		if e.IsComplex {
			remote.SSHCommand = fmt.Sprintf("ssh %s", e.Alias)
		} else {
			remote.Host = e.HostName
			remote.User = e.User
			remote.Port = e.Port
			remote.KeyFile = e.KeyFile
		}
		remotes = append(remotes, remote)
	}
	return remotes
}

// RenderBlock formats a slice of RcloneRemote into the delimited block string.
func RenderBlock(remotes []model.RcloneRemote) string {
	var sb strings.Builder
	sb.WriteString(BlockBeginTag + "\n")
	for i, r := range remotes {
		sb.WriteString(fmt.Sprintf("[%s]\n", r.Name))
		sb.WriteString(fmt.Sprintf("type = %s\n", r.Type))
		if r.SSHCommand != "" {
			sb.WriteString(fmt.Sprintf("ssh = %s\n", r.SSHCommand))
		} else {
			if r.Host != "" {
				sb.WriteString(fmt.Sprintf("host = %s\n", r.Host))
			}
			if r.User != "" {
				sb.WriteString(fmt.Sprintf("user = %s\n", r.User))
			}
			if r.Port != "" {
				sb.WriteString(fmt.Sprintf("port = %s\n", r.Port))
			}
			if r.KeyFile != "" {
				sb.WriteString(fmt.Sprintf("key_file = %s\n", r.KeyFile))
			}
		}
		if i < len(remotes)-1 {
			sb.WriteString("\n")
		}
	}
	sb.WriteString(BlockEndTag + "\n")
	return sb.String()
}

// ExtractExternalSections parses rclone.conf content, separates prefix and suffix
// around any existing managed block, and collects section names defined outside the block.
func ExtractExternalSections(content string) (map[string]bool, string, string) {
	externalSections := make(map[string]bool)

	beginIdx := strings.Index(content, BlockBeginTag)
	endIdx := strings.Index(content, BlockEndTag)

	var prefix, suffix string
	if beginIdx != -1 && endIdx != -1 && endIdx > beginIdx {
		prefix = content[:beginIdx]
		suffix = content[endIdx+len(BlockEndTag):]
		// Clean up leading newline in suffix if present
		if strings.HasPrefix(suffix, "\n") {
			suffix = suffix[1:]
		}
	} else {
		prefix = content
		suffix = ""
	}

	scanner := bufio.NewScanner(strings.NewReader(prefix + "\n" + suffix))
	for scanner.Scan() {
		line := scanner.Text()
		matches := sectionRegex.FindStringSubmatch(line)
		if len(matches) == 2 {
			externalSections[matches[1]] = true
		}
	}

	return externalSections, prefix, suffix
}

// SyncRcloneConfig performs the synchronization following isolation and conflict policies.
func SyncRcloneConfig(entries []model.SSHEntry, opts model.SyncOptions) (SyncResult, error) {
	var result SyncResult

	var existingContent string
	if data, err := os.ReadFile(opts.RcloneConfigPath); err == nil {
		existingContent = string(data)
	}

	externalSections, prefix, suffix := ExtractExternalSections(existingContent)

	var validEntries []model.SSHEntry
	for _, entry := range entries {
		if externalSections[entry.Alias] {
			result.SkippedConflicts = append(result.SkippedConflicts, entry.Alias)
			if opts.Verbose {
				fmt.Printf("[WARN] Remote '%s' already exists in manual rclone config; skipping.\n", entry.Alias)
			}
			continue
		}
		validEntries = append(validEntries, entry)
		result.AddedRemotes = append(result.AddedRemotes, entry.Alias)
	}

	remotes := BuildRemotes(validEntries)
	result.RenderedBlock = RenderBlock(remotes)

	if opts.DryRun {
		return result, nil
	}

	// Create backup if file exists and backup not disabled
	if !opts.NoBackup && existingContent != "" {
		bakPath, err := helper.CreateBackup(opts.RcloneConfigPath)
		if err != nil {
			return result, fmt.Errorf("failed to create backup: %w", err)
		}
		result.BackupPath = bakPath
	}

	// Assemble final content
	var finalContent strings.Builder
	trimmedPrefix := strings.TrimRight(prefix, "\r\n")
	if trimmedPrefix != "" {
		finalContent.WriteString(trimmedPrefix)
		finalContent.WriteString("\n\n")
	}

	finalContent.WriteString(result.RenderedBlock)

	trimmedSuffix := strings.TrimLeft(suffix, "\r\n")
	if trimmedSuffix != "" {
		finalContent.WriteString("\n")
		finalContent.WriteString(trimmedSuffix)
	}

	if err := helper.AtomicWriteFile(opts.RcloneConfigPath, []byte(finalContent.String()), 0600); err != nil {
		return result, fmt.Errorf("failed writing rclone config: %w", err)
	}

	return result, nil
}
