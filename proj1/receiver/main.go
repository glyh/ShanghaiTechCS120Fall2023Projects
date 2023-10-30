// This example simply captures data from your default microphone until you press Enter, after which it plays back the captured audio.
package main

import (
	"fmt"
	"math"
	"time"

	"github.com/gen2brain/malgo"
  "github.com/mjibson/go-dsp/fft"
)

const sampleRate = 44100
const preamble_duration = 1000 * time.Millisecond
const preamble_start_freq = 1000.0
const preamble_final_freq = 8000.0

const slice_duration = 100 * time.Millisecond

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
		// rb.Write(pSample)

		// check whether we can start to work, the following conditions need to met: 
		// 1. we have enough samples to accept a preamble
		// 2. in the last slice the peak frequency is "around" 8000Hz
		// 3. the peak frequency in the last 4 slices follows the characteristic of the chirp signal
		if frameCountAll >= samples_required {
			slice_width := int(math.Ceil(slice_duration.Seconds() * sampleRate))
			for i := 0; i < 4; i++ {
				to_analyze := rb.CopyStrideRight(i * slice_width, slice_width)
				
				spectrum := fft.FFTReal(to_analyze)
				fmt.Printf("FFT output: %v\n", spectrum)
				// TODO: run FFT on to_analyze and calculate peak frequency, keep a few peak 
				// frequency and compare the similarity of the sequence to our preamble
			}
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
	if rb.tail == 0 && rb.head == 0 {
		rb.head++ // overwrites behavior
	}
}

func (rb * RingBuffer) CopyStrideRight(rbegin int , count int) []float64 {
	length := rb.tail - rb.head
	end := len(rb.inner)
	if rb.tail < rb.head {
		length = rb.tail + (end - rb.head)
	}
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
