package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode"
)

func getPackages(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var packages []string
	inPackagesBlock := false
	packageLineRegex := regexp.MustCompile(`^\s*([a-zA-Z0-9_.+-]+)\s*,?\s*$`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		if strings.Contains(line, "packages = [") {
			inPackagesBlock = true
			continue
		}

		if inPackagesBlock && strings.Contains(line, "]") {
			break
		}

		if inPackagesBlock {
			line = strings.TrimSpace(line)
			matches := packageLineRegex.FindStringSubmatch(line)
			if len(matches) == 2 {
				packages = append(packages, matches[1])
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return packages, nil
}

func addPackage(filePath, pkg string) error {
	input, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("could not read file: %w", err)
	}

	// gets the variable under let
	variables, err := getVariables(filePath)
	if err != nil {
		return fmt.Errorf("could not get variables from file: %w", err)
	}

	// searches for pkgs prefix
	prefixFound := false
	for _, v := range variables {
		// Check if pkgs variable is already defined
		if strings.HasPrefix(v, "pkgs") {
			prefixFound = true
			break
		}
	}

	if !prefixFound {
		// Insert pkgs variable if missing
		if err := insertVariable(filePath, "pkgs", "import nixpkgs { inherit system; }"); err != nil {
			return err
		}
		input, err = os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("could not re-read file after variable insertion: %w", err)
		}
	}

	lines := strings.Split(string(input), "\n")
	var (
		inBlock         = false
		alreadyPresent  = false
		blockStartIndex = -1 // index of [
		blockEndIndex   = -1 // index of ]
		indent          = ""
	)

	// adds the prefix
	fullPkgName := "pkgs." + pkg
	// Find the packages block and check if package is already present
	for i, line := range lines {
		trim := strings.TrimSpace(line)

		if strings.Contains(trim, fullPkgName) {
			alreadyPresent = true // already in list
		}

		if strings.Contains(trim, "packages = [") {
			inBlock = true // start block
			blockStartIndex = i
			indent = getLineIndentation(line) + "  "
			continue
		}

		if inBlock && strings.Contains(trim, "]") {
			blockEndIndex = i // end block
			break
		}
	}

	if alreadyPresent {
		fmt.Println("Package already exists:", pkg)
		return nil
	}

	// If block not found, create and retry
	if blockStartIndex == -1 || blockEndIndex == -1 {
		createBlock(filePath, "mkShell", "packages = [", "]")
		// Re-read file and retry adding the package
		return addPackage(filePath, pkg)
	}

	newLines := append([]string{}, lines[:blockEndIndex]...)
	newLines = append(newLines, indent+fullPkgName)
	newLines = append(newLines, lines[blockEndIndex:]...)

	output := strings.Join(newLines, "\n")
	if err := os.WriteFile(filePath, []byte(output), 0644); err != nil {
		return fmt.Errorf("failed to write updated file: %w", err)
	}

	fmt.Println("Added package:", pkg)
	return nil
}

func getLineIndentation(line string) string {
	for i, r := range line {
		if !unicode.IsSpace(r) {
			return line[:i]
		}
	}
	return ""
}

func removePackage(filePath, pkg string) error {
	input, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("could not read file: %w", err)
	}

	lines := strings.Split(string(input), "\n")
	packageFound := false
	fullPkgName := "pkgs." + pkg
	newLines := []string{}

	// Remove the package line if present
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, fullPkgName) && !strings.HasPrefix(trimmed, "#") {
			if strings.Contains(trimmed, fullPkgName) {
				packageFound = true
				continue
			}
		}
		newLines = append(newLines, line)
	}

	if !packageFound {
		fmt.Println("Package not found:", pkg)
		return nil
	}

	output := strings.Join(newLines, "\n")
	if err := os.WriteFile(filePath, []byte(output), 0644); err != nil {
		return fmt.Errorf("failed to write updated file: %w", err)
	}

	fmt.Println("Removed package:", pkg)
	return nil
}

func getVariables(filePath string) ([]string, error) {
	input, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("could not read file: %w", err)
	}

	lines := strings.Split(string(input), "\n")
	var variables []string
	inLetBlock := false

	varDeclRegex := regexp.MustCompile(`^\s*([a-zA-Z0-9_.-]+)\s*=`)

	for _, line := range lines {
		trim := strings.TrimSpace(line)

		if strings.HasPrefix(trim, "let") {
			inLetBlock = true
			continue
		}
		if inLetBlock && strings.HasPrefix(trim, "in") {
			inLetBlock = false
			continue
		}
		if inLetBlock {
			if strings.HasPrefix(trim, "#") || trim == "" {
				continue
			}
			matches := varDeclRegex.FindStringSubmatch(trim)
			if len(matches) == 2 {
				varName := matches[1]
				variables = append(variables, varName)
			}
		}
	}

	return variables, nil
}

func insertVariable(filePath, varName, varValue string) error {
	input, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("could not read file: %w", err)
	}

	lines := strings.Split(string(input), "\n")
	letIndex := -1
	inIndex := -1

	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "let") {
			letIndex = i
		} else if trim == "in" || strings.HasPrefix(trim, "in ") || strings.HasPrefix(trim, "in{") {
			inIndex = i
			break
		}
	}

	if letIndex == -1 || inIndex == -1 {
		return fmt.Errorf("could not find 'let ... in' block in %s", filePath)
	}

	indent := ""
	for i := letIndex + 1; i < inIndex; i++ {
		trim := strings.TrimSpace(lines[i])
		if trim != "" && !strings.HasPrefix(trim, "#") {
			indent = getLineIndentation(lines[i])
			break
		}
	}

	if indent == "" {
		indent = getLineIndentation(lines[letIndex]) + "  "
	}

	newLine := fmt.Sprintf("%s%s = %s;", indent, varName, varValue)
	newLines := append(lines[:inIndex], append([]string{newLine}, lines[inIndex:]...)...)

	output := strings.Join(newLines, "\n")
	if err := os.WriteFile(filePath, []byte(output), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
