package FileSystem

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"strings"
	"time"
)

// Disk
// first index in number of blocks
// second is block size
// I need 6144 blocks for the data - one for the superblock
// I'll cheese the 'bitmaps' as booleans so I need 6144 bytes (6 blocks) for the data 'bitmap'
// and I'll need inodes and an inode bitmap. I'll setup my inodes to be 64 bytes and if
// I have 512 of them, then I need 32 blocks for inodes
// furthermore I'll need 1 block for the inode 'bitmap'
// total blocks: 6144+1+32+6+1 => 6184
var Disk [66184][BLOCK_SIZE]byte
var RootFolder INode

const (
	INODE_SIZE = 64
	BLOCK_SIZE = 1024
)

type SuperBlock struct {
	INodeStart       int //the block location of the beginning of the inodes
	RootDirInode     int //the inode number of the root folder
	FreeBlockStart   int //the block number where the beginning of the booleans for the free blocks is found
	InodeBitmapStart int //block number of the inode 'bitmap'
	DataBlockStart   int //the block number of the beginning of the datablocks
}

type INode struct {
	IsValid        bool //true if this inode is a real file
	IsDirectory    bool //true if this file is actually a directory entry
	version        int  //at the moment this is here mostly to make the inodes be 64 bytes
	DirectBlock1   int
	DirectBlock2   int
	DirectBlock3   int
	IndirectBlock  int
	CreateTime     int64
	LastModifyTime int64
}

type DirectoryEntry struct {
	Inode int
	Name  [20]byte //I suggested 12 in class, but I realize that 20 will make this an even 32 bytes
}

type DirectoryBlock [32]DirectoryEntry

type IndirectBlock [128]int

const (
	CREATE = iota
	READ
	WRITE
	APPEND
)

func InitializeFileSystem() {
	//order on the Disk will be Superblock in block 0, inode bitmap in block 1, free block bitmap  blocks 2-7
	//inodes in blocks 8-39 and datablocks in blocks 40-end
	supBlock := SuperBlock{
		INodeStart:       8,
		RootDirInode:     1,
		FreeBlockStart:   2,
		InodeBitmapStart: 1,
		DataBlockStart:   40,
	}
	superblockBytes := EncodeToBytes(supBlock)
	copy(Disk[0][:], superblockBytes)
	createInodeBitmap(supBlock)
	createFreeBlockBitmap(supBlock)
	createInodes(supBlock)
	createRootDir(supBlock)
}

func createFreeBlockBitmap(block SuperBlock) {
	//unlike the inode bitmap, the free block bitmap will take up multiple blocks
	wholeFreeBlockBitmap := make([][BLOCK_SIZE]bool, 1)
	for bitmapBlock := block.InodeBitmapStart; bitmapBlock < block.INodeStart; bitmapBlock++ {
		var currentFreeBlockBitmap [1024]bool //should be all false by default
		wholeFreeBlockBitmap = append(wholeFreeBlockBitmap, currentFreeBlockBitmap)
	}
	writeFreeBlockBitmapToDisk(wholeFreeBlockBitmap, block)
}

func createInodeBitmap(block SuperBlock) {
	//the inode bitmap will be in block 1 and will hold 512 booleans
	var inodeBitmap [512]bool //all set to zero by default
	writeInodeBitmapToDisk(inodeBitmap, block)
}

func createInodes(sblock SuperBlock) {
	//here we will create all 512 INodes in the filesystem as invalid files
	for iNodeNum := 0; iNodeNum < 512; iNodeNum++ {
		currentInode := INode{} //make empty with all fields having false/zero value
		inodeBytes := EncodeToBytes(currentInode)
		inodeblock := (iNodeNum * INODE_SIZE / BLOCK_SIZE) + sblock.INodeStart //this is all integer division, so result is floor division
		inodeOffSet := iNodeNum * INODE_SIZE % BLOCK_SIZE
		copy(Disk[inodeblock][inodeOffSet:inodeOffSet+INODE_SIZE], inodeBytes)
	}
}

