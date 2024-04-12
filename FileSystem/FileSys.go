package FileSystem

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"log"
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
	inode int
	name  [20]byte //I suggested 12 in class, but I realize that 20 will make this an even 32 bytes
}

type DirectoryBlock [32]DirectoryEntry

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
	for bitmapBlock := block.InodeBitmapStart; bitmapBlock < block.INodeStart; bitmapBlock++ {
		currentFreeBlockBitmap := make([]bool, BLOCK_SIZE) //should be all false by default
		currentFreeAsBytes := EncodeToBytes(currentFreeBlockBitmap)
		copy(Disk[bitmapBlock][:], currentFreeAsBytes)
	}
}

func createInodeBitmap(block SuperBlock) {
	//the inode bitmap will be in block 1 and will hold 512 booleans
	var inodeBitmap [512]bool //all set to zero by default
	bitbmapBytes := EncodeToBytes(inodeBitmap)
	copy(Disk[block.INodeStart][:], bitbmapBytes)
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
	rootBlock := createDirectoryFile(0, sblock.RootDirInode)
	rootBlockBytes := EncodeToBytes(rootBlock)
	copy(Disk[rootFolder.DirectBlock1][:], rootBlockBytes)
	//now write inode of root folder
}

func createDirectoryFile(parentInode int, folderinode int) DirectoryBlock {
	dot := DirectoryEntry{
		inode: folderinode,
	}
	dot.name[0] = '.'
	dotdot := DirectoryEntry{
		inode: parentInode,
	}
	dotdot.name[0] = '.'
	dotdot.name[1] = '.'
	return DirectoryBlock{dot, dotdot}
}

func writeFreeBlockBitmapToDisk(bitmap [][BLOCK_SIZE]bool, sblock SuperBlock) {
	for loc, bitmapPart := range bitmap {
		bitmapBytes := EncodeToBytes(bitmapPart)
		copy(Disk[sblock.FreeBlockStart+loc][:], bitmapBytes)
	}
}

func ReadFreeBlockBitmap(sblock SuperBlock) [][BLOCK_SIZE]bool {
	//I decided to cheese this just a little to make life a little easier see below to do it right
	freeBlockBitmap := make([][BLOCK_SIZE]bool, sblock.InodeBitmapStart-sblock.FreeBlockStart)
	count := 0
	for bitmapBlockNum := sblock.FreeBlockStart; bitmapBlockNum < sblock.INodeStart; bitmapBlockNum++ {
		var freeBlockBitmapPart [BLOCK_SIZE]bool
		err := json.Unmarshal(Disk[bitmapBlockNum][:], &freeBlockBitmapPart)
		if err != nil {
			log.Fatal(err)
		}
		freeBlockBitmap[count] = freeBlockBitmapPart
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

func writeInodeBitmapToDisk(bitmap []bool, sblock SuperBlock) {
	bitmapbytes := EncodeToBytes(bitmap)
	copy(Disk[sblock.InodeBitmapStart][:], bitmapbytes)
}

func ReadINodeBitmap(block SuperBlock) []bool {
	iNodeBitmap := make([]bool, BLOCK_SIZE)
	err := json.Unmarshal(Disk[block.InodeBitmapStart][:], &iNodeBitmap) //a little bit of a cheat, let standard library convert for me
	if err != nil {
		log.Fatal("Error getting the INodeBitmap", err)
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
	fmt.Println("uncompressed size (bytes): ", len(buf.Bytes()))
	return buf.Bytes()
}
