package fsrename

import "testing"
import "github.com/stretchr/testify/assert"

func TestScanner(t *testing.T) {
	input := NewFileStream()
	output := NewFileStream()
	worker := NewGlobScanner()
	worker.SetInput(input)
	worker.SetOutput(output)
	worker.Start()

	input <- MustNewFileEntry("tests/scanner")
	input <- nil
	assert.NotNil(t, output)

	entries := readEntries(t, output)
	assert.Equal(t, 6, len(entries))
}

func TestFilterWorkerEmtpy(t *testing.T) {
	input := NewFileStream()
	output := NewFileStream()
	worker := NewFileFilter()
	worker.SetInput(input)
	worker.SetOutput(output)
	worker.Start()

	e, err := NewFileEntry("tests/autoload.php")
	assert.Nil(t, err)
	input <- e
	input <- nil
	assert.NotNil(t, output)
	entries := readEntries(t, output)
	assert.Equal(t, 1, len(entries))
}

func TestSimpleRegExpOnPHPFiles(t *testing.T) {
	input := NewFileStream()
	scanner := NewGlobScanner()
	scanner.SetInput(input)
	scanner.Start()
	chain := scanner.Chain(NewFileFilter()).Chain(NewRegExpFilterWithPattern("\\.php$"))

	input <- MustNewFileEntry("tests/php_files")
	input <- nil

	output := chain.Output()
	assert.NotNil(t, output)
	entries := readEntries(t, output)
	assert.Equal(t, 2, len(entries))
}

func TestSimpleFilePipe(t *testing.T) {
	input := NewFileStream()
	scanner := NewGlobScanner()
	scanner.SetInput(input)
	scanner.Start()

	filter := scanner.Chain(&FileFilter{
		&BaseWorker{stop: make(CondVar, 1)},
	})

	input <- MustNewFileEntry("tests/scanner")
	input <- nil
	output := filter.Output()
	assert.NotNil(t, output)
	entries := readEntries(t, output)
	assert.Equal(t, 5, len(entries))
}

func TestSimpleReverseSorter(t *testing.T) {
	input := NewFileStream()
	scanner := NewGlobScanner()
	scanner.SetInput(input)
	scanner.Start()
	chain := scanner.
		Chain(NewFileFilter()).
		Chain(NewReverseSorter())

	input <- MustNewFileEntry("tests")
	input <- nil

	output := chain.Output()
	assert.NotNil(t, output)
	readEntries(t, output)
}

func readEntries(t *testing.T, input chan *FileEntry) []FileEntry {
	var entries []FileEntry
	for {
		select {
		case entry := <-input:
			t.Log("entry", entry)
			if entry == nil {
				return entries
			}
			entries = append(entries, *entry)
			break
		}
	}
	return entries
}
