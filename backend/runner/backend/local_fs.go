package backend

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cloudwego/eino/adk/filesystem"
)

// LocalFSBackend implements filesystem.Backend using local filesystem
type LocalFSBackend struct{}

// NewLocalFSBackend creates a new LocalFSBackend
func NewLocalFSBackend() *LocalFSBackend {
	return &LocalFSBackend{}
}

// LsInfo lists file information under the given path
func (b *LocalFSBackend) LsInfo(ctx context.Context, req *filesystem.LsInfoRequest) ([]filesystem.FileInfo, error) {
	path := req.Path
	if path == "" {
		path = "."
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var result []filesystem.FileInfo
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		result = append(result, filesystem.FileInfo{
			Path:       entry.Name(),
			IsDir:      entry.IsDir(),
			Size:       info.Size(),
			ModifiedAt: info.ModTime().Format("2006-01-02T15:04:05Z"),
		})
	}
	return result, nil
}

// Read reads file content with support for line-based offset and limit
func (b *LocalFSBackend) Read(ctx context.Context, req *filesystem.ReadRequest) (*filesystem.FileContent, error) {
	data, err := os.ReadFile(req.FilePath)
	if err != nil {
		return nil, err
	}

	content := string(data)

	// Handle line-based offset and limit
	if req.Offset > 0 || req.Limit > 0 {
		lines := strings.Split(content, "\n")

		// offset is 1-based, convert to 0-based
		start := req.Offset - 1
		if start < 0 {
			start = 0
		}
		if start > len(lines) {
			return &filesystem.FileContent{Content: ""}, nil
		}

		end := start + req.Limit
		if req.Limit <= 0 {
			end = len(lines)
		}
		if end > len(lines) {
			end = len(lines)
		}

		content = strings.Join(lines[start:end], "\n")
		if req.Limit > 0 && end < len(lines) {
			content += "\n"
		}
	}

	return &filesystem.FileContent{Content: content}, nil
}

// GrepRaw searches for content matching the specified pattern
func (b *LocalFSBackend) GrepRaw(ctx context.Context, req *filesystem.GrepRequest) ([]filesystem.GrepMatch, error) {
	var matches []filesystem.GrepMatch

	pattern := req.Pattern
	if req.CaseInsensitive {
		pattern = "(?i)" + pattern
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	searchPath := req.Path
	if searchPath == "" {
		searchPath = "."
	}

	err = filepath.Walk(searchPath, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			return nil
		}

		// Check glob filter
		if req.Glob != "" {
			matched, err := filepath.Match(req.Glob, filepath.Base(filePath))
			if err != nil || !matched {
				return nil
			}
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil
		}

		lines := strings.Split(string(data), "\n")
		for lineNum, line := range lines {
			if re.MatchString(line) {
				matches = append(matches, filesystem.GrepMatch{
					Content: line,
					Path:    filePath,
					Line:    lineNum + 1,
				})
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return matches, nil
}

// GlobInfo returns file information matching the glob pattern
func (b *LocalFSBackend) GlobInfo(ctx context.Context, req *filesystem.GlobInfoRequest) ([]filesystem.FileInfo, error) {
	basePath := req.Path
	if basePath == "" {
		basePath = "."
	}

	matches, err := filepath.Glob(filepath.Join(basePath, req.Pattern))
	if err != nil {
		return nil, err
	}

	var result []filesystem.FileInfo
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}
		result = append(result, filesystem.FileInfo{
			Path:       match,
			IsDir:      info.IsDir(),
			Size:       info.Size(),
			ModifiedAt: info.ModTime().Format("2006-01-02T15:04:05Z"),
		})
	}

	return result, nil
}

// Write creates or updates file content
func (b *LocalFSBackend) Write(ctx context.Context, req *filesystem.WriteRequest) error {
	dir := filepath.Dir(req.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(req.FilePath, []byte(req.Content), 0644)
}

// Edit replaces string occurrences in a file
func (b *LocalFSBackend) Edit(ctx context.Context, req *filesystem.EditRequest) error {
	data, err := os.ReadFile(req.FilePath)
	if err != nil {
		return err
	}

	content := string(data)
	if !strings.Contains(content, req.OldString) {
		return err
	}

	var newContent string
	if req.ReplaceAll {
		newContent = strings.ReplaceAll(content, req.OldString, req.NewString)
	} else {
		newContent = strings.Replace(content, req.OldString, req.NewString, 1)
	}

	return os.WriteFile(req.FilePath, []byte(newContent), 0644)
}