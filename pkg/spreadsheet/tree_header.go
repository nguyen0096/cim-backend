package spreadsheet

import "strings"

type TreeHeader []string

type TreeHeaderStr string

func (mh TreeHeader) String() TreeHeaderStr {
	return TreeHeaderStr(strings.Join(mh, "."))
}

func (mh TreeHeaderStr) TreeHeader() TreeHeader {
	return TreeHeader(strings.Split(string(mh), "."))
}
