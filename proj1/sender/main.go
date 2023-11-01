// Reference: https://github.com/gordonklaus/portaudio/blob/master/examples/stereoSine.go

// Useful articles: http://www.sunshine2k.de/articles/coding/crc/understanding_crc.html#ch4

package main

import (
	"slices"
	"time"

	"github.com/ebitengine/oto/v3"

	"fmt"
	"math"
	"math/big"
	"math/rand"
	"os"
)
const mod_duration = 500 * time.Millisecond
const mod_low_freq = 1000.0
const mod_high_freq = 17000.0
const mod_width = mod_high_freq - mod_low_freq
const mod_freq_step = 50.0
// const mod_freq_range_num = 80
const mod_freq_range_num = 2
const mod_freq_range_width = mod_width / mod_freq_range_num
var freq_diff_lower_bound float64

var mod_state_num uint64
var sym_size uint64

var bit_per_sym int

// the finest difference we can tell with sample rate fs is fs/L where L is the length of the signal(L = t * fs), thus to differentiate by 20hz, 1/t = 20hz, t = 1/20s = 500ms

const preamble_duration = 500 * time.Millisecond
const preamble_start_freq = 100.0
const preamble_final_freq = 500.0

const len_length = 2

type BitString = []*big.Int

func read_bitstring(s string) BitString {
	out := make(BitString, len(s))
	for i, v := range(s) { 
		bit := int64(0)
		if v == '1' {
			bit = 1
		} 
		out[i] = big.NewInt(bit)
	}
	return out
}

