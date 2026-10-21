package services

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"ssh2rclone/internal/model"
)

func TestFormatRemoteName(t *testing.T) {
	entry := model.SSHEntry{
		Alias:    "myserver",
		HostName: "server.example.com",
		User:     "deploy",
		Port:     "2200",
	}

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{"default alias", "{alias}", "myserver"},
		{"alternate name", "{name}", "myserver"},
		{"user and alias", "{user}_{alias}", "deploy_myserver"},
		{"username alias", "{username}_{alias}", "deploy_myserver"},
		{"host and port", "{host}:{port}", "server.example.com:2200"},
		{"hostname token", "{hostname}", "server.example.com"},
		{"fallback not triggered", "{user:-root}_{alias}", "deploy_myserver"},
		{"literal chars", "sftp-{alias}", "sftp-myserver"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := FormatRemoteName(entry, tt.template)
			if res != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, res)
			}
		})
	}
}

func TestFormatRemoteName_Fallbacks(t *testing.T) {
	entryWithoutUser := model.SSHEntry{
		Alias:    "srv",
		HostName: "srv.lan",
		User:     "",
	}

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{"fallback triggered", "{user:-root}_{alias}", "root_srv"},
		{"empty without fallback", "{user}_{alias}", "_srv"},
		{"empty template fallback to alias", "", "srv"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := FormatRemoteName(entryWithoutUser, tt.template)
			if res != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, res)
			}
		})
	}
}

func TestSanitizeRemoteName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal-name", "normal-name"},
		{"spaces in name", "spaces-in-name"},
		{"[bad]", "bad"},
		{"brackets [inside]", "brackets-inside"},
		{"tab\tand\nnewline", "tab-and-newline"},
		{"---trim-edges---", "trim-edges"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			res := SanitizeRemoteName(tt.input)
			if res != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, res)
			}
		})
	}
}

func TestBuildRemotes_InternalDuplicateDetection(t *testing.T) {
	entries := []model.SSHEntry{
		{Alias: "srv1", HostName: "10.0.0.1"},
		{Alias: "srv2", HostName: "10.0.0.1"}, // Duplicate when template is {host}
	}

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	remotes := BuildRemotes(entries, "{host}")

	w.Close()
	os.Stderr = oldStderr
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if len(remotes) != 1 {
		t.Fatalf("expected 1 remote due to collision, got %d", len(remotes))
	}
	if remotes[0].Name != "10.0.0.1" {
		t.Fatalf("expected remote name 10.0.0.1, got %s", remotes[0].Name)
	}
	if !strings.Contains(output, "Warning: duplicate remote name") {
		t.Fatalf("expected duplicate warning in stderr, got: %s", output)
	}
}

func TestRemoteNameFormatterInterface(t *testing.T) {
	formatter := NewRemoteNameFormatter()
	entry := model.SSHEntry{
		Alias:    "srv1",
		HostName: "srv1.internal",
		User:     "admin",
	}

	name := formatter.Format(entry, "{user:-root}_{alias}")
	if name != "admin_srv1" {
		t.Fatalf("expected admin_srv1, got %s", name)
	}
}

