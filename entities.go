package main

import (
	"database/sql"
	"database/sql/driver"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/nbd-wtf/go-nostr"
	"gorm.io/gorm"
)

// Note godoc
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
	Urls     pq.StringArray    `json:"urls"`
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
	ID             uint           `gorm:"primaryKey" json:"-" db:"id"`
	UID            uuid.UUID      `gorm:"type:uuid;" db:"uid"`
	EventId        string         `gorm:"type:text;not null; unique; index;" json:"event_id" db:"event_id"`
	Pubkey         string         `gorm:"type:varchar(100);index,type:btree;not null;" json:"pubkey" db:"pubkey"`
	Kind           int            `gorm:"type:int;not null;index;" json:"kind" db:"kind"`
	EventCreatedAt int64          `gorm:"type:bigint; not null" json:"event_created_at" db:"event_created_at"`
	Content        string         `gorm:"type:text;" json:"content" db:"content"`
	TagsFull       string         `gorm:"type:text;" json:"tags" db:"tags_full"`
	Ptags          pq.StringArray `gorm:"type:text[];index:idx_notes_ptags,type:gin" json:"-" db:"ptags"`
	Etags          pq.StringArray `gorm:"type:text[];index:idx_notes_etags,type:gin" json:"-" db:"etags"`
	Sig            string         `gorm:"type:varchar(200);not null;" json:"sig" db:"sig"`
	Garbage        bool           `gorm:"type:bool;not null;default:false" json:"-" db:"garbage"`
	Raw            []byte         `gorm:"type:jsonb;not null;" json:"-" db:"raw"`
	CreatedAt      sql.NullTime   `gorm:"type:TIMESTAMPTZ;default:current_timestamp" json:"-" db:"created_at"`
	UpdatedAt      sql.NullTime   `gorm:"type:TIMESTAMPTZ;default:null" json:"-" db:"updated_at"`
	Root           bool           `gorm:"type:bool;not null;default:false;index;comment:Is this the root note" json:"-" db:"root"`
	Urls           pq.StringArray `gorm:"type:text[];index:idx_notes_urls,type:gin" json:"urls" db:"urls"`
	ProfileID      *uint          `gorm:"type:bigint;default null;" json:"profile_id,omitempty" db:"profile_id"`
}

func (entity *Note) BeforeCreate(tx *gorm.DB) error {
	newUUID, err := uuid.NewV7()
	tx.Statement.SetColumn("uid", newUUID)
	return err
}

func (entity *Note) BeforeUpdate(tx *gorm.DB) error {
	entity.UpdatedAt.Time = time.Now()
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
	ID          uint           `gorm:"primaryKey" json:"-" db:"id"`
	UID         uuid.UUID      `gorm:"type:uuid;" db:"uid"`
	Pubkey      string         `gorm:"index,type:btree;not null;unique;type:varchar(100)" json:"pubkey" db:"pubkey"`
	Name        NullString     `gorm:"type:varchar(255)" json:"name" db:"name"`
	About       NullString     `gorm:"type:text" json:"about" db:"about"`
	Picture     NullString     `gorm:"type:varchar(255)" json:"picture" db:"picture"`
	Website     NullString     `gorm:"type:varchar(255)" json:"website" db:"website"`
	Nip05       NullString     `gorm:"type:varchar(255)" json:"nip05" db:"nip05"`
	Lud16       NullString     `gorm:"type:varchar(255)" json:"lud16" db:"lud16"`
	DisplayName NullString     `gorm:"type:varchar(255)" json:"display_name" db:"display_name"`
	Raw         []byte         `gorm:"not null;type:jsonb" json:"-" db:"raw"`
	Urls        pq.StringArray `gorm:"type:text[];index:idx_profile_urls,type:gin" json:"urls" db:"urls"`
	CreatedAt   sql.NullTime   `gorm:"type:timestamp;default:current_timestamp" json:"-" db:"created_at"`
	UpdatedAt   sql.NullTime   `gorm:"type:timestamp;default:null" json:"-" db:"updated_at"`
	Followed    bool           `gorm:"type:bool;default:false;not null" json:"followed" db:"-"`
	Blocked     bool           `gorm:"type:bool;default:false;not null" json:"blocked" db:"-"`
	//Notes       []Note         `gorm:"foreignKey:ProfileID;references:ID"`
}