func createRootDir(sblock SuperBlock) {
	//rather than reading the existing inode in, since I know they are all empty, I'll make a new one and write it to disk
	rootFolder := INode{
		IsValid:        true,
		IsDirectory:    true,
		version:        0,
		DirectBlock1:   40, //since this happens before any other allocation, just grab block 40
		DirectBlock2:   0,
		DirectBlock3:   0,
		IndirectBlock:  0,
		CreateTime:     time.Now().Unix(),
		LastModifyTime: time.Now().Unix(),
	}
	//now we need to mark the root inode as used
	inodeBitmap := ReadINodeBitmap(sblock)
	inodeBitmap[sblock.RootDirInode] = true //claim the inode for the root folder
	writeInodeBitmapToDisk(inodeBitmap, sblock)
	//and let's claim that direct block 40
	freeBlockBitmap := ReadFreeBlockBitmap(sblock)
	freeBlockBitmap[0][rootFolder.DirectBlock1] = true
	writeFreeBlockBitmapToDisk(freeBlockBitmap, sblock)
	rootBlock := CreateDirectoryFile(0, sblock.RootDirInode)
	rootBlockBytes := EncodeToBytes(rootBlock)
	copy(Disk[rootFolder.DirectBlock1][:], rootBlockBytes)
	rootFolderAsBytes := EncodeToBytes(rootFolder)
	copy(Disk[sblock.INodeStart][INODE_SIZE*sblock.RootDirInode:INODE_SIZE*sblock.RootDirInode+INODE_SIZE], rootFolderAsBytes)
	RootFolder = rootFolder
}

func CreateDirectoryFile(parentInode int, folderinode int) DirectoryBlock {
	dot := DirectoryEntry{
		Inode: folderinode,
	}
	dot.Name[0] = '.'
	dotdot := DirectoryEntry{
		Inode: parentInode,
	}
	dotdot.Name[0] = '.'
	dotdot.Name[1] = '.'
	return DirectoryBlock{dot, dotdot}
}

func writeFreeBlockBitmapToDisk(bitmap [][BLOCK_SIZE]bool, sblock SuperBlock) {
	for loc, bitmapPart := range bitmap {
		for blockLoc, bit := range bitmapPart {
			if bit {
				Disk[loc+sblock.FreeBlockStart][blockLoc] = 1
			} else {
				Disk[loc+sblock.FreeBlockStart][blockLoc] = 0
			}
		}
	}
}

func ReadFreeBlockBitmap(sblock SuperBlock) [][BLOCK_SIZE]bool {
	//I decided to cheese this just a little to make life a little easier see below to do it right
	freeBlockBitmap := make([][BLOCK_SIZE]bool, sblock.INodeStart-sblock.FreeBlockStart)

	for bitmapBlockNum := sblock.FreeBlockStart; bitmapBlockNum < sblock.INodeStart; bitmapBlockNum++ {
		for bitLoc := 0; bitLoc < BLOCK_SIZE; bitLoc++ {
			if Disk[bitmapBlockNum][bitLoc] != 0 {
				freeBlockBitmap[bitmapBlockNum-sblock.FreeBlockStart][bitLoc] = true
			} else {
				freeBlockBitmap[bitmapBlockNum-sblock.FreeBlockStart][bitLoc] = false
			}
		}
	}
	return freeBlockBitmap
}

//this is my original - do it right version.
//func ReadFreeBlockBitmap(sblock SuperBlock) []bool {
//	freeBlockBitmap := make([]bool, BLOCK_SIZE)
//	for bitmapBlock := sblock.FreeBlockStart; bitmapBlock < sblock.INodeStart; bitmapBlock++ {
//		freeBlockBitmapPart := make([]bool, BLOCK_SIZE)
//		err := json.Unmarshal(Disk[bitmapBlock][:], &freeBlockBitmapPart)
//		if err != nil {
//			log.Fatal(err)
//		}
//		freeBlockBitmap = append(freeBlockBitmap, freeBlockBitmapPart...)
//	}
//	return freeBlockBitmap
//}

func writeInodeBitmapToDisk(bitmap [512]bool, sblock SuperBlock) {
	//I ended up having to copy bit by bit (bool by bool) there was no scope for being lazy
	for loc, bit := range bitmap {
		if bit {
			Disk[sblock.InodeBitmapStart][loc] = 1
		} else {
			Disk[sblock.InodeBitmapStart][loc] = 0
		}
	}
}

