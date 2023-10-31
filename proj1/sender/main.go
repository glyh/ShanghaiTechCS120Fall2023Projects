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

const modulate_duration = 400 * time.Millisecond
const delta = modulate_duration
const zero_freq = 1000.0
const one_freq = 4000.0

const preamble_duration = 500 * time.Millisecond
const preamble_start_freq = 5000.0
const preamble_final_freq = 10000.0

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

	msg := read_bitstring("010100111000")
	modulate(c, msg, opts.SampleRate)
}

type DataSig struct {
	data BitString
	low []float32
	high []float32
	offset int
	sampleRate int
}

func (c *DataSig) Read(buf []byte) (int, error) {
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

func encode_int(l int) BitString {
	output := BitString{}
	for l != 0 {
		last_bit := byte(l & 1)
		l = l >> 1
		output = append(output, last_bit)
	}
	slices.Reverse(output)
	return output

}

const crc_length = 16

func calculate_crc(_msg BitString) BitString {
	// TODO: Implement CRC
	// append crc_length 0s after the end of the message
	length := len(_msg)
	crc := 0
	msg := make(BitString, length + crc_length)
	copy(msg[0:length], _msg)
	for i := 0; i < length; i++ {
		crc <<= 1
		if msg[i] == 1 {
			crc |= 1
		  // CRC-16/AUG-CCITT
			msg[i] ^= 1
			crc_coeffs := []int{12, 5, 0}
			for _, offset := range(crc_coeffs) {
				msg[i+crc_length-offset] ^= 1
			}
		}
	}
	return pad_bitstring(crc_length, encode_int(crc))
}

func pad_bitstring(length int, msg BitString) BitString {
	if len(msg) > length {
		panic("input bitstring too long")
	}
	return append(make(BitString, length - len(msg)), msg...)
}

func modulate(c *oto.Context, message BitString, sampleRate int) {

	fmt.Printf("Trying to modulate %v\n", message)

	fmt.Println("Sending preamble")
	preamble_sig := c.NewPlayer(&PreambleSig{
		offset: 0,
		sampleRate: sampleRate })
	preamble_sig.Play()
	time.Sleep(preamble_duration)

	one_modulated := modulate_bit(true, float64(sampleRate))
	zero_modulated := modulate_bit(false, float64(sampleRate))

	crc := calculate_crc(message)
	fmt.Printf("Calculating CRC, got %v\n", crc)

	length := len(message) + len(crc) // CRC is of fixed length so we're safe to do this
	length_encoded := encode_int(length)

	// we fix the length to 16bit
	if len(length_encoded) > 16 {
		panic("message too long")
	}
	length_encoded = pad_bitstring(16, length_encoded)

	fmt.Printf("Encoding length(data+CRC): %d, encoded as %v\n", length, length_encoded)
	length_sig := c.NewPlayer(&DataSig{data: length_encoded, high: one_modulated, low: zero_modulated, sampleRate: sampleRate})
	length_sig.Play()
	time.Sleep(time.Duration(len(length_encoded)) * modulate_duration)

	// fmt.Printf("Sending separator sequence as silence\n")
	// time.Sleep(modulate_duration)

	fmt.Printf("Trying to modulate %v and prepend CRC\n", message)
	output := append(crc, message...)
	fmt.Printf("Got %v\n", output)
	data_sig := c.NewPlayer(&DataSig{data: output, high: one_modulated, low: zero_modulated, sampleRate: sampleRate})
	data_sig.Play()
	time.Sleep(time.Duration(len(message)) * modulate_duration + delta)

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
