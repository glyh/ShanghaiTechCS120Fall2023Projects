package main

import (
	"github.com/ebitengine/oto/v3"
	"math"
	"fmt"
)

const sampleRate = 44100
const data_byte_length = 4

type DoubleSine struct {
	phase float64
	phaseDelta float64
}

func NewDoubleSine(sampleRate float64) *DoubleSine {
	return &DoubleSine{
		phase: 0,
		phaseDelta: 2 * math.Pi / sampleRate,
	}
}

func (d *DoubleSine) Read(buf []byte) (int, error) {
	for i := 0; i < len(buf); i += 4 {
		f := float32(math.Sin(1000 * d.phase) + math.Sin(10000 * d.phase)) 
		bs := math.Float32bits(f)
		buf[i] = byte(bs)
		buf[i+1] = byte(bs>>8)
		buf[i+2] = byte(bs>>16)
		buf[i+3] = byte(bs>>24)
		d.phase = math.Mod(d.phase + d.phaseDelta, (2 * math.Pi))
	}
	return len(buf) / 4 * 4, nil
}

func main() {
	opts := &oto.NewContextOptions{}

	opts.SampleRate = 44100
	opts.ChannelCount = 1

	opts.Format = oto.FormatFloat32LE

	c, ready, err := oto.NewContext(opts)
	chk(err)
	<-ready

	p := c.NewPlayer(NewDoubleSine(sampleRate))
	p.Play()
	fmt.Println("Press 'Enter' to Exit")
	fmt.Scanln()

}

func chk(err error) {
	if err != nil {
		panic(err)
	}
}
