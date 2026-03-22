package main

import (
	"archive/tar"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	source := flag.String("source", "", "directory to archive")
	output := flag.String("output", "", "output tar.gz path")
	flag.Parse()

	if strings.TrimSpace(*source) == "" || strings.TrimSpace(*output) == "" {
		fmt.Fprintln(os.Stderr, "both --source and --output are required")
		os.Exit(1)
	}

	if err := packagePlugin(*source, *output); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func packagePlugin(source, output string) error {
	source = filepath.Clean(source)
	output = filepath.Clean(output)

	info, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("stat source: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("source must be a directory: %s", source)
	}

	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	file, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	defer file.Close()

	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	baseDir := filepath.Base(source)
	return filepath.Walk(source, func(path string, entry os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		rel, err := filepath.Rel(source, path)
		if err != nil {
			return fmt.Errorf("resolve relative path: %w", err)
		}

		name := filepath.ToSlash(filepath.Join(baseDir, rel))
		if rel == "." {
			name = filepath.ToSlash(baseDir)
		}

		header, err := tar.FileInfoHeader(entry, "")
		if err != nil {
			return fmt.Errorf("create tar header for %s: %w", path, err)
		}

		header.Name = name
		header.Uid = 0
		header.Gid = 0
		header.Uname = "root"
		header.Gname = "root"
		header.Mode = archiveMode(path, entry)
		if entry.IsDir() && !strings.HasSuffix(header.Name, "/") {
			header.Name += "/"
		}

		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("write tar header for %s: %w", path, err)
		}

		if entry.IsDir() {
			return nil
		}

		in, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open file %s: %w", path, err)
		}
		defer in.Close()

		if _, err := io.Copy(tarWriter, in); err != nil {
			return fmt.Errorf("write file %s: %w", path, err)
		}

		return nil
	})
}

func archiveMode(path string, info os.FileInfo) int64 {
	if info.IsDir() {
		return 0o755
	}

	slashed := filepath.ToSlash(path)
	if strings.Contains(slashed, "/server/dist/") {
		return 0o755
	}

	return 0o644
}
