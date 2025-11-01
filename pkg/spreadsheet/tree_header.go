package spreadsheet

import "strings"

type FileHeader struct {
	// HeaderRoot is the root node of the header tree. (Sheet metadata)
	HeaderRoot *HeaderNode

	// MergeCellLookup is a map of cell coordinates to the merge cell range. (Sheet metadata)
	MergeCellLookup map[int]map[int]string
}

type HeaderBranch []string

// HeaderBranchStr is a string representation of a header branch.
// It's a list of header values separated by dots.
// Example: "Header1.Header2.Header3"
type HeaderBranchStr string

func (mh HeaderBranch) String() HeaderBranchStr {
	return HeaderBranchStr(strings.Join(mh, "."))
}

func (mh HeaderBranchStr) ToBranch() HeaderBranch {
	return HeaderBranch(strings.Split(string(mh), "."))
}
