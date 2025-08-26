package main

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

func applyToFlake(currentPath string) error {
	// apply flk changes
	if err := applyShellHook(currentPath + "/.flk/devenv/shellhook.sh"); err != nil {
		return err
	}
	if err := applyBuildPhaseScript(currentPath + "/.flk/derivation/build.sh"); err != nil {
		return err
	}
	if err := applyInstallPhaseScript(currentPath + "/.flk/derivation/install.sh"); err != nil {
		return err
	}
	if err := applyPackagesToFlake(currentPath + "/flake.nix"); err != nil {
		return err
	}
	return nil
}

// apply phase script
func applyPhaseScript(scriptPath, phaseName string) error {
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("stat %s: %w", scriptPath, err)
	}

	scriptContent, err := os.ReadFile(scriptPath)
	if err != nil {
		return fmt.Errorf("could not read %s: %w", scriptPath, err)
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

	startMarker := phaseName + "Phase = ''"
	endMarker := "'';"

	startIdx := strings.Index(flake, startMarker)
	if startIdx == -1 {
		return insertPhaseBlock(flake, filePath, scriptContent, phaseName, startMarker, endMarker)
	}
	return updatePhaseBlock(flake, filePath, scriptContent, startIdx, startMarker, endMarker)
}

// insert a new phase block into mkDerivation
func insertPhaseBlock(flake, filePath string, scriptContent []byte, phaseName, startMarker, endMarker string) error {
	blockStart := strings.Index(flake, "pkgs.stdenv.mkDerivation {")
	if blockStart == -1 {
		blockStart = strings.Index(flake, "pkgs.mkDerivation {")
	}
	if blockStart == -1 {
		return fmt.Errorf("could not find mkDerivation block in flake.nix; cannot insert %sPhase block", phaseName)
	}

	mkStart := blockStart + strings.Index(flake[blockStart:], "{") + 1
	endIdx := findClosingBrace(flake, mkStart)

	baseIndent := getBaseIndentation(flake, blockStart)
	attributeIndent := baseIndent + "  "
	contentIndent := baseIndent + "    "

	indentedContent := indentScriptContent(scriptContent, contentIndent)
	block := "\n" + attributeIndent + startMarker + "\n" + indentedContent + "\n" + attributeIndent + endMarker + "\n"

	newFlake := insertBlockAtPosition(flake, block, endIdx, baseIndent)

	if err := os.WriteFile(filePath, []byte(newFlake), 0644); err != nil {
		return fmt.Errorf("could not write to %s: %w", filePath, err)
	}

	if !strings.Contains(newFlake, startMarker) {
		return fmt.Errorf("failed to insert %sPhase block into mkDerivation", phaseName)
	}
	return nil
}

// update existing phase block content
func updatePhaseBlock(flake, filePath string, scriptContent []byte, startIdx int, startMarker, endMarker string) error {
	rest := flake[startIdx:]
	endRel := strings.Index(rest, endMarker)
	if endRel == -1 {
		return fmt.Errorf("end marker %q not found in %s", endMarker, filePath)
	}
	endIdx := startIdx + endRel + len(endMarker)

	baseIndent := getBaseIndentationFromPhase(flake, startIdx)
	attributeIndent := baseIndent + "  "
	contentIndent := baseIndent + "    "

	indentedContent := indentScriptContent(scriptContent, contentIndent)
	replacement := attributeIndent + startMarker + "\n" + indentedContent + "\n" + attributeIndent + endMarker

	lineStart := strings.LastIndex(flake[:startIdx], "\n")
	if lineStart == -1 {
		lineStart = 0
	} else {
		lineStart++
	}

	newFlake := flake[:lineStart] + replacement + flake[endIdx:]

	if err := os.WriteFile(filePath, []byte(newFlake), 0644); err != nil {
		return fmt.Errorf("could not write to %s: %w", filePath, err)
	}
	return nil
}

// find closing brace for mkDerivation block
func findClosingBrace(flake string, start int) int {
	depth := 1
	endIdx := start
	for endIdx < len(flake) {
		if flake[endIdx] == '{' {
			depth++
		} else if flake[endIdx] == '}' {
			depth--
			if depth == 0 {
				break
			}
		}
		endIdx++
	}
	return endIdx
}

// get base indentation by searching for defaultPackage
func getBaseIndentation(flake string, blockStart int) string {
	searchEnd := blockStart + 50
	if searchEnd > len(flake) {
		searchEnd = len(flake)
	}
	searchText := flake[:searchEnd]
	lines := strings.Split(searchText, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		if strings.Contains(strings.TrimSpace(line), "defaultPackage =") {
			return getLineIndentation(line)
		}
	}

	// fallback to attribute indent
	start := blockStart + strings.Index(flake[blockStart:], "{") + 1
	blockContent := flake[start:]
	blockLines := strings.Split(blockContent, "\n")
	for _, line := range blockLines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "=") && !strings.HasPrefix(trimmed, "}") {
			lineIndent := getLineIndentation(line)
			if len(lineIndent) >= 2 {
				return lineIndent[:len(lineIndent)-2]
			}
		}
	}
	return ""
}

