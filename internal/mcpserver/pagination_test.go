package mcpserver

import "testing"

func TestNewPagination(t *testing.T) {
	cur := "next"
	cases := []struct {
		name                        string
		offset, pageSize, totalHits int
		resultsOnPage               int
		nextCursor                  *string
		wantPage, wantTotalPages    int
		wantHasMore                 bool
	}{
		{"empty", 0, 20, 0, 0, nil, 1, 0, false},
		{"partial single page", 0, 20, 5, 5, nil, 1, 1, false},
		{"exact single page", 0, 20, 20, 20, nil, 1, 1, false},
		{"first of three", 0, 20, 45, 45, &cur, 1, 3, true},
		{"middle of three", 20, 20, 45, 45, &cur, 2, 3, true},
		{"last of three", 40, 20, 45, 45, nil, 3, 3, false},
		{"exact multiple two pages", 0, 20, 40, 40, &cur, 1, 2, true},
		{"zero page size guarded", 0, 0, 10, 10, nil, 1, 1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := newPagination(tc.offset, tc.pageSize, tc.totalHits, tc.resultsOnPage, tc.nextCursor)
			if p.Page != tc.wantPage {
				t.Errorf("Page = %d, want %d", p.Page, tc.wantPage)
			}
			if p.TotalPages != tc.wantTotalPages {
				t.Errorf("TotalPages = %d, want %d", p.TotalPages, tc.wantTotalPages)
			}
			if p.HasMore != tc.wantHasMore {
				t.Errorf("HasMore = %v, want %v", p.HasMore, tc.wantHasMore)
			}
			if p.TotalHits != tc.totalHits {
				t.Errorf("TotalHits = %d, want %d", p.TotalHits, tc.totalHits)
			}
			if p.PageSize != tc.pageSize {
				t.Errorf("PageSize = %d, want %d", p.PageSize, tc.pageSize)
			}
		})
	}
}

// Page arithmetic is in document units (total_hits), while results_on_page
// reports what the caller actually received. Deriving pages from the result
// count would overstate them whenever a document matches on several lines.
func TestNewPaginationPagesOnDocumentsNotResults(t *testing.T) {
	cur := "next"
	// 30 matching documents carrying 100 result lines, 20 documents per page.
	p := newPagination(0, 20, 30, 100, &cur)

	if p.TotalPages != 2 {
		t.Errorf("TotalPages = %d, want 2 (ceil(30 documents / 20)), not %d (from results)",
			p.TotalPages, (100+19)/20)
	}
	if p.TotalHits != 30 {
		t.Errorf("TotalHits = %d, want 30 (documents)", p.TotalHits)
	}
	if p.ResultsOnPage != 100 {
		t.Errorf("ResultsOnPage = %d, want 100", p.ResultsOnPage)
	}
}
