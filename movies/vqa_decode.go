package movies

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"
	"io"
	"math"

	"github.com/JoshuaDoes/adpcm-go"

	"github.com/opennox/vqa-decode/algo"
)

type VqaDecoder struct {
	adpcmLeftDec  *adpcm.Decoder
	adpcmRightDec *adpcm.Decoder

	curFrame   *image.NRGBA
	currentCbf []color.NRGBA
}

const vqaCbfId = "CBF"
const vqaVprzId = "VPRZ"
const vqaVptrId = "VPTR"

func (vqa *VqaFile) initDecoder() {
	vqa.dec = new(VqaDecoder)
	vqa.dec.adpcmLeftDec = adpcm.NewDecoder(1)
	if vqa.Header.ChannelsCount > 1 {
		vqa.dec.adpcmRightDec = adpcm.NewDecoder(1)
	}

	vqa.dec.curFrame = image.NewNRGBA(image.Rect(0, 0, int(vqa.Header.SizeX), int(vqa.Header.SizeY)))
	draw.Draw(vqa.dec.curFrame, vqa.dec.curFrame.Bounds(), image.Black, image.Point{}, draw.Src)
}

func (vqa *VqaFile) decodeSnd2Chunk(data []byte) [][2]int16 {
	if vqa.Header.ChannelsCount == 1 {
		var out []int
		vqa.dec.adpcmLeftDec.Decode(data, &out)
		var samples [][2]int16
		for _, smpl := range out {
			var sample [2]int16
			sample[0] = int16(smpl)
			samples = append(samples, sample)
		}
		return samples
	}
	var halfCnt = len(data) / 2
	var left = data[:halfCnt]
	var right = data[halfCnt:]
	// Nox have a weird notion of endianness... INSIDE a byte, seriously?
	for idx := range left {
		left[idx] = ((left[idx] & 0xf0) >> 4) + ((left[idx] & 0x0f) << 4)
	}
	for idx := range right {
		right[idx] = ((right[idx] & 0xf0) >> 4) + ((right[idx] & 0x0f) << 4)
	}
	var leftOut []int
	vqa.dec.adpcmLeftDec.Decode(left, &leftOut)
	var rightOut []int
	vqa.dec.adpcmRightDec.Decode(right, &rightOut)
	var samples = make([][2]int16, len(leftOut))
	for idx := range leftOut {
		var sample [2]int16
		sample[0] = int16(leftOut[idx])
		sample[1] = int16(rightOut[idx])
		samples[idx] = sample
	}
	return samples
}

func (vqa *VqaFile) writeFrameBlock(blockIdx int, cnt int, x *int, y int, ignoreAlpha bool) {
	var colorOffset = blockIdx * int(vqa.Header.BlockSizeX) * int(vqa.Header.BlockSizeY)
	for ; cnt > 0; cnt-- {
		for yBlockOff := 0; yBlockOff < int(vqa.Header.BlockSizeY); yBlockOff++ {
			var coordY = int(vqa.Header.BlockSizeY)*y + yBlockOff
			for xBlockOff := 0; xBlockOff < int(vqa.Header.BlockSizeX); xBlockOff++ {
				var coordX = int(vqa.Header.BlockSizeX)**x + xBlockOff
				var colorCoord = colorOffset + yBlockOff*int(vqa.Header.BlockSizeX) + xBlockOff
				if colorCoord < len(vqa.dec.currentCbf) {
					var color = vqa.dec.currentCbf[colorCoord]
					if ignoreAlpha {
						color.A = math.MaxUint8
					}
					if ignoreAlpha || color.A != 0 {
						vqa.dec.curFrame.SetNRGBA(coordX, coordY, color)
					}
				}
			}
		}
		*x++
	}
}

func (vqa *VqaFile) decodeVptrSubchunk(data []byte) {
	var xBlockCnt = int(vqa.Header.SizeX) / int(vqa.Header.BlockSizeX)
	var yBlockCnt = int(vqa.Header.SizeY) / int(vqa.Header.BlockSizeY)
	var dataOff = 0
	for y := 0; y < yBlockCnt; y++ {
		for x := 0; x < xBlockCnt; {
			var cmdData = uint16(data[dataOff])
			cmdData |= uint16(data[dataOff+1]) << 8
			dataOff += 2
			var cmd = cmdData >> 13
			if cmd == 0 {
				var skipCount = cmdData & 0x1fff
				x += int(skipCount)
			} else if cmd == 1 {
				var blockIdx = cmdData & 0xff
				var count = ((cmdData & 0x1f00) + 0x100) >> 7
				vqa.writeFrameBlock(int(blockIdx), int(count), &x, y, true)
			} else if cmd == 2 {
				var blockIdx = cmdData & 0xff
				var count = ((cmdData & 0x1f00) + 0x100) >> 7
				vqa.writeFrameBlock(int(blockIdx), 1, &x, y, true)
				for ; count > 0; count-- {
					blockIdx = uint16(data[dataOff])
					dataOff++
					vqa.writeFrameBlock(int(blockIdx), 1, &x, y, true)
				}
			} else if cmd == 3 || cmd == 4 {
				var blockIdx = cmdData & 0x1fff
				vqa.writeFrameBlock(int(blockIdx), 1, &x, y, cmd == 3)
			} else if cmd == 5 || cmd == 6 {
				var blockIdx = cmdData & 0x1fff
				var count = uint16(data[dataOff])
				dataOff++
				vqa.writeFrameBlock(int(blockIdx), int(count), &x, y, cmd == 5)
			}
		}
	}
}

func (vqa *VqaFile) decodeCbfSubchunk(data []byte) {
	var pixelsCnt = len(data) / 2
	var newCbf = make([]color.NRGBA, pixelsCnt)
	for i := 0; i < pixelsCnt; i++ {
		var pixelData = (uint16(data[i*2+1]) << 8) | uint16(data[i*2])
		var pixel color.NRGBA
		pixel.A = math.MaxUint8 - uint8(((pixelData>>15)&0x1)*math.MaxUint8)
		pixel.R = uint8(((pixelData >> 10) & 0x1f) * math.MaxUint8 / 31)
		pixel.G = uint8(((pixelData >> 5) & 0x1f) * math.MaxUint8 / 31)
		pixel.B = uint8(((pixelData) & 0x1f) * math.MaxUint8 / 31)
		newCbf[i] = pixel
	}
	vqa.dec.currentCbf = newCbf
}

func (vqa *VqaFile) decodeVqfChunk(data []byte) (bool, image.NRGBA) {
	var dataBuf = bytes.NewReader(data)
	var frameUpdated = false
	for dataBuf.Len() > 0 {
		curPos, _ := dataBuf.Seek(0, io.SeekCurrent)
		if (curPos & 1) == 1 {
			_, _ = dataBuf.ReadByte()
		}
		var subChunk VqaChunkHeader
		binary.Read(dataBuf, binary.BigEndian, &subChunk)
		var chunkData = make([]byte, subChunk.Size)
		dataBuf.Read(chunkData)
		if subChunk.Id[3] == 'Z' {
			chunkData = algo.DecodeFormat80Auto(chunkData)
		}
		if string(subChunk.Id[:3]) == vqaCbfId {
			vqa.decodeCbfSubchunk(chunkData)
		} else if string(subChunk.Id[:]) == vqaVptrId || string(subChunk.Id[:]) == vqaVprzId {
			vqa.decodeVptrSubchunk(chunkData)
			frameUpdated = true
		}
	}
	return frameUpdated, *vqa.dec.curFrame
}
