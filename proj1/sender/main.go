// Reference: https://github.com/gordonklaus/portaudio/blob/master/examples/stereoSine.go

// Useful articles: http://www.sunshine2k.de/articles/coding/crc/understanding_crc.html#ch4

package main

import (
	"slices"
	"time"

	"github.com/ebitengine/oto/v3"

	"fmt"
	"math"
)

const modulate_duration = 700 * time.Millisecond
const modulate_low_freq = 500.0
const modulate_high_freq = 15000.0

const bit_per_sym = 10
const sym_num = 1 << bit_per_sym

const preamble_duration = 500 * time.Millisecond
const preamble_start_freq = 5000.0
const preamble_final_freq = 10000.0

type BitString = []int

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

	msg := read_bitstring("000000000000000000000000000000")
	modulate(c, msg, opts.SampleRate)
}

type DataSig struct {
	data BitString
	sym_mod [][]float32
	// high []float32
	offset int
	sampleRate int
}

func (c *DataSig) Read(buf []byte) (int, error) {
	// number of frame per single symbol
	frame_per_sym := len(c.sym_mod[0])
	// // how many symbols have we sent
	// symbol_sent := c.offset / frame_per_sym
	// // how many bits have we sent
	// bit_sent := symbol_sent * bit_per_sym
	// // if we already send enough symbol to represent all the data in c.data, terminate
	// if symbol_sent * bit_per_sym >= len(c.data) {
	// 	return 0, nil
	// }

	for buf_offset := 0; buf_offset < len(buf); buf_offset += 4 {
		symbol_sent := c.offset / frame_per_sym
		symbol_frame_id := c.offset % frame_per_sym
		if symbol_sent >= len(c.data) {
			return buf_offset, nil
		}
		sym := c.data[symbol_sent]
		if symbol_frame_id == 0 {
			fmt.Printf("%d ", sym)
		}
		cur_f := c.sym_mod[sym][symbol_frame_id]
		bs := math.Float32bits(cur_f)

		buf[buf_offset] = byte(bs)
		buf[buf_offset+1] = byte(bs>>8)
		buf[buf_offset+2] = byte(bs>>16)
		buf[buf_offset+3] = byte(bs>>24)
		c.offset += 1
	}
	return len(buf) / 4 * 4, nil
}

// uses a linear chirp here
// f(t) = sin(2pi ((c / 2)t^2 + f0t) )
type PreambleSig struct {
	offset int
	sampleRate int
}

func (p *PreambleSig) Read(buf []byte) (int, error) {
	chirp_rate := (preamble_final_freq - preamble_start_freq) / preamble_duration.Seconds()
	fs := float64(p.sampleRate)
	length := int(fs * preamble_duration.Seconds())
	for i := 0; i < len(buf) / 4; i++ {
		if p.offset >= length {
			return i * 4, nil
		}
		f := float32(math.Sin(2 * math.Pi * (
			chirp_rate / 2.0 / fs / fs * float64(p.offset * p.offset) + 
			preamble_start_freq / fs * float64(p.offset))))
		bs := math.Float32bits(f)
		buf_offset := 4 * i
		buf[buf_offset] = byte(bs)
	  buf[buf_offset+1] = byte(bs>>8)
	  buf[buf_offset+2] = byte(bs>>16)
	  buf[buf_offset+3] = byte(bs>>24)
		p.offset += 1
	}
	return len(buf) / 4 * 4, nil
}

func modulate_syms(sampleRate float64) [][]float32 {
	sampleNum := int(math.Ceil(modulate_duration.Seconds() * sampleRate))
	ret := make([][]float32, sym_num)
	for i := 0; i < sym_num; i++ {
			ret[i] = make([]float32, sampleNum)
			ratio := float64(i) / float64(sym_num)
			freq := modulate_low_freq * ratio + modulate_high_freq * (1 - ratio) 
			// fmt.Printf("frequency for %d is %f\n", i, freq)
			phase := 0.0
			phaseDelta := 2 * math.Pi / sampleRate
			
			for j := 0; j < sampleNum; j++ {
				ret[i][j] = float32(math.Sin(freq * phase))
				phase = math.Mod(phase + phaseDelta, 2 * math.Pi)
			} 
	}
	return ret
}

func encode_int(l int) BitString {
	output := BitString{}
	mask := (1 << bit_per_sym) - 1
	for l != 0 {
		last_bit := l & mask
		l = l >> bit_per_sym
		output = append(output, last_bit)
	}
	slices.Reverse(output)
	return output
}


const len_hash = 4
func calculate_hash(msg BitString) BitString {
	ret := 0
	for i, v := range(msg) {
		ret ^= i * v
	}
	return pad_bitstring(len_hash, encode_int(ret))
}

