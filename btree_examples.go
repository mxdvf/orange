package main

func mockInsert(tree *BTree) {
	tree.Insert(20)
	tree.Insert(10)
	tree.Insert(11)
	tree.Insert(24)
	tree.Insert(6)
	tree.Insert(28)
	tree.Insert(32)
	tree.Insert(18)
	tree.Insert(26)
	tree.Insert(25)

	// after our complete tree construction:
	tree.Insert(27)
	tree.Insert(2)
	tree.Insert(48)
	tree.Insert(1)
	tree.Insert(21)
	tree.Insert(22)
	tree.Insert(4)
	tree.Insert(5) // very amazing testcase, my tree solved it when literally i couldn't figure out what might happen here
}
