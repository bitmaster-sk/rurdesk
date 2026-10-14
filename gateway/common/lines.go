package common

import (
	"bufio"
	"bytes"
	"errors"
	"io"
)

const DefaultMaxOutputLineBytes = 4 * 1024 * 1024

// The slice passed to onLine is reused for the next line, so copy it to keep it.
func ReadLines(reader io.Reader, maxLineBytes int, onLine func(line []byte)) (int, error) {
	if maxLineBytes <= 0 {
		maxLineBytes = DefaultMaxOutputLineBytes
	}
	buffered := bufio.NewReaderSize(reader, 64*1024)
	var line []byte
	isOversized := false
	skipped := 0
	for {
		chunk, err := buffered.ReadSlice('\n')
		if !isOversized && len(line)+len(bytes.TrimSuffix(chunk, []byte{'\n'})) > maxLineBytes {
			isOversized = true
			line = line[:0]
		}
		if !isOversized {
			line = append(line, chunk...)
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		if isOversized {
			skipped++
		} else if content := bytes.TrimSuffix(line, []byte{'\n'}); err == nil || len(content) > 0 {
			onLine(content)
		}
		line = line[:0]
		isOversized = false
		if errors.Is(err, io.EOF) {
			return skipped, nil
		}
		if err != nil {
			return skipped, err
		}
	}
}
