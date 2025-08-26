package main

import (
	"fmt"
	"os"
	"strings"
)

// ensure defaultPackage exists
func ensureDerivation(filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("could not read %s: %w", filePath, err)
	}
	flake := string(content)

	// check for derivation
	if strings.Contains(flake, "defaultPackage =") ||
		strings.Contains(flake, "pkgs.stdenv.mkDerivation") ||
		strings.Contains(flake, "mkDerivation {") {
		return nil
	}

	pkgsList, _ := getPackagesFromPackageYML()

	// detect indentation
	indent := ""
	lines := strings.Split(flake, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "devShells ") || strings.HasPrefix(trimmed, "devShells=") {
			indent = getLineIndentation(line)
			break
		}
	}
	if indent == "" {
		// fallback to first non-empty
		inOutputs := false
		for _, line := range lines {
			if strings.Contains(line, "outputs =") {
				inOutputs = true
				continue
			}
			if inOutputs && strings.Contains(line, "{") {
				continue
			}
			if inOutputs && strings.TrimSpace(line) != "" && !strings.Contains(line, "{") && !strings.Contains(line, "}") {
				indent = getLineIndentation(line)
				break
			}
		}
	}
	if indent == "" {
		// fallback to two spaces
		indent = "  "
	}

	// build mkDerivation block
	var block []string
	block = append(block, indent+"defaultPackage = pkgs.stdenv.mkDerivation {")
	block = append(block, indent+"  pname = \"default\";")
	block = append(block, indent+"  version = \"0.1\";")
	block = append(block, indent+"  src = ./.;")

	if len(pkgsList) == 0 {
		block = append(block, indent+"  buildInputs = [];")
	} else {
		block = append(block, indent+"  buildInputs = [")
		for _, pkg := range pkgsList {
			block = append(block, indent+"    "+pkg)
		}
		block = append(block, indent+"  ];")
	}
	block = append(block, indent+"};")
	block = append(block, "")

	// find insert index
	insertIdx, err := findInsertIndex(flake)
	if err != nil {
		return fmt.Errorf("could not find insertion point in %s: %w", filePath, err)
	}

	// insert block
	newFlake := flake[:insertIdx]
	if len(newFlake) > 0 && newFlake[len(newFlake)-1] != '\n' {
		newFlake += "\n"
	}
	newFlake += strings.Join(block, "\n") + flake[insertIdx:]

	if err := os.WriteFile(filePath, []byte(newFlake), 0644); err != nil {
		return fmt.Errorf("could not write to %s: %w", filePath, err)
	}
	return nil
}

// find insert index
func findInsertIndex(flake string) (int, error) {
	// after devShells block
	devIdx := strings.Index(flake, "devShells")
	if devIdx != -1 {
		braceRel := strings.Index(flake[devIdx:], "{")
		if braceRel != -1 {
			openIdx := devIdx + braceRel
			depth := 0
			started := false
			for i := openIdx; i < len(flake); i++ {
				ch := flake[i]
				switch ch {
				case '{':
					depth++
					started = true
				case '}':
					depth--
					if started && depth == 0 {
						j := i + 1
						// skip whitespace
						for j < len(flake) && (flake[j] == ' ' || flake[j] == '\n' || flake[j] == '\r' || flake[j] == ';' || flake[j] == '\t') {
							j++
						}
						return j, nil
					}
				}
			}
		}
	}

	// after in block
	inIdx := strings.Index(flake, "in {")
	if inIdx == -1 {
		inIdx = strings.Index(flake, "in{")
	}
	if inIdx == -1 {
		return -1, fmt.Errorf("could not find 'devShells' or 'in {'")
	}
	braceRel := strings.Index(flake[inIdx:], "{")
	if braceRel == -1 {
		return -1, fmt.Errorf("malformed 'in' block")
	}
	openIdx := inIdx + braceRel
	nlRel := strings.Index(flake[openIdx:], "\n")
	if nlRel != -1 {
		return openIdx + nlRel + 1, nil
	}
	return openIdx + 1, nil
}
