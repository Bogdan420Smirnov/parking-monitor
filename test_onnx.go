package main

import (
	"log"

	onnx "github.com/yalue/onnxruntime_go"
)



func main() {
	// Указываем полный путь к библиотеке
	onnx.SetSharedLibraryPath("C:/onnxruntime/onnxruntime-win-x64-1.28.0/lib/onnxruntime.dll")

	err := onnx.InitializeEnvironment()
	if err != nil {
		log.Fatalf("Failed to initialize ONNX: %v", err)
	}
	defer onnx.DestroyEnvironment()

	log.Println("ONNX Runtime successfully initialized!")
}
