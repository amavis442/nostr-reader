package main

import "math"

type Pagination struct {
	Data interface{} `json:"data"`

	Limit      int    `json:"limit,omitempty;query:limit"`
	Page       int    `json:"page,omitempty;query:page"`
	Sort       string `json:"sort,omitempty;query:sort"`
	TotalRows  int64  `json:"total_rows"`
	TotalPages int    `json:"total_pages"`

	Pages       int64 `json:"pages"`
	Total       int64 `json:"total"`
	PerPage     int   `json:"per_page"`
	Offset      int64 `json:"offset"`
	CurrentPage int   `json:"current_page"`
	LastPage    int64 `json:"last_page"`
	From        int   `json:"from"`
	To          int   `json:"to"`
	Since       int   `json:"since"`
	Renew       bool  `json:"renew"`
	MaxId       int64 `json:"maxid"`
}

func (p *Pagination) GetOffset() int {
	return (p.GetPage() - 1) * p.GetLimit()
}

func (p *Pagination) GetLimit() int {
	if p.Limit == 0 {
		p.Limit = 10
	}
	return p.Limit
}

func (p *Pagination) GetPage() int {
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

func (p *Pagination) SetTotal(recordCount int64) {
	p.Total = recordCount
	p.SetPages(recordCount)
}

func (p *Pagination) SetLimit(limit int) {
	p.Limit = limit
	p.PerPage = limit
}

func (p *Pagination) SetSince(since int) {
	p.Since = since
}

func (p *Pagination) SetCurrentPage(current_page int) {
	p.CurrentPage = current_page
	p.SetOffset()
	p.SetLastPage()
	p.setFrom()
}

func (p *Pagination) SetPages(recordCount int64) {
	p.Pages = int64(math.Ceil(float64(recordCount) / float64(p.Limit)))
	p.LastPage = p.Pages
}

func (p *Pagination) SetOffset() {
	p.Offset = int64((p.CurrentPage - 1) * p.Limit)
}

func (p *Pagination) SetLastPage() {
	p.LastPage = int64(math.Ceil(float64(p.Total) / float64(p.Limit)))
}

func (p *Pagination) setFrom() {
	p.From = (p.CurrentPage - 1) * p.Limit
}

func (p *Pagination) SetTo() {
	if p.CurrentPage == int(p.LastPage) {
		p.To = int(p.Total)
	} else {
		p.To = (p.CurrentPage-1)*p.Limit + p.Limit // Not correct at end
	}
}

func (p *Pagination) SetMaxId(maxid int64) {
	p.MaxId = maxid
}

func (p *Pagination) SetRenew(renew bool) {
	p.Renew = renew
}
