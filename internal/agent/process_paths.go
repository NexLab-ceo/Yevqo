package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// safeExistingSubdirectory resolves a fixed relative directory while rejecting
// symlinks in every component. It prevents release metadata from redirecting
// deploy, rollback, or cleanup operations outside the site's own directory.
func safeExistingSubdirectory(root, relative string) (string, error) {
	if filepath.IsAbs(relative) || strings.Contains(relative, "\\") {
		return "", fmt.Errorf("invalid managed directory path")
	}
	clean := filepath.Clean(relative)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("managed directory must stay inside its root")
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve managed root: %w", err)
	}
	current := root
	for _, part := range strings.Split(clean, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return "", err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("managed path component is not a real directory: %s", part)
		}
		resolved, err := filepath.EvalSymlinks(current)
		if err != nil {
			return "", err
		}
		rel, err := filepath.Rel(resolvedRoot, resolved)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("managed directory resolves outside its root")
		}
	}
	return current, nil
}

// ensureSafeSubdirectory creates a fixed relative directory tree one component
// at a time and refuses pre-existing symlinks.
func ensureSafeSubdirectory(root, relative string, mode os.FileMode) (string, error) {
	if filepath.IsAbs(relative) || strings.Contains(relative, "\\") {
		return "", fmt.Errorf("invalid managed directory path")
	}
	clean := filepath.Clean(relative)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("managed directory must stay inside its root")
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve managed root: %w", err)
	}
	current := root
	for _, part := range strings.Split(clean, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			if err := os.Mkdir(current, mode); err != nil && !os.IsExist(err) {
				return "", fmt.Errorf("create managed directory %s: %w", part, err)
			}
			info, err = os.Lstat(current)
		}
		if err != nil {
			return "", err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("managed path component is not a real directory: %s", part)
		}
		resolved, err := filepath.EvalSymlinks(current)
		if err != nil {
			return "", err
		}
		rel, err := filepath.Rel(resolvedRoot, resolved)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("managed directory resolves outside its root")
		}
	}
	return current, nil
}
