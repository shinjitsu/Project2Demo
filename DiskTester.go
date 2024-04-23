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

	FileSystem.Write(&newFileInode, contentToWrite)
	fmt.Println(FileSystem.Read(newFileInode))
	newDirectoryInode, newInodeNum := FileSystem.Open(FileSystem.CREATE, "NewDir",
		FileSystem.RootFolder)
	directoryBlock := FileSystem.CreateDirectoryFile(FileSystem.ReadSuperBlock().RootDirInode, newInodeNum)
	FileSystem.Write(&newDirectoryInode, FileSystem.EncodeToBytes(directoryBlock))
	file2Inode, _ := FileSystem.Open(FileSystem.CREATE, "FileInSubdir", newDirectoryInode)
	dataToWrite := []byte("Help I'm stuck in a virtual file System")
	FileSystem.Write(&file2Inode, dataToWrite)
	fmt.Println(FileSystem.Read(file2Inode))
}
