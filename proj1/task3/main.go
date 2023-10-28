// Reference: https://github.com/gordonklaus/portaudio/blob/master/examples/stereoSine.go

package main

import (
	"fmt"
	"math"

	"github.com/gordonklaus/portaudio"
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
	portaudio.Initialize()
	defer portaudio.Terminate()
	sampleRate := 44100.0
	msg := read_bitstring("010100111")
	modulate(msg, sampleRate)
	// modulate(do_4b5b(msg), sampleRate)
	fmt.Println("Message successfully modulated and played")
	fmt.Scanln()
}

const modulate_duration = 1.0 // 1sec
const zero_freq = 40000.0
const one_freq = 80000.0

func modulate_bit(b bool, sampleRate float64) []float32 {
	sampleNum := int(math.Ceil(modulate_duration * sampleRate))
	result := make([]float32, sampleNum)

	freq := zero_freq
	if (b) {
		freq = one_freq
	}

	for i := 0; i < sampleNum; i++ {
		t := float64(i) / sampleRate
		result[i] = float32(math.Sin(2 * math.Pi * freq * t))
	} 

	return result
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

func process_bits(message BitString, sampleRate float64, sig chan bool) func([][]float32) {
	one_modulated := modulate_bit(true, sampleRate)
	zero_modulated := modulate_bit(false, sampleRate)
	frame_id := 0.0
	return func (out [][]float32) {
		bit_index := int(frame_id / sampleRate)
		if bit_index >= len(message) {
			sig <- true
		} else {
			in_frame_delta := math.Mod(frame_id, sampleRate)
			out_offset := 0
			if sampleRate - in_frame_delta < 
			for ; sampleRate - in_frame_delta < float64(len(out[0]) - out_offset); {
				copy(out[out_offset:out_offset])

			}
			frame_id += len(out)
		}

		// fmt.Printf("Modulated as %v\n", out[0])
		// if offset >= len(message) {
		// 	sig <- true
		// } else {
		// 	fmt.Printf("Playing %d\n", message[offset])
		// 	if message[offset] == 1 {
		// 		copy(out[0], one_modulated)
		// 	} else {
		// 		copy(out[0], zero_modulated)
		// 	}
		// 	fmt.Printf("Modulated as %v\n", out[0])
		// }
	}
}

func modulate(message BitString, sampleRate float64) {
	fmt.Printf("Trying to modulate %v", message)
	stop := make(chan bool)
	stream, err := portaudio.OpenDefaultStream(0, 1, sampleRate, 0, process_bits(message, sampleRate, stop))
	chk(err)
	stream.Start()
	<-stop 
	stream.Stop()
} 

func demodulate(message []float64) BitString {
	return BitString{}
}

func chk(err error) {
	if err != nil {
		panic(err)
	}
}