// const crc_length = 16
// func calculate_crc(_msg BitString) BitString {
// 	// append crc_length 0s after the end of the message
// 	length := len(_msg)
// 	crc := 0
// 	msg := make(BitString, length + crc_length)
// 	copy(msg[0:length], _msg)
// 	for i := 0; i < length; i++ {
// 		crc <<= 1
// 		if msg[i] == 1 {
// 			crc |= 1
// 		  // CRC-16/AUG-CCITT
// 			msg[i] ^= 1
// 			crc_coeffs := []int{12, 5, 0}
// 			for _, offset := range(crc_coeffs) {
// 				msg[i+crc_length-offset] ^= 1
// 			}
// 		}
// 	}
// 	return pad_bitstring(crc_length, encode_int(crc))
// }

func pad_bitstring(length int, msg BitString) BitString {
	if len(msg) > length {
		panic("input bitstring too long")
	}
	return append(make(BitString, length - len(msg)), msg...)
}

func convert_base(message BitString, bit_per_sym int) BitString {
	out := BitString{}
	for i := 0; i < len(message); i += bit_per_sym {
		cur := 0
		for j := 0; j < bit_per_sym; j++ {
			cur = cur << 1
			if i + j < len(message) {
				cur |= message[i + j]
			}
		}
		out = append(out, cur)
	}
	return out
}

func modulate(c *oto.Context, message BitString, sampleRate int) {

	fmt.Printf("Trying to modulate %v\n", message)
	message = convert_base(message, bit_per_sym)
	fmt.Printf("We got %v after converting to 2^%d base", message, bit_per_sym)

	fmt.Println("Sending preamble")
	preamble_sig := c.NewPlayer(&PreambleSig{
		offset: 0,
		sampleRate: sampleRate })
	preamble_sig.Play()
	time.Sleep(preamble_duration)

	modulated_syms := modulate_syms(float64(sampleRate)) 

	hash := calculate_hash(message)
	fmt.Printf("Calculating Hash, got %v\n", hash)

	length := len(message) + len(hash) // CRC is of fixed length so we're safe to do this
	length_encoded := encode_int(length)

	// we fix the length to 16bit
	if len(length_encoded) > 4 {
		panic("message too long")
	}
	length_encoded = pad_bitstring(4, length_encoded)

	fmt.Printf("Encoding length(data+CRC): %d, encoded as %v\n", length, length_encoded)
	fmt.Printf("Trying to modulate %v and prepend CRC\n", message)

	output := append(length_encoded, hash...)
	output = append(output, message...)

	fmt.Printf("Got whole packet %v\n", output)
	// output = do_4b5b(output)
	// fmt.Printf("4B5B encoded as %v\n", output)

	data_sig := c.NewPlayer(&DataSig{data: output, sym_mod: modulated_syms, sampleRate: sampleRate})
	data_sig.Play()
	time.Sleep(time.Duration(math.Ceil(float64(len(output)) / float64(bit_per_sym)) + 1.0) * modulate_duration)

	fmt.Println("Message successfully modulated and played")
} 

func demodulate(message []float64) BitString {
	return BitString{}
}

func chk(err error) {
	if err != nil {
		panic(err)
	}
}

// func do_4b5b(message BitString) BitString {
// 	// this is a modified 4B5B where we swap code of 0000 and 0011
// 	// since length are very likely to send a zero in the beginning avoiding a bunch of continous one/zero are better
// 	map_4b5b := map[byte][]byte {
// 		0b0011 : {1, 1, 1, 1, 0},
// 		0b0001 : {0, 1, 0, 0, 1},
// 		0b0010 : {1, 0, 1, 0, 0},
// 		0b0000 : {1, 0, 1, 0, 1},
// 		0b0100 : {0, 1, 0, 1, 0},
// 		0b0101 : {0, 1, 0, 1, 1},
// 		0b0110 : {0, 1, 1, 1, 0},
// 		0b0111 : {0, 1, 1, 1, 1},
// 		0b1000 : {1, 0, 0, 1, 0},
// 		0b1001 : {1, 0, 0, 1, 1},
// 		0b1010 : {1, 0, 1, 1, 0},
// 		0b1011 : {1, 0, 1, 1, 1},
// 		0b1100 : {1, 1, 0, 1, 0},
// 		0b1101 : {1, 1, 0, 1, 1},
// 		0b1110 : {1, 1, 1, 0, 0},
// 		0b1111 : {1, 1, 1, 0, 1},
// 	}
//
// 	groups := len(message) / 4
// 	if len(message) % 4 != 0 {
// 		groups += 1
// 	}
//
// 	len_out := groups * 5
// 	out := make(BitString, len_out)
// 	for i, k := 0, 0; i < len(message); i += 4 {
// 		var code byte
// 		code = 0
// 		for j := 0; j < 4; j++ {
// 			if i + j >= len(message) {
// 				code = code << 1
// 			} else {
// 				code = code << 1 | message[i + j]
// 			}
// 		}
// 		mapped := map_4b5b[code]
// 		copy(out[k:k+5], mapped)
// 		k += 5
// 	}
// 	return out
// }
