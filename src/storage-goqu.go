package main

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/doug-martin/goqu/v9"

	// import the dialect
	_ "github.com/doug-martin/goqu/v9/dialect/postgres"
	_ "github.com/lib/pq"
)

/**
 * We neede an active database connection object.
 * The filter is used for certain words in de posts we want to filter out, because they can be spam
 */
type GQ struct {
	Db     *goqu.Database
	Config *DbConfig
}

/**
 * Connect to postgresql database
 */
func (st *GQ) Connect(ctx context.Context) {

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable TimeZone=Europe/Amsterdam",
		st.Config.Host,
		st.Config.User,
		st.Config.Password,
		st.Config.Dbname,
		st.Config.Port)

	pgDb, err := sql.Open("postgres", dsn)
	if err != nil {
		panic(err.Error())
	}
	var logger goqu.Logger
	st.Db = goqu.New("postgres", pgDb)
	st.Db.Logger(logger)
	fmt.Println("Connect() -> Connected to database:", st.Config.Dbname)
}

/* As reference
func (st *GQ) prefixFields(refStruct interface{}, tableName string) []string {
	ift := reflect.TypeOf(refStruct)
	name := ift.Name()
	aliases := make([]string, 0)

	for i := 0; i < ift.NumField(); i++ {
		t := ift.Field(i).Tag.Get("db")
		if len(t) > 0 {
			alias := fmt.Sprintf("%s.%s as %s__%s", tableName, t, name, t)
			aliases = append(aliases, alias)
			//fmt.Println("Tag is ", t, " and alias is ", alias)
		}
	}
	return aliases
}

type Prefix struct {
	name  interface{}
	table string
}

func (st *GQ) prefix(args ...Prefix) string {
	a := make([]string, 0)

	for _, arg := range args {
		fmt.Println(arg.table)
		aliases := st.prefixFields(arg.name, arg.table)
		a = append(a, aliases...)
	}

	selectFields := ""
	for _, alias := range a {
		selectFields = selectFields + alias + ","
	}
	selectFields = selectFields[:len(selectFields)-1]

	return selectFields
}
*/

type NotesAndProfiles struct {
	Notes    Note
	Profiles Profile
}

/**
 * Do not show all data in an endless scrol page, but paginate it for easy access
 * and ignore the garbage tagged posts
 *
 */
func (st *GQ) GetPagination(ctx context.Context, p *Pagination, options Options) error {
	var notesAndProfiles []NotesAndProfiles

	/*
		t1 := Prefix{name: Note{}, table: "notes"}
		t2 := Prefix{name: Profile{}, table: "profiles"}
		selectFields := st.prefix(t1, t2)
		fmt.Println(selectFields)
	*/

	dialect := goqu.Dialect("postgres")
	selectDataset := dialect.
		From("notes").
		//Select(goqu.L(selectFields)).
		Select(NotesAndProfiles{}).
		Join(
			goqu.T("profiles"),
			goqu.On(goqu.Ex{"profiles.pubkey": goqu.I("notes.pubkey")}),
		).
		Where(goqu.Ex{"kind": 1}, goqu.Ex{"root": true}, goqu.Ex{"garbage": false}).
		Limit(p.Limit)

	if p.Offset > 0 {
		selectDataset.Offset(uint(p.Offset))
	}
	sql, args, err := selectDataset.ToSQL()

	if err != nil {
		fmt.Println("An error occurred while generating the SQL", err.Error())
	} else {
		fmt.Println(sql, args)
	}
	//st.Db.Query(sql)
	err = st.Db.ScanStructs(&notesAndProfiles, sql)

	if err != nil {
		fmt.Println(err.Error())
	}

	fmt.Println(sql)
	for _, row := range notesAndProfiles {
		fmt.Println(row.Notes.EventId, row.Profiles.Name)
	}

	//dialect := goqu.Dialect("postgres")
	/*



		tx := st.GormDB.Debug().Model(&Note{}).
			Select([]string{"notes.id",
				"notes.event_id", "notes.pubkey",
				"notes.kind", "notes.event_created_at",
				"notes.content", "notes.tags_full::json",
				"notes.sig", "notes.etags",
				"notes.ptags", "notes.urls as nodeUrls",
				"profiles.name", "profiles.about",
				"profiles.picture", "profiles.website",
				"profiles.nip05", "profiles.lud16",
				"profiles.display_name", "profiles.urls as profileUrls",
				"follows.pubkey follow",
				"bookmarks.event_id bookmarked"}).
			//Joins("LEFT JOIN profiles ON (profiles.pubkey = notes.pubkey)").
			Joins("LEFT JOIN profiles ON profiles.pubkey = notes.pubkey").
			Joins("LEFT JOIN blocks ON (blocks.pubkey = notes.pubkey)").
			//Joins("LEFT JOIN seens on (seens.event_id = notes.event_id)").
			Where("notes.kind = 1").
			Where("notes.root = true").
			Where("blocks.pubkey IS NULL").
			//Where("seens.event_id IS NULL").
			Where("notes.garbage = false")

		// Needs a time stamp that stays the same else total will change with every call
		if p.Since > 0 && (!options.BookMark && !options.Follow) {
			var maxTimeStamp int64 = time.Now().Unix()
			if !p.Renew && p.Maxid > 0 {
				st.GormDB.Model(&Note{}).
					Select(`MAX(notes.event_created_at) event_created_at`).
					Where("notes.id <= ?", p.Maxid).Scan(&maxTimeStamp)
			}
			since := maxTimeStamp - int64(p.Since*60*60*24)
			tx.Where("notes.event_created_at > ?", fmt.Sprintf("%d", since))
		}
		if p.Since == 0 && (!options.BookMark && !options.Follow) {
			var maxTimeStamp int64 = time.Now().Unix()
			since := maxTimeStamp - int64(1*60*60*24)
			tx.Where("notes.event_created_at > ?", fmt.Sprintf("%d", since))
		}

		if options.Follow {
			tx.Joins("JOIN follows ON (follows.pubkey = notes.pubkey)")
		} else {
			tx.Joins("LEFT JOIN follows ON (follows.pubkey = notes.pubkey)")
			tx.Where("follows.pubkey is null")
		}

		if options.BookMark {
			tx.Joins("JOIN bookmarks ON (bookmarks.event_id = notes.event_id)")
		} else {
			tx.Joins("LEFT JOIN bookmarks ON (bookmarks.event_id = notes.event_id)")
		}

		if !p.Renew {
			tx.Where("notes.id <= ?", fmt.Sprintf("%d", p.Maxid))
		}

		var count int64
		tx.Count(&count)

		tx.Limit(p.Limit)
		if p.Offset > 0 {
			tx.Offset(int(p.Offset))
		}

		tx.Order("notes.event_created_at DESC")
		fmt.Println("HIER WEL HE")
		var results []PaginationResult
		tx.Scan(&results)
		for _, item := range results {

			fmt.Println(item.Note.EventId) //, item.Name, item.Follow, item.Content, item.TagsFull)
		}
		fmt.Println("AND DONE>.............")
	*/
	return nil
}
