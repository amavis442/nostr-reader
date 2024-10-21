package main

import "math"

type Pagination struct {
	Data interface{} `json:"data"`

	Limit      uint   `json:"limit,omitempty" query:"limit"`
	Page       uint   `json:"page,omitempty" query:"page"`
	Sort       string `json:"sort,omitempty" query:"sort"`
	TotalRows  uint64 `json:"total_rows"`
	TotalPages uint   `json:"total_pages"`

	Pages       uint64 `json:"pages"`
	Total       uint64 `json:"total"`
	PerPage     uint   `json:"per_page"`
	Offset      uint64 `json:"offset"`
	CurrentPage uint   `json:"current_page"`
	LastPage    uint64 `json:"last_page"`
	From        uint   `json:"from"`
	To          uint   `json:"to"`
	Since       uint   `json:"since"`
	Renew       bool   `json:"renew"`
	Maxid       uint64 `json:"maxid"`
}

func (p *Pagination) GetOffset() uint {
	return (p.GetPage() - 1) * p.GetLimit()
}

func (p *Pagination) GetLimit() uint {
	if p.Limit == 0 {
		p.Limit = 10
	}
	return p.Limit
}

func (p *Pagination) GetPage() uint {
	if p.Page == 0 {
		p.Page = 1
	}
	return p.Page
}

func (p *Pagination) GetSort() string {
	if p.Sort == "" {
		p.Sort = "Id desc"
	}
	return p.Sort
}

func (p *Pagination) SetTotal(recordCount uint64) {
	p.Total = recordCount
	p.SetPages(recordCount)
}

func (p *Pagination) SetLimit(limit uint) {
	p.Limit = limit
	p.PerPage = limit
}

func (p *Pagination) SetSince(since uint) {
	p.Since = since
}

func (p *Pagination) SetCurrentPage(current_page uint) {
	p.CurrentPage = current_page
	p.SetOffset()
	p.SetLastPage()
	p.setFrom()
}

func (p *Pagination) SetPages(recordCount uint64) {
	p.Pages = uint64(math.Ceil(float64(recordCount) / float64(p.Limit)))
	p.LastPage = p.Pages
}

func (p *Pagination) SetOffset() {
	p.Offset = uint64((p.CurrentPage - 1) * p.Limit)
}

func (p *Pagination) SetLastPage() {
	p.LastPage = uint64(math.Ceil(float64(p.Total) / float64(p.Limit)))
}

func (p *Pagination) setFrom() {
	p.From = (p.CurrentPage - 1) * p.Limit
}

func (p *Pagination) SetTo() {
	if p.CurrentPage == uint(p.LastPage) {
		p.To = uint(p.Total)
	} else {
		p.To = (p.CurrentPage-1)*p.Limit + p.Limit // Not correct at end
	}
}

func (p *Pagination) SetMaxId(maxid int) {
	p.Maxid = uint64(maxid)
}

func (p *Pagination) SetRenew(renew bool) {
	p.Renew = renew
}
