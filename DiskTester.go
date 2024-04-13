package main

import (
	"Project2Demo/FileSystem"
	"fmt"
)

func main() {
	FileSystem.InitializeFileSystem()

	newFileInode, _ := FileSystem.Open(FileSystem.CREATE, "Text.txt", FileSystem.RootFolder)
	contentToWrite := []byte("OS Programming Project2: A virtual File System. Summary: Now that you have built your shells and looked at file systems a bit, we will implement a virtual file system.Due:  Mon April 8th by 11:59pm submitted on github Starting Out: create a new go package as a subdirectory of the project you built in project1 (in project3 we will put these two together and use both.) Details: In a separate package which is a sub-directory of your previous project (call it something like filesystem) • create a package-wide variable called Disk which will be an array of bytes of the appropriate size. (you may make it the 2 dimensional array of the sort that we talked about in class with the inner part of the array being the block size)◦ This will default to all zerosCreate a struct for the inode (go struct). We will assume old school file systems that had only one user, so no need to keep track of users and groups needs to include at least the following:")

	FileSystem.Write(newFileInode, contentToWrite)
	fmt.Println(FileSystem.Read(newFileInode))
}
