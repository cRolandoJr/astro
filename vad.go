package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
)

const (
	sampleRate  = 16000
	frameMs     = 30
	samples     = (sampleRate * frameMs) / 1000
	frameBytes  = samples * 2 // 16-bit mono
	preFrames   = 10          // 300ms pre-roll to avoid chopping first syllable
)

type vadState int

const (
	stateSilence vadState = iota
	stateSpeech
	stateTrail
)

// captureAdaptiveVAD grabs audio from microphone and applies adaptive RMS-based VAD.
func captureAdaptiveVAD(wavPath string, maxSecs int, trailSec float64) error {
	if trailSec <= 0 {
		trailSec = 1.5
	}
	trailFrames := int((trailSec * 1000) / frameMs)

	cmd := exec.Command("rec", "-q", "-c", "1", "-r", "16000", "-t", "raw", "-e", "signed-integer", "-b", "16", "-")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("error opening stdout of rec: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error starting rec: %w", err)
	}

	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()

	var recordedFrames [][]byte
	var preRoll [][]byte

	state := stateSilence
	var trailCount int
	var framesCount int
	maxFrames := (maxSecs * 1000) / frameMs

	noiseFloor := 10000.0
	thresholdFactor := 2.5

	buf := make([]byte, frameBytes)
	for {
		n, err := io.ReadFull(stdout, buf)
		if n < frameBytes {
			if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
				return fmt.Errorf("error reading PCM: %w", err)
			}
			break
		}
		framesCount++

		rms := calcRMS(buf)
		
		if rms > 0 {
			if rms < noiseFloor {
				noiseFloor = rms
			} else {
				// slow adaptation upwards
				noiseFloor += (rms - noiseFloor) * 0.001
			}
		}

		threshold := noiseFloor * thresholdFactor
		if threshold < 100 {
			threshold = 100
		}

		isSpeech := rms > threshold

		switch state {
		case stateSilence:
			if isSpeech {
				state = stateSpeech
				recordedFrames = append(recordedFrames, preRoll...)
				recordedFrames = append(recordedFrames, clone(buf))
			} else {
				preRoll = append(preRoll, clone(buf))
				if len(preRoll) > preFrames {
					preRoll = preRoll[1:]
				}
			}
		case stateSpeech:
			recordedFrames = append(recordedFrames, clone(buf))
			if !isSpeech {
				state = stateTrail
				trailCount = 1
			}
		case stateTrail:
			recordedFrames = append(recordedFrames, clone(buf))
			if isSpeech {
				state = stateSpeech
			} else {
				trailCount++
				if trailCount >= trailFrames {
					goto Done
				}
			}
		}

		if framesCount >= maxFrames {
			goto Done
		}
	}

Done:
	if len(recordedFrames) == 0 {
		return fmt.Errorf("timeout") // Special error handled by input.go
	}
	return writeWAV(wavPath, recordedFrames)
}

func calcRMS(buf []byte) float64 {
	var sum float64
	count := len(buf) / 2
	for i := 0; i < len(buf); i += 2 {
		val := int16(binary.LittleEndian.Uint16(buf[i : i+2]))
		sum += float64(val) * float64(val)
	}
	return math.Sqrt(sum / float64(count))
}

func clone(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}

func writeWAV(path string, frames [][]byte) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var dataLen uint32
	for _, fr := range frames {
		dataLen += uint32(len(fr))
	}
	fileSize := dataLen + 36

	header := make([]byte, 44)
	copy(header[0:4], "RIFF")
	binary.LittleEndian.PutUint32(header[4:8], fileSize)
	copy(header[8:12], "WAVE")
	copy(header[12:16], "fmt ")
	binary.LittleEndian.PutUint32(header[16:20], 16)
	binary.LittleEndian.PutUint16(header[20:22], 1)
	binary.LittleEndian.PutUint16(header[22:24], 1)
	binary.LittleEndian.PutUint32(header[24:28], 16000)
	binary.LittleEndian.PutUint32(header[28:32], 16000*2)
	binary.LittleEndian.PutUint16(header[32:34], 2)
	binary.LittleEndian.PutUint16(header[34:36], 16)
	copy(header[36:40], "data")
	binary.LittleEndian.PutUint32(header[40:44], dataLen)

	if _, err := f.Write(header); err != nil {
		return err
	}

	for _, fr := range frames {
		if _, err := f.Write(fr); err != nil {
			return err
		}
	}
	return nil
}
