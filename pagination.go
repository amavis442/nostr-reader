package main

// Return to client API
type Pagination struct {
	Cursor         uint64 `json:"cursor"`
	NextCursor     uint64 `json:"next_cursor"`
	PreviousCursor uint64 `json:"previous_cursor"`
	StartId        uint64 `json:"start_id"`
	EndId          uint64 `json:"end_id"`
	PerPage        uint   `json:"per_page,omitempty" query:"per_page"`
	Total          uint64 `json:"total"`
	Since          uint   `json:"since"`
	Sort           string `json:"sort,omitempty" query:"sort"`
}

func (p *Pagination) SetCursor(cursor uint64) {
	p.Cursor = cursor
}

func (p *Pagination) GetPage() uint64 {
	return p.Cursor
}

func (p *Pagination) SetStartId(start_id uint64) {
	p.StartId = start_id
}

func (p *Pagination) GetStartId() uint64 {
	return p.StartId
}

func (p *Pagination) SetEndId(end_id uint64) {
	p.EndId = end_id
}

func (p *Pagination) GetEndId() uint64 {
	return p.EndId
}

/*
 * How many records per page should be shown
 */
func (p *Pagination) SetPerPage(perPage uint) {
	p.PerPage = perPage
}

func (p *Pagination) GetPerPage() uint {
	if p.PerPage == 0 {
		p.PerPage = 10
	}
	return p.PerPage
}

/*
 * Number of records
 */
func (p *Pagination) SetTotal(recordCount uint64) {
	p.Total = recordCount
}

func (p *Pagination) GetSort() string {
	if p.Sort == "" {
		p.Sort = "Id desc"
	}
	return p.Sort
}

func (p *Pagination) SetSince(since uint) {
	p.Since = since
}

func (p *Pagination) GetSince() uint {
	if p.Since == 0 {
		p.Since = 1
	}
	return p.Since
}
