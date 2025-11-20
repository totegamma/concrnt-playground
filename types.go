package main

import (
	"time"
	"github.com/lib/pq"
)

type DocumentType string

const (
	DocumentTypeCreate DocumentType = "create"
	DocumentTypeDelete DocumentType = "delete"
)

type Document struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value"`

	Reference string `json:"reference,omitempty"`

	Signer string `json:"signer"`
	KeyID  string `json:"keyID,omitempty"`

	Owner string `json:"owner,omitempty"`

	Type   DocumentType `json:"type"`
	Schema string       `json:"schema,omitempty"`

	Policy         string `json:"policy,omitempty"`
	PolicyParams   string `json:"policyParams,omitempty"`
	PolicyDefaults string `json:"policyDefaults,omitempty"`

	SignedAt time.Time `json:"signedAt"`
}

type Commit struct {
	Document  string `json:"document"`
	Signature string `json:"signature"`
	Option    string `json:"option"`
}

// --- db models ---

type CommitOwner struct {
	CommitLogID string    `json:"commit_log_id" gorm:"type:text;primaryKey"`
	CommitLog   CommitLog `json:"-" gorm:"constraint:OnDelete:CASCADE;"`
	Owner       string    `json:"owner" gorm:"type:text;index;primaryKey"`
	CDate       time.Time `json:"cdate" gorm:"->;<-:create;type:timestamp with time zone;not null;default:clock_timestamp()"`
}

type CommitLog struct {
	ID        string    `json:"id" gorm:"primaryKey;type:text"`
	Document  string    `json:"document" gorm:"type:json"`
	Signature string    `json:"signature" gorm:"type:char(130)"`
	CDate     time.Time `json:"cdate" gorm:"type:timestamp with time zone;not null;default:clock_timestamp()"`
}

type RecordKey struct {
	Owner    string `json:"owner" gorm:"primaryKey;type:text"`
	Key      string `json:"key" gorm:"primaryKey;type:text"`
	RecordID string `json:"recordID" gorm:"type:text"`
	Record   Record `json:"record" gorm:"foreignKey:RecordID;references:DocumentID;constraint:OnDelete:CASCADE;"`
}

type Record struct {
	DocumentID string         `json:"id" gorm:"primaryKey;type:text"`
	Document   CommitLog      `json:"-" gorm:"foreignKey:DocumentID;references:ID;constraint:OnDelete:CASCADE;"`
	Owner      string         `json:"owner" gorm:"type:text"`
	Signer     string         `json:"signer" gorm:"type:text"`
	Schema     string         `json:"schema" gorm:"type:text"`
	Value      string         `json:"value" gorm:"type:text"`
	Reference  string         `json:"reference" gorm:"type:text"`
	Referenced pq.StringArray `json:"referenced" gorm:"type:text[]"`
	CDate      time.Time      `json:"cdate" gorm:"->;<-:create;type:timestamp with time zone;not null;default:clock_timestamp()"`
}

// ----------------

