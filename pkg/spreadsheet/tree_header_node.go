package spreadsheet

import "fmt"

type HeaderNode struct {
	TopLeftCell     Cell
	BottomRightCell Cell
	Value           string
	// IsEmpty represents if the node and all its children are empty
	IsEmpty    bool
	SubHeaders []*HeaderNode
	Lookup     map[string]*HeaderNode
}

func (hn *HeaderNode) SetLookup(value string, node *HeaderNode) error {
	if hn.Lookup == nil {
		hn.Lookup = make(map[string]*HeaderNode)
	}
	if _, ok := hn.Lookup[value]; ok {
		return fmt.Errorf("lookup value already exists")
	}
	hn.Lookup[value] = node
	return nil
}

func (ht HeaderNode) Width() int {
	return ht.BottomRightCell.Col - ht.TopLeftCell.Col + 1
}
