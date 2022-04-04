package main

import (
	"fmt"
	"image/png"
	"os"

	"github.com/noxworld-dev/vqa-decode/movies"
	"github.com/youpy/go-wav"
)

func main() {
	var filename = "noxlogo.VQA"
	println(filename)
	vqa, handle, err := movies.OpenMovie(filename)
	defer handle.Close()
	if err != nil {
		println(err.Error())
	}
	//vqa.DumpAudio()
	//vqa.DumpVideo()
	var allSamples []wav.Sample
	var namepart = filename[:len(filename)-4]
	var soundName = namepart + ".wav"
	var frameFolderName = namepart
	os.Mkdir(frameFolderName, os.ModeDir)
	var frameId = 0
	for {
		frame, samples, err := vqa.DecodeNextFrame()
		if err != nil {
			break
		}
		allSamples = append(allSamples, movies.ConvertSamples(samples)...)
		if frame != nil {
			var frameName = fmt.Sprintf("%s/%05d.png", frameFolderName, frameId)
			frameFile, _ := os.Create(frameName)
			frameId++
			defer frameFile.Close()
			println(frameName)
			png.Encode(frameFile, frame)
		}
	}

	println(soundName)
	soundFile, _ := os.Create(soundName)
	defer soundFile.Close()
	var writer = wav.NewWriter(soundFile, uint32(len(allSamples)), uint16(vqa.Header.ChannelsCount), uint32(vqa.Header.SampleRate), uint16(vqa.Header.BitsPerSample))
	writer.WriteSamples(allSamples)
}
