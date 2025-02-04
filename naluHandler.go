package naluhandler

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
)

var (
	Init_table    = []int{0, 1, 2, 0}
	Init_table_3B = []int{0, 1, 0}
	InitialNALU   = []byte{0, 0, 0, 1}
	Initial3BNALU = []byte{0, 0, 1}
)

const stride = 4096

var pl = fmt.Println
var pf = fmt.Printf

type H264FileHandler struct {
	H264File        *os.File
	FileReader      *bufio.Reader
	FileReadBuffer  []byte
	SplittedNalUnit []byte
}

func InitFileHandler(filename string) *H264FileHandler {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalln(err)
	}

	r := bufio.NewReader(file)
	buf := make([]byte, 0, stride)

	FileHandler := &H264FileHandler{
		H264File:       file,
		FileReader:     r,
		FileReadBuffer: buf,
	}

	return FileHandler
}

// ReadFile read FileReadBuffer fer size bytes from file
// return read bytes, err
func (h *H264FileHandler) GetNalUnit(nalu_ch chan []byte) {
	h.SplittedNalUnit = []byte{}
	total := 0
	sentbytes := 0
	for { // cnt := 0; cnt < 10; cnt++
		readDataBytes, err := io.ReadFull(h.FileReader, h.FileReadBuffer[:cap(h.FileReadBuffer)])
		h.FileReadBuffer = h.FileReadBuffer[:readDataBytes] // stream data, not nalu yet
		if err != nil {
			if err == io.EOF {
				break
			}
			if err != io.ErrUnexpectedEOF {
				log.Fatalln(err)
				panic(err)
			}
		}

		total += readDataBytes
		currentLength := len(h.SplittedNalUnit)
		h.SplittedNalUnit = append(h.SplittedNalUnit, h.FileReadBuffer...)

		var startPositions = []int{}
		if currentLength > 4 {
			offset := currentLength - 4
			startPositions = findStartSequencePosition(h.SplittedNalUnit[offset:])
			for idx, val := range startPositions {
				startPositions[idx] = val + offset
			}
		} else {
			startPositions = findStartSequencePosition(h.SplittedNalUnit)
			if len(startPositions) > 2 {
				startPositions = startPositions[1:]
			}
		}

		if len(startPositions) >= 1 && startPositions[0] != 0 {
			//start := startPositions[0]
			start := 0
			for _, end := range startPositions {
				if end-start == 0 {
					pl(startPositions)
					pl("why 0?")
				}
				nalu_ch <- h.SplittedNalUnit[start:end]
				start = end
				sentbytes += (end - start)
			}
			//pf("[%d-]: %x\n",start ,nalu[:4])
			h.SplittedNalUnit = h.SplittedNalUnit[start:]
			//pf("[%d-]: %x\n",start , h.SplittedNalUnit[:4])
			//h.SplittedNalUnit = append(h.SplittedNalUnit, h.FileReadBuffer[start:]...)
		}

	}
	//last nal unit
	nalu_ch <- h.SplittedNalUnit
	sentbytes += len(h.SplittedNalUnit)

	pl("total Read bytes from file: ", total)
	close(nalu_ch)
	h.H264File.Close()
	return
}

// return nalu unit start offset
func findStartSequencePosition(bytes []byte) []int {
	ch := make(chan int, 2)

	go kmp(bytes, InitialNALU, ch)
	go kmp(bytes, Initial3BNALU, ch)
	pos := make([]int, 0, 8)
	flag := 2
	for offset := range ch {
		//case: go routines is end
		if offset == -1 {
			// every go routines are end
			if flag += offset; flag == 0 {
				break
			}
		} else {
			pos = append(pos, offset)
		}
	}
	close(ch)

	// no start point
	if len(pos) == 0 {
		return pos
	}

	sort.Ints(pos)

	startpoints := make([]int, 0, 8)
	for i := 0; i < len(pos)-1; i++ {
		if pos[i+1]-pos[i] == 1 {
			startpoints = append(startpoints, pos[i])
		} else {
			startpoints = append(startpoints, pos[i+1])
		}
	}
	// make to unique slice
	keys := make(map[int]struct{})
	res := make([]int, 0)
	for _, val := range startpoints {
		if _, ok := keys[val]; ok {
			continue
		} else {
			keys[val] = struct{}{}
			res = append(res, val)
		}
	}
	// dont need last nal unit start point
	// becase cannot know end of last this time
	return res
}

