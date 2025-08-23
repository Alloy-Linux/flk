package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
)

func generateInputs(filePath string) {
	var newLines []string
	var extraInputs []string
	skipBlock := false

	file, err := os.Open(filePath)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trim := strings.TrimSpace(line)
		condensed := strings.Join(strings.Fields(trim), "")

		// If we are inside an inputs block, parse it
		if skipBlock {
			// Check for the end of the block
			if condensed == "};" {
				skipBlock = false
				continue
			}
			// If not the end, it's a line with an input to preserve
			if trim != "" {
				extraInputs = append(extraInputs, trim)
			}
			continue
		}

		// Look for the start of an inputs block
		if strings.HasPrefix(condensed, "inputs={") {
			skipBlock = true
			if strings.HasSuffix(condensed, "};") {
				skipBlock = false
			}
			continue
		}

		if strings.HasPrefix(trim, "inputs.") {
			parts := strings.SplitN(line, ".", 2)
			if len(parts) == 2 {
				extraInputs = append(extraInputs, strings.TrimSpace(parts[1]))
			}
			continue
		}

		// If none of the above, it's a regular line
		newLines = append(newLines, line)
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	// Build the standard block
	inputBlock := []string{
		"  inputs = {",
		`    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";`,
		`    flake-utils.url = "github:numtide/flake-utils";`,
	}

	// Anti-duplicate stuff
	for _, inp := range extraInputs {
		trimmedInp := strings.TrimSpace(inp)
		if strings.HasPrefix(trimmedInp, "nixpkgs.") || strings.HasPrefix(trimmedInp, "flake-utils.") {
			continue
		}
		inputBlock = append(inputBlock, "    "+trimmedInp)
	}
	inputBlock = append(inputBlock, "  };")

	// Insert the new block right after the {
	finalLines := []string{}
	inserted := false
	for _, l := range newLines {
		finalLines = append(finalLines, l)
		if !inserted && strings.TrimSpace(l) == "{" {
			finalLines = append(finalLines, inputBlock...)
			inserted = true
		}
	}

	err = os.WriteFile(filePath, []byte(strings.Join(finalLines, "\n")), 0644)
	if err != nil {
		log.Fatal(err)
	}
}

func createBlock(filePath string, location string, startBlock string, endBlock string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	var newLines []string
	blockInserted := false

	// loop over lines and insert text
	for _, line := range lines {
		newLines = append(newLines, line) // Keep the original line
		if strings.Contains(line, location) && !blockInserted {
			indent := getLineIndentation(line)
			newLines = append(newLines, fmt.Sprintf("%s  %s", indent, startBlock))
			newLines = append(newLines, fmt.Sprintf("%s  %s", indent, endBlock))
			blockInserted = true
		}
	}

	if !blockInserted {
		return fmt.Errorf("location '%s' not found in file", location)
	}

	// Write the modified content back to the file
	outputFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file for writing: %w", err)
	}
	defer outputFile.Close()

	writer := bufio.NewWriter(outputFile)
	for _, line := range newLines {
		fmt.Fprintln(writer, line)
	}
	return writer.Flush()
}
