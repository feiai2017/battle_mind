package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/feiai2017/battle_mind/internal/model"
	"github.com/feiai2017/battle_mind/internal/service"
)

func main() {
	input := flag.String("input", "", "battle report json file or directory")
	outputDir := flag.String("output-dir", "out", "directory for generated analyze request json files")
	flag.Parse()

	if strings.TrimSpace(*input) == "" {
		fmt.Fprintln(os.Stderr, "usage: go run ./cmd/convertbatch -input <file-or-dir> [-output-dir out]")
		os.Exit(1)
	}

	files, err := collectInputFiles(*input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "collect input files failed: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create output dir failed: %v\n", err)
		os.Exit(1)
	}

	converted := 0
	for _, file := range files {
		if err := convertFile(file, *outputDir); err != nil {
			fmt.Fprintf(os.Stderr, "convert %s failed: %v\n", file, err)
			os.Exit(1)
		}
		converted++
	}

	fmt.Printf("converted %d file(s) into %s\n", converted, *outputDir)
}

func collectInputFiles(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		if !isBattleReportJSON(path) {
			return nil, fmt.Errorf("input file must be a .json file: %s", path)
		}
		return []string{path}, nil
	}

	var files []string
	err = filepath.WalkDir(path, func(current string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if isBattleReportJSON(current) {
			files = append(files, current)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, errors.New("no battle report json files found")
	}
	return files, nil
}

func isBattleReportJSON(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	return strings.HasSuffix(name, ".json") && !strings.HasSuffix(name, ".analyze_request.json")
}

func convertFile(inputPath, outputDir string) error {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return err
	}

	var report model.BattleReport
	if err := json.Unmarshal(data, &report); err != nil {
		return fmt.Errorf("parse battle report json: %w", err)
	}

	result := service.ConvertBattleReportToAnalyzeRequest(report)
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal analyze request: %w", err)
	}

	baseName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	outputPath := filepath.Join(outputDir, baseName+".analyze_request.json")
	if err := os.WriteFile(outputPath, append(encoded, '\n'), 0o644); err != nil {
		return err
	}

	fmt.Printf("%s -> %s\n", inputPath, outputPath)
	return nil
}
