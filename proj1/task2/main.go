// Reference: https://github.com/gordonklaus/portaudio/blob/master/examples/stereoSine.go

package main

import (
	"github.com/gordonklaus/portaudio"
	"math"
	"fmt"
)

const sampleRate = 44100

func main() {
	portaudio.Initialize()
	defer portaudio.Terminate()
	s := newWave(sampleRate)
	defer s.Close()
	chk(s.Start())
	fmt.Println("Press 'Enter' to Exit")
	fmt.Scanln()
	chk(s.Stop())
}

type Wave struct {
	*portaudio.Stream
	sampleRate float64
	phaseDelta float64
	phase float64
}

func newWave(sampleRate float64) *Wave {
	// phaseDelta = 2 Pi / Fs
	s := &Wave{nil, sampleRate, 2 * math.Pi / sampleRate, 0.0}
	var err error
	s.Stream, err = portaudio.OpenDefaultStream(0, 1, sampleRate, 0, s.processAudio)
	chk(err)
	return s
}

func (g *Wave) processAudio(out [][]float32) {
	phase := g.phase
	for i := range out[0] {
		out[0][i] = float32(math.Sin(1000 * phase) + math.Sin(10000 * phase))
		phase = phase + g.phaseDelta
	}
	g.phase = math.Mod(phase, (2 * math.Pi))
}

func chk(err error) {
	if err != nil {
		panic(err)
	}
}
