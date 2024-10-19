package main

import (
	"database/sql/driver"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/nbd-wtf/go-nostr"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Event struct {
	Event    *nostr.Event      `json:"event"`
	Profile  Profile           `json:"profile"`
	Etags    []string          `json:"-"`
	Ptags    []string          `json:"-"`
	Garbage  bool              `json:"gargabe"`
	Children map[string]*Event `json:"children"`
	Tree     int64             `json:"tree"`
	RootId   string            `json:"-"`
	Bookmark bool              `json:"bookmark"`
	Content  string            `json:"content"`
	Refs     Refs              `json:"refs"`
	Urls     []string          `json:"-"`
}

type Relay struct {
	ID        uint      `json:"-"`
	Url       string    `gorm:"not null; unique; index,type:btree;type:varchar(255)" json:"url"`
	Read      bool      `gorm:"default: false;" json:"read"`
	Write     bool      `gorm:"default: false;" json:"write"`
	Search    bool      `gorm:"default: false;" json:"search"`
	CreatedAt time.Time `gorm:"default:current_timestamp" json:"-"`
	UpdatedAt time.Time `gorm:"default:null" json:"-"`
}

func (entity *Relay) BeforeUpdate(tx *gorm.DB) error {
	entity.UpdatedAt = time.Now()
	return nil
}

type Note struct {
	ID             uint           `gorm:"primaryKey" json:"-"`
	UID            uuid.UUID      `gorm:"type:char(36);"`
	EventId        string         `gorm:"not null; unique; index;type:text" json:"event_id"`
	Pubkey         string         `gorm:"index,type:btree;not null;type:varchar(100)" json:"pubkey"`
	Kind           int            `gorm:"not null;index;" json:"kind"`
	EventCreatedAt int64          `gorm:"not null" json:"event_created_at"`
	Content        string         `json:"content"`
	TagsFull       string         `json:"tags"`
	Ptags          pq.StringArray `gorm:"type:text[];index:idx_notes_ptags,type:gin" json:"-"`
	Etags          pq.StringArray `gorm:"type:text[];index:idx_notes_etags,type:gin" json:"-"`
	Sig            string         `gorm:"not null;type:varchar(200)" json:"sig"`
	Garbage        bool           `gorm:"default:false" json:"-"`
	Raw            datatypes.JSON `json:"-"`
	Reaction       []Reaction     `gorm:"default:null" json:"reactions"`
	CreatedAt      time.Time      `gorm:"default:current_timestamp" json:"-"`
	UpdatedAt      time.Time      `gorm:"default:null" json:"-"`
	Root           bool           `gorm:"default:false;index;comment:Is this the root note" json:"-"`
	Urls           pq.StringArray `gorm:"type:text[];index:idx_notes_urls,type:gin" json:"-"`
}

func (entity *Note) BeforeCreate(tx *gorm.DB) error {
	newUUID, err := uuid.NewV7()
	tx.Statement.SetColumn("uid", newUUID)
	return err
}

func (entity *Note) BeforeUpdate(tx *gorm.DB) error {
	entity.UpdatedAt = time.Now()
	tx.Statement.SetColumn("updated_at", time.Now())
	return nil
}

type Notification struct {
	ID        uint `gorm:"primaryKey" json:"id"`
	NoteID    uint `gorm:"not null"  json:"note_id"`
	Note      Note
	Seen      bool      `gorm:"default:false" json:"seen"`
	CreatedAt time.Time `gorm:"default:current_timestamp" json:"-"`
	UpdatedAt time.Time `gorm:"default:null" json:"-"`
}

func (entity *Notification) BeforeUpdate(tx *gorm.DB) error {
	entity.UpdatedAt = time.Now()
	return nil
}

type Profile struct {
	ID          uint           `json:"-"`
	UID         uuid.UUID      `gorm:"type:char(36);"`
	Pubkey      string         `gorm:"index,type:btree;not null;unique;type:varchar(100)" json:"pubkey"`
	Name        string         `gorm:"type:varchar(255)" json:"name"`
	About       string         `gorm:"type:text" json:"about"`
	Picture     string         `gorm:"type:varchar(255)" json:"picture"`
	Website     string         `gorm:"type:varchar(255)" json:"website"`
	Nip05       string         `gorm:"type:varchar(255)" json:"nip05"`
	Lud16       string         `gorm:"type:varchar(255)" json:"lud16"`
	DisplayName string         `gorm:"type:varchar(255)" json:"display_name"`
	Raw         datatypes.JSON `json:"-"`
	Urls        pq.StringArray `gorm:"type:text[];index:idx_profile_urls,type:gin" json:"-"`
	CreatedAt   time.Time      `gorm:"default:current_timestamp" json:"-"`
	UpdatedAt   time.Time      `gorm:"default:null" json:"-"`
	Followed    bool           `gorm:"type:bool;default:false" json:"followed"`
	Blocked     bool           `gorm:"type:bool;default:false" json:"blocked"`
}

