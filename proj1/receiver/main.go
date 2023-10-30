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

const sampleRate = 44100
const preamble_duration = 400 * time.Millisecond
const preamble_start_freq = 1000.0
const preamble_final_freq = 8000.0

const slice_duration = 50 * time.Millisecond
const slice_num = 8
const cutoff_variance_preamble = 1.4e7 

func main() {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, func(message string) {
		fmt.Printf("LOG <%v>\n", message)
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
	rb := newRb(samples_required * 2)

	chirp_rate := (preamble_final_freq - preamble_start_freq) / preamble_duration.Seconds()

	frameCountAll := 0
	onRecvFrames := func(pSample2, pSample []byte, framecount uint32) {
		if(framecount > uint32(rb.Length())) {
			panic("ring buffer too small")
		}
		if len(pSample) % data_width != 0 {
			panic("weird input: sample bytes length not multiple of data width")
		}

		frameCountAll += int(framecount)
		// TODO: fix this
		for i := 0; i < len(pSample); i += data_width {
			bits := binary.LittleEndian.Uint32(pSample[i:i+4])
			f := math.Float32frombits(bits) 
			rb.Write(float64(f))
		}

		// check whether we can start to work, the following conditions need to met: 
		// 1. we have enough samples to accept a preamble
		// 2. in the last slice the peak frequency is "around" 8000Hz
		// 3. the peak frequency in the last slice_num slices follows the characteristic of the chirp signal
		if frameCountAll >= samples_required {
			slice_width := int(math.Ceil(slice_duration.Seconds() * sampleRate))
			variance := 0.0
			for i := 0; i < slice_num; i++ {
				to_analyze := rb.CopyStrideRight(i * slice_width, slice_width)
				
				spectrum := fft.FFTReal(to_analyze)
				
				L := len(to_analyze)

				energy := make([]float64, L/2+1)
				energy[0] = cmplx.Abs(spectrum[0]) / float64(L)
				for i := 1; i < L / 2; i += 1 {
					energy[i] = 2 * cmplx.Abs(spectrum[i]) / float64(L)
				}

				var max_energy_freq float64
				var max_energy float64
				max_energy_freq = 0.0
				max_energy = 0
				for i := 0; i < L / 2 + 1; i++ {
					// energy[i] correponds to frequency Fs * i/L
					if energy[i] > max_energy {
						max_energy = energy[i]
						max_energy_freq = sampleRate * float64(i) / float64(L)
					}
				}
				// fmt.Printf("Around frequency %f we have max energy at slice[-%d]\n", max_energy_freq, i)

				avg_freq := preamble_final_freq - (float64(i) + 0.5) * slice_duration.Seconds() * chirp_rate
				// fmt.Printf("We expect frequency %f\n", avg_freq)
				delta := avg_freq - max_energy_freq
				variance += delta * delta 
			}
			if variance < cutoff_variance_preamble {
				fmt.Println("Preamble detected!")
			}
			// fmt.Println("We have a total variance of %f", variance)
		}
	}

	fmt.Println("Recording...")
	captureCallbacks := malgo.DeviceCallbacks{
		Data: onRecvFrames,
	}
	device, err := malgo.InitDevice(ctx.Context, deviceConfig, captureCallbacks)
	defer device.Uninit()
	chk(err)

	err = device.Start()
	chk(err)

	fmt.Println("Press Enter to stop recording...")
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
