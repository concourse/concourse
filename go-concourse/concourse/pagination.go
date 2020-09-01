package concourse

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/peterhellberg/link"
)

type Pagination struct {
	Next     *Page
	Previous *Page
}

func paginationFromHeaders(header http.Header) (Pagination, error) {
	var pagination Pagination
	var nextPage Page
	var previousPage Page
	var err error

	linkGroup := link.ParseHeader(header)
	nextPageLink := linkGroup["next"]

	if nextPageLink != nil {
		nextPage, err = pageFromURI(nextPageLink.String())
		if err != nil {
			return Pagination{}, err
		}
		pagination.Next = &nextPage
	}

	previousPageLink := linkGroup["previous"]
	if previousPageLink != nil {
		previousPage, err = pageFromURI(previousPageLink.String())
		if err != nil {
			return Pagination{}, err
		}
		pagination.Previous = &previousPage
	}

	return pagination, nil
}

type Page struct {
	From       int
	To         int
	Limit      int
	Timestamps bool
}

func pageFromURI(uri string) (Page, error) {
	var page Page

	url, err := url.Parse(uri)
	if err != nil {
		return Page{}, err
	}
	params := url.Query()
	page.From, _ = strconv.Atoi(params.Get("from"))
	page.To, _ = strconv.Atoi(params.Get("to"))
	page.Limit, _ = strconv.Atoi(params.Get("limit"))

	return page, nil
}

func (p Page) QueryParams() url.Values {
	queryParams := url.Values{}
	if p.From > 0 {
		queryParams.Add("from", strconv.Itoa(p.From))
	}

	if p.To > 0 {
		queryParams.Add("to", strconv.Itoa(p.To))
	}

	if p.Limit > 0 {
		queryParams.Add("limit", strconv.Itoa(p.Limit))
	}

	if p.Timestamps {
		queryParams.Add("timestamps", "true")
	}

	return queryParams
}
