package license

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetect(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		content  string
		wantSPDX string
	}{
		{
			name:     "MIT",
			filename: "LICENSE",
			content:  "MIT License\n\nCopyright (c) 2026 Test\n\nPermission is hereby granted, free of charge...",
			wantSPDX: "MIT",
		},
		{
			name:     "Apache-2.0",
			filename: "LICENSE",
			content:  "Apache License\nVersion 2.0, January 2004\nhttp://www.apache.org/licenses/",
			wantSPDX: "Apache-2.0",
		},
		{
			name:     "GPL-3.0",
			filename: "COPYING",
			content:  "GNU GENERAL PUBLIC LICENSE\nVersion 3, 29 June 2007\n\nCopyright (C) 2007 Free Software Foundation",
			wantSPDX: "GPL-3.0-only",
		},
		{
			name:     "GPL-2.0",
			filename: "LICENSE",
			content:  "GNU GENERAL PUBLIC LICENSE\nVersion 2, June 1991\n\nCopyright (C) 1989, 1991 Free Software Foundation",
			wantSPDX: "GPL-2.0-only",
		},
		{
			name:     "BSD-3-Clause",
			filename: "LICENSE",
			content:  "BSD 3-Clause License\n\nRedistribution and use in source and binary forms...\nNeither the name of the copyright holder...",
			wantSPDX: "BSD-3-Clause",
		},
		{
			name:     "BSD-2-Clause",
			filename: "LICENSE",
			content:  "BSD 2-Clause License\n\nRedistribution and use in source and binary forms...",
			wantSPDX: "BSD-2-Clause",
		},
		{
			name:     "ISC",
			filename: "LICENSE",
			content:  "ISC License\n\nCopyright (c) 2026\n\nPermission to use, copy, modify...",
			wantSPDX: "ISC",
		},
		{
			name:     "MPL-2.0",
			filename: "LICENSE",
			content:  "Mozilla Public License Version 2.0\n\n1. Definitions",
			wantSPDX: "MPL-2.0",
		},
		{
			name:     "Unlicense",
			filename: "LICENSE",
			content:  "This is free and unencumbered software released into the public domain.",
			wantSPDX: "Unlicense",
		},
		{
			name:     "LGPL-2.1",
			filename: "LICENSE",
			content:  "GNU Lesser General Public License\nVersion 2.1, February 1999",
			wantSPDX: "LGPL-2.1-only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, tt.filename), []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}

			result := Detect(dir)
			if result.SPDX != tt.wantSPDX {
				t.Errorf("Detect().SPDX = %q, want %q", result.SPDX, tt.wantSPDX)
			}
			if result.FilePath != tt.filename {
				t.Errorf("Detect().FilePath = %q, want %q", result.FilePath, tt.filename)
			}
		})
	}
}

func TestDetect_NoFile(t *testing.T) {
	dir := t.TempDir()
	result := Detect(dir)
	if result.SPDX != "" || result.FilePath != "" {
		t.Errorf("Detect(empty dir) = %+v, want zero Result", result)
	}
}

func TestDetect_UnknownContent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "LICENSE"), []byte("Some proprietary license text"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := Detect(dir)
	if result.SPDX != "" {
		t.Errorf("Detect().SPDX = %q, want empty for unknown license", result.SPDX)
	}
	if result.FilePath != "LICENSE" {
		t.Errorf("Detect().FilePath = %q, want %q", result.FilePath, "LICENSE")
	}
}

func TestDetect_SPDXHeader(t *testing.T) {
	dir := t.TempDir()
	content := "// SPDX-License-Identifier: MIT\n\nSome license text..."
	if err := os.WriteFile(filepath.Join(dir, "LICENSE"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	result := Detect(dir)
	if result.SPDX != "MIT" {
		t.Errorf("Detect().SPDX = %q, want %q", result.SPDX, "MIT")
	}
}

func TestDetect_CaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	// Create lowercase "license" file
	if err := os.WriteFile(filepath.Join(dir, "license"), []byte("MIT License\n\nPermission is hereby granted, free of charge..."), 0o644); err != nil {
		t.Fatal(err)
	}

	result := Detect(dir)
	if result.SPDX != "MIT" {
		t.Errorf("Detect().SPDX = %q, want %q", result.SPDX, "MIT")
	}
	if result.FilePath != "license" {
		t.Errorf("Detect().FilePath = %q, want %q (actual filename)", result.FilePath, "license")
	}
}

func TestDetect_LICENSEmd(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "LICENSE.md"), []byte("# MIT License\n\nPermission is hereby granted, free of charge..."), 0o644); err != nil {
		t.Fatal(err)
	}

	result := Detect(dir)
	if result.SPDX != "MIT" {
		t.Errorf("Detect().SPDX = %q, want %q", result.SPDX, "MIT")
	}
	if result.FilePath != "LICENSE.md" {
		t.Errorf("Detect().FilePath = %q, want %q", result.FilePath, "LICENSE.md")
	}
}

func TestDetect_PriorityOrder(t *testing.T) {
	dir := t.TempDir()
	// Create both LICENSE and LICENSE.md — LICENSE should win
	if err := os.WriteFile(filepath.Join(dir, "LICENSE"), []byte("MIT License"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "LICENSE.md"), []byte("Apache License Version 2.0"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := Detect(dir)
	if result.FilePath != "LICENSE" {
		t.Errorf("Detect().FilePath = %q, want %q (higher priority)", result.FilePath, "LICENSE")
	}
}
