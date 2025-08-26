package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// Boilerplate content for new flake.nix
var boilerplateContent = `
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils, ... }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
      in {
        devShells = {
          default = pkgs.mkShell {
            packages = [

            ];

            shellHook = ''
              echo "Development environment loaded"
            '';
          };
        };
      }
    );
}
`

func main() {
	var file string // --file file

	var rootCmd = &cobra.Command{
		Use:   "flk",
		Short: "Flk is a simple tool to manage nix files",
	}

	// `flk flake`
	var flakeCmd = &cobra.Command{
		Use:   "flake",
		Short: "Manage Nix flakes",
	}

	// `flk flake init`
	var initCmd = &cobra.Command{
		Use:   "init",
		Short: "Initialize a new flake",
		Run: func(cmd *cobra.Command, args []string) {
			// determine target flake path (use --file if provided)
			target := file
			if target == "" {
				target = "flake.nix"
			}

			// create/write flake first
			f, err := os.Create(target)
			if err != nil {
				log.Fatal(err)
			}
			_, err = f.Write([]byte(boilerplateContent))
			f.Close()
			if err != nil {
				log.Fatal(err)
			}

			if err := generateFlk(target); err != nil {
				log.Println(err)
			}

			filePath, err := resolveFile(target)
			if err != nil {
				log.Fatal(err)
			}
			generateInputs(filePath)
		},
	}

	// `flk package`
	var packageCmd = &cobra.Command{
		Use:   "package",
		Short: "Manage packages",
	}

	// `flk package add <package>`
	var addCmd = &cobra.Command{
		Use:   "add <package>",
		Short: "Add a package",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			pkg := args[0]
			filePath, err := resolveFile(file)
			if err != nil {
				log.Fatal(err)
			}
			err = addPackage(filePath, pkg)

			if err != nil {
				log.Fatal(err)
			}

			generateInputs(filePath)
		},
	}

	// `flk package remove <package>`
	var removeCmd = &cobra.Command{
		Use:   "remove <package>",
		Short: "Remove a package",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			pkg := args[0]
			filePath, err := resolveFile(file)
			if err != nil {
				log.Fatal(err)
			}
			removePackage(filePath, pkg)

			generateInputs(filePath)
		},
	}

	// `flk package list`
	var listCmd = &cobra.Command{
		Use:   "list",
		Short: "List all packages",
		Run: func(cmd *cobra.Command, args []string) {
			filePath, err := resolveFile(file)
			if err != nil {
				log.Fatal(err)
			}
			pkgs, err := getPackages(filePath)
			if err != nil {
				log.Fatal(err)
			}

			if len(pkgs) == 0 {
				log.Println("No packages found")
			} else {
				log.Println("Packages:")
				for _, p := range pkgs {
					log.Printf(" - %s", p)
				}
			}
		},
	}

	// `flk flake apply`
	var applyCmd = &cobra.Command{
		Use:   "apply",
		Short: "Apply changes from .flk to flake.nix",
		Run: func(cmd *cobra.Command, args []string) {
			// gets current working directory
			wd, err := os.Getwd()
			if err != nil {
				log.Fatal(err)
			}
			if err := ensureShellHookBlock(wd + "/flake.nix"); err != nil {
				log.Fatal(err)
			}
			if err := applyToFlake(wd); err != nil {
				log.Fatal(err)
			}

		},
	}

	// Add --file flag to subcommands
	addCmd.Flags().StringVarP(&file, "file", "f", "", "Path to flake.nix file")
	removeCmd.Flags().StringVarP(&file, "file", "f", "", "Path to flake.nix file")
	listCmd.Flags().StringVarP(&file, "file", "f", "", "Path to flake.nix file")
	initCmd.Flags().StringVarP(&file, "file", "f", "", "Path to flake.nix file")

	// Command tree
	flakeCmd.AddCommand(initCmd)
	flakeCmd.AddCommand(applyCmd)
	packageCmd.AddCommand(addCmd, removeCmd, listCmd)
	rootCmd.AddCommand(flakeCmd, packageCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

// returns the file path to use e.g the flake.nix
func resolveFile(flag string) (string, error) {
	if flag != "" {
		return flag, nil
	}

	defaultPath := "flake.nix"
	if _, err := os.Stat(defaultPath); err == nil {
		absPath, err := filepath.Abs(defaultPath)
		if err != nil {
			return "", fmt.Errorf("could not resolve flake.nix path: %w", err)
		}
		return absPath, nil
	}

	return "", fmt.Errorf("no --file given and flake.nix not found in current directory")
}

// function that makes a .flk folder and extracts shellHook from the provided flake file
func generateFlk(flakePath string) error {
	shellHookLines, shErr := getLinesBetween(flakePath, "shellHook = ''", "'';")

	pkgs, pkErr := getPackages(flakePath)

	if shErr != nil && (pkErr != nil || len(pkgs) == 0) {
		return nil
	}

	// ensure .flk exists
	if err := os.MkdirAll(".flk", 0755); err != nil {
		return fmt.Errorf("could not create .flk folder: %w", err)
	}
	// ensure .flk/devenv exists
	if err := os.MkdirAll(".flk/devenv", 0755); err != nil {
		return fmt.Errorf("could not create .flk/devenv folder: %w", err)
	}

	// If shellHook was found, write it out
	if shErr == nil {
		outf, err := os.Create(".flk/devenv/shellhook.sh")
		if err != nil {
			return fmt.Errorf("could not create .flk/devenv/shellhook.sh: %w", err)
		}
		defer outf.Close()

		for _, line := range shellHookLines {
			if _, err := outf.WriteString(line + "\n"); err != nil {
				return fmt.Errorf("could not write to .flk/devenv/shellhook.sh: %w", err)
			}
		}
	}

	// make .flk/derivation
	if err := os.MkdirAll(".flk/derivation", 0755); err != nil {
		return fmt.Errorf("could not create .flk/derivation folder: %w", err)
	}

	// Write a YAML file with pname, version, src, and packages fields
	yf, err := os.Create(".flk/derivation/package.yml")
	if err != nil {
		return fmt.Errorf("could not create .flk/derivation/package.yml: %w", err)
	}
	defer yf.Close()

	pname := "default"
	version := "0.1"
	src := "./."

	if _, err := yf.WriteString(fmt.Sprintf("pname: %s\nversion: %s\nsrc: %s\npackages:\n", pname, version, src)); err != nil {
		return fmt.Errorf("could not write to .flk/derivation/package.yml: %w", err)
	}
	if pkErr == nil && len(pkgs) > 0 {
		for _, p := range pkgs {
			if _, err := yf.WriteString("  - " + p + "\n"); err != nil {
				return fmt.Errorf("could not write to .flk/package.yml: %w", err)
			}
		}
	}

	// Ensure mkDerivation block exists
	if err := ensureDerivation(flakePath); err != nil {
		return fmt.Errorf("could not ensure derivation in %s: %w", flakePath, err)
	}

	// Make a build.sh in /derivation
	bf, err := os.Create(".flk/derivation/build.sh")
	if err != nil {
		return fmt.Errorf("could not create .flk/derivation/build.sh: %w", err)
	}
	defer bf.Close()
	if err := applyBuildPhaseScript(bf.Name()); err != nil {
		return fmt.Errorf("could not apply build phase script: %w", err)
	}

	// Make an install.sh in /derivation
	instf, err := os.Create(".flk/derivation/install.sh")
	if err != nil {
		return fmt.Errorf("could not create .flk/derivation/install.sh: %w", err)
	}
	defer instf.Close()

	// Write default install.sh content
	defaultInstallContent := `

	`
	if _, err := instf.WriteString(defaultInstallContent); err != nil {
		return fmt.Errorf("could not write to .flk/derivation/install.sh: %w", err)
	}

	if err := applyInstallPhaseScript(instf.Name()); err != nil {
		return fmt.Errorf("could not apply install phase script: %w", err)
	}

	return nil
}

func getLinesBetween(filePath, start, end string) ([]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("could not open file: %w", err)
	}
	defer f.Close()

	trimmedStart := strings.TrimSpace(start)
	trimmedEnd := strings.TrimSpace(end)

	scanner := bufio.NewScanner(f)
	var lines []string
	capturing := false

	for scanner.Scan() {
		raw := scanner.Text()
		t := strings.TrimSpace(raw)

		if !capturing {
			if strings.Contains(t, trimmedStart) {
				capturing = true
				if idx := strings.Index(t, trimmedStart); idx != -1 {
					rest := strings.TrimSpace(t[idx+len(trimmedStart):])
					if rest != "" {
						lines = append(lines, rest)
					}
				}
				continue
			}
		} else {
			if strings.Contains(t, trimmedEnd) {
				if idx := strings.Index(t, trimmedEnd); idx > 0 {
					prefix := strings.TrimSpace(t[:idx])
					if prefix != "" {
						lines = append(lines, prefix)
					}
				}
				return lines, nil
			}
			if t != "" {
				lines = append(lines, t)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning file: %w", err)
	}

	return nil, fmt.Errorf("end marker not found")
}