func kmp(bytes []byte, find []byte, ch chan int) {
	byteLength := len(bytes)
	patternLength := len(find)

	var table []int

	if patternLength == 4 {
		table = Init_table
	} else {
		table = Init_table_3B
	}

	i := 0
	for i < byteLength {
		j := 0
		for i < byteLength && j < patternLength {
			if bytes[i] == find[j] {
				i++
				j++
			} else {
				if j != 0 {
					j = table[j-1]
				} else {
					i++
				}
			}
		}
		// case: find pattern
		if j == patternLength {
			ch <- i - j
		}

		// case: end of bytes
		if i == byteLength {
			ch <- -1
			return
		}
	}
}

type params struct {
	bytesToRemove int
	offset        int
	ratio         bool
	reverse       bool
}

// InitialNALU = []byte{0, 0, 0, 1}
//Initial3BNALU = []byte{0, 0, 1}

// deleteNbytes only delete nalu's RBSP except header (ratio or constant)
// parameter (object data, mount to delete, offset, based on data ratio or constant value, start  header or tail)
func removeNBytes(nalu []byte, p *params) []byte {
	if len(nalu) == 0 {
		pl("no data to be deleted")
		return nil
	} else if p.bytesToRemove < 1 {
		pl("nalu copy mode: ", len(nalu), "Bytes are copied")
		return nalu
	} else if p.offset < 0 || p.offset > 99 {
		pl("Invalid offset-value")
		return nil
	}
	N := p.bytesToRemove

	// if len(nalu) < 105 {
	//     return nalu
	// }

	var startcodesize int
	if nalu[2] == Initial3BNALU[2] {
		startcodesize = 3
	} else {
		startcodesize = 4
	}

	// pps and sps is exception
	if nalu[startcodesize]&0x1f == 7 || nalu[startcodesize]&0x1f == 8 || len(nalu) < 105 {
		return nalu
	}

	offset := int(float64(p.offset*(len(nalu)-startcodesize-1))*0.01) + startcodesize + 1 // +1 is nalu header size
	ratio := p.ratio
	reverse := p.reverse

	var deletedNalu []byte
	copiedNalu := make([]byte, len(nalu))
	copy(copiedNalu, nalu)
	//  data ratio based, value must to be haved 1 ~ 99
	if ratio {
		if N > 99 || N < 1 {
			//pl("Invalid value: ratio must be 1~99 ", N)
			return nalu
		}

		sizeToDelete := int(float64(len(nalu)*N) * 0.01)
		if !reverse {
			deletedNalu = copiedNalu[:offset]
			if offset+sizeToDelete > len(nalu) {
				return deletedNalu
			}

			deletedNalu = append(deletedNalu, copiedNalu[offset+sizeToDelete:]...)
		} else {
			deletedNalu = copiedNalu[:len(nalu)-sizeToDelete+1]
		}

		// constant based, value don't be haved value over nalu's RBSP length
	} else {
		if len(nalu)-1 < N {
			//pl("Invalid value: constant must not be over nalu's RBSP length ", N)
			return nalu
		}

		if !reverse {
			deletedNalu = copiedNalu[:offset] // +1 is for includeing header
			if offset+N > len(nalu)-1 {
				return deletedNalu
			}
			deletedNalu = append(deletedNalu, copiedNalu[offset+N:]...)

		} else {
			deletedNalu = copiedNalu[:len(nalu)+1-N]
		}
	}

	return deletedNalu
}

var RatioForDeleting = 0

// func main(){
//     for start_offset := 5; start_offset <= 100; start_offset += 5{
//         p := &params{
//             bytesToRemove:     RatioForDeleting,
//             offset:            start_offset,
//             ratio:             true,
//             reverse:           false,
//         }

//         if RatioForDeleting + start_offset > 100 {
//             pl("\n !!! out of data !!!")
//             break
//         }

