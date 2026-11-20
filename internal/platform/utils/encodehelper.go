package utils

import (
	"bytes"
	"compress/flate"
	"compress/gzip"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

func BrotliEncode(input []byte) ([]byte, error) {
	// 创建一个新的 byte 输出流
	var buf bytes.Buffer
	// 创建一个新的 brotli 输出流
	brotliWriter := brotli.NewWriter(&buf)
	// 将 input byte 数组写入到此输出流中
	_, err := brotliWriter.Write(input)
	if err != nil {
		_ = brotliWriter.Close()
		return nil, err
	}
	if err := brotliWriter.Close(); err != nil {
		return nil, err
	}
	// 返回压缩后的 bytes 数组
	return buf.Bytes(), nil
}

func DeflateEncode(input []byte) ([]byte, error) {
	// 创建一个新的 byte 输出流
	var buf bytes.Buffer
	// 创建一个新的 gzip 输出流
	deflateWriter, err := flate.NewWriter(&buf, flate.DefaultCompression)

	// 将 input byte 数组写入到此输出流中
	_, err = deflateWriter.Write(input)
	if err != nil {
		_ = deflateWriter.Close()
		return nil, err
	}
	if err := deflateWriter.Close(); err != nil {
		return nil, err
	}
	// 返回压缩后的 bytes 数组
	return buf.Bytes(), nil
}

func GZipEncode(input []byte) ([]byte, error) {
	// 创建一个新的 byte 输出流
	var buf bytes.Buffer
	// 创建一个新的 gzip 输出流
	gzipWriter := gzip.NewWriter(&buf)
	// 将 input byte 数组写入到此输出流中
	_, err := gzipWriter.Write(input)
	if err != nil {
		_ = gzipWriter.Close()
		return nil, err
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, err
	}
	// 返回压缩后的 bytes 数组
	return buf.Bytes(), nil
}

func ZstdEncode(input []byte) ([]byte, error) {
	var buf bytes.Buffer
	zstdWriter, err := zstd.NewWriter(&buf)
	if err != nil {
		return nil, err
	}
	_, err = zstdWriter.Write(input)
	if err != nil {
		_ = zstdWriter.Close()
		return nil, err
	}
	if err := zstdWriter.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
