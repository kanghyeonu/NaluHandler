package fileio

import (
	"io/ioutil"
	"os"
)

// ReadFile reads the content of a file and returns it as a string.
func ReadFile(filename string) (string, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// WriteFile writes the given content to a file.
func WriteFile(filename string, content string) error {
	return ioutil.WriteFile(filename, []byte(content), 0644)
}

// AppendToFile appends the given content to a file.
func AppendToFile(filename string, content string) error {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(content)
	if err != nil {
		return err
	}
	return nil
}
