package main

import (
	"database/sql/driver"
	"time"

	"github.com/lib/pq"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

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
}

func (entity *Note) BeforeUpdate(tx *gorm.DB) error {
	entity.UpdatedAt = time.Now()
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
	ID          uint      `json:"-"`
	Pubkey      string    `gorm:"index,type:btree;not null;unique;type:varchar(100)" json:"pubkey"`
	Name        string    `json:"name"`
	About       string    `json:"about"`
	Picture     string    `json:"picture"`
	Website     string    `json:"website"`
	Nip05       string    `json:"nip05"`
	Lud16       string    `json:"lud16"`
	DisplayName string    `json:"display_name"`
	Raw         string    `json:"-"`
	CreatedAt   time.Time `gorm:"default:current_timestamp" json:"-"`
	UpdatedAt   time.Time `gorm:"default:null" json:"-"`
}

func (entity *Profile) BeforeUpdate(tx *gorm.DB) error {
	entity.UpdatedAt = time.Now()
	return nil
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
