package vfs

type Cache interface {
	CreateDir(doc *DirDoc) error
	UpdateDir(doc *DirDoc) error
	DirByID(fileID string) (*DirDoc, error)
	DirByPath(name string) (*DirDoc, error)
	DirFiles(doc *DirDoc) (files []*FileDoc, dirs []*DirDoc, err error)

	CreateFile(doc *FileDoc) error
	UpdateFile(doc *FileDoc) error
	FileByID(fileID string) (*FileDoc, error)
	FileByPath(name string) (*FileDoc, error)

	DirOrFileByID(fileID string) (string, *DirDoc, *FileDoc, error)

	Len() int
}
