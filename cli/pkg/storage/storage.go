package storage

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var maxWorkers = 128

type Scheme string

const (
	SchemeDisk Scheme = "file"
	SchemeS3   Scheme = "s3"
	SchemeGCS  Scheme = "gs"
)

type ListResult struct {
	Path  string
	Error error
}

type Storage interface {
	RootURL() string
	RootExists() (bool, error)
	Get(path string) ([]byte, error)
	Put(path string, data []byte) error
	PutDirectory(localPath string, storagePath string) error
	GetDirectory(storagePath string, localPath string) error
	Delete(path string) error

	// List files in a path non-recursively
	//
	// Returns a list of paths, prefixed with the given path, that can be passed straight to Get().
	// Directories are not listed.
	// If path does not exist, an empty list will be returned.
	List(path string) ([]string, error)

	// List files in a path recursively
	ListRecursive(results chan<- ListResult, folder string)

	MatchFilenamesRecursive(results chan<- ListResult, folder string, filename string)

	// PrepareRunEnv prepares a replicate run, returning a list of
	// environment variables
	PrepareRunEnv() ([]string, error)
}

// SplitURL splits a storage URL into <scheme>://<path>
func SplitURL(storageURL string) (scheme Scheme, bucket string, root string, err error) {
	u, err := url.Parse(storageURL)
	if err != nil {
		return "", "", "", err
	}
	switch u.Scheme {
	case "":
		return SchemeDisk, "", u.Path, nil
	case "file":
		return SchemeDisk, "", u.Host + u.Path, nil
	case "s3":
		return SchemeS3, u.Host, strings.TrimPrefix(u.Path, "/"), nil
	case "gs":
		return SchemeGCS, u.Host, strings.TrimPrefix(u.Path, "/"), nil
	}
	return "", "", "", fmt.Errorf("Unknown storage backend: %s", u.Scheme)
}

func ForURL(storageURL string) (Storage, error) {
	scheme, bucket, root, err := SplitURL(storageURL)
	if err != nil {
		return nil, err
	}
	switch scheme {
	case SchemeDisk:
		return NewDiskStorage(root)
	case SchemeS3:
		return NewS3Storage(bucket, root)
	case SchemeGCS:
		return NewGCSStorage(bucket, root)
	}

	return nil, fmt.Errorf("Unknown storage backend: %s", scheme)
}

var putDirectorySkip = []string{".replicate", ".git", "venv", ".mypy_cache"}

type fileToPut struct {
	Source string
	Dest   string
}

func putDirectoryFiles(localPath string, storagePath string) ([]fileToPut, error) {
	result := []fileToPut{}
	err := filepath.Walk(localPath, func(currentPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			for _, dir := range putDirectorySkip {
				if info.Name() == dir {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Strip local path
		relativePath, err := filepath.Rel(localPath, currentPath)
		if err != nil {
			return err
		}

		result = append(result, fileToPut{
			Source: currentPath,
			Dest:   path.Join(storagePath, relativePath),
		})
		return nil
	})
	return result, err
}

// NeedsCaching returns true if the storage is slow and needs caching
func NeedsCaching(storage Storage) bool {
	_, isDiskStorage := storage.(*DiskStorage)
	return !isDiskStorage
}
