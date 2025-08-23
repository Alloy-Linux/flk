package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

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
			f, err := os.Create("flake.nix")
			if err != nil {
				log.Fatal("Could not create flake.nix:", err)
			}
			defer f.Close()

			_, err = f.Write([]byte(boilerplateContent))
			if err != nil {
				log.Fatal("Could not write to flake.nix:", err)
			}

			fmt.Println("Initialized new flake: flake.nix")

			filePath, err := resolveFile(file)
			if err != nil {
				log.Fatal(err)
			}
			generateInputs(filePath)
			fmt.Println("Done")
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
				log.Fatal("Error adding package:", err)
				fmt.Println("Retrying...")
			}

			generateInputs(filePath)
			fmt.Println("Done")
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
			fmt.Println("Done")
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
				log.Fatal("Error reading packages:", err)
			}

			if len(pkgs) == 0 {
				fmt.Println("No packages found.")
			} else {
				fmt.Println("Packages in", filePath)
			}
			for _, p := range pkgs {
				fmt.Println("- " + p)
			}
		},
	}

	// Add --file flag to subcommands
	addCmd.Flags().StringVarP(&file, "file", "f", "", "Path to flake.nix file")
	removeCmd.Flags().StringVarP(&file, "file", "f", "", "Path to flake.nix file")
	listCmd.Flags().StringVarP(&file, "file", "f", "", "Path to flake.nix file")

	// Command tree
	flakeCmd.AddCommand(initCmd)
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

	// Check if flake.nix exists in current directory
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
