package readers

import (
	"fmt"

	"github.com/joshdk/go-junit"
)

type InputReader interface {
	Read() ([]byte, error)
}

func ReadJUnitReport(reader InputReader) ([]junit.Suite, error) {
	xmlBuffer, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read from pipe: %v", err)
	}

	suites, err := junit.Ingest(xmlBuffer)
	if err != nil {
		return nil, fmt.Errorf("failed to ingest JUnit xml: %v", err)
	}

	return suites, nil
}
