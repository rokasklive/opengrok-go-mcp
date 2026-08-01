package mcpserver

// newPagination derives top-level page metadata from the absolute result
// offset, the page size, the total hit count, the number of results actually
// serialized on this page, and the next-page cursor.
//
// totalHits is OpenGrok's count of matching *documents*, which is also the unit
// it paginates over — so page_size bounds documents, and page arithmetic is
// derived from totalHits. resultsOnPage is what the caller received, which for
// code search counts matching *lines* and can exceed page_size when a document
// matches on several lines.
func newPagination(offset, pageSize, totalHits, resultsOnPage int, nextCursor *string) Pagination {
	p := Pagination{
		PageSize:      pageSize,
		TotalHits:     totalHits,
		ResultsOnPage: resultsOnPage,
		NextCursor:    nextCursor,
		HasMore:       nextCursor != nil,
	}
	if pageSize > 0 {
		p.Page = offset/pageSize + 1
		p.TotalPages = (totalHits + pageSize - 1) / pageSize
	} else {
		p.Page = 1
		if totalHits > 0 {
			p.TotalPages = 1
		}
	}
	return p
}
