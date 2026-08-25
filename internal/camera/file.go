package camera

import (
	"encoding/json"
	"image"
	"image/color"
	"io"
	"os/exec"
	"sync"
	"time"
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

	// Зацикливаем видео с помощью -stream_loop -1
	cmd := exec.Command("ffmpeg",
		"-stream_loop", "-1",
		"-i", filePath,
		"-f", "rawvideo",
		"-pix_fmt", "bgr24",
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
	for {
		// Читаем ровно один кадр
		n := 0
		for n < frameSize {
			read, err := c.stdout.Read(buf[n:])
			if err != nil {
				if err == io.EOF {
					// Если EOF, то ffmpeg перезапустится автоматически из-за -stream_loop -1
					// Но на всякий случай подождём и продолжим
					time.Sleep(100 * time.Millisecond)
					continue
				}
				return
			}
			if read == 0 {
				return
			}
			n += read
		}

		// Проверяем, не пустой ли кадр
		empty := true
		for i := 0; i < 10 && i < frameSize; i++ {
			if buf[i] != 0 {
				empty = false
				break
			}
		}
		if empty {
			continue
		}

		c.mu.Lock()
		img := c.frame
		for y := 0; y < c.height; y++ {
			for x := 0; x < c.width; x++ {
				idx := (y*c.width + x) * 3
				b := buf[idx]
				g := buf[idx+1]
				r := buf[idx+2]
				img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
			}
		}
		c.mu.Unlock()

		// Задержка для имитации FPS (~30 fps)
		time.Sleep(33 * time.Millisecond)
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