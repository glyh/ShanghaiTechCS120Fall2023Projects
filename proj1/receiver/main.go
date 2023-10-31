// This example simply captures data from your default microphone until you press Enter, after which it plays back the captured audio.
package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/cmplx"
	"time"

	"github.com/gen2brain/malgo"
        "github.com/mjibson/go-dsp/fft"
)

const shift_duration = 100 * time.Millisecond
const modulate_duration_gap = 200 * time.Millisecond

const modulate_duration = 700 * time.Millisecond
const modulate_low_freq = 2000.0
const modulate_high_freq = 20000.0

const bit_per_sym = 5
const sym_num = 1 << bit_per_sym

const sampleRate = 44100.0
const preamble_duration = 500 * time.Millisecond
// in general we want higher preamble freqency to distinguish from the noises
const preamble_start_freq = 5000.0
const preamble_final_freq = 10000.0

const slice_duration = 30 * time.Millisecond
const slice_inner_duration = 13 * time.Millisecond
const slice_num = 8
// the issue with this is: the bigger this is we can tollerant weaker signals but the possibility 
// of misidentification increases
const cutoff_variance_preamble = 1e7

// const self_correction_after_sym = 8 // do a self correction every 8 symbols

type BitString = []byte

func sig_to_energy_at_freq(to_analyze []float64) []float64 {
	spectrum := fft.FFTReal(to_analyze)
	
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

func get_energy_by_sym(energy []float64, sym byte, fs float64, L int) float64 {
	ratio := float64(sym) / float64(sym_num)
	freq_expect := ratio * modulate_low_freq + (1 - ratio) * modulate_high_freq
	return energy[int(freq_expect * float64(L) / fs)]
}

func main() {
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
	samples_required = max(samples_required, int(math.Ceil(2 * modulate_duration.Seconds() * sampleRate)))
	rb := newRb(samples_required * 5)

	chirp_rate := (preamble_final_freq - preamble_start_freq) / preamble_duration.Seconds()

	frameCountAll := 0
	is_idle := true
	received := BitString{}

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
					// if i == 0 && math.Abs(avg_freq - max_energy_freq) < slice_duration.Seconds() * chirp_rate {
					// 	freq_shift = max_energy_freq - freq_shift
					// }
					// fmt.Printf("We expect frequency %f\n", avg_freq)
					delta := avg_freq - max_energy_freq + freq_shift
					variance += delta * delta 
				}
				if variance < cutoff_variance_preamble {
					fmt.Println("Preamble detected!")
					fmt.Printf("Receiving: ")
					is_idle = false
					frameCountAll = 0
				}
				fmt.Println("We have a total variance of %lf", variance)
			}
			// we reset and start to count frames we have
		} else {
			// we're in working mode
			bits_read := len(received)
			modulated_width := int(math.Ceil(modulate_duration.Seconds() * sampleRate))
			bits_expected := frameCountAll / modulated_width
			left_over := frameCountAll % modulated_width
			gap_width := int(math.Ceil(modulate_duration_gap.Seconds() * sampleRate))
			for ; bits_read < bits_expected; bits_read++ {
				// leave slice_width empty so we're more likely get a good result from fourier transform
				to_analyze := rb.CopyStrideRight(gap_width + left_over + (bits_expected - bits_read - 1) * modulated_width, modulated_width - 2 * gap_width)
				L := len(to_analyze)
				energy_cur := sig_to_energy_at_freq(to_analyze)

				// energy[i] correponds to frequency Fs * i/L
				// let Fs * i/L = zero_freq, then i = zero_freq / Fs * L
				max_energy_freq := sampleRate * float64(arg_max(energy_cur)) / float64(L)
				// say max_energy_freq = ratio * modulate_low_freq + (1 - ratio) * modulate_high_freq
				// then ratio = (modulate_high_freq - max_energy_freq) / (modulate_high_freq - modulate_low_freq)
				ratio := (modulate_high_freq - max_energy_freq) / (modulate_high_freq - modulate_low_freq)
				// we know that ratio = i / sym_num

				sym := int(math.Round(ratio * sym_num))
				received = append(received, byte(sym))
				fmt.Printf("%d ", sym)

				if false {
				// if len(received) >= 2 {

					cur_sym := received[len(received) - 1]
					last_sym := received[len(received) - 2]
					if cur_sym != last_sym {
						shift_width := int(math.Ceil(shift_duration.Seconds() * sampleRate))
						// we got two different adjacent signal hence we can do correction here.
						// note we count from right to left
						current_end := (left_over + (bits_expected - bits_read - 1) * modulated_width)
						last_end := current_end + modulated_width
						energy_last := sig_to_energy_at_freq(rb.CopyStrideRight(last_end, modulated_width))


						last_shift_right_end := last_end - shift_width
						energy_last_shift_right := sig_to_energy_at_freq(rb.CopyStrideRight(last_shift_right_end, modulated_width))
						current_shift_left_end := current_end + shift_width
						energy_current_shift_left := sig_to_energy_at_freq(rb.CopyStrideRight(current_shift_left_end, modulated_width))

						energy_cur_at_freq := get_energy_by_sym(energy_cur, cur_sym, sampleRate, L)
						energy_cur_shift_left_at_freq := get_energy_by_sym(energy_current_shift_left, cur_sym, sampleRate, L)
						energy_last_at_freq := get_energy_by_sym(energy_last, last_sym, sampleRate, L)
						energy_last_shift_right_at_freq := get_energy_by_sym(energy_last_shift_right, last_sym, sampleRate, L)
						// fmt.Printf("(%f, %f) ", (energy_cur_at_freq - energy_cur_shift_left_at_freq)/ energy_cur_at_freq, (energy_last_at_freq - energy_last_shift_right_at_freq) / energy_last_at_freq)
						if energy_cur_shift_left_at_freq > energy_cur_at_freq && energy_last_shift_right_at_freq < energy_last_at_freq {
							// we should shift left
							// simulated by adding some frames at the very beginning
							frameCountAll += shift_width
							fmt.Printf("← ")
						} else if energy_cur_shift_left_at_freq < energy_cur_at_freq && energy_last_shift_right_at_freq > energy_last_at_freq{
							// we should shift right
							// simulated by removing some frames at the very beginning
							frameCountAll -= shift_width
							fmt.Printf("→ ")
						}
					}

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
	// fmt.Printf("we have (head, tail) = (%d, %d) data in rb of length %d\n", rb.head, rb.tail, len(rb.inner))
	// fmt.Printf("(%d, %d, %d) ", rb.head, rb.tail, len(rb.inner))
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
