package main

import (
	"Project2Demo/FileSystem"
	"fmt"
	"unsafe"
)

func main() {
	testNode := FileSystem.INode{
		IsValid:        false,
		IsDirectory:    false,
		DirectBlock1:   0,
		DirectBlock2:   0,
		DirectBlock3:   0,
		IndirectBlock:  0,
		CreateTime:     0,
		LastModifyTime: 0,
	}
	fmt.Println(unsafe.Sizeof(testNode))
	dirEntry := FileSystem.DirectoryEntry{}
	fmt.Println("Size of Directory Entry: ", unsafe.Sizeof(dirEntry))
}
