package db

// Return to client API
type Pagination struct {
	Cursor         uint64 `json:"cursor"`
	NextCursor     uint64 `json:"next_cursor"`
	PreviousCursor uint64 `json:"previous_cursor"`
	PerPage        uint   `json:"per_page,omitempty" query:"per_page"`
	Since          uint   `json:"since"`
}

func (p *Pagination) SetCursor(cursor uint64) {
	p.Cursor = cursor
}

func (p *Pagination) SetPrev(prev uint64) {
	p.PreviousCursor = prev
}

func (p *Pagination) GetPrev() uint64 {
	return p.PreviousCursor
}

func (p *Pagination) SetNext(next_cursor uint64) {
	p.NextCursor = next_cursor
}

func (p *Pagination) GetNext() uint64 {
	return p.NextCursor
}

func (p *Pagination) GetPage() uint64 {
	return p.Cursor
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

func (p *Pagination) SetSince(since uint) {
	p.Since = since
}

func (p *Pagination) GetSince() uint {
	if p.Since == 0 {
		p.Since = 1
	}
	return p.Since
}
