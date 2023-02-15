package internal

import (
	"bufio"
	"os"
	"strings"
)

// ReadLines 读取一个文件，去掉空行和多余的空格
func ReadLines(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		if len(line) == 0 {
			continue
		}

		lines = append(lines, line)
	}

	return lines, scanner.Err()
}

// ReadLine 只读取第一行非空行
func ReadLine(filename string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		if len(line) == 0 {
			continue
		}

		return line, nil
	}

	return "", scanner.Err()
}
