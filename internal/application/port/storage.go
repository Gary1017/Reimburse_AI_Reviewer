package port

import "context"

// FileStorage defines file storage operations
type FileStorage interface {
	Save(ctx context.Context, path string, content []byte) error
	Read(ctx context.Context, path string) ([]byte, error)
	Exists(ctx context.Context, path string) bool
	Delete(ctx context.Context, path string) error
	GetFullPath(relativePath string) string
}

// FolderManager defines folder management operations
type FolderManager interface {
	CreateFolder(ctx context.Context, name string) (string, error)
	GetPath(name string) string
	Exists(name string) bool
	Delete(ctx context.Context, name string) error
	SanitizeName(name string) string
}
