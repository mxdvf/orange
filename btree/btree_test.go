package btree

import (
	"fmt"
	"testing"
)

func TestSimple(t *testing.T) {
	n := NewLeafNode(2)
	n.debugPrint()
	fmt.Println(n.nodeType())
}
