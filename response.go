package calisero

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ResponseMeta is what the API reports in the headers of an answer: the request's
// trace ID, the request rate limit and, for a created message or verification,
// the account's daily sending limit. A count is nil when the answer did not carry
// it.
type ResponseMeta struct {
	// TraceID is the request's trace ID (X-Trace-Id): quote it to Calisero
	// support, or look the request up in the dashboard under Developers → Debug.
	// Empty when the answer did not carry it.
	TraceID string

	// RateLimitLimit is how many requests the API user may make a minute
	// (X-RateLimit-Limit).
	RateLimitLimit *int

	// RateLimitRemaining is how many requests the API user may still make this
	// minute (X-RateLimit-Remaining).
	RateLimitRemaining *int

	// DailyLimit is how many messages the account can send in a day
	// (X-Daily-Limit); nil when no daily limit applies or the answer is not to a
	// created message or verification.
	DailyLimit *int

	// DailyRemaining is how many messages the account can still send today, after
	// this one (X-Daily-Remaining); nil when no daily limit applies.
	DailyRemaining *int
}

// Response is a single resource as the API answers it: the resource (Data) and
// what the answer's headers report (ResponseMeta).
type Response[T any] struct {
	// Data is the resource.
	Data T

	// ResponseMeta is what the answer's headers report: the trace ID, the request
	// rate limit and, for a created message or verification, what is left of the
	// account's daily sending limit.
	ResponseMeta ResponseMeta
}

// Page is a page of a list as the API answers it: its items, the links to the
// other pages, where it stands in the whole list, and what the answer's headers
// report.
type Page[T any] struct {
	// Data are the items of the page, at most 50; empty on an empty page.
	Data []T

	// Links are the URLs of the neighbouring pages.
	Links PaginationLinks

	// Meta is where the page stands in the whole list.
	Meta PaginationMeta

	// ResponseMeta is what the answer's headers report: the trace ID and the
	// request rate limit.
	ResponseMeta ResponseMeta
}

// HasNextPage reports whether a next page exists.
func (p *Page[T]) HasNextPage() bool {
	return p.Links.Next != ""
}

// PaginationLinks are the URLs of the pages around a page; empty when there is none.
type PaginationLinks struct {
	First string `json:"first"`

	// Last is always empty: the API does not count the pages.
	Last string `json:"last"`

	Prev string `json:"prev"`

	// Next is empty on the last page.
	Next string `json:"next"`
}

// PaginationMeta is where a page stands in the whole list.
type PaginationMeta struct {
	CurrentPage int `json:"current_page"`

	// From is the position of the page's first item in the whole list; nil on an
	// empty page.
	From *int `json:"from"`

	Path string `json:"path"`

	PerPage int `json:"per_page"`

	// To is the position of the page's last item in the whole list; nil on an
	// empty page.
	To *int `json:"to"`
}

// ListOptions are the options of the calls that list resources.
type ListOptions struct {
	// Page is the page to read, from 1, each holding 50 items; 0 reads the first one.
	Page int
}

// readPage reads a page as the API sends it, each item with read.
func readPage[T any](value any, read func(any, string) (T, error)) (*Page[T], error) {
	const context = "page"

	r := newReader(value, context)
	data := readList(r, r.array("data"), context+".data", read)
	links := readPaginationLinks(r.object("links"), context+".links", r)
	meta := readPaginationMeta(r.object("meta"), context+".meta", r)

	if r.err != nil {
		return nil, r.err
	}

	return &Page[T]{Data: data, Links: links, Meta: meta}, nil
}

func readPaginationLinks(value map[string]any, context string, parent *reader) PaginationLinks {
	if parent.err != nil {
		return PaginationLinks{}
	}

	r := newReader(value, context)
	links := PaginationLinks{First: r.optStr("first"), Last: r.optStr("last"), Prev: r.optStr("prev"), Next: r.optStr("next")}
	parent.fail(r.err)

	return links
}

func readPaginationMeta(value map[string]any, context string, parent *reader) PaginationMeta {
	if parent.err != nil {
		return PaginationMeta{}
	}

	r := newReader(value, context)
	meta := PaginationMeta{
		CurrentPage: r.integer("current_page"),
		From:        r.optInt("from"),
		Path:        r.str("path"),
		PerPage:     r.integer("per_page"),
		To:          r.optInt("to"),
	}
	parent.fail(r.err)

	return meta
}

// single reads an answer holding one resource as its data.
func single[T any](res *result, read func(any, string) (T, error), context string) (*Response[T], error) {
	r := newReader(res.body, context+" answer")
	data := r.object("data")

	if r.err != nil {
		return nil, res.unexpected(r.err)
	}

	resource, err := read(data, context)
	if err != nil {
		return nil, res.unexpected(err)
	}

	return &Response[T]{Data: resource, ResponseMeta: readResponseMeta(res.header)}, nil
}

// page reads an answer holding a page of resources.
func page[T any](res *result, read func(any, string) (T, error)) (*Page[T], error) {
	resources, err := readPage(res.body, read)
	if err != nil {
		return nil, res.unexpected(err)
	}

	resources.ResponseMeta = readResponseMeta(res.header)

	return resources, nil
}

// pathID is an id in a request path, escaped: an empty one is refused, since it
// would change the request's meaning (DELETE /messages/ instead of DELETE
// /messages/{id}).
func pathID(id, name string) (string, error) {
	if strings.TrimSpace(id) == "" {
		return "", fmt.Errorf("calisero: the %s must not be empty", name)
	}

	return url.PathEscape(id), nil
}

// pageQuery is the query of a list call: the page, sent only from the second one on.
func pageQuery(page int) (url.Values, error) {
	if page < 0 {
		return nil, fmt.Errorf("calisero: the page must be a positive number, got %d", page)
	}

	query := url.Values{}

	if page > 1 {
		query.Set("page", strconv.Itoa(page))
	}

	return query, nil
}

// errNilRequest is the error of a call given no request.
func errNilRequest(call string) error {
	return errors.New("calisero: " + call + " needs a request")
}
