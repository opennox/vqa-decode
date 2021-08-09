package algo

import (
	"bytes"
	"encoding/binary"
)

func replicatePrevious(dstBuf *bytes.Buffer, srcIdx int, cnt int) {
	var dst = dstBuf.Bytes()
	var dstIdx = len(dst)
	var appendDst = make([]byte, cnt)

	for i := 0; i < cnt; i++ {
		if srcIdx+i >= dstIdx {
			var appendOff = srcIdx + i - dstIdx
			appendDst[i] = appendDst[appendOff]
		} else if dstIdx-srcIdx == 1 {
			appendDst[i] = dst[dstIdx-1]
		} else {
			appendDst[i] = dst[srcIdx+i]
		}
	}
	dstBuf.Write(appendDst)
}

func DecodeFormat80Auto(src []byte) []byte {
	// The first byte being null means we use relative decoding
	var relative = src[0] == 0
	if relative {
		src = src[1:]
	}
	return DecodeFormat80(src, relative)
}

func DecodeFormat80(src []byte, relative bool) []byte {
	var dstBuf bytes.Buffer
	var srcBuf = bytes.NewReader(src)
	for {
		if srcBuf.Len() == 0 {
			break
		}
		i, err := srcBuf.ReadByte()
		if err != nil {
			break
		}
		if (i & 0x80) == 0 {
			secondByte, err := srcBuf.ReadByte()
			if err != nil {
				break
			}
			var count = ((i & 0x70) >> 4) + 3
			var rpos = (int((i & 0xf)) << 8) + int(secondByte)

			var dstIdx = dstBuf.Len()
			replicatePrevious(&dstBuf, dstIdx-rpos, int(count))
		} else if (i & 0x40) == 0 {
			var count = int(i & 0x3F)
			if count == 0 {
				return dstBuf.Bytes()
			}
			var tmpBuf = make([]byte, count)
			_, e := srcBuf.Read(tmpBuf)
			if e != nil {
				break
			}
			dstBuf.Write(tmpBuf)
		} else {
			var count3 = i & 0x3F
			if count3 == 0x3E {
				var count uint16
				if binary.Read(srcBuf, binary.LittleEndian, &count) != nil {
					break
				}
				var color, err = srcBuf.ReadByte()
				if err != nil {
					break
				}

				for i := 0; i < int(count); i++ {
					dstBuf.WriteByte(color)
				}
			} else {
				var count = int(count3) + 3
				if count3 == 0x3F {
					var scount uint16
					if binary.Read(srcBuf, binary.BigEndian, &scount) != nil {
						break
					}
					count = int(scount)
				}
				var destCnt uint16
				if binary.Read(srcBuf, binary.LittleEndian, &destCnt) != nil {
					break
				}
				var srcIndex = int(destCnt)
				var dstIndex = dstBuf.Len()
				if relative {
					srcIndex = dstIndex - int(destCnt)
				}

				var dstBytes = dstBuf.Bytes()
				var dstAppend = make([]byte, count)

				for i := 0; i < count; i++ {
					if srcIndex+i >= len(dstBytes) {
						var appendOff = srcIndex + i - len(dstBytes)
						dstAppend[i] = dstAppend[appendOff]
					} else {
						dstAppend[i] = dstBytes[srcIndex+i]
					}
				}
				dstBuf.Write(dstAppend)
			}
		}
	}
	return dstBuf.Bytes()
}
