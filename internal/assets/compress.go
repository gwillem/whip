package assets

import (
	"io"

	"github.com/klauspost/compress/zstd"
)

func Compress(in io.Reader, out io.Writer) error {
	enc, err := zstd.NewWriter(out, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		return err
	}
	_, err = io.Copy(enc, in)
	if err != nil {
		enc.Close()
		return err
	}
	return enc.Close()
}

func Decompress(in io.Reader, out io.Writer) error {
	dec, err := zstd.NewReader(in)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, dec)
	if err != nil {
		dec.Close()
		return err
	}
	dec.Close()
	return nil
}
