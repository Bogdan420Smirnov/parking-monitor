package camera

import (
	"image"
	"image/color"
	"io"
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
}

// NewFileCamera создаёт камеру для чтения видеофайла через ffmpeg
func NewFileCamera(filePath string) (*FileCamera, error) {
	// Используем rgb24, чтобы получить каналы в порядке R,G,B
	cmd := exec.Command("ffmpeg", "-i", filePath, "-f", "rawvideo", "-pix_fmt", "rgb24", "-")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// Узнаём разрешение видео (можно сделать через ffprobe, но пока зададим явно)
	// Если у вас другое разрешение, укажите его здесь
	width, height := 1280, 720
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
	buf := make([]byte, c.width*c.height*3) // 3 байта на пиксель (RGB)
	for {
		n, err := c.stdout.Read(buf)
		if err != nil {
			break
		}
		if n != len(buf) {
			continue // неполный кадр, пропускаем
		}
		c.mu.Lock()
		img := c.frame
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
	}
}

// ReadFrame возвращает последний кадр
func (c *FileCamera) ReadFrame() image.Image {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.frame == nil {
		return nil
	}
	// Возвращаем копию
	dup := image.NewRGBA(c.frame.Bounds())
	copy(dup.Pix, c.frame.Pix)
	return dup
}

// Close останавливает ffmpeg
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