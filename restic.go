package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
)

type Snapshot struct {
	ID       string    `json:"id"`
	ShortID  string    `json:"short_id"`
	Time     time.Time `json:"time"`
	Hostname string    `json:"hostname"`
	Paths    []string  `json:"paths"`
	Summary  struct {
		DataAdded uint64 `json:"data_added"`
	} `json:"summary"`
}

type TreeNode struct {
	Name     string `json:"name"`
	Type     string `json:"type"` // "file" or "dir"
	Path     string `json:"path"`
	Size     uint64 `json:"size"`
	MTime    string `json:"mtime"`
}

func resticSnapshots() ([]Snapshot, error) {
	out, err := exec.Command("restic", "snapshots", "--json").Output()
	if err != nil {
		return nil, wrapErr(err)
	}
	var snaps []Snapshot
	if err := json.Unmarshal(out, &snaps); err != nil {
		return nil, err
	}
	sort.Slice(snaps, func(i, j int) bool { return snaps[i].Time.After(snaps[j].Time) })
	return snaps, nil
}

// resticLs lists direct children of path within a snapshot (non-recursive).
func resticLs(snapshotID, path string) ([]TreeNode, error) {
	args := []string{"ls", snapshotID, "--json"}
	if path != "" {
		args = append(args, path)
	}
	out, err := exec.Command("restic", args...).Output()
	if err != nil {
		return nil, wrapErr(err)
	}

	var nodes []TreeNode
	seen := map[string]bool{}
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var raw struct {
			MessageType string `json:"message_type"`
			Path        string `json:"path"`
			Type        string `json:"type"`
			Size        uint64 `json:"size"`
			MTime       string `json:"mtime"`
		}
		if err := dec.Decode(&raw); err != nil {
			break
		}
		if raw.MessageType != "node" || raw.Path == "" {
			continue
		}
		rel := strings.TrimPrefix(raw.Path, path)
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" || strings.Contains(rel, "/") {
			// not a direct child (restic ls is recursive by default for the given path's subtree)
			continue
		}
		if seen[raw.Path] {
			continue
		}
		seen[raw.Path] = true
		nodes = append(nodes, TreeNode{
			Name:  rel,
			Type:  raw.Type,
			Path:  raw.Path,
			Size:  raw.Size,
			MTime: raw.MTime,
		})
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Type != nodes[j].Type {
			return nodes[i].Type == "dir"
		}
		return nodes[i].Name < nodes[j].Name
	})
	return nodes, nil
}

func resticRestore(snapshotID, includePath, target string) (string, error) {
	args := []string{"restore", snapshotID, "--target", target, "--include", includePath}
	out, err := exec.Command("restic", args...).CombinedOutput()
	if err != nil {
		return string(out), wrapErr(err)
	}
	return string(out), nil
}

func resticForget(snapshotID string, prune bool) (string, error) {
	args := []string{"forget", snapshotID}
	if prune {
		args = append(args, "--prune")
	}
	out, err := exec.Command("restic", args...).CombinedOutput()
	if err != nil {
		return string(out), wrapErr(err)
	}
	return string(out), nil
}

func resticFind(pattern string) (string, error) {
	out, err := exec.Command("restic", "find", pattern).CombinedOutput()
	if err != nil {
		return string(out), wrapErr(err)
	}
	return string(out), nil
}

func wrapErr(err error) error {
	if ee, ok := err.(*exec.ExitError); ok {
		return fmt.Errorf("restic error: %s", strings.TrimSpace(string(ee.Stderr)))
	}
	return err
}