//         objectFileName := "./" + strconv.Itoa(p.bytesToRemove)+ "per_" + strconv.Itoa(p.offset)+ "offset.h264"
//         // objectFileName := "./" + strconv.Itoa(p.bytesToRemove)+ "bytes_" + strconv.Itoa(p.offset)+ "offset_ratio-" + strconv.FormatBool(p.ratio) + "_reverse-"+strconv.FormatBool(p.reverse) +".h264"
//         // if p.reverse {
//         //     objectFileName = "./" + strconv.Itoa(p.bytesToRemove)+ "bytes_" + "ratio:" + strconv.FormatBool(p.ratio) + "_reverse:"+strconv.FormatBool(p.reverse) + ".h264"
//         // }
//         if p.bytesToRemove == 0{
//             objectFileName = "./copiedFile.h264"
//             start_offset += 100
//         }
//         f1, err := os.Create(objectFileName)
//         if err != nil {
//             panic(err)
//         }
//         defer f1.Close()
//         w := bufio.NewWriter(f1)

//         /*----------------------------------------------------*/
//         filename := "pir-LOA_TimeOfEnd_720p.h264"
//         h := InitFileHandler(filename)
//         nalu_ch := make(chan []byte)

//         start := time.Now()
//         go h.GetNalUnit(nalu_ch)

//         num_of_Nalu := 0
//         receivedbytes := 0

//         total_writtenBytes := 0

//         var arr_naluLen []int
//         maxLen := 0
//         minLen := 999999999999
//         naluLen := 0
//         for nalu := range nalu_ch{
//             num_of_Nalu ++
//             naluLen = len(nalu)
//             if naluLen < 1{
//                 pl(len(nalu), " nalu size received")
//                 panic("error: Nalu size is under 1")
//             }
//             receivedbytes += naluLen
//             //pl(nalu)
//             arr_naluLen = append(arr_naluLen, naluLen)

//             if maxLen < naluLen{
//                 maxLen = naluLen
//             }

//             if minLen > naluLen{
//                 minLen = naluLen
//             }

//             //deleteNbytes(nalu, size, offset, ratio, reverse)
//             deletedNalu := removeNBytes(nalu, p) // remove 100 byte, offset = 50 bytes, constant
//             if deletedNalu == nil{
//                 pl("error: removeNBytes: nothing to write")
//                 break
//             }
//             n, err := w.Write(deletedNalu)
//             if err != nil{
//                 break
//             }
//             total_writtenBytes += n

//             if num_of_Nalu > 600{
//                 break
//             }
//         }

//         w.Flush()

//         elapsed := time.Since(start)
//         pl("read done, process time", elapsed)
//         pl()

//         // make unique slice
//         keys := make(map[int]struct{})
//         nonDup_NaluLen := make([]int, 0)
//         for _, val := range arr_naluLen {
//             if _, ok := keys[val]; ok {
//                 continue
//             } else {
//                 keys[val] = struct{}{}
//                 nonDup_NaluLen = append(nonDup_NaluLen, val)
//             }
//         }

//         sort.Ints(nonDup_NaluLen)

//         pl("File Name: ", objectFileName)
//         pl("total written bytes: ", total_writtenBytes)
//         pl("total nalu bytes size: ", receivedbytes)
//         pl("original data - fixed data = ", receivedbytes-total_writtenBytes)
//         pl("num. of nalu: ", num_of_Nalu)
//         pl("num. of unique nalu size: ", len(nonDup_NaluLen))
//         pl("minLen: ", minLen, "\tmaxLen: ", maxLen)
//         pl("avrLen: ", receivedbytes/num_of_Nalu, "midLen:", nonDup_NaluLen[len(nonDup_NaluLen)/2])

//         // make mp4 file

//         // cmd := exec.Command("ffmpeg -i "+ strings.Trim(objectFileName, "./") +" -c:v copy "+ strings.Trim(objectFileName, "./h264") +".mp4")
// 	    // cmd.Stdout = os.Stdout
// 	    // if err := cmd.Run(); err != nil{
// 		//     fmt.Println(err)
//         //     break
// 	    // }
//         // cmd = exec.Command("rm " + objectFileName)
//         // cmd.Stdout = os.Stdout
// 	    // if err := cmd.Run(); err != nil{
// 		//     fmt.Println(err)
//         //     break
// 	    // }
//     }
// }
