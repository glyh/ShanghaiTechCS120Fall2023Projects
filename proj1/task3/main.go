// Reference: https://github.com/gordonklaus/portaudio/blob/master/examples/stereoSine.go

package main

import (
	"time"

	"github.com/ebitengine/oto/v3"

	"fmt"
	"math"
)

type BitString = []byte

func read_bitstring(s string) BitString {
	out := make(BitString, len(s))
	for i, v := range(s) { 
		if v == '0' {
			out[i] = 0
		} else {
			out[i] = 1
		}
	}
	return out
}

func main() {
	opts := &oto.NewContextOptions{}

	opts.SampleRate = 44100
	opts.ChannelCount = 1

	opts.Format = oto.FormatFloat32LE

	c, ready, err := oto.NewContext(opts)
	chk(err)
	<-ready

	// portaudio.Initialize()
	// defer portaudio.Terminate()
	// sampleRate := 44100.0
	msg := read_bitstring("010100111")
	modulate(c, msg, opts.SampleRate)
}

const modulate_duration = 300 * time.Millisecond
const zero_freq = 1000.0
const one_freq = 4000.0

type ChanSound struct {
	data BitString
	low []float32
	high []float32
	offset int
	sampleRate int
}

func (c *ChanSound) Read(buf []byte) (int, error) {
	sample_len := len(c.high)
	index := c.offset / sample_len
	if index >= len(c.data) {
		return 0, nil
	}
	head := c.offset
	tail := c.offset + len(buf) / 4

	for ; c.offset < tail; c.offset += 1 {
		buf_offset := (c.offset - head) * 4
		cur_index := c.offset / sample_len
		cur_offset_sig := c.offset % sample_len
		if cur_index >= len(c.data) {
			return buf_offset, nil
		}
		cur_bit := c.data[cur_index]
		cur_f := c.high[cur_offset_sig]
		if cur_bit == 0 {
			cur_f = c.low[cur_offset_sig]
		}

		bs := math.Float32bits(cur_f)

		buf[buf_offset] = byte(bs)
		buf[buf_offset+1] = byte(bs>>8)
		buf[buf_offset+2] = byte(bs>>16)
		buf[buf_offset+3] = byte(bs>>24)
	}
	return len(buf) / 4 * 4, nil
}

func modulate_bit(b bool, sampleRate float64) []float32 {
	sampleNum := int(math.Ceil(modulate_duration.Seconds() * sampleRate))
	result := make([]float32, sampleNum)

	freq := zero_freq
	if (b) {
		freq = one_freq
	}

	phase := 0.0
	phaseDelta := 2 * math.Pi / sampleRate
	
	for i := 0; i < sampleNum; i++ {
		result[i] = float32(math.Sin(freq * phase))
		phase = math.Mod(phase + phaseDelta, 2 * math.Pi)
	} 

	return result
}

func modulate(c *oto.Context, message BitString, sampleRate int) {
	one_modulated := modulate_bit(true, float64(sampleRate))
	zero_modulated := modulate_bit(false, float64(sampleRate))
	fmt.Printf("Trying to modulate %v\n", message)

	p := c.NewPlayer(&ChanSound{data: message, high: one_modulated, low: zero_modulated, sampleRate: sampleRate})
	p.Play()

	time.Sleep(time.Duration(len(message)) * modulate_duration)
	fmt.Println("Message successfully modulated and played\n")
} 

func demodulate(message []float64) BitString {
	return BitString{}
}

func chk(err error) {
	if err != nil {
		panic(err)
	}
}

func do_4b5b(message BitString) BitString {
	map_4b5b := map[byte][]byte {
		0b0000 : {1, 1, 1, 1, 0},
		0b0001 : {0, 1, 0, 0, 1},
		0b0010 : {1, 0, 1, 0, 0},
		0b0011 : {1, 0, 1, 0, 1},
		0b0100 : {0, 1, 0, 1, 0},
		0b0101 : {0, 1, 0, 1, 1},
		0b0110 : {0, 1, 1, 1, 0},
		0b0111 : {0, 1, 1, 1, 1},
		0b1000 : {1, 0, 0, 1, 0},
		0b1001 : {1, 0, 0, 1, 1},
		0b1010 : {1, 0, 1, 1, 0},
		0b1011 : {1, 0, 1, 1, 1},
		0b1100 : {1, 1, 0, 1, 0},
		0b1101 : {1, 1, 0, 1, 1},
		0b1110 : {1, 1, 1, 0, 0},
		0b1111 : {1, 1, 1, 0, 1},
	}

	groups := len(message) / 4
	if len(message) % 4 != 0 {
		groups += 1
	}

	len_out := groups * 5
	out := make(BitString, len_out)
	for i, k := 0, 0; i < len(message); i += 4 {
		var code byte
		code = 0
		for j := 0; j < 4; j++ {
			if i + j >= len(message) {
				code = code << 1
			} else {
				code = code << 1 | message[i + j]
			}
		}
		mapped := map_4b5b[code]
		copy(out[k:k+5], mapped)
		k += 5
	}
	return out
}
