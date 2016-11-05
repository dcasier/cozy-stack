package vfs

import (
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/dcasier/cozy-stack/couchdb"
	"github.com/dcasier/cozy-stack/couchdb/mango"
	"github.com/dcasier/cozy-stack/web/jsonapi"
)

// DirDoc is a struct containing all the informations about a
// directory. It implements the couchdb.Doc and jsonapi.Object
// interfaces.
type DirDoc struct {
	// Type of document. Useful to (de)serialize and filter the data
	// from couch.
	Type string `json:"type"`
	// Qualified file identifier
	ObjID string `json:"_id,omitempty"`
	// Directory revision
	ObjRev string `json:"_rev,omitempty"`
	// Directory name
	Name string `json:"name"`
	// Parent folder identifier
	FolderID string `json:"folder_id"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Directory path on VFS
	Fullpath string   `json:"path"`
	Tags     []string `json:"tags"`

	parent *DirDoc
	files  []*FileDoc
	dirs   []*DirDoc
}

// ID returns the directory qualified identifier - see couchdb.Doc interface
func (d *DirDoc) ID() string {
	return d.ObjID
}

// Rev returns the directory revision - see couchdb.Doc interface
func (d *DirDoc) Rev() string {
	return d.ObjRev
}

// DocType returns the directory document type - see couchdb.Doc
// interface
func (d *DirDoc) DocType() string {
	return FsDocType
}

// SetID is used to change the directory qualified identifier - see
// couchdb.Doc interface
func (d *DirDoc) SetID(id string) {
	d.ObjID = id
}

// SetRev is used to change the directory revision - see couchdb.Doc
// interface
func (d *DirDoc) SetRev(rev string) {
	d.ObjRev = rev
}

// Path is used to generate the file path
func (d *DirDoc) Path(c *Context) (string, error) {
	if d.Fullpath == "" {
		parent, err := d.Parent(c)
		if err != nil {
			return "", err
		}
		parentPath, err := parent.Path(c)
		if err != nil {
			return "", err
		}
		d.Fullpath = path.Join(parentPath, d.Name)
	}
	return d.Fullpath, nil
}

// Parent returns the parent directory document
func (d *DirDoc) Parent(c *Context) (*DirDoc, error) {
	parent, err := getParentDir(c, d.parent, d.FolderID)
	if err != nil {
		return nil, err
	}
	d.parent = parent
	return parent, nil
}

// SelfLink is used to generate a JSON-API link for the directory (part of
// jsonapi.Object interface)
func (d *DirDoc) SelfLink() string {
	return "/files/" + d.ObjID
}

// Relationships is used to generate the content relationship in JSON-API format
// (part of the jsonapi.Object interface)
//
// TODO: pagination
func (d *DirDoc) Relationships() jsonapi.RelationshipMap {
	l := len(d.files) + len(d.dirs)
	i := 0

	data := make([]jsonapi.ResourceIdentifier, l)
	for _, child := range d.dirs {
		data[i] = jsonapi.ResourceIdentifier{ID: child.ID(), Type: child.DocType()}
		i++
	}

	for _, child := range d.files {
		data[i] = jsonapi.ResourceIdentifier{ID: child.ID(), Type: child.DocType()}
		i++
	}

	contents := jsonapi.Relationship{Data: data}

	var parent jsonapi.Relationship
	if d.ID() != RootFolderID {
		parent = jsonapi.Relationship{
			Links: &jsonapi.LinksList{
				Related: "/files/" + d.FolderID,
			},
			Data: jsonapi.ResourceIdentifier{
				ID:   d.FolderID,
				Type: FsDocType,
			},
		}
	}

	return jsonapi.RelationshipMap{
		"parent":   parent,
		"contents": contents,
	}
}

// Included is part of the jsonapi.Object interface
func (d *DirDoc) Included() []jsonapi.Object {
	var included []jsonapi.Object
	for _, child := range d.dirs {
		included = append(included, child)
	}
	for _, child := range d.files {
		included = append(included, child)
	}
	return included
}

// FetchFiles is used to fetch direct children of the directory.
//
// @TODO: add pagination control
func (d *DirDoc) FetchFiles(c *Context) (err error) {
	d.files, d.dirs, err = fetchChildren(c, d)
	return err
}

// NewDirDoc is the DirDoc constructor. The given name is validated.
func NewDirDoc(name, folderID string, tags []string, parent *DirDoc) (doc *DirDoc, err error) {
	if err = checkFileName(name); err != nil {
		return
	}

	if folderID == "" {
		folderID = RootFolderID
	}

	tags = uniqueTags(tags)

	createDate := time.Now()
	doc = &DirDoc{
		Type:     DirType,
		Name:     name,
		FolderID: folderID,

		CreatedAt: createDate,
		UpdatedAt: createDate,
		Tags:      tags,

		parent: parent,
	}

	return
}

// GetDirDoc is used to fetch directory document information
// form the database.
func GetDirDoc(c *Context, fileID string, withChildren bool) (*DirDoc, error) {
	doc := &DirDoc{}
	err := couchdb.GetDoc(c.db, FsDocType, fileID, doc)
	if couchdb.IsNotFoundError(err) {
		err = ErrParentDoesNotExist
	}
	if err != nil {
		return nil, err
	}
	if doc.Type != DirType {
		return nil, os.ErrNotExist
	}
	if withChildren {
		err = doc.FetchFiles(c)
	}
	return doc, err
}

// GetDirDocFromPath is used to fetch directory document information from
// the database from its path.
func GetDirDocFromPath(c *Context, name string, withChildren bool) (*DirDoc, error) {
	var doc *DirDoc
	var err error

	var docs []*DirDoc
	sel := mango.Equal("path", path.Clean(name))
	req := &couchdb.FindRequest{Selector: sel, Limit: 1}
	err = couchdb.FindDocs(c.db, FsDocType, req, &docs)
	if err != nil {
		return nil, err
	}
	if len(docs) == 0 {
		return nil, os.ErrNotExist
	}
	doc = docs[0]

	if withChildren {
		err = doc.FetchFiles(c)
	}
	return doc, err
}

// CreateDirectory is the method for creating a new directory
func CreateDirectory(c *Context, doc *DirDoc) (err error) {
	name, err := doc.Path(c)
	if err != nil {
		return err
	}

	err = c.fs.Mkdir(name, 0755)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			c.fs.Remove(name)
		}
	}()

	return couchdb.CreateDoc(c.db, doc)
}

// CreateRootDirectory creates the root folder for this context
func CreateRootDirectory(c *Context) (err error) {
	root := &DirDoc{
		Type:     DirType,
		ObjID:    RootFolderID,
		Fullpath: "/",
	}
	err = c.fs.MkdirAll(root.Fullpath, 0755)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			c.fs.Remove(root.Fullpath)
		}
	}()

	return couchdb.CreateNamedDocWithDB(c.db, root)
}

// ModifyDirMetadata modify the metadata associated to a directory. It
// can be used to rename or move the directory in the VFS.
func ModifyDirMetadata(c *Context, olddoc *DirDoc, patch *DocPatch) (newdoc *DirDoc, err error) {
	cdate := olddoc.CreatedAt
	patch, err = normalizeDocPatch(&DocPatch{
		Name:      &olddoc.Name,
		FolderID:  &olddoc.FolderID,
		Tags:      &olddoc.Tags,
		UpdatedAt: &olddoc.UpdatedAt,
	}, patch, cdate)

	if err != nil {
		return
	}

	newdoc, err = NewDirDoc(*patch.Name, *patch.FolderID, *patch.Tags, nil)
	if err != nil {
		return
	}

	var parent *DirDoc
	if newdoc.FolderID != olddoc.FolderID {
		parent, err = newdoc.Parent(c)
	} else {
		parent = olddoc.parent
	}

	if err != nil {
		return
	}

	newdoc.SetID(olddoc.ID())
	newdoc.SetRev(olddoc.Rev())
	newdoc.CreatedAt = cdate
	newdoc.UpdatedAt = *patch.UpdatedAt
	newdoc.parent = parent
	newdoc.files = olddoc.files
	newdoc.dirs = olddoc.dirs

	oldpath, err := olddoc.Path(c)
	if err != nil {
		return
	}
	newpath, err := newdoc.Path(c)
	if err != nil {
		return
	}

	if oldpath != newpath {
		err = safeRenameDirectory(c, oldpath, newpath)
		if err != nil {
			return
		}
		err = bulkUpdateDocsPath(c, oldpath, newpath)
		if err != nil {
			return
		}
	}

	err = couchdb.UpdateDoc(c.db, newdoc)
	return
}

// @TODO remove this method and use couchdb bulk updates instead
func bulkUpdateDocsPath(c *Context, oldpath, newpath string) error {
	var children []*DirDoc
	sel := mango.StartWith("path", oldpath+"/")
	req := &couchdb.FindRequest{Selector: sel}
	err := couchdb.FindDocs(c.db, FsDocType, req, &children)
	if err != nil || len(children) == 0 {
		return err
	}

	errc := make(chan error)

	for _, child := range children {
		go func(child *DirDoc) {
			if !strings.HasPrefix(child.Fullpath, oldpath+"/") {
				errc <- fmt.Errorf("Child has wrong base directory")
			} else {
				child.Fullpath = path.Join(newpath, child.Fullpath[len(oldpath)+1:])
				errc <- couchdb.UpdateDoc(c.db, child)
			}
		}(child)
	}

	for range children {
		if e := <-errc; e != nil {
			err = e
		}
	}

	return err
}

func fetchChildren(c *Context, parent *DirDoc) (files []*FileDoc, dirs []*DirDoc, err error) {
	var docs []*dirOrFile
	sel := mango.Equal("folder_id", parent.ID())
	req := &couchdb.FindRequest{Selector: sel, Limit: 10}
	err = couchdb.FindDocs(c.db, FsDocType, req, &docs)
	if err != nil {
		return
	}

	for _, doc := range docs {
		typ, dir, file := doc.refine()
		switch typ {
		case FileType:
			file.parent = parent
			files = append(files, file)
		case DirType:
			dir.parent = parent
			dirs = append(dirs, dir)
		}
	}

	return
}

func safeRenameDirectory(c *Context, oldpath, newpath string) error {
	newpath = path.Clean(newpath)
	oldpath = path.Clean(oldpath)

	if !path.IsAbs(newpath) || !path.IsAbs(oldpath) {
		return fmt.Errorf("paths should be absolute")
	}

	if strings.HasPrefix(newpath, oldpath+"/") {
		return ErrForbiddenDocMove
	}

	_, err := c.fs.Stat(newpath)
	if err == nil {
		return os.ErrExist
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	return c.fs.Rename(oldpath, newpath)
}

var (
	_ couchdb.Doc    = &DirDoc{}
	_ jsonapi.Object = &DirDoc{}
)
