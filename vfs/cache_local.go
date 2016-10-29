package vfs

import (
	"fmt"
	"os"
	"path"
	"sync"

	"github.com/cozy/cozy-stack/couchdb"
	"github.com/cozy/cozy-stack/couchdb/mango"
)

type LocalCache struct {
	c *Context

	mud  sync.RWMutex       // mutex for directories data-structures
	lrud *LRUCache          // lru cache for directories
	pthd map[string]*string // path directory to id map

	muf  sync.RWMutex       // mutex for files data-structures
	lruf *LRUCache          // lru cache for files
	pthf map[string]*string // (folderID, name) file pair to id map
}

func NewLocalCache(c *Context, maxEntries int) *LocalCache {
	var cache *LocalCache

	dirEviction := func(key LRUKey, value interface{}) {
		if doc, ok := value.(*DirDoc); ok {
			delete(cache.pthd, doc.Fullpath)
		}
	}

	fileEviction := func(key LRUKey, value interface{}) {
		if doc, ok := value.(*FileDoc); ok {
			delete(cache.pthf, genFilePathID(doc.FolderID, doc.Name))
		}
	}

	cache = new(LocalCache)
	cache.c = c
	cache.pthd = make(map[string]*string)
	cache.pthf = make(map[string]*string)
	cache.lrud = &LRUCache{MaxEntries: maxEntries, OnEvicted: dirEviction}
	cache.lruf = &LRUCache{MaxEntries: maxEntries, OnEvicted: fileEviction}
	return cache
}

func (fc *LocalCache) CreateDir(doc *DirDoc) error {
	var err error
	if err = doc.calcPath(fc.c); err != nil {
		return err
	}
	err = couchdb.CreateDoc(fc.c.db, doc)
	if err != nil {
		return err
	}
	fc.touchDir(doc)
	return nil
}

func (fc *LocalCache) UpdateDir(doc *DirDoc) error {
	var err error
	if err = doc.calcPath(fc.c); err != nil {
		return err
	}
	err = couchdb.UpdateDoc(fc.c.db, doc)
	if err != nil {
		fc.rmDir(doc)
		return err
	}
	fc.touchDir(doc)
	return nil
}

func (fc *LocalCache) DirByID(fileID string) (doc *DirDoc, err error) {
	var ok bool
	if doc, ok = fc.dirCachedByID(fileID); ok {
		return
	}

	doc = &DirDoc{}
	err = couchdb.GetDoc(fc.c.db, FsDocType, fileID, doc)
	if couchdb.IsNotFoundError(err) {
		err = ErrParentDoesNotExist
	} else if err == nil && doc.Type != DirType {
		err = os.ErrNotExist
	}
	if err != nil {
		return
	}

	fc.touchDir(doc)
	return
}

func (fc *LocalCache) DirByPath(name string) (doc *DirDoc, err error) {
	var ok bool
	if doc, ok = fc.dirCachedByPath(name); ok {
		return
	}

	var docs []*DirDoc
	sel := mango.Equal("path", path.Clean(name))
	req := &couchdb.FindRequest{Selector: sel, Limit: 1}
	err = couchdb.FindDocs(fc.c.db, FsDocType, req, &docs)
	if err != nil {
		return nil, err
	}
	if len(docs) == 0 {
		return nil, os.ErrNotExist
	}
	doc = docs[0]

	fc.touchDir(doc)
	return
}

func (fc *LocalCache) DirFiles(doc *DirDoc) (files []*FileDoc, dirs []*DirDoc, err error) {
	var docs []*dirOrFile
	sel := mango.Equal("folder_id", doc.ID())
	req := &couchdb.FindRequest{Selector: sel, Limit: 10}
	err = couchdb.FindDocs(fc.c.db, FsDocType, req, &docs)
	if err != nil {
		return
	}

	for _, doc := range docs {
		typ, dir, file := doc.refine()
		switch typ {
		case FileType:
			fc.touchFile(file)
			files = append(files, file)
		case DirType:
			fc.touchDir(dir)
			dirs = append(dirs, dir)
		}
	}

	return
}

func (fc *LocalCache) CreateFile(doc *FileDoc) error {
	err := couchdb.CreateDoc(fc.c.db, doc)
	if err != nil {
		return err
	}
	fc.touchFile(doc)
	return nil
}

func (fc *LocalCache) UpdateFile(doc *FileDoc) error {
	err := couchdb.UpdateDoc(fc.c.db, doc)
	if err != nil {
		fc.rmFile(doc)
		return err
	}
	fc.touchFile(doc)
	return nil
}

func (fc *LocalCache) FileByID(fileID string) (doc *FileDoc, err error) {
	var ok bool
	if doc, ok = fc.fileCachedByID(fileID); ok {
		return
	}

	doc = &FileDoc{}
	err = couchdb.GetDoc(fc.c.db, FsDocType, fileID, doc)
	if err != nil {
		return nil, err
	}

	if doc.Type != FileType {
		return nil, os.ErrNotExist
	}

	return doc, nil
}

