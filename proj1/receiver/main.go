// This example simply captures data from your default microphone until you press Enter, after which it plays back the captured audio.
package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
	"math/cmplx"
	"slices"
	"time"

	"os"

	"github.com/gen2brain/malgo"
	"github.com/mjibson/go-dsp/fft"
)


const mod_duration = 800 * time.Millisecond
const mod_low_freq = 1000.0
const mod_high_freq = 17000.0
const mod_width = mod_high_freq - mod_low_freq
const mod_freq_step = 140.0
const mod_freq_range_num = 10
const mod_freq_range_width = mod_width / mod_freq_range_num
const gap_freq = mod_freq_step / 2

const mod_duration_gap = 20 * time.Millisecond
// const mod_freq_range_num = 80
var mod_state_num *big.Int
var sym_size *big.Int

var bit_per_sym int

const shift_duration = 120 * time.Millisecond

const len_length = 2

func pad_bitstring(length int, msg BitString) BitString {
	if len(msg) > length {
		panic("input bitstring too long")
	}
	return append(make(BitString, length - len(msg)), msg...)
}

func calculate_hash(msg BitString) *big.Int {
	ret := big.NewInt(0)
	for i, v := range(msg) {
		prod := big.NewInt(int64(i))
		prod.Mul(prod, v)
		ret.Xor(ret, v)
	}
	return ret
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

const sampleRate = 44100.0

const sleep_duration = 500 * time.Millisecond
const preamble_duration = 800 * time.Millisecond
// in general we want higher preamble freqency to distinguish from the noises
const preamble_start_freq = 1000.0
const preamble_final_freq = 5000.0

const slice_duration = 40 * time.Millisecond
const slice_inner_duration = 10 * time.Millisecond
const slice_num = 10
// the issue with this is: the bigger this is we can tollerant weaker signals but the possibility 
// of misidentification increases
const cutoff_variance_preamble = 0.1

// const self_correction_after_sym = 8 // do a self correction every 8 symbols

type BitString = []*big.Int


// is_idle := true
var is_idle bool
func sig_to_energy_at_freq(to_analyze []float64) []float64 {
	spectrum := fft.FFTReal(to_analyze)
	// if !is_idle {
	// 	fmt.Printf("%v", to_analyze)
	// 	fmt.Printf("%v", spectrum)
	// }
	
	L := len(to_analyze)

	energy := make([]float64, L/2+1)
	energy[0] = cmplx.Abs(spectrum[0]) / float64(L)
	for i := 1; i < L / 2; i += 1 {
		energy[i] = 2 * cmplx.Abs(spectrum[i]) / float64(L)
	}
	return energy
}

func arg_max(s []float64) int {
	ans := -1
	val := 0.0
	for i, v := range(s) {
		if ans == -1 || v > val {
			ans, val = i, v
		}
	}
	return ans
}

// func get_energy_by_sym(energy []float64, sym int, fs float64, L int) float64 {
// 	ratio := float64(sym) / float64(sym_num)
// 	freq_expect := ratio * modulate_low_freq + (1 - ratio) * modulate_high_freq
// 	return energy[int(freq_expect * float64(L) / fs)]
// }

func main() {
	f := mod_freq_range_width / mod_freq_step
	mod_state_num = big.NewInt(int64(f))
	sym_size = big.NewInt(1)
	for i := 0; i < mod_freq_range_num; i++ {
		sym_size.Mul(sym_size, mod_state_num)
	}
	bit_per_sym = sym_size.BitLen() - 1

	is_idle = true
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, func(message string) {
		// fmt.Printf("LOG <%v>\n", message)
	})
	chk(err)
	defer func() {
		_ = ctx.Uninit()
		ctx.Free()
	}()

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Duplex)
	deviceConfig.Capture.Format = malgo.FormatF32
	deviceConfig.Capture.Channels = 1
	deviceConfig.SampleRate = sampleRate
	deviceConfig.Alsa.NoMMap = 1

	data_width := int(malgo.SampleSizeInBytes(deviceConfig.Capture.Format))
	samples_required := int(math.Ceil(preamble_duration.Seconds() * sampleRate))
	samples_required = max(samples_required, int(math.Ceil(2 * mod_duration.Seconds() * sampleRate)))
	rb := newRb(samples_required * 10)

	chirp_rate := (preamble_final_freq - preamble_start_freq) / preamble_duration.Seconds()

	frameCountAll := 0
	received := BitString{}

	packet_length := 0
	// packet_rest := BitString{}
	do_offset := 0


	onRecvFrames := func(pSample2, pSample []byte, framecount uint32) {
		if(framecount > uint32(rb.Length())) {
			panic("ring buffer too small")
		}
		if len(pSample) % data_width != 0 {
			panic("weird input: sample bytes length not multiple of data width")
		}

		frameCountAll += int(framecount)
		for i := 0; i < len(pSample); i += data_width {
			bits := binary.LittleEndian.Uint32(pSample[i:i+4])
			f := math.Float32frombits(bits) 
			rb.Write(float64(f))
		}
		if is_idle {
			slice_width := int(math.Ceil(slice_duration.Seconds() * sampleRate))
			slice_inner_width := int(math.Ceil(slice_inner_duration.Seconds() * sampleRate))
			// check whether we can start to work, the following conditions need to met: 
			// 1. we have enough samples to accept a preamble
			// 2. in the last slice the peak frequency is "around" 8000Hz
			// 3. the peak frequency in the last slice_num slices follows the characteristic of the chirp signal
			if frameCountAll >= samples_required {
				variance := 0.0
				freq_shift := 0.0
				for i := 0; i < slice_num; i++ {
					to_analyze := rb.CopyStrideRight(i * slice_width + slice_inner_width, slice_width - 2 * slice_inner_width)

					energy := sig_to_energy_at_freq(to_analyze)
					L := len(to_analyze)

					max_energy_freq := sampleRate * float64(arg_max(energy)) / float64(L)

					avg_freq := preamble_final_freq - (float64(i) + 0.5) * slice_duration.Seconds() * chirp_rate
					if i == 0 && math.Abs(avg_freq - max_energy_freq) < slice_duration.Seconds() * chirp_rate {
						freq_shift = max_energy_freq - avg_freq
					}
					// fmt.Printf("We expect frequency %f\n", avg_freq)
					delta := (avg_freq - max_energy_freq + freq_shift) / avg_freq
					variance += delta * delta 
				}
				fmt.Printf("Variance: %f\n", variance)
				if variance < cutoff_variance_preamble {
					fmt.Println("Preamble detected!")
					fmt.Printf("Receiving: ")
					is_idle = false
					time_shift := freq_shift / chirp_rate
					frameCountAll = int(time_shift) * sampleRate
				}
			}
			// we reset and start to count frames we have
		} else {
			sleep_frames := int(math.Ceil(sleep_duration.Seconds() * float64(sampleRate)))
			if frameCountAll <= sleep_frames { return }
			// we're in working mode
			bits_read := len(received)
			modulated_width := int(math.Ceil(mod_duration.Seconds() * sampleRate))
			frameCountAllEffective := (frameCountAll - sleep_frames)
			bits_expected := frameCountAllEffective / modulated_width
			left_over := frameCountAllEffective % modulated_width
			gap_width := int(math.Ceil(mod_duration_gap.Seconds() * sampleRate))
			for ; bits_read < bits_expected; bits_read++ {
				// leave slice_width empty so we're more likely get a good result from fourier transform
				to_analyze := rb.CopyStrideRight(gap_width + left_over + (bits_expected - bits_read - 1) * modulated_width, modulated_width - 2 * gap_width)
				L := len(to_analyze)
				energy_cur := sig_to_energy_at_freq(to_analyze)
				// energy[i] correponds to frequency Fs * i/L

				sym := big.NewInt(0)
				start_freq := mod_high_freq - mod_freq_range_width - gap_freq
				// fmt.Printf("[")
				for k := mod_freq_range_num - 1; k >= 0; k-- {
					end_freq := start_freq + mod_freq_range_width
					// Fs * i_start / L = start_freq 
					i_start := int(start_freq * float64(L) / sampleRate)
					i_end := int(end_freq * float64(L) / sampleRate)
					// if k == 0 {
					// 	for j, v := range(energy_cur[i_start:i_end]) {
					// 		i := j + i_start
					// 		fmt.Printf("(%.2f,%.2f) ", sampleRate * float64(i) / float64(L), v)
					// 	}
					// }
					// fmt.Println("")
					max_energy_freq := sampleRate * float64(arg_max(energy_cur[i_start:i_end]) + i_start) / float64(L)
					// max_energy_freq = start_freq + part * 20
					part := int(math.Round((max_energy_freq - (start_freq + gap_freq)) / mod_freq_step))
					// fmt.Printf("In [%f, %f] we have max frequency of %f, interpreted as %d\n", start_freq, end_freq, max_energy_freq, part)
					// fmt.Printf("%.2f:%d ", max_energy_freq, part)
					fmt.Printf("[%f %f] -> %f %d\n", start_freq + gap_freq, end_freq + gap_freq, max_energy_freq, part)
					sym.Mul(sym, sym_size)
					sym.Add(sym, big.NewInt(int64(part)))
					start_freq -= mod_freq_range_width
				}	
				// fmt.Printf("]\n")

				// // let Fs * i/L = zero_freq, then i = zero_freq / Fs * L
				// max_energy_freq := sampleRate * float64(arg_max(energy_cur)) / float64(L)
				// // say max_energy_freq = ratio * modulate_low_freq + (1 - ratio) * modulate_high_freq
				// // then ratio = (modulate_high_freq - max_energy_freq) / (modulate_high_freq - modulate_low_freq)
				// ratio := (modulate_high_freq - max_energy_freq) / (modulate_high_freq - modulate_low_freq)
				// // we know that ratio = i / sym_num
				//
				// sym := int(math.Round(ratio * sym_num))
				received = append(received, sym)
				fmt.Printf("%d ", sym)
				// first bit of length must be 0 if we never sent data over length 10000
				if len(received) == do_offset + 1 && !(sym.IsInt64() && sym.Int64() == 0) {
					// magic
					do_offset += 1
					fmt.Printf("[discard] ")
				} else if len(received) - do_offset <= len_length {
					packet_length = (packet_length << bit_per_sym) | int(received[len(received)-1].Int64())
					if len(received) - do_offset == len_length && packet_length == 0 {
						do_offset += 1
						fmt.Printf("[ignore leading 0]")
					}
				// } else if len(received) - do_offset <= len_length + len_hash {
				// 	packet_hash = (packet_hash << bit_per_sym) | received[len(received)-1]
				} else if len(received) - do_offset == len_length + packet_length + 1 {
					// packet_rest = received[len_length + len_hash:]
					finale(packet_length, do_offset, received)
				}
			}
		}
	}

	fmt.Println("Waiting for sender to send data")
	captureCallbacks := malgo.DeviceCallbacks{
		Data: onRecvFrames,
	}
	device, err := malgo.InitDevice(ctx.Context, deviceConfig, captureCallbacks)
	defer device.Uninit()
	chk(err)

	err = device.Start()
	chk(err)

	fmt.Println("Press Enter to exit...")
	fmt.Scanln()                                   
}

