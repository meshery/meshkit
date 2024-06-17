package utils

import (
	"archive/tar"
	"bytes"
)

type TarWriter struct {
	Writer *tar.Writer
	Buffer *bytes.Buffer
}

func NewTarWriter() *TarWriter {
	buffer := bytes.Buffer{}
	return &TarWriter{
		Writer: tar.NewWriter(&buffer),
		Buffer: &buffer,
	}
}

func (tw *TarWriter) Compress(name string, data []byte) error {
	header := tar.Header{
		Name: name,
		Size: int64(len(data)),
		Mode: 777,
	}
	err := tw.Writer.WriteHeader(&header)
	if err != nil {
		return ErrCompressToTarGZ(err, "")
	}

	_, err = tw.Writer.Write(data)
	if err != nil {
		return ErrCompressToTarGZ(err, "")
	}
	return nil
}

func (tw *TarWriter) Close() {
	_ = tw.Writer.Flush()
	_ = tw.Writer.Close()
}