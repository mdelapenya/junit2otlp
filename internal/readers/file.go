package readers

import "os"

type FileReader struct {
	testFile string
}

func NewFileReader(testFile string) *FileReader {
	return &FileReader{testFile: testFile}
}

func (tr *FileReader) Read() ([]byte, error) {
	b, err := os.ReadFile(tr.testFile)
	if err != nil {
		return []byte{}, err
	}

	return b, nil
}
