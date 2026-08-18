@echo off
echo Setting up environment...

set CGO_ENABLED=1

REM Пути для ONNX Runtime
set ONNX_INCLUDE=C:\onnxruntime\onnxruntime-win-x64-1.28.0\include
set ONNX_LIB=C:\onnxruntime\onnxruntime-win-x64-1.28.0\lib



REM Путь к MinGW (gcc)
set MINGW_PATH=C:\msys64\ucrt64\bin

REM Флаги для CGO
set CGO_CFLAGS=-I%ONNX_INCLUDE%
set CGO_LDFLAGS=-L%ONNX_LIB% -lonnxruntime

REM Пути в PATH
set PATH=%ONNX_LIB%;%MINGW_PATH%;%PATH%

REM Прокси
set HTTP_PROXY=http://smirnov_b:1234@bsd-proxy.bolid.ru:3128
set HTTPS_PROXY=http://smirnov_b:1234@bsd-proxy.bolid.ru:3128

echo Environment set.