func (fc *LocalCache) FileByPath(name string) (doc *FileDoc, err error) {
	dirpath := path.Dir(name)
	parent, err := fc.DirByPath(dirpath)
	if err != nil {
		return
	}

	folderID, filename := parent.ID(), path.Base(name)

	var ok bool
	if doc, ok = fc.fileCachedByFolderID(folderID, filename); ok {
		return
	}

	selector := mango.And(
		mango.Equal("folder_id", folderID),
		mango.Equal("name", filename),
		mango.Equal("type", FileType),
	)

	var docs []*FileDoc
	req := &couchdb.FindRequest{
		Selector: selector,
		Limit:    1,
	}
	err = couchdb.FindDocs(fc.c.db, FsDocType, req, &docs)
	if err != nil {
		return
	}
	if len(docs) == 0 {
		err = os.ErrNotExist
		return
	}

	doc = docs[0]
	fc.touchFile(doc)
	return
}

func (fc *LocalCache) DirOrFileByID(fileID string) (typ string, dirDoc *DirDoc, fileDoc *FileDoc, err error) {
	var ok bool
	if dirDoc, ok = fc.dirCachedByID(fileID); ok {
		typ = DirType
		return
	}

	if fileDoc, ok = fc.fileCachedByID(fileID); ok {
		typ = FileType
		return
	}

	dirOrFile := &dirOrFile{}
	err = couchdb.GetDoc(fc.c.db, FsDocType, fileID, dirOrFile)
	if err != nil {
		return
	}

	typ, dirDoc, fileDoc = dirOrFile.refine()
	return
}

func (fc *LocalCache) Len() int {
	fc.mud.RLock()
	fc.muf.RLock()
	defer fc.mud.Unlock()
	defer fc.muf.Unlock()
	return fc.lrud.Len() + fc.lrud.Len()
}

func (fc *LocalCache) touchDir(doc *DirDoc) {
	fc.mud.Lock()
	defer fc.mud.Unlock()
	key := LRUKey(doc.ObjID)
	if olddoc, ok := fc.lrud.Get(key); ok {
		delete(fc.pthd, olddoc.(*DirDoc).Fullpath)
	}
	fc.lrud.Add(key, doc)
	fc.pthd[doc.Fullpath] = &doc.ObjID
}

func (fc *LocalCache) touchFile(doc *FileDoc) {
	fc.muf.Lock()
	defer fc.muf.Unlock()
	key := LRUKey(doc.ObjID)
	if olddoc, ok := fc.lruf.Get(key); ok {
		f := olddoc.(*FileDoc)
		delete(fc.pthf, genFilePathID(f.FolderID, f.Name))
	}
	fc.lruf.Add(key, doc)
	fc.pthf[genFilePathID(doc.FolderID, doc.Name)] = &doc.ObjID
}

func (fc *LocalCache) rmDir(doc *DirDoc) {
	fc.mud.Lock()
	defer fc.mud.Unlock()
	fc.lrud.Remove(LRUKey(doc.ObjID))
}

func (fc *LocalCache) rmFile(doc *FileDoc) {
	fc.muf.Lock()
	defer fc.muf.Unlock()
	fc.lruf.Remove(LRUKey(doc.ObjID))
}

func (fc *LocalCache) dirCachedByID(fileID string) (*DirDoc, bool) {
	fc.mud.Lock()
	defer fc.mud.Unlock()
	if v, ok := fc.lrud.Get(LRUKey(fileID)); ok {
		return v.(*DirDoc), true
	} else {
		return nil, false
	}
}

func (fc *LocalCache) dirCachedByPath(name string) (*DirDoc, bool) {
	fc.mud.Lock()
	defer fc.mud.Unlock()
	pid, ok := fc.pthd[name]
	if ok {
		v, _ := fc.lrud.Get(LRUKey(*pid))
		return v.(*DirDoc), true
	} else {
		return nil, false
	}
}

func (fc *LocalCache) fileCachedByID(fileID string) (*FileDoc, bool) {
	fc.muf.Lock()
	defer fc.muf.Unlock()
	if v, ok := fc.lruf.Get(LRUKey(fileID)); ok {
		return v.(*FileDoc), true
	} else {
		return nil, false
	}
}

func (fc *LocalCache) fileCachedByFolderID(folderID, name string) (*FileDoc, bool) {
	fc.muf.Lock()
	defer fc.muf.Unlock()
	pid, ok := fc.pthf[genFilePathID(folderID, name)]
	if ok {
		v, _ := fc.lruf.Get(LRUKey(*pid))
		return v.(*FileDoc), true
	} else {
		return nil, false
	}
}

func genFilePathID(folderID, name string) string {
	return fmt.Sprintf("%s/%s", folderID, name)
}
