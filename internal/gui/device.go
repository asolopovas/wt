package gui

import "os"

func detectDevice() string {
	switch os.Getenv("WT_ZIPFORMER_PROVIDER") {
	case "cuda":
		return "GPU CUDA (sherpa-onnx)"
	case "nnapi":
		return "NPU NNAPI (sherpa-onnx)"
	default:
		return "CPU"
	}
}