func (profile *Profile) BeforeCreate(tx *gorm.DB) (err error) {
	newUUID, err := uuid.NewV7()
	profile.UID = newUUID
	//tx.Statement.SetColumn("uid", newUUID)
	return
}

func (profile *Profile) BeforeUpdate(tx *gorm.DB) (err error) {
	profile.UpdatedAt = time.Now()
	//tx.Statement.SetColumn("updated_at", time.Now())
	return
}

type Refs struct {
	Event   map[string]*nostr.Event `json:"event"`
	Profile map[string]*Profile     `json:"profile"`
}

type Block struct {
	ID        uint      `json:"-"`
	Pubkey    string    `gorm:"index,type:btree;not null;unique;type:varchar(100)" json:"pubkey"`
	CreatedAt time.Time `gorm:"default:current_timestamp" json:"-"`
	UpdatedAt time.Time `gorm:"default:null" json:"-"`
}

func (entity *Block) BeforeUpdate(tx *gorm.DB) error {
	entity.UpdatedAt = time.Now()
	return nil
}

type Follow struct {
	ID        uint      `json:"-"`
	Pubkey    string    `gorm:"index,type:btree;unique;type:varchar(100)"  json:"pubkey"`
	CreatedAt time.Time `gorm:"default:current_timestamp" json:"-"`
	UpdatedAt time.Time `gorm:"default:null" json:"-"`
}

func (entity *Follow) BeforeUpdate(tx *gorm.DB) error {
	entity.UpdatedAt = time.Now()
	return nil
}

type Seen struct {
	ID        uint   `json:"-"`
	EventId   string `gorm:"index,type:btree;not null;unique;type:varchar(100)" json:"event_id"`
	NoteID    uint
	CreatedAt time.Time `gorm:"default:current_timestamp" json:"-"`
	UpdatedAt time.Time `gorm:"default:null" json:"-"`
}

func (entity *Seen) BeforeUpdate(tx *gorm.DB) error {
	entity.UpdatedAt = time.Now()
	return nil
}

type Tree struct {
	ID           uint      `json:"-"`
	EventId      string    `gorm:"index,type:btree;not null;unique;type:varchar(100)" json:"event_id"`
	RootEventId  string    `gorm:"index,type:btree;not null;type:varchar(100)" json:"root_event_id"`
	ReplyEventId string    `gorm:"index,type:btree;not null;type:varchar(100)" json:"reply_event_id"`
	CreatedAt    time.Time `gorm:"default:current_timestamp" json:"-"`
	UpdatedAt    time.Time `gorm:"default:null" json:"-"`
}

func (entity *Tree) BeforeUpdate(tx *gorm.DB) error {
	entity.UpdatedAt = time.Now()
	return nil
}

type Bookmark struct {
	ID        uint   `json:"-"`
	EventId   string `gorm:"index;not null;unique;type:varchar(100)" json:"event_id"`
	NoteID    uint
	CreatedAt time.Time `gorm:"default:current_timestamp" json:"-"`
	UpdatedAt time.Time `gorm:"default:null" json:"-"`
}

func (entity *Bookmark) BeforeUpdate(tx *gorm.DB) error {
	entity.UpdatedAt = time.Now()
	return nil
}

type Vote string

const (
	Like    Vote = "like"
	Dislike Vote = "dislike"
)

func (p *Vote) Scan(value interface{}) error {
	*p = Vote(value.([]byte))
	return nil
}

func (p Vote) Value() (driver.Value, error) {
	return string(p), nil
}

type Reaction struct {
	ID            uint      `json:"-"`
	Pubkey        string    `gorm:"index:idx_vote_tables_pubkey_target;unique;not null" json:"pubkey"`
	Content       string    `gorm:"not null"  json:"content"`
	CurrentVote   Vote      `gorm:"type:vote;not null"  json:"vote"`
	TargetEventId string    `gorm:"index:idx_vote_tables_pubkey_target;unique;not null" json:"target_event_id"`
	FromEventId   string    `gorm:"index;not null"  json:"from_event_id"`
	NoteID        uint      `gorm:"default:null" json:"-"`
	CreatedAt     time.Time `gorm:"default:current_timestamp" json:"-"`
	UpdatedAt     time.Time `gorm:"default:null" json:"-"`
}

func (entity *Reaction) BeforeUpdate(tx *gorm.DB) error {
	entity.UpdatedAt = time.Now()
	return nil
}
