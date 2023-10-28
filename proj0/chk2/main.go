// https://github.com/ebitengine/oto
package main

import (
    "bytes"

    "github.com/ebitengine/oto/v3"
    "github.com/hajimehoshi/go-mp3"

        "bufio"
        "fmt"
        "math"
        "os"
        "time"

        "github.com/gordonklaus/portaudio"
)

func main() {

  portaudio.Initialize()
  defer portaudio.Terminate()

  record_duration := 10.0 // we record for 10s

    // Read the mp3 file into memory
    fileBytes, err := os.ReadFile("./mu.mp3")
    if err != nil {
        panic("reading my-file.mp3 failed: " + err.Error())
    }

    // Convert the pure bytes into a reader object that can be used with the mp3 decoder
    fileBytesReader := bytes.NewReader(fileBytes)

    // Decode file
    decodedMp3, err := mp3.NewDecoder(fileBytesReader)
    if err != nil {
        panic("mp3.NewDecoder failed: " + err.Error())
    }

    // Prepare an Oto context (this will use your default audio device) that will
    // play all our sounds. Its configuration can't be changed later.

    op := &oto.NewContextOptions{}

    // Usually 44100 or 48000. Other values might cause distortions in Oto
    op.SampleRate = 44100

    // Number of channels (aka locations) to play sounds from. Either 1 or 2.
    // 1 is mono sound, and 2 is stereo (most speakers are stereo). 
    op.ChannelCount = 2

    // Format of the source. go-mp3's format is signed 16bit integers.
    op.Format = oto.FormatSignedInt16LE

    // Remember that you should **not** create more than one context
    otoCtx, readyChan, err := oto.NewContext(op)
    if err != nil {
        panic("oto.NewContext failed: " + err.Error())
    }
    // It might take a bit for the hardware audio devices to be ready, so we wait on the channel.
    <-readyChan

    // Create a new 'player' that will handle our sound. Paused by default.
    player := otoCtx.NewPlayer(decodedMp3)
    

  fmt.Printf("Press any key to start recording for %f secs", record_duration)
  bufio.NewReader(os.Stdin).ReadBytes('\n') 

    // Play starts playing the sound and returns without waiting for it (Play() is async).
    player.Play()


  sample_rate := 44100.0 // this is the frame one sec for a 44100 hz sampled input
  frame_per_buf := int(math.Round(sample_rate * record_duration))+1
  recorded := make([]int32, frame_per_buf)

  input_stream, err := portaudio.OpenDefaultStream(1, 0, sample_rate, frame_per_buf, recorded)
  chk(err)
  defer input_stream.Close()

  chk(input_stream.Start())
  chk(input_stream.Read())
  chk(input_stream.Stop())

  err = player.Close()
  if err != nil {
      panic("player.Close failed: " + err.Error())
  }
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

func chk(err error) {
	if err != nil {
		panic(err)
	}
}

