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
		if value == "" {
			return fmt.Errorf("duplicate empty header values found at cell [%s] - headers must have unique non-empty values within the same parent", node.TopLeftCell.String())
		}
		return fmt.Errorf("duplicate header value '%s' found at cell [%s] - headers must have unique values within the same parent", value, node.TopLeftCell.String())
	}
	hn.Lookup[value] = node
	return nil
}

func (ht HeaderNode) Width() int {
	return ht.BottomRightCell.Col - ht.TopLeftCell.Col + 1
}
