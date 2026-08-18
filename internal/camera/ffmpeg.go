package camera

import (
	"image"
	"image/color"
	"io"
	"os/exec"
	"sync"
)

type Camera struct {
	url    string
	cmd    *exec.Cmd
	stdout io.ReadCloser
	mu     sync.Mutex
	frame  *image.RGBA
	width  int
	height int
	closed bool
}

// NewCamera создаёт камеру, подключается к RTSP через ffmpeg
func NewCamera(url string) (*Camera, error) {
	// ffmpeg -i rtsp://... -f rawvideo -pix_fmt bgr24 -
	cmd := exec.Command("ffmpeg", "-i", url, "-f", "rawvideo", "-pix_fmt", "bgr24", "-")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// Задайте разрешение вашей камеры (можно узнать через ffprobe)
	width, height := 1920, 1080
	frame := image.NewRGBA(image.Rect(0, 0, width, height))

	c := &Camera{
		url:    url,
		cmd:    cmd,
		stdout: stdout,
		width:  width,
		height: height,
		frame:  frame,
	}
	go c.loop()
	return c, nil
}

func (c *Camera) loop() {
	buf := make([]byte, c.width*c.height*3) // BGR24 = 3 байта на пиксель
	for {
		n, err := c.stdout.Read(buf)
		if err != nil {
			break
		}
		if n != len(buf) {
			continue // неполный кадр, пропускаем
		}
		// Конвертируем BGR -> RGBA
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
	}
}

// ReadFrame возвращает последний полученный кадр
func (c *Camera) ReadFrame() image.Image {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.frame == nil {
		return nil
	}
	// Возвращаем копию, чтобы избежать изменения извне
	dup := image.NewRGBA(c.frame.Bounds())
	copy(dup.Pix, c.frame.Pix)
	return dup
}

// Close останавливает процесс ffmpeg
func (c *Camera) Close() error {
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