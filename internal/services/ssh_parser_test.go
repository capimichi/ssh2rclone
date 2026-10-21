package services_test

import (
	"os"
	"path/filepath"
	"testing"

	"ssh2rclone/internal/model"
	"ssh2rclone/internal/services"
)

func TestParseSSHConfig(t *testing.T) {
	tmpDir := t.TempDir()
	sshConfigFile := filepath.Join(tmpDir, "config")

	content := `
Host simple1
    HostName 10.0.0.1
    User customuser
    Port 22
    IdentityFile ~/.ssh/id_rsa

Host simple2
    HostName 10.0.0.2
    # Inherits User globaluser and Port 2222 from Host *

Host jumper
    HostName 10.0.0.3
    ProxyJump jumpbox.example.com

Host proxied
    HostName 10.0.0.4
    ProxyCommand nc -X 5 -x 127.0.0.1:1080 %h %p

Host multi1 multi2
    HostName 10.0.0.5

Host *
    User globaluser
    Port 2222
    ServerAliveInterval 60
`

	if err := os.WriteFile(sshConfigFile, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write test ssh config: %v", err)
	}

	entries, err := services.ParseSSHConfig(sshConfigFile, model.SyncOptions{AllSSH: false})
	if err != nil {
		t.Fatalf("unexpected error parsing ssh config: %v", err)
	}

	// Expected concrete hosts: simple1, simple2, jumper, proxied, multi1, multi2 (Host * is ignored)
	entryMap := make(map[string]model.SSHEntry)
	for _, e := range entries {
		entryMap[e.Alias] = e
	}

	if _, exists := entryMap["*"]; exists {
		t.Fatalf("wildcard '*' should not be included as a remote entry")
	}

	// Test simple1
	s1, ok := entryMap["simple1"]
	if !ok {
		t.Fatalf("missing simple1")
	}
	if s1.HostName != "10.0.0.1" || s1.User != "customuser" || s1.Port != "22" || s1.IsComplex {
		t.Fatalf("unexpected simple1 attributes: %+v", s1)
	}

	// Test simple2 inheritance
	s2, ok := entryMap["simple2"]
	if !ok {
		t.Fatalf("missing simple2")
	}
	if s2.HostName != "10.0.0.2" || s2.User != "globaluser" || s2.Port != "2222" || s2.IsComplex {
		t.Fatalf("unexpected simple2 attributes: %+v", s2)
	}

	// Test jumper
	jmp, ok := entryMap["jumper"]
	if !ok {
		t.Fatalf("missing jumper")
	}
	if !jmp.IsComplex || jmp.ProxyJump != "jumpbox.example.com" {
		t.Fatalf("jumper should be complex with ProxyJump")
	}

	// Test proxied
	px, ok := entryMap["proxied"]
	if !ok {
		t.Fatalf("missing proxied")
	}
	if !px.IsComplex || px.ProxyCommand == "" {
		t.Fatalf("proxied should be complex with ProxyCommand")
	}

	// Test multi1 & multi2
	if _, ok := entryMap["multi1"]; !ok {
		t.Fatalf("missing multi1")
	}
	if _, ok := entryMap["multi2"]; !ok {
		t.Fatalf("missing multi2")
	}
}

func TestParseSSHConfigAllSSHFlag(t *testing.T) {
	tmpDir := t.TempDir()
	sshConfigFile := filepath.Join(tmpDir, "config")

	content := `
Host simple
    HostName 10.0.0.1
`
	if err := os.WriteFile(sshConfigFile, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write test ssh config: %v", err)
	}

	entries, err := services.ParseSSHConfig(sshConfigFile, model.SyncOptions{AllSSH: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 || !entries[0].IsComplex {
		t.Fatalf("with AllSSH=true, entry should be marked as IsComplex")
	}
}

func TestSSHParserInterface(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config")
	content := `Host web
    HostName 192.168.1.10
    User webmaster
`
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	parser := services.NewSSHParser()
	entries, err := parser.Parse(cfgPath, model.SyncOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 || entries[0].Alias != "web" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
}

