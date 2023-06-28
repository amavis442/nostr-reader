package main

import "math"

type Pagination struct {
	Data        []Event `json:"data"`
	Pages       int64   `json:"pages"`
	Total       int64   `json:"total"`
	Limit       int     `json:"limit"`
	PerPage     int     `json:"per_page"`
	Offset      int64   `json:"offset"`
	CurrentPage int     `json:"current_page"`
	LastPage    int64   `json:"last_page"`
	From        int     `json:"from"`
	To          int     `json:"to"`
}

func (p *Pagination) SetTotal(recordCount int64) {
	p.Total = recordCount
	p.SetPages(recordCount)
}

func (p *Pagination) SetLimit(limit int) {
	p.Limit = limit
	p.PerPage = limit
}

func (p *Pagination) SetCurrentPage(current_page int) {
	p.CurrentPage = current_page
	p.SetOffset()
	p.SetLastPage()
	p.setFrom()
	p.SetTo()
}

func (p *Pagination) SetPages(recordCount int64) {
	p.Pages = int64(math.Floor(float64(recordCount) / float64(p.Limit)))
	p.LastPage = p.Pages
}

func (p *Pagination) SetOffset() {
	p.Offset = int64((p.CurrentPage - 1) * p.Limit)
}

func (p *Pagination) SetLastPage() {
	p.LastPage = int64(math.Floor(float64(p.Total) / float64(p.Limit)))
}

func (p *Pagination) setFrom() {
	p.From = (p.CurrentPage - 1) * p.Limit
}

func (p *Pagination) SetTo() {
	p.To = (p.CurrentPage-1)*p.Limit + p.Limit // Not correct at end
}
