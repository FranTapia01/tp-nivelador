package safe_socket

import (
	"errors"
	"io"
)

var ErrUnexpectedEOF = errors.New("unexpected EOF: socket closed during transfer")

func SendAll(socket io.Writer, bytes []byte) error {
	totalSent := 0
	for totalSent < len(bytes) {
		n, err := socket.Write(bytes[totalSent:])
		if err != nil {
			return err
		}
		totalSent += n
	}
	return nil
}

func RecvAll(socket io.Reader, size int) ([]byte, error) {
	buff := make([]byte, size)
	totalRead := 0

	for totalRead < size {
		n, err := socket.Read(buff[totalRead:])
		if n > 0 {
			totalRead += n
		}
		if err != nil {
			if errors.Is(err, io.EOF) && totalRead < size {
				return nil, ErrUnexpectedEOF
			}
			return nil, err
		}
	}
	return buff, nil
}