func ReadINodeBitmap(block SuperBlock) [512]bool {
	var iNodeBitmap [512]bool
	bitMapOnDisk := Disk[block.InodeBitmapStart]
	for bitNum := 0; bitNum < 512; bitNum++ {
		iNodeBitmap[bitNum] = bitMapOnDisk[bitNum] != 0 //if the byte is zero, bit is false, non-zero is true
	}
	return iNodeBitmap
}

func ReadSuperBlock() SuperBlock {
	sBlock := SuperBlock{}
	decoder := gob.NewDecoder(bytes.NewReader(Disk[0][:]))
	err := decoder.Decode(&sBlock)
	if err != nil {
		log.Fatal("Unable to Decode superblock - better blue Screen", err)
	}
	return sBlock
}

// from https://gist.github.com/SteveBate/042960baa7a4795c3565
func EncodeToBytes(p interface{}) []byte {

	buf := bytes.Buffer{}
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(p)
	if err != nil {
		log.Fatal(err)
	}
	//fmt.Println("uncompressed size (bytes): ", len(buf.Bytes()))
	return buf.Bytes()
}

// Open return values are first INodeStructure and second INode Number
func Open(mode int, name string, parentDir INode) (INode, int) {
	if !parentDir.IsDirectory || !parentDir.IsValid {
		log.Fatal("Tried to open file with invalid directory")
	}
	BlockWhereWeFindDirectoryEntry := parentDir.DirectBlock1 //I'm going to cheat here and only check direct block one since we would need more than 30 files otherwise
	DirectoryBlockBytes := Disk[BlockWhereWeFindDirectoryEntry]
	directoryEntryBlock := DirectoryBlock{}
	decoder := gob.NewDecoder(bytes.NewReader(DirectoryBlockBytes[:]))
	err := decoder.Decode(&directoryEntryBlock)
	if err != nil {
		log.Fatal("Error decoding Directory block: ", err)
	}
	validDirectoryEntries := 0
	for _, entry := range directoryEntryBlock {
		fmt.Println("DEBUG FileName: ", string(entry.Name[:]))
		if string(entry.Name[:]) == name {
			return getInodeFromDisk(entry.Inode), entry.Inode //if file is here, I'll just return it and the Inode Number for now
		}
		if entry.Inode == 0 { //once we get to invalid entries, get out of look
			break
		}
		validDirectoryEntries++
	}
	//if we got here then the file wasn't in the directory
	if mode == CREATE {
		newInode, newInodeNum := createNewInode(ReadSuperBlock())
		newFile := DirectoryEntry{
			Inode: newInodeNum,
		}
		for num, char := range name {
			if num >= 20 {
				break
			}
			newFile.Name[num] = byte(char)
		}
		directoryEntryBlock[validDirectoryEntries] = newFile
		//write the directory entry back to the disk block
		currentDirectoryBlockBytes := EncodeToBytes(directoryEntryBlock)
		copy(Disk[parentDir.DirectBlock1][:], currentDirectoryBlockBytes)
		return newInode, newInodeNum
	}
	return INode{}, 0 //if we got here, return invalid/0 inode
}

// return value will be the INode data structure, and the Inode Number
func createNewInode(sBlock SuperBlock) (INode, int) {
	inodeBitmap := ReadINodeBitmap(sBlock)
	freeInodeLoc := sBlock.RootDirInode        //we will begin looking for a free inode starting with the root node
	for ; freeInodeLoc < 512; freeInodeLoc++ { //there are only 512 possible inodes
		if inodeBitmap[freeInodeLoc] == false { //once we find an unused one stop
			inodeBitmap[freeInodeLoc] = true
			break
		}
	}
	if freeInodeLoc >= 511 {
		log.Fatal("All out of Inodes") //in a real file system I would return the 0/invalid inode
	}
	writeInodeBitmapToDisk(inodeBitmap, sBlock) //let's write it back with our new inode claimed
	newInode := INode{
		IsValid:        true,
		IsDirectory:    false,
		version:        0,
		DirectBlock1:   0,
		DirectBlock2:   0,
		DirectBlock3:   0,
		IndirectBlock:  0,
		CreateTime:     time.Now().Unix(),
		LastModifyTime: time.Now().Unix(),
	}
	writeInodeToDisk(newInode, freeInodeLoc, sBlock)
	return newInode, freeInodeLoc
}