func finale(packet_length int, do_offset int, received BitString){
	modulo := int(received[do_offset+len_length].Int64())
	packet_hash := received[do_offset+len_length+1:do_offset+len_length+1+1]
	packet_data := received[do_offset+len_length+1+1:]
	fmt.Printf("\nGot packet of length %d with modulo %d, hash %v, content %v", packet_length, modulo, packet_hash, packet_data)
	fmt.Printf("Writing packet to disk named received.txt")
	file, err := os.Create("received.txt")
	chk(err)
	defer file.Close()
	writer := bufio.NewWriter(file)
	defer writer.Flush()
	length_bin := bit_per_sym * len(packet_data) - (bit_per_sym - modulo)
	output := make([]byte, length_bin)
	for id, v := range(packet_data) {
		for i := 0; i < bit_per_sym; i++ {
			cur_offset := id * bit_per_sym + (bit_per_sym - 1 - i)
			if cur_offset >= len(output) {
				continue
			}
			output[cur_offset] = byte(v.Bit(i))
		}
	}
	for _, bit := range(output) {
		if bit == 0 {
			writer.WriteString("0")
		} else {
			writer.WriteString("1")
		}
	}
	os.Exit(0)
}

func chk(err error) {
	if err != nil {
		panic(err)
	}
}

