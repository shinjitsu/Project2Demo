package main

import (
	"Project2Demo/FileSystem"
	"fmt"
	"log"
	"os"
)

func main() {
	// this is I think the test I promised you - except that maybe the string is too short
	FileSystem.InitializeFileSystem()

	newFileInode, _ := FileSystem.Open(FileSystem.CREATE, "Text.txt", FileSystem.RootFolder)
	stringContents, err := os.ReadFile("testInput.txt")
	if err != nil {
		log.Fatal("Oh yikes why couldn't we open the file!?!?!")
	}
	contentToWrite := []byte(stringContents)

	FileSystem.Write(newFileInode, contentToWrite)
	fmt.Println(FileSystem.Read(newFileInode))
}
