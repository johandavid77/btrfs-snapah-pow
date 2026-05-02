package btrfs

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Manager struct{}

func NewManager() *Manager {
	return &Manager{}
}

type SnapshotInfo struct {
	ID         uint64    `json:"id"`
	UUID       string    `json:"uuid"`
	ParentUUID string    `json:"parent_uuid"`
	Path       string    `json:"path"`
	CreatedAt  time.Time `json:"created_at"`
	IsReadOnly bool      `json:"is_readonly"`
}

func (m *Manager) CreateSnapshot(source, dest string, readonly bool) error {
	args := []string{"subvolume", "snapshot"}
	if readonly {
		args = append(args, "-r")
	}
	args = append(args, source, dest)

	cmd := exec.Command("btrfs", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("btrfs snapshot failed: %w - %s", err, output)
	}
	return nil
}

func (m *Manager) DeleteSnapshot(path string) error {
	cmd := exec.Command("btrfs", "subvolume", "delete", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("btrfs delete failed: %w - %s", err, output)
	}
	return nil
}

func (m *Manager) ListSnapshots(basePath string) ([]SnapshotInfo, error) {
	cmd := exec.Command("btrfs", "subvolume", "list", "-t", "-u", basePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("btrfs list failed: %w", err)
	}
	return m.parseList(string(output), basePath)
}

func (m *Manager) SetReadOnly(path string, readonly bool) error {
	value := "false"
	if readonly {
		value = "true"
	}
	cmd := exec.Command("btrfs", "property", "set", "-ts", path, "ro", value)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("btrfs property set failed: %w - %s", err, output)
	}
	return nil
}

func (m *Manager) SendSnapshot(snapshotPath, parentPath string) (*bytes.Buffer, error) {
	args := []string{"send"}
	if parentPath != "" {
		args = append(args, "-p", parentPath)
	}
	args = append(args, snapshotPath)

	var buf bytes.Buffer
	cmd := exec.Command("btrfs", args...)
	cmd.Stdout = &buf
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("btrfs send failed: %w - %s", err, stderr.String())
	}
	return &buf, nil
}

func (m *Manager) ReceiveSnapshot(destPath string, data *bytes.Buffer) error {
	cmd := exec.Command("btrfs", "receive", destPath)
	cmd.Stdin = data
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("btrfs receive failed: %w - %s", err, stderr.String())
	}
	return nil
}

func SnapshotPath(baseDir, name string) string {
	ts := time.Now().Format("20060102_150405")
	if name != "" {
		return filepath.Join(baseDir, ".snapshots", fmt.Sprintf("%s_%s", name, ts))
	}
	return filepath.Join(baseDir, ".snapshots", ts)
}

func (m *Manager) parseList(output, basePath string) ([]SnapshotInfo, error) {
	var snapshots []SnapshotInfo
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "ID") || strings.HasPrefix(line, "--") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}

		id, _ := strconv.ParseUint(fields[1], 10, 64)
		pathIdx := strings.Index(line, "path ")
		var path string
		if pathIdx > 0 {
			path = strings.TrimSpace(line[pathIdx+5:])
		}

		snapshots = append(snapshots, SnapshotInfo{
			ID:   id,
			Path: filepath.Join(basePath, path),
		})
	}

	return snapshots, nil
}