func writeInodeToDisk(inode INode, InodeNum int, sblock SuperBlock) {
	InodeAsBytes := EncodeToBytes(inode)
	InodeBlock := (BLOCK_SIZE / INODE_SIZE) / InodeNum //once again this is floor integer division
	InodeLocInBlock := BLOCK_SIZE % InodeNum
	copy(Disk[sblock.INodeStart+InodeBlock][INODE_SIZE*InodeLocInBlock:INODE_SIZE*InodeLocInBlock+INODE_SIZE], InodeAsBytes)
}

func getInodeFromDisk(inodeNum int) INode {
	INodeBlock := inodeNum / (BLOCK_SIZE / INODE_SIZE) //there are 32 inodes per block, again int/floor division
	InodeOffset := inodeNum % (BLOCK_SIZE / INODE_SIZE)
	InodeFromDisk := INode{}
	InodeAsBytes := Disk[INodeBlock][InodeOffset : InodeOffset+INODE_SIZE]
	decoder := gob.NewDecoder(bytes.NewReader(InodeAsBytes))
	err := decoder.Decode(&InodeFromDisk)
	if err != nil {
		log.Fatal("Error decoding Inode from disk - better blue Screen", err)
	}
	return InodeFromDisk
}

func Unlink(inodeNumToDelete int, parentDir INode) {
	BlockWhereWeFindDirectoryEntry := parentDir.DirectBlock1 //I'm going to cheat here and only check direct block one since we would need more than 30 files otherwise
	DirectoryBlockBytes := Disk[BlockWhereWeFindDirectoryEntry]
	directoryEntryBlock := DirectoryBlock{}
	decoder := gob.NewDecoder(bytes.NewReader(DirectoryBlockBytes[:]))
	err := decoder.Decode(&directoryEntryBlock)
	if err != nil {
		log.Fatal("Error decoding Directory block: ", err)
	}
	validDirectoryEntries := 0
	for _, entry := range directoryEntryBlock {
		if entry.Inode == inodeNumToDelete {
			directoryEntryBlock[validDirectoryEntries] = DirectoryEntry{} //put empty one here
			inodeBitmap := ReadINodeBitmap(ReadSuperBlock())
			inodeBitmap[entry.Inode] = false
			writeInodeBitmapToDisk(inodeBitmap, ReadSuperBlock())
			inodeStruct := getInodeFromDisk(entry.Inode)
			inodeStruct.IsValid = false
			writeInodeToDisk(inodeStruct, entry.Inode, ReadSuperBlock())
			//now write directory structure back out to disk
			currentDirectoryBlockBytes := EncodeToBytes(directoryEntryBlock)
			copy(Disk[parentDir.DirectBlock1][:], currentDirectoryBlockBytes)
		}
		validDirectoryEntries++
	}
	//if we got here then we tried to delete a file not in this directory
	log.Fatal("Tried to delete file not in folder")
}

func Read(file INode) string { //I told some of you who asked that you can assume all text files, so I'll return a string
	if !file.IsValid || file.IsDirectory {
		return "" //maybe we should error, but I'll just return nothing
	}
	//I'm going to use string.Builder - which I didn't introduce in your class, but you can use + and it will be less efficient but will work
	fileContents := strings.Builder{}
	firstBlock := Disk[file.DirectBlock1]
	fileContents.Write(firstBlock[:])
	if file.DirectBlock2 == 0 {
		return fileContents.String()
	}
	secondBlock := Disk[file.DirectBlock2]
	fileContents.Write(secondBlock[:])
	if file.DirectBlock3 == 0 {
		return fileContents.String()
	}
	thirdBlock := Disk[file.DirectBlock3]
	fileContents.Write(thirdBlock[:])
	if file.IndirectBlock == 0 {
		return fileContents.String()
	}
	//now things get more complicated, we need to read from the indirect block
	indirectBlockVal := getIndirectBlock(file)
	for _, blockNum := range indirectBlockVal {
		if blockNum == 0 {
			break
		} else {
			fileContents.Write(Disk[blockNum][:])
		}
	}
	return fileContents.String()
}

