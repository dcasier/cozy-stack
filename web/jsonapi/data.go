package jsonapi

import (
	"encoding/json"

	"github.com/dcasier/cozy-stack/couchdb"
)

// Object is an interface to serialize something to a JSON-API Object
type Object interface {
	couchdb.Doc
	SelfLink() string
	Relationships() RelationshipMap
	Included() []Object
}

// Meta is a container for the couchdb revision, in JSON-API land
type Meta struct {
	Rev string `json:"rev"`
}

// LinksList is the common links used in JSON-API for the top-level or a
// resource object
// See http://jsonapi.org/format/#document-links
type LinksList struct {
	Self    string `json:"self,omitempty"`
	Related string `json:"related,omitempty"`
	Prev    string `json:"prev,omitempty"`
	Next    string `json:"next,omitempty"`
}

// ResourceIdentifier is an object, used in relationships, to identify an
// individual resource
// See http://jsonapi.org/format/#document-resource-identifier-objects
type ResourceIdentifier struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// Relationship is a resource linkage, as described in JSON-API
// See http://jsonapi.org/format/#document-resource-object-relationships
//
// Data can be a single ResourceIdentifier for to-one relationships,
// or an array of them for to-many relationships.
type Relationship struct {
	Links *LinksList  `json:"links,omitempty"`
	Data  interface{} `json:"data"`
}

// ResourceIdentifier returns the resource identifier of the relationship.
func (r *Relationship) ResourceIdentifier() (*ResourceIdentifier, bool) {
	if m, ok := r.Data.(map[string]interface{}); ok {
		idd, _ := m["id"].(string)
		typ, _ := m["type"].(string)
		return &ResourceIdentifier{ID: idd, Type: typ}, true
	}
	return nil, false
}

// RelationshipMap is a map of relationships
// See http://jsonapi.org/format/#document-resource-object-relationships
type RelationshipMap map[string]Relationship

// ObjectMarshalling is a JSON-API object
// See http://jsonapi.org/format/#document-resource-objects
type ObjectMarshalling struct {
	Type          string           `json:"type"`
	ID            string           `json:"id"`
	Attributes    *json.RawMessage `json:"attributes"`
	Meta          Meta             `json:"meta"`
	Links         LinksList        `json:"links,omitempty"`
	Relationships RelationshipMap  `json:"relationships,omitempty"`
}

// GetRelationship returns the relationship with the given name from
// the relationships map.
func (o *ObjectMarshalling) GetRelationship(name string) (*Relationship, bool) {
	rel, ok := o.Relationships[name]
	if !ok {
		return nil, false
	}
	return &rel, true
}

// MarshalObject serializes an Object to JSON.
// It returns a json.RawMessage that can be used a in Document.
func MarshalObject(o Object) (json.RawMessage, error) {
	id := o.ID()
	rev := o.Rev()
	self := o.SelfLink()
	rels := o.Relationships()

	o.SetID("")
	o.SetRev("")
	defer func() {
		o.SetID(id)
		o.SetRev(rev)
	}()

	b, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}

	data := ObjectMarshalling{
		Type:          o.DocType(),
		ID:            id,
		Attributes:    (*json.RawMessage)(&b),
		Meta:          Meta{Rev: rev},
		Links:         LinksList{Self: self},
		Relationships: rels,
	}
	return json.Marshal(data)
}
