package main

import (
	"fmt"
	"os"
	"strings"
)

func applyToFlake(currentPath string) error {
	// apply .flk changes to flake
	if err := applyShellHook(currentPath + "/.flk/shellhook.sh"); err != nil {
		return err
	}
	return nil
}

func ensureShellHookBlock(filePath string) error {
	// ensure shellHook exists inside mkShell
	// read flake.nix file
	flakeBytes, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			if boilerplateContent == "" {
				return fmt.Errorf("flake.nix not found and no boilerplate available")
			}
			if err := os.WriteFile(filePath, []byte(boilerplateContent), 0644); err != nil {
				return fmt.Errorf("could not create %s: %w", filePath, err)
			}
			return nil
		}
		return fmt.Errorf("could not read %s: %w", filePath, err)
	}
	flake := string(flakeBytes)

	startMarker := "shellHook = ''"
	endMarker := "'';"

	if strings.Contains(flake, startMarker) {
		return nil
	}

	mkShellIdx := strings.Index(flake, "default = pkgs.mkShell {")
	if mkShellIdx == -1 {
		mkShellIdx = strings.Index(flake, "default=pkgs.mkShell {")
	}
	if mkShellIdx == -1 {
		mkShellIdx = strings.Index(flake, "pkgs.mkShell {")
	}

	// find mkShell block
	if mkShellIdx != -1 {
		openRel := strings.Index(flake[mkShellIdx:], "{")
		if openRel != -1 {
			openIdx := mkShellIdx + openRel
			nlRel := strings.Index(flake[openIdx:], "\n")
			if nlRel != -1 {
				insertIdx := openIdx + nlRel + 1

				// insert after opening brace
				candidateStart := insertIdx
				for candidateStart < len(flake) && (flake[candidateStart] == '\n' || flake[candidateStart] == '\r') {
					candidateStart++
				}
				var indent string
				i := candidateStart
				for i < len(flake) && (flake[i] == ' ' || flake[i] == '\t') {
					indent += string(flake[i])
					i++
				}
				if indent == "" {
					mkLineStart := strings.LastIndex(flake[:mkShellIdx], "\n")
					var baseIndent string
					if mkLineStart == -1 {
						for _, ch := range flake[:mkShellIdx] {
							if ch == ' ' || ch == '\t' {
								baseIndent += string(ch)
							} else {
								break
							}
						}
					} else {
						linePrefix := flake[mkLineStart+1 : mkShellIdx]
						for _, ch := range linePrefix {
							if ch == ' ' || ch == '\t' {
								baseIndent += string(ch)
							} else {
								break
							}
						}
					}
					indent = baseIndent + "  "
				}

				shell := fmt.Sprintf("%sshellHook = ''\n%ss  echo \"Development environment loaded\"\n%ss'';\n", indent, indent, indent)
				newFlake := flake[:insertIdx] + shell + flake[insertIdx:]
				if err := os.WriteFile(filePath, []byte(newFlake), 0644); err != nil {
					return fmt.Errorf("could not write to %s: %w", filePath, err)
				}
				return nil
			}
		}

		i := mkShellIdx
		depth := 0
		foundStart := false
		for ; i < len(flake); i++ {
			ch := flake[i]
			if ch == '{' {
				depth++
				foundStart = true
			} else if ch == '}' {
				depth--
				if foundStart && depth == 0 {
					insertIdx := i

					candidateStart := insertIdx
					for candidateStart < len(flake) && (flake[candidateStart] == '\n' || flake[candidateStart] == '\r') {
						candidateStart++
					}
					var indent string
					j := candidateStart
					for j < len(flake) && (flake[j] == ' ' || flake[j] == '\t') {
						indent += string(flake[j])
						j++
					}
					if indent == "" {
						mkLineStart := strings.LastIndex(flake[:insertIdx], "\n")
						var baseIndent string
						if mkLineStart == -1 {
							for _, ch := range flake[:insertIdx] {
								if ch == ' ' || ch == '\t' {
									baseIndent += string(ch)
								} else {
									break
								}
							}
						} else {
							linePrefix := flake[mkLineStart+1 : insertIdx]
							for _, ch := range linePrefix {
								if ch == ' ' || ch == '\t' {
									baseIndent += string(ch)
								} else {
									break
								}
							}
						}
						indent = baseIndent + "  "
					}

					shell := fmt.Sprintf("\n%sshellHook = ''\n%ss  echo \"Development environment loaded\"\n%ss'';\n", indent, indent, indent)
					newFlake := flake[:insertIdx] + shell + flake[insertIdx:]
					if err := os.WriteFile(filePath, []byte(newFlake), 0644); err != nil {
						return fmt.Errorf("could not write to %s: %w", filePath, err)
					}
					return nil
				}
			}
		}
	}

	// append at end if needed
	newShellHook := fmt.Sprintf("%s\n  echo \"Development environment loaded\"\n%s", startMarker, endMarker)
	flake += "\n" + newShellHook
	if err := os.WriteFile(filePath, []byte(flake), 0644); err != nil {
		return fmt.Errorf("could not write to %s: %w", filePath, err)
	}

	return nil
}

func applyShellHook(shellHookFile string) error {
	if _, err := os.Stat(shellHookFile); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("stat %s: %w", shellHookFile, err)
	}

	shellHookContent, err := os.ReadFile(shellHookFile)
	if err != nil {
		return fmt.Errorf("could not read %s: %w", shellHookFile, err)
	}

	filePath, err := resolveFile("")
	if err != nil {
		return fmt.Errorf("could not resolve flake.nix path: %w", err)
	}

	flakeBytes, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("could not read %s: %w", filePath, err)
	}
	flake := string(flakeBytes)

	startMarker := "shellHook = ''"
	endMarker := "'';"

	startIdx := strings.Index(flake, startMarker)
	if startIdx == -1 {
		return fmt.Errorf("start marker %q not found in %s", startMarker, filePath)
	}

	rest := flake[startIdx:]
	endRel := strings.Index(rest, endMarker)
	if endRel == -1 {
		return fmt.Errorf("end marker %q not found in %s", endMarker, filePath)
	}
	endIdx := startIdx + endRel + len(endMarker)

	// compute heredoc indentation and replace contents
	lineStartIdx := strings.LastIndex(flake[:startIdx], "\n")
	var linePrefix string
	if lineStartIdx == -1 {
		linePrefix = flake[:startIdx]
	} else {
		linePrefix = flake[lineStartIdx+1 : startIdx]
	}

	indent := ""
	for _, ch := range linePrefix {
		if ch == ' ' || ch == '\t' {
			indent += string(ch)
		} else {
			break
		}
	}

	contentIndent := indent + "  "

	rawLines := strings.Split(strings.TrimRight(string(shellHookContent), "\n"), "\n")
	for i, l := range rawLines {
		if l == "" {
			rawLines[i] = ""
		} else {
			rawLines[i] = contentIndent + l
		}
	}
	indented := strings.Join(rawLines, "\n")
	replacement := startMarker + "\n" + indented + "\n" + indent + endMarker

	newFlake := flake[:startIdx] + replacement + flake[endIdx:]

	if err := os.WriteFile(filePath, []byte(newFlake), 0644); err != nil {
		return fmt.Errorf("could not write to %s: %w", filePath, err)
	}

	return nil
}
