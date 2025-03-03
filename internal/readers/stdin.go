package readers

import (
	"bufio"
	"fmt"
	"os"
)

type PipeReader struct{}

func (pr *PipeReader) Read() ([]byte, error) {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return nil, err
	}

	if (stat.Mode() & os.ModeCharDevice) == 0 {
		var buf []byte
		scanner := bufio.NewScanner(os.Stdin)

		// 64KB initial buffer, 1MB max buffer size
		// was seeing large failure messages causing parsing to fail
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		for scanner.Scan() {
			buf = append(buf, scanner.Bytes()...)
		}

		if err := scanner.Err(); err != nil {
			return nil, err
		}

		return buf, nil
	}

	return nil, fmt.Errorf("there is no data in the pipe")
}