// get base indentation from existing phase block
func getBaseIndentationFromPhase(flake string, startIdx int) string {
	searchEnd := startIdx + 50
	if searchEnd > len(flake) {
		searchEnd = len(flake)
	}
	searchText := flake[:searchEnd]
	lines := strings.Split(searchText, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		if strings.Contains(strings.TrimSpace(line), "defaultPackage =") {
			return getLineIndentation(line)
		}
	}

	// fallback to phase indent
	phaseLineStart := strings.LastIndex(flake[:startIdx], "\n")
	if phaseLineStart != -1 {
		phaseLine := flake[phaseLineStart+1 : startIdx]
		phaseIndent := getLineIndentation(phaseLine)
		if len(phaseIndent) >= 2 {
			return phaseIndent[:len(phaseIndent)-2]
		}
	}
	return ""
}

// indent script content with proper indentation
func indentScriptContent(scriptContent []byte, contentIndent string) string {
	rawLines := strings.Split(strings.TrimRight(string(scriptContent), "\n"), "\n")
	for i, l := range rawLines {
		if l == "" {
			rawLines[i] = ""
		} else {
			rawLines[i] = contentIndent + l
		}
	}
	return strings.Join(rawLines, "\n")
}

// insert block at the correct position in mkDerivation
func insertBlockAtPosition(flake, block string, endIdx int, baseIndent string) string {
	closingBraceLineStart := strings.LastIndex(flake[:endIdx], "\n")
	closingBraceLineEnd := strings.Index(flake[endIdx:], "\n")

	if closingBraceLineStart != -1 && closingBraceLineEnd != -1 {
		closingBraceLine := flake[endIdx : endIdx+closingBraceLineEnd]
		if strings.TrimSpace(closingBraceLine) == "}" || strings.TrimSpace(closingBraceLine) == "};" {
			newFlake := flake[:endIdx] + block + baseIndent + "};"
			nextLineStart := endIdx + len(closingBraceLine)
			return newFlake + flake[nextLineStart:]
		}
	}
	return flake[:endIdx] + block + flake[endIdx:]
}

// apply buildPhase script
func applyBuildPhaseScript(buildScriptPath string) error {
	return applyPhaseScript(buildScriptPath, "build")
}

// apply installPhase script
func applyInstallPhaseScript(installScriptPath string) error {
	return applyPhaseScript(installScriptPath, "install")
}

// apply packages to flake.nix
func applyPackagesToFlake(filePath string) error {
	// read flake.nix file
	flakeBytes, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("could not read %s: %w", filePath, err)
	}
	flake := string(flakeBytes)

	// Load package metadata from YAML
	yamlBytes, err := os.ReadFile(".flk/derivation/package.yml")
	if err != nil {
		return fmt.Errorf("could not read .flk/derivation/package.yml: %w", err)
	}
	var pkg PackageYAML
	if err := yaml.Unmarshal(yamlBytes, &pkg); err != nil {
		return fmt.Errorf("could not unmarshal .flk/derivation/package.yml: %w", err)
	}
	pname := pkg.Pname
	if pname == "" {
		pname = "default"
	}
	version := pkg.Version
	if version == "" {
		version = "0.1"
	}
	src := pkg.Src
	if src == "" {
		src = "./."
	}

	// Update pname
	flake = updateFieldInDefaultPackage(flake, "pname", fmt.Sprintf("\"%s\"", pname))
	// Update version
	flake = updateFieldInDefaultPackage(flake, "version", fmt.Sprintf("\"%s\"", version))
	// Update src
	flake = updateFieldInDefaultPackage(flake, "src", src)

	// find buildInputs = [] block
	buildInputsStart := strings.Index(flake, "buildInputs = [")
	buildInputsEnd := strings.Index(flake[buildInputsStart:], "]")
	if buildInputsStart != -1 && buildInputsEnd != -1 {
		indent := ""
		for i := buildInputsStart - 1; i >= 0; i-- {
			if flake[i] == '\n' {
				j := i + 1
				for j < buildInputsStart && (flake[j] == ' ' || flake[j] == '\t') {
					indent += string(flake[j])
					j++
				}
				break
			}
		}
		// set buildInputs indent
		extraIndent := indent + "  "
		before := flake[:buildInputsStart+len("buildInputs = [")]
		after := flake[buildInputsStart+buildInputsEnd:]
		var pkgsStr string
		for _, pkgName := range pkg.Packages {
			pkgsStr += "\n" + extraIndent + "pkgs." + pkgName
		}
		if len(pkg.Packages) > 0 {
			pkgsStr += "\n" + indent
		}
		flake = before + pkgsStr + after
	}

	if err := os.WriteFile(filePath, []byte(flake), 0644); err != nil {
		return fmt.Errorf("could not write to %s: %w", filePath, err)
	}
	return nil
}

// structure of .flk/package.yml
type PackageYAML struct {
	Pname    string   `yaml:"pname"`
	Version  string   `yaml:"version"`
	Src      string   `yaml:"src"`
	Packages []string `yaml:"packages"`
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
