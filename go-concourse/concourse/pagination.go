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
	Since      int
	Until      int
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
	page.Since, _ = strconv.Atoi(params.Get("since"))
	page.Until, _ = strconv.Atoi(params.Get("until"))
	page.Limit, _ = strconv.Atoi(params.Get("limit"))

	return page, nil
}

func (p Page) QueryParams() url.Values {
	queryParams := url.Values{}
	if p.Until > 0 {
		queryParams.Add("until", strconv.Itoa(p.Until))
	}

	if p.Since > 0 {
		queryParams.Add("since", strconv.Itoa(p.Since))
	}

	if p.Limit > 0 {
		queryParams.Add("limit", strconv.Itoa(p.Limit))
	}

	if p.Timestamps {
		queryParams.Add("timestamps", "true")
	}

	return queryParams
}
