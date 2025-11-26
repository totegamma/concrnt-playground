package main

import (
	"github.com/lib/pq"
	"time"
)

const (
	SchemaDelete string = "https://schema.concrnt.net/delete.json"
)

type Policy struct {
	URL      string  `json:"url"`
	Params   *string `json:"params,omitempty"`
	Defaults *string `json:"defaults,omitempty"`
}

type Document[T any] struct {
	// CIP-1
	Key   *string `json:"key,omitempty"`
	Value T       `json:"value"`

	Author string `json:"author"`

	ContentType *string `json:"contentType,omitempty"`
	Schema      *string `json:"schema,omitempty"`

	SignedAt time.Time `json:"signedAt"`

	// CIP-5
	MemberOf *[]string `json:"memberOf,omitempty"`

	// CIP-6
	Owner          *string `json:"owner,omitempty"`
	Associate      *string `json:"associate,omitempty"`
	AssociationKey *string `json:"associationKey,omitempty"`

	// CIP-8
	Policies *[]Policy `json:"policies,omitempty"`
}

type SchemaDeleteType string

type Proof struct {
	Type      string `json:"type"`
	Signature string `json:"signature"`
}

type SignedDocument struct {
	Document string `json:"document"`
	Proof    Proof  `json:"proof"`
}

// --- db models ---

type CommitOwner struct {
	CommitLogID string    `json:"commit_log_id" gorm:"type:text;primaryKey"`
	CommitLog   CommitLog `json:"-" gorm:"constraint:OnDelete:CASCADE;"`
	Owner       string    `json:"owner" gorm:"type:text;index;primaryKey"`
	CDate       time.Time `json:"cdate" gorm:"->;<-:create;type:timestamp with time zone;not null;default:clock_timestamp()"`
}

type CommitLog struct {
	ID       string    `json:"id" gorm:"primaryKey;type:text"`
	Document string    `json:"document" gorm:"type:text"`
	Proof    string    `json:"proof" gorm:"type:text"`
	CDate    time.Time `json:"cdate" gorm:"type:timestamp with time zone;not null;default:clock_timestamp()"`
}

type RecordKey struct {
	Owner    string `json:"owner" gorm:"primaryKey;type:text"`
	Key      string `json:"key" gorm:"primaryKey;type:text"`
	RecordID string `json:"recordID" gorm:"type:text"`
	Record   Record `json:"record" gorm:"foreignKey:RecordID;references:DocumentID;constraint:OnDelete:CASCADE;"`
}

type Record struct {
	DocumentID  string         `json:"id" gorm:"primaryKey;type:text"`
	Document    CommitLog      `json:"-" gorm:"foreignKey:DocumentID;references:ID;constraint:OnDelete:CASCADE;"`
	ContentType string         `json:"contentType" gorm:"type:text"`
	Author      string         `json:"author" gorm:"type:text"`
	Schema      string         `json:"schema" gorm:"type:text"`
	Value       string         `json:"value" gorm:"type:jsonb"`
	Reference   string         `json:"reference" gorm:"type:text"`
	Referenced  pq.StringArray `json:"referenced" gorm:"type:text[]"`
	CDate       time.Time      `json:"cdate" gorm:"->;<-:create;type:timestamp with time zone;not null;default:clock_timestamp()"`
}

type CollectionMember struct {
	CollectionID string `json:"collectionID" gorm:"primaryKey;type:text"`
	Collection   Record `json:"-" gorm:"foreignKey:CollectionID;references:DocumentID;constraint:OnDelete:CASCADE;"`
	ItemID       string `json:"itemID" gorm:"primaryKey;type:text"`
	Item         Record `json:"-" gorm:"foreignKey:ItemID;references:DocumentID;constraint:OnDelete:CASCADE;"`
}

type Association struct {
	TargetID string `json:"targetID" gorm:"primaryKey;type:text"`
	Target   Record `json:"-" gorm:"foreignKey:TargetID;references:DocumentID;constraint:OnDelete:CASCADE;"`
	ItemID   string `json:"itemID" gorm:"primaryKey;type:text"`
	Item     Record `json:"-" gorm:"foreignKey:ItemID;references:DocumentID;constraint:OnDelete:CASCADE;"`
	Owner    string `json:"owner" gorm:"primaryKey;type:text"`
}

// ----------------
