// Reference:
// https://github.com/gordonklaus/portaudio/blob/master/examples/record.go
// https://github.com/gordonklaus/portaudio/blob/master/examples/mp3.go

package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/gordonklaus/portaudio"
)

func chk(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
  portaudio.Initialize()
  defer portaudio.Terminate()

  sample_rate := 44100.0 // this is the frame one sec for a 44100 hz sampled input
  record_duration := 10.0 // we record for 10s
  frame_per_buf := int(math.Round(sample_rate * record_duration))+1
  recorded := make([]int32, frame_per_buf)

  input_stream, err := portaudio.OpenDefaultStream(1, 0, sample_rate, frame_per_buf, recorded)
  chk(err)
  defer input_stream.Close()

  fmt.Printf("Press any key to start recording for %f secs", record_duration)
  bufio.NewReader(os.Stdin).ReadBytes('\n') 

  chk(input_stream.Start())
  chk(input_stream.Read())
  chk(input_stream.Stop())

  fmt.Printf("Finish recording, press any key to play back")
  bufio.NewReader(os.Stdin).ReadBytes('\n') 

  output_stream, err := portaudio.OpenDefaultStream(0, 1, sample_rate, frame_per_buf, &recorded)
  chk(err)
  defer output_stream.Close()

  chk(output_stream.Start())
  defer output_stream.Stop()

  output_stream.Write()
  time.Sleep(time.Duration(record_duration) * time.Second)

  fmt.Printf("Done playing audio, quiting")
}
