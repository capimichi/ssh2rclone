package helper

import (
	"os"
)

// MockFileHelper is an in-memory implementation of FileHelper for unit tests.
type MockFileHelper struct {
	Files   map[string][]byte
	Backups []string
	FailOn  map[string]error
}

// NewMockFileHelper creates a new MockFileHelper.
func NewMockFileHelper() *MockFileHelper {
	return &MockFileHelper{
		Files:  make(map[string][]byte),
		FailOn: make(map[string]error),
	}
}

func (m *MockFileHelper) ReadFile(path string) ([]byte, error) {
	if err, ok := m.FailOn["ReadFile:"+path]; ok {
		return nil, err
	}
	content, ok := m.Files[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return content, nil
}

func (m *MockFileHelper) AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	if err, ok := m.FailOn["AtomicWriteFile:"+path]; ok {
		return err
	}
	m.Files[path] = data
	return nil
}

func (m *MockFileHelper) CreateBackup(srcPath string) (string, error) {
	if err, ok := m.FailOn["CreateBackup:"+srcPath]; ok {
		return "", err
	}
	if !m.FileExists(srcPath) {
		return "", nil
	}
	bakPath := srcPath + ".bak"
	m.Files[bakPath] = m.Files[srcPath]
	m.Backups = append(m.Backups, bakPath)
	return bakPath, nil
}

func (m *MockFileHelper) FileExists(path string) bool {
	_, ok := m.Files[path]
	return ok
}
