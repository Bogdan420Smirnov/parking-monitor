package detector

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	//"image/jpeg"
	"math"
	"sort"
	//"log"
	//"os"
	"github.com/nfnt/resize"

	onnx "github.com/yalue/onnxruntime_go"
	//"golang.org/x/image/draw"
	//"github.com/fogleman/gg"
)

type Detection struct {
	Bbox  [4]float32
	Class int
	Score float32
}

type Detector interface {
	Detect(img image.Image) ([]Detection, error)
	Close() error
}

type YOLODetector struct {
	session       *onnx.AdvancedSession
	inputTensor   *onnx.Tensor[float32]
	outputTensor  *onnx.Tensor[float32]
	confThreshold float32
}

const (
	modelInputWidth  = 640
	modelInputHeight = 640
	numClasses       = 80
	numPredictions   = 8400
)

func NewYOLODetector(modelPath string, confThreshold float32, useGPU bool) (*YOLODetector, error) {
	options, err := onnx.NewSessionOptions()
	if err != nil {
		return nil, fmt.Errorf("failed to create session options: %w", err)
	}
	defer options.Destroy()

	inputShape := []int64{1, 3, modelInputHeight, modelInputWidth}
	inputData := make([]float32, 3*modelInputWidth*modelInputHeight)
	inputTensor, err := onnx.NewTensor(inputShape, inputData)
	if err != nil {
		return nil, fmt.Errorf("failed to create input tensor: %w", err)
	}

	outputShape := []int64{1, 4 + numClasses, numPredictions}
	outputData := make([]float32, (4+numClasses)*numPredictions)
	outputTensor, err := onnx.NewTensor(outputShape, outputData)
	if err != nil {
		inputTensor.Destroy()
		return nil, fmt.Errorf("failed to create output tensor: %w", err)
	}

	session, err := onnx.NewAdvancedSession(
		modelPath,
		[]string{"images"},
		[]string{"output0"},
		[]onnx.Value{inputTensor},
		[]onnx.Value{outputTensor},
		options,
	)
	if err != nil {
		inputTensor.Destroy()
		outputTensor.Destroy()
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &YOLODetector{
		session:       session,
		inputTensor:   inputTensor,
		outputTensor:  outputTensor,
		confThreshold: confThreshold,
	}, nil
}

func (d *YOLODetector) Close() error {
	if d.session != nil {
		d.session.Destroy()
	}
	if d.inputTensor != nil {
		d.inputTensor.Destroy()
	}
	if d.outputTensor != nil {
		d.outputTensor.Destroy()
	}
	return nil
}

func (d *YOLODetector) Detect(img image.Image) ([]Detection, error) {
	err := d.preprocess(img)
	if err != nil {
		return nil, fmt.Errorf("preprocessing failed: %w", err)
	}
	err = d.session.Run()
	if err != nil {
		return nil, fmt.Errorf("inference failed: %w", err)
	}
	outputData := d.outputTensor.GetData()//.([]float32)
	detections := d.postprocess(outputData)
	return detections, nil
}

func (d *YOLODetector) preprocess(img image.Image) error {
	data := d.inputTensor.GetData()//.([]float32)
	//width := modelInputWidth
	//height := modelInputHeight
	channelSize := modelInputHeight * modelInputWidth

	if len(data) < (channelSize * 3) {
        return fmt.Errorf("destination tensor only holds %d floats, needs %d (make sure it's the right shape!)", len(data), channelSize*3)
    }
	redChannel := data[0:channelSize]
    greenChannel := data[channelSize : channelSize*2]
    blueChannel := data[channelSize*2 : channelSize*3]

	srcBounds := img.Bounds()
    srcW := srcBounds.Dx()
    srcH := srcBounds.Dy()

    // Вычисляем масштаб для вписывания в 640x640
    scale := math.Min(float64(640)/float64(srcW), float64(640)/float64(srcH))
    newW := int(float64(srcW) * scale)
    newH := int(float64(srcH) * scale)

    // Изменяем размер с сохранением пропорций
    resized := resize.Resize(uint(newW), uint(newH), img, resize.Lanczos3)

    // Создаём холст 640x640 с серым фоном (значение 114 для YOLO)
    dstImg := image.NewRGBA(image.Rect(0, 0, 640, 640))
    gray := color.RGBA{114, 114, 114, 255}
    draw.Draw(dstImg, dstImg.Bounds(), &image.Uniform{gray}, image.Point{}, draw.Src)

    // Вставляем изменённое изображение по центру
    xOffset := (640 - newW) / 2
    yOffset := (640 - newH) / 2
    draw.Draw(dstImg, image.Rect(xOffset, yOffset, xOffset+newW, yOffset+newH), resized, image.Point{}, draw.Src)

    // Сохраняем параметры для постобработки (глобальные переменные)
    /* letterboxXOffset = xOffset
    letterboxYOffset = yOffset
    letterboxScale = scale */

    // Заполняем тензор из LetterBox-изображения
    i := 0
    for y := 0; y < 640; y++ {
        for x := 0; x < 640; x++ {
            r, g, b, _ := dstImg.At(x, y).RGBA()
            redChannel[i] = float32(r>>8) / 255.0
            greenChannel[i] = float32(g>>8) / 255.0
            blueChannel[i] = float32(b>>8) / 255.0
            i++
        }
    }

    return nil
} 

/* func (d *YOLODetector) postprocess(outputData []float32) []Detection {
	const iouThreshold = 0.45
	confThreshold := d.confThreshold
	var detections []Detection
	for i := 0; i < numPredictions; i++ {
		base := i * (4 + numClasses)
		cx := outputData[base]
		cy := outputData[base+1]
		w := outputData[base+2]
		h := outputData[base+3]
		maxScore := float32(0.0)
		classID := 0
		for j := 0; j < numClasses; j++ {
			score := outputData[base+4+j]
			if score > maxScore {
				maxScore = score
				classID = j
			}
		}
		if classID == 2 && maxScore > confThreshold {
			x1 := cx - w/2
			y1 := cy - h/2
			x2 := cx + w/2
			y2 := cy + h/2
			detections = append(detections, Detection{
				Bbox:  [4]float32{x1, y1, x2, y2},
				Class: classID,
				Score: maxScore,
			})
		}
	}
	return nonMaxSuppression(detections, iouThreshold)
} */



func (d *YOLODetector) postprocess(outputData []float32) []Detection {
    const numClasses = 80
    const confThreshold = 0.5
    const iouThreshold = 0.45

    numPredictions := 8400
    var detections []Detection

    // Предполагаем порядок [каналы, предсказания]
    for idx := 0; idx < numPredictions; idx++ {
        // Извлекаем координаты (нормализованные относительно 640x640)
        cx := outputData[idx]                       // канал 0
        cy := outputData[8400+idx]                  // канал 1
        w := outputData[2*8400+idx]                 // канал 2
        h := outputData[3*8400+idx]                 // канал 3

        // Находим класс с максимальной вероятностью
        maxScore := float32(0.0)
        classID := 0
        for col := 0; col < numClasses; col++ {
            score := outputData[4*8400 + col*8400 + idx]
            if score > maxScore {
                maxScore = score
                classID = col
            }
        }

        if classID == 2 && maxScore > confThreshold {
            // Преобразуем в [x1, y1, x2, y2] в координатах 640x640
            x1 := cx - w/2
            y1 := cy - h/2
            x2 := cx + w/2
            y2 := cy + h/2
            detections = append(detections, Detection{
                Bbox:  [4]float32{x1, y1, x2, y2},
                Class: classID,
                Score: maxScore,
            })
        }
    }

    // NMS
    return nonMaxSuppression(detections, iouThreshold)
}

func nonMaxSuppression(detections []Detection, iouThreshold float32) []Detection {
	if len(detections) == 0 {
		return detections
	}
	sort.Slice(detections, func(i, j int) bool {
		return detections[i].Score > detections[j].Score
	})
	var result []Detection
	for len(detections) > 0 {
		best := detections[0]
		result = append(result, best)
		detections = detections[1:]
		var filtered []Detection
		for _, d := range detections {
			if iou(best.Bbox, d.Bbox) <= iouThreshold {
				filtered = append(filtered, d)
			}
		}
		detections = filtered
	}
	return result
}

func iou(box1, box2 [4]float32) float32 {
	x1 := float32(math.Max(float64(box1[0]), float64(box2[0])))
	y1 := float32(math.Max(float64(box1[1]), float64(box2[1])))
	x2 := float32(math.Min(float64(box1[2]), float64(box2[2])))
	y2 := float32(math.Min(float64(box1[3]), float64(box2[3])))
	if x2 < x1 || y2 < y1 {
		return 0.0
	}
	interArea := (x2 - x1) * (y2 - y1)
	box1Area := (box1[2] - box1[0]) * (box1[3] - box1[1])
	box2Area := (box2[2] - box2[0]) * (box2[3] - box2[1])
	unionArea := box1Area + box2Area - interArea
	if unionArea <= 0 {
		return 0.0
	}
	return interArea / unionArea
}