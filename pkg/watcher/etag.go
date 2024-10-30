package watcher

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"io"
	"runtime"
)

const (
	BlockSize int64 = 4194304
)

type Reader interface {
	Read([]byte) (int, error)
	ReadAt([]byte, int64) (int, error)
}

func GetEtagByReader(reader Reader, size int64) string {
	buffer := make([]byte, 0, 21)
	if count := blockCount(size); count > 1 {
		buffer = getHugeEtag(reader, count)
	} else {
		buffer = getTinyEtag(reader, buffer)
	}
	return base64.URLEncoding.EncodeToString(buffer)
}

func getTinyEtag(reader Reader, buffer []byte) []byte {
	buffer = append(buffer, 0x16)
	buffer = getSha1ByReader(buffer, reader)
	return buffer
}

func doEtagWork(reader Reader, offsetChan <-chan int, conseqChan chan<- map[int][]byte) {
	for offset := range offsetChan {
		data := io.NewSectionReader(reader, int64(offset)*BlockSize, BlockSize)
		sha1 := getSha1ByReader(nil, data)
		conseqChan <- map[int][]byte{
			offset: sha1,
		}
	}
}

func getHugeEtag(reader Reader, count int64) []byte {
	conseqChan := make(chan map[int][]byte, count)
	offsetChan := make(chan int, count)

	for i := 1; i <= runtime.NumCPU(); i++ {
		go doEtagWork(reader, offsetChan, conseqChan)
	}

	for offset := 0; offset < int(count); offset++ {
		offsetChan <- offset
	}

	close(offsetChan)

	return getSha1ByConseqChan(conseqChan, count)
}

func getSha1ByConseqChan(conseqChan chan map[int][]byte, count int64) (conseq []byte) {
	sha1Map := make(map[int][]byte, 0)
	for i := 0; i < int(count); i++ {
		eachChan := <-conseqChan
		for k, v := range eachChan {
			sha1Map[k] = v
		}
	}
	blockSha1 := make([]byte, 0, count*20)
	for i := 0; int64(i) < count; i++ {
		blockSha1 = append(blockSha1, sha1Map[i]...)
	}
	conseq = make([]byte, 0, 21)
	conseq = append(conseq, 0x96)
	conseq = getSha1ByReader(conseq, bytes.NewReader(blockSha1))
	return
}

func getSha1ByReader(buffer []byte, reader Reader) []byte {
	hash := sha1.New()
	io.Copy(hash, reader)
	return hash.Sum(buffer)
}

func blockCount(size int64) int64 {
	if size > BlockSize {
		count := size / BlockSize
		if size&BlockSize == 0 {
			return count
		}
		return count + 1
	}
	return 1
}