type RingBuffer struct {
	head int
	tail int
	inner []float64
}

func newRb(size int) RingBuffer {
	return RingBuffer{
		head: 0,
		tail: 1,
		inner: make([]float64, size + 1),
	}
}

func (rb *RingBuffer) Length() int{
	return int(len(rb.inner))
}
func (rb *RingBuffer) Write(f float64) {
	rb.inner[rb.tail] = f
	rb.tail = (rb.tail + 1) % len(rb.inner)
	if rb.tail == rb.head {
		rb.head = (rb.head + 1) % len(rb.inner)
	}
}

func (rb * RingBuffer) CopyStrideRight(rbegin int , count int) []float64 {
	length := rb.tail - rb.head
	end := len(rb.inner)
	if rb.tail < rb.head {
		length = rb.tail + (end - rb.head)
	}
	// fmt.Printf("We have %d data in rb, at most %d data allowed.\n", length, len(rb.inner))
	if length < count + rbegin {
		panic("RB doesn't have enough data")
	}
	r_edge := rb.tail - rbegin
	if r_edge <= 0 {
		r_edge += end
	}
	result := make([]float64, count)
	if r_edge >= count {
		copy(result, rb.inner[r_edge-count:r_edge])
	} else {
		copy(result[count-r_edge:count], rb.inner[0:r_edge])
		copy(result[0:count-r_edge], rb.inner[end-(count-r_edge):end])
	}
	return result
}