func random_bit_string_of_length(l int) BitString {
	out := make(BitString, l)
	for i := 0; i < len(out); i++ {
		out[i] = big.NewInt(rand.Int63n(2))
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

	msg := random_bit_string_of_length(10000)
	modulate(c, msg, opts.SampleRate)
}

type DataSig struct {
	data BitString
	// sym_mod [][]float32
	// high []float32
	offset int
	sampleRate int
}

func (c *DataSig) Read(buf []byte) (int, error) {
	// number of frame per single symbol
	frame_per_sym := int(math.Ceil(float64(c.sampleRate) * mod_duration.Seconds()))

	mod_state_num_b := big.NewInt(int64(mod_state_num))

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
		phase := math.Mod(2 * math.Pi * float64(symbol_frame_id) / float64(c.sampleRate), 2 * math.Pi)
		cur_f := 0.0
		index_at_range_k_b := big.NewInt(0)
		for k := 0; k < mod_freq_range_num; k++ {
			// index_at_range_k := (sym >> (3 * k)) & ((1 << 3) - 1)
			// index_at_range_k := sym % mod_state_num
			sym.DivMod(sym, mod_state_num_b, index_at_range_k_b)
			index_at_range_k, _ := index_at_range_k_b.Float64()
			freq_at_range_k := index_at_range_k * mod_freq_step + mod_low_freq + mod_freq_step * float64(k)
			if symbol_frame_id == 0 {
				fmt.Printf("(%f) ", freq_at_range_k)
			}
			cur_f += math.Sin(freq_at_range_k * phase)
			// sym /= mod_state_num
		}
		// c.sym_mod[sym][symbol_frame_id]
		bs := math.Float32bits(float32(cur_f))

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
	chirp_rate := (float64(preamble_final_freq) - preamble_start_freq) / preamble_duration.Seconds()
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

func encode_int(_l int64) BitString {
	l := big.NewInt(_l)

	output := BitString{}
	mask := big.NewInt(1)
	mask.Lsh(mask, uint(bit_per_sym))
	mask.Sub(mask, big.NewInt(1))
	for !(l.IsInt64() && l.Int64() == 0) {
		last_bit := big.NewInt(0)
		last_bit.And(l, mask)
		l.Rsh(l, uint(bit_per_sym))
		output = append(output, last_bit)
	}
	slices.Reverse(output)
	return output
}

const len_hash = 4
func calculate_hash(msg BitString) *big.Int {
	ret := big.NewInt(0)
	for i, v := range(msg) {
		prod := big.NewInt(int64(i))
		prod.Mul(prod, v)
		ret.Xor(ret, v)
	}
	return ret
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
// 			crc_coeffs := []{12, 5, 0}
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
	zeros := make(BitString, length - len(msg))
	for i, _ := range(zeros) {
		zeros[i] = big.NewInt(0)
	}
	return append(zeros, msg...)
}

func convert_base(message BitString, bit_per_sym int) BitString {
	out := BitString{}
	for i := uint64(0); i < uint64(len(message)); i += uint64(bit_per_sym) {
		fmt.Printf("[%d]:[%d!]", i, bit_per_sym)
		cur := big.NewInt(0)
		for j := 0; j < bit_per_sym; j++ {
			cur.Lsh(cur, 1)
			if i + uint64(j) < uint64(len(message)) {
				cur.Or(cur, message[int(i)+j])
			}
		}
		out = append(out, cur)
	}
	return out
}

func modulate(c *oto.Context, message BitString, sampleRate int) {

	f := mod_freq_range_width / mod_freq_step
	mod_state_num = uint64(f)
	sym_size = 1
	for i := 0; i < mod_freq_range_num; i++ {
		sym_size = sym_size * mod_state_num
		// fmt.Printf("%d.", sym_size)
	}
	// fmt.Println("")
	bit_per_sym = int(math.Log2(float64(sym_size))) - 1
	freq_diff_lower_bound = 1.0 / mod_duration.Seconds()
	if freq_diff_lower_bound > mod_freq_step {
		fmt.Printf("Frequency difference(%f) for modulation is too small compare to the lower limit %f\n", mod_freq_step, freq_diff_lower_bound)
		os.Exit(1)
	}

	fmt.Printf("We're spliting frequency domain [%f %f] into %d pieces, where each piece is of width %f\n", mod_low_freq, mod_high_freq, mod_freq_range_num, mod_freq_range_width)
	fmt.Printf("Inside these pieces, there's %d states where each state is %f Hz apart\n", mod_state_num, mod_freq_step)

	sym_size_b := big.NewInt(1)
	mod_state_num_b := big.NewInt(int64(mod_state_num))
	for i := 0; i < mod_freq_range_num; i++ {
		sym_size_b.Mul(sym_size_b, mod_state_num_b)
	}
	bit_per_sym_b := sym_size_b.BitLen() - 1
	fmt.Printf("Rounding down the symbol set from %d(%d) to contain 2^%d symbols for simplicity\n", sym_size, sym_size_b, bit_per_sym_b)
	// os.Exit(0)

	// fmt.Printf("Trying to modulate %v\n", message)
	modulo := len(message) % bit_per_sym
	message = convert_base(message, bit_per_sym)

	fmt.Println("Sending preamble")
	preamble_sig := c.NewPlayer(&PreambleSig{
		offset: 0,
		sampleRate: sampleRate })
	preamble_sig.Play()
	time.Sleep(preamble_duration)

	// modulated_syms := modulate_syms(float64(sampleRate)) 

	hash := calculate_hash(message)
	fmt.Printf("Calculating Hash, got %v\n", hash)

	length := len(message) + 1 + 1
	// the 1 above is for module
	length_encoded := encode_int(int64(length))

	// we fix the length to 16bit
	if len(length_encoded) > len_length {
		panic("message too long")
	}
	length_encoded = pad_bitstring(len_length, length_encoded)

	fmt.Printf("Encoding length(data+hash): %d, encoded as %v\n", length, length_encoded)
	// fmt.Printf("Trying to modulate %v and prepend CRC\n", message)

	output := append(length_encoded, big.NewInt(int64(modulo)))
	output = append(output, hash)
	output = append(output, message...)

	fmt.Printf("Got whole packet %v\n", output)
	// output = do_4b5b(output)
	// fmt.Printf("4B5B encoded as %v\n", output)

	data_sig := c.NewPlayer(&DataSig{data: output, sampleRate: sampleRate})
	data_sig.Play()
	time.Sleep(time.Duration(math.Ceil(float64(len(output))) + 1.0) * mod_duration)

	fmt.Println("Message successfully modulated and played")
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