func Write(file INode, content []byte) {
	numCompleteBlocks := len(content) / BLOCK_SIZE
	hasLeftovers := len(content)%BLOCK_SIZE > 0
	block := 0
	for ; block < numCompleteBlocks; block++ {
		if block == 1 {
			copy(Disk[file.DirectBlock1][:], content[BLOCK_SIZE*block:BLOCK_SIZE*(block+1)])
		} else if block == 2 {
			copy(Disk[file.DirectBlock2][:], content[BLOCK_SIZE*block:BLOCK_SIZE*(block+1)])
		} else if block == 3 {
			copy(Disk[file.DirectBlock3][:], content[BLOCK_SIZE*block:BLOCK_SIZE*(block+1)])
		} else {
			indirectBlockVal := getIndirectBlock(file)
			for indirectBlockNum, blockLoc := range indirectBlockVal {
				if blockLoc != 0 {
					copy(Disk[blockLoc][:], content[BLOCK_SIZE*block:BLOCK_SIZE*(block+1)])
					block++
					if block >= numCompleteBlocks {
						break
					}
				} else { //at this point we are appending to the file, we need to allocate blocks that we write to
					newBlock := allocateNewBlock(ReadSuperBlock())
					indirectBlockVal[indirectBlockNum] = newBlock
					//write the actual data to disk
					copy(Disk[newBlock][:], content[BLOCK_SIZE*block:BLOCK_SIZE*(block+1)])
					block++
				}
			}
			//if we wrote anything to indirect blocks, then write the indirect block block again just incase
			indirectBlockBytes := EncodeToBytes(indirectBlockVal)
			//write the indirect block to disk
			copy(Disk[file.IndirectBlock][:], indirectBlockBytes)
		}
	}
	if hasLeftovers {
		leftovers := content[(len(content)/BLOCK_SIZE)*block:]
		if numCompleteBlocks == 0 {
			copy(Disk[file.DirectBlock1][:], leftovers)
		} else if numCompleteBlocks == 1 {
			copy(Disk[file.DirectBlock2][:], leftovers)
		} else if numCompleteBlocks == 2 {
			copy(Disk[file.DirectBlock3][:], leftovers)
		} else {
			indirectBlockVal := getIndirectBlock(file)
			finalBlockLoc := indirectBlockVal[numCompleteBlocks-3] //minus 3 for the three direct blocks
			if finalBlockLoc != 0 {
				copy(Disk[finalBlockLoc][:], leftovers)
			} else {
				newBlock := allocateNewBlock(ReadSuperBlock())
				indirectBlockVal[numCompleteBlocks-3] = newBlock
				indirectBlockBytes := EncodeToBytes(indirectBlockVal)
				//write the indirect block to disk
				copy(Disk[file.IndirectBlock][:], indirectBlockBytes)
				//write the actual data to disk
				copy(Disk[newBlock][:], leftovers)
			}
		}
	}
}

// returns location of newly allocated block
func allocateNewBlock(sblock SuperBlock) int {
	freeBlockBitmap := ReadFreeBlockBitmap(sblock)
	blockNum := 0
	for _, bitmapBlock := range freeBlockBitmap {
		for locInBlock, bit := range bitmapBlock {
			if !bit {
				//this bit is available
				freeBlockBitmap[blockNum][locInBlock] = true
				writeFreeBlockBitmapToDisk(freeBlockBitmap, sblock)
				return blockNum
			} else {
				blockNum++
			}
		}
	}
	log.Fatal("Unable to allocate a free block")
	return 0
}

func getIndirectBlock(file INode) IndirectBlock {
	if file.IndirectBlock == 0 {
		return IndirectBlock{}
	}
	//now we need to do the indirect blocks
	indirectBlockBytes := getIndirectBlockFromDisk(file.IndirectBlock)
	indirectBlockVal := IndirectBlock{}
	decoder := gob.NewDecoder(bytes.NewReader(indirectBlockBytes[:]))
	err := decoder.Decode(&indirectBlockVal)
	if err != nil { //the better blue screen came from jetbrains AI
		log.Fatal("Error decoding IndirectBlock from disk - better blue Screen", err)
	}
	return indirectBlockVal
}

func getIndirectBlockFromDisk(indirectBlockNum int) [1024]byte {
	return Disk[indirectBlockNum]
}
