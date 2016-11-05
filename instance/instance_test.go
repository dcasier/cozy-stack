package instance

import (
	"fmt"
	"os"
	"testing"

	"github.com/dcasier/cozy-stack/couchdb"
	"github.com/dcasier/cozy-stack/couchdb/mango"
	"github.com/dcasier/cozy-stack/vfs"
	"github.com/sourcegraph/checkup"
	"github.com/stretchr/testify/assert"
)

func TestGetInstanceNoDB(t *testing.T) {
	instance, err := Get("no.instance.cozycloud.cc")
	if assert.Error(t, err, "An error is expected") {
		assert.Nil(t, instance)
		assert.Contains(t, err.Error(), "No instance", "the error is not explicit")
		assert.Contains(t, err.Error(), "no.instance.cozycloud.cc", "the error is not explicit")
	}
}

func TestCreateInstance(t *testing.T) {
	instance, err := Create("test.cozycloud.cc", "en", nil)
	if assert.NoError(t, err) {
		assert.NotEmpty(t, instance.ID())
		assert.Equal(t, instance.Domain, "test.cozycloud.cc")
	}
}

func TestGetWrongInstance(t *testing.T) {
	instance, err := Get("no.instance.cozycloud.cc")
	if assert.Error(t, err, "An error is expected") {
		assert.Nil(t, instance)
		assert.Contains(t, err.Error(), "No instance", "the error is not explicit")
		assert.Contains(t, err.Error(), "no.instance.cozycloud.cc", "the error is not explicit")
	}
}

func TestGetCorrectInstance(t *testing.T) {
	instance, err := Get("test.cozycloud.cc")
	if assert.NoError(t, err, "An error is expected") {
		assert.NotNil(t, instance)
		assert.Equal(t, instance.Domain, "test.cozycloud.cc")
	}
}

func TestInstanceHasRootFolder(t *testing.T) {
	var root vfs.DirDoc
	prefix := getDBPrefix(t, "test.cozycloud.cc")
	err := couchdb.GetDoc(prefix, vfs.FsDocType, vfs.RootFolderID, &root)
	if assert.NoError(t, err) {
		assert.Equal(t, root.Fullpath, "/")
	}
}

func TestInstanceHasIndexes(t *testing.T) {
	var results []*vfs.DirDoc
	prefix := getDBPrefix(t, "test.cozycloud.cc")
	req := &couchdb.FindRequest{Selector: mango.Equal("path", "/")}
	err := couchdb.FindDocs(prefix, vfs.FsDocType, req, &results)
	assert.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestMain(m *testing.M) {
	const CouchDBURL = "http://localhost:5984/"
	const TestPrefix = "dev/"

	db, err := checkup.HTTPChecker{URL: CouchDBURL}.Check()
	if err != nil || db.Status() != checkup.Healthy {
		fmt.Println("This test need couchdb to run.")
		os.Exit(1)
	}
	couchdb.DeleteDB(globalDBPrefix, instanceType)
	couchdb.DeleteDB("test.cozycloud.cc/", vfs.FsDocType)
	os.RemoveAll("/usr/local/var/cozy2/")

	os.Exit(m.Run())
}

func getDBPrefix(t *testing.T, domain string) string {
	instance, err := Get(domain)
	if !assert.NoError(t, err, "Should get instance %v", domain) {
		t.FailNow()
	}
	return instance.GetDatabasePrefix()
}
