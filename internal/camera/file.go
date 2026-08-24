package camera

import (
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
)

type FileCamera struct {
	path   string
	cmd    *exec.Cmd
	stdout io.ReadCloser
	mu     sync.Mutex
	frame  *image.RGBA
	width  int
	height int
	closed bool
	saved  bool
}

func probeVideo(filePath string) (int, int, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=width,height", "-of", "json", filePath)
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, err
	}
	var data struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &data); err != nil {
		return 0, 0, err
	}
	if len(data.Streams) == 0 {
		return 0, 0, nil
	}
	return data.Streams[0].Width, data.Streams[0].Height, nil
}

func NewFileCamera(filePath string) (*FileCamera, error) {
	width, height, err := probeVideo(filePath)
	if err != nil {
		width, height = 1280, 720
	}

	// Простая команда, без лишних флагов
	cmd := exec.Command("ffmpeg",
		"-i", filePath,
		"-f", "rawvideo",
		"-pix_fmt", "rgb24",
		"-")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	frame := image.NewRGBA(image.Rect(0, 0, width, height))
	c := &FileCamera{
		path:   filePath,
		cmd:    cmd,
		stdout: stdout,
		width:  width,
		height: height,
		frame:  frame,
	}
	go c.loop()
	return c, nil
}

func (c *FileCamera) loop() {
	frameSize := c.width * c.height * 3
	buf := make([]byte, frameSize)
	log.Printf("Camera loop started, frameSize=%d", frameSize)

	for {
		// Читаем ровно один кадр
		n := 0
		for n < frameSize {
			read, err := c.stdout.Read(buf[n:])
			if err != nil {
				if err == io.EOF {
					log.Println("EOF reached")
					return
				}
				log.Printf("Read error: %v", err)
				return
			}
			if read == 0 {
				log.Println("Read 0 bytes")
				return
			}
			n += read
		}
		log.Printf("Read full frame, n=%d", n)

		// Проверяем, не пустой ли кадр
		empty := true
		for i := 0; i < 10 && i < frameSize; i++ {
			if buf[i] != 0 {
				empty = false
				break
			}
		}
		if empty {
			log.Println("Empty frame, skipping")
			continue
		}

		c.mu.Lock()
		img := c.frame
		// rgb24 -> RGBA
		for y := 0; y < c.height; y++ {
			for x := 0; x < c.width; x++ {
				idx := (y*c.width + x) * 3
				r := buf[idx]
				g := buf[idx+1]
				b := buf[idx+2]
				img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
			}
		}
		c.mu.Unlock()

		// Сохраняем первый кадр
		if !c.saved {
			c.saved = true
			outFile, err := os.Create("test_from_go.jpg")

			if err == nil {
				jpeg.Encode(outFile, img, &jpeg.Options{Quality: 90})
				outFile.Close()
				log.Println("Saved first frame to test_from_go.jpg")
			} else {
				log.Printf("Failed to save frame: %v", err)
			}
		}
	}
}

func (c *FileCamera) ReadFrame() image.Image {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.frame == nil {
		return nil
	}
	dup := image.NewRGBA(c.frame.Bounds())
	copy(dup.Pix, c.frame.Pix)
	return dup
}

func (c *FileCamera) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Process.Kill()
	}
	return nil
}