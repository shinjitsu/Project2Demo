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
	newFileInode, firstInodeNun := FileSystem.Open(FileSystem.CREATE, "Text.txt", FileSystem.RootFolder)
	stringContents, err := os.ReadFile("testInput.txt")
	if err != nil {
		log.Fatal("Oh yikes why couldn't we open the file!?!?!")
	}
	contentToWrite := []byte(stringContents)

	FileSystem.Write(&newFileInode, firstInodeNun, contentToWrite)
	fileContents := FileSystem.Read(&newFileInode)
	fmt.Println(fileContents)
	newDirectoryInode, newInodeNum := FileSystem.Open(FileSystem.CREATE, "NewDir",
		FileSystem.RootFolder)
	directoryBlock, newDirectoryInode := FileSystem.CreateDirectoryFile(FileSystem.ReadSuperBlock().RootDirInode, newInodeNum)
	bytesForDirectoryBlock := FileSystem.EncodeToBytes(directoryBlock)
	FileSystem.Write(&newDirectoryInode, newInodeNum, bytesForDirectoryBlock)
	file2Inode, lastFileInodeNum := FileSystem.Open(FileSystem.CREATE, "FileInSubdir", newDirectoryInode)
	dataToWrite := []byte("Help I'm stuck in a virtual file System\n    ")
	FileSystem.Write(&file2Inode, lastFileInodeNum, dataToWrite)
	fileInSubdirectoryContents := FileSystem.Read(&file2Inode)
	fmt.Println(fileInSubdirectoryContents)
}