func (profile *Profile) BeforeCreate(tx *gorm.DB) (err error) {
	newUUID, err := uuid.NewV7()
	profile.UID = newUUID
	//tx.Statement.SetColumn("uid", newUUID)
	return
}

func (profile *Profile) BeforeUpdate(tx *gorm.DB) (err error) {
	profile.UpdatedAt.Time = time.Now()
	//tx.Statement.SetColumn("updated_at", time.Now())
	return
}

type Refs struct {
	Event   map[string]*nostr.Event `json:"event"`
	Profile map[string]*Profile     `json:"profile"`
}

type Block struct {
	ID        uint      `gorm:"primaryKey" json:"-"`
	Pubkey    string    `gorm:"index,type:btree;not null;unique;type:varchar(100)" json:"pubkey"`
	CreatedAt time.Time `gorm:"type:timestamp;default:current_timestamp" json:"-"`
	UpdatedAt time.Time `gorm:"type:timestamp;default:null" json:"-"`
}

func (entity *Block) BeforeUpdate(tx *gorm.DB) error {
	entity.UpdatedAt = time.Now()
	return nil
}

type Follow struct {
	ID        uint      `gorm:"primaryKey" json:"-"`
	Pubkey    string    `gorm:"index,type:btree;unique;type:varchar(100)"  json:"pubkey"`
	CreatedAt time.Time `gorm:"type:timestamp;default:current_timestamp" json:"-"`
	UpdatedAt time.Time `gorm:"type:timestamp;default:null" json:"-"`
}

func (entity *Follow) BeforeUpdate(tx *gorm.DB) error {
	entity.UpdatedAt = time.Now()
	return nil
}

type Seen struct {
	ID        uint      `gorm:"primaryKey" json:"-"`
	EventId   string    `gorm:"type:varchar(100);index,type:btree;not null;unique;" json:"event_id"`
	NoteID    uint      `gorm:"type:bigint;index,type:btree;not null;unique;" json:"-"`
	CreatedAt time.Time `gorm:"type:timestamp;default:current_timestamp" json:"-"`
	UpdatedAt time.Time `gorm:"type:timestamp;default:null" json:"-"`
}

func (entity *Seen) BeforeUpdate(tx *gorm.DB) error {
	entity.UpdatedAt = time.Now()
	return nil
}

type Tree struct {
	ID           uint      `gorm:"primaryKey" json:"-"`
	EventId      string    `gorm:"type:varchar(100);index,type:btree;not null;unique;" json:"event_id"`
	RootEventId  string    `gorm:"type:varchar(100);index,type:btree;not null;" json:"root_event_id"`
	ReplyEventId string    `gorm:"type:varchar(100);index,type:btree;not null;" json:"reply_event_id"`
	CreatedAt    time.Time `gorm:"type:timestamp;default:current_timestamp" json:"-"`
	UpdatedAt    time.Time `gorm:"type:timestamp;default:null" json:"-"`
}

func (entity *Tree) BeforeUpdate(tx *gorm.DB) error {
	entity.UpdatedAt = time.Now()
	return nil
}

type Bookmark struct {
	ID        uint      `gorm:"primaryKey" json:"-"`
	EventId   string    `gorm:"index;not null;unique;type:varchar(100)" json:"event_id"`
	NoteID    *uint     `gorm:"type:bigint;index,type:btree;not null;unique;" json:"-"`
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
	ID            uint      `gorm:"primaryKey" json:"-"`
	Pubkey        string    `gorm:"index:idx_vote_tables_pubkey_target;unique;not null" json:"pubkey"`
	Content       string    `gorm:"not null"  json:"content"`
	CurrentVote   Vote      `gorm:"type:vote;not null"  json:"vote"`
	TargetEventId string    `gorm:"index:idx_vote_tables_pubkey_target;unique;not null" json:"target_event_id"`
	FromEventId   string    `gorm:"index;not null"  json:"from_event_id"`
	NoteID        uint      `gorm:"type:bigint;index,type:btree;not null;" json:"-"`
	CreatedAt     time.Time `gorm:"default:current_timestamp" json:"-"`
	UpdatedAt     time.Time `gorm:"default:null" json:"-"`
}

func (entity *Reaction) BeforeUpdate(tx *gorm.DB) error {
	entity.UpdatedAt = time.Now()
	return nil
}
