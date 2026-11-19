package preview

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/yorukot/superfile/src/internal/common"
)

func getBatSyntaxHighlightedContent(
	itemPath string,
	previewLine int,
	background string,
	batCmd string,
) (string, error) {
	// --plain: use the plain style without line numbers and decorations
	// --force-colorization: force colorization for non-interactive shell output
	// --line-range <:m>: only read from line 1 to line "m"
	batArgs := []string{itemPath, "--plain", "--force-colorization", "--line-range", fmt.Sprintf(":%d", previewLine)}

	// set timeout for the external command execution to 500ms max
	ctx, cancel := context.WithTimeout(context.Background(), common.DefaultPreviewTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, batCmd, batArgs...)

	fileContentBytes, err := cmd.Output()
	if err != nil {
		slog.Error("Error render code highlight", "error", err)
		return "", err
	}

	fileContent := string(fileContentBytes)
	if !common.Config.TransparentBackground {
		fileContent = setBatBackground(fileContent, background)
	}
	return fileContent, nil
}

func setBatBackground(input string, background string) string {
	tokens := strings.Split(input, "\x1b[0m")
	backgroundStyle := lipgloss.NewStyle().Background(lipgloss.Color(background))
	for idx, token := range tokens {
		tokens[idx] = backgroundStyle.Render(token)
	}
	return strings.Join(tokens, "\x1b[0m")
}

// Check if bat is an executable in PATH and whether to use bat or batcat as command
func checkBatCmd() string {
	if _, err := exec.LookPath("bat"); err == nil {
		return "bat"
	}
	// on ubuntu bat executable is called batcat
	if _, err := exec.LookPath("batcat"); err == nil {
		return "batcat"
	}
	return ""
}

func isImageFile(filename string) bool {
	return common.ImageExtensions[strings.ToLower(filepath.Ext(filename))]
}

// findPython returns the python executable name, or empty string.
func findPython() string {
	for _, name := range []string{"python", "python3"} {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}
	return ""
}

// isDocumentFile returns the converter tool name if the file is a supported document, or empty string.
func isDocumentFile(filename string) string {
	return common.DocumentExtensions[strings.ToLower(filepath.Ext(filename))]
}

// checkDocTools detects which document conversion tools are available.
func checkDocTools() (hasPdftotext bool, hasPandoc bool) {
	_, err := exec.LookPath("pdftotext")
	hasPdftotext = err == nil
	_, err = exec.LookPath("pandoc")
	hasPandoc = err == nil
	return
}

// getDocumentTextContent converts a document file to plain text for preview.
// Tool availability is checked at call time rather than startup, because the
// PATH visible to the Go process may differ from what was seen during init.
func getDocumentTextContent(itemPath string, converter string, previewHeight int, _ bool, _ bool) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*common.DefaultPreviewTimeout)
	defer cancel()

	var cmd *exec.Cmd

	switch converter {
	case "pdftotext":
		pdfCmd, err := exec.LookPath("pdftotext")
		if err != nil {
			return "", fmt.Errorf("pdftotext not found in PATH")
		}
		// pdftotext -l <pages> -layout file.pdf -  (output to stdout)
		cmd = exec.CommandContext(ctx, pdfCmd,
			"-l", fmt.Sprintf("%d", max(1, previewHeight/3)),
			"-layout",
			itemPath,
			"-",
		)
	case "pandoc":
		pandocCmd, err := exec.LookPath("pandoc")
		if err != nil {
			return "", fmt.Errorf("pandoc not found in PATH")
		}
		// pandoc converts document formats to GitHub Flavored Markdown
		// (preserves tables, lists, and basic structure better than plain text)
		cmd = exec.CommandContext(ctx, pandocCmd,
			"-t", "gfm",
			"--wrap=none",
			itemPath,
		)
	case "xlsx2text":
		// Try python with openpyxl for reliable XLSX reading
		pythonCmd := findPython()
		if pythonCmd == "" {
			// Fallback to pandoc
			pandocCmd, err := exec.LookPath("pandoc")
			if err != nil {
				return "", fmt.Errorf("neither python nor pandoc available for xlsx")
			}
			cmd = exec.CommandContext(ctx, pandocCmd, "-t", "plain", "--wrap=none", itemPath)
		} else {
			script := `import sys,openpyxl
wb=openpyxl.load_workbook(sys.argv[1],read_only=True,data_only=True)
for ws in wb:
 print(f"  [{ws.title}]")
 rows=[]
 for r in ws.iter_rows(values_only=True):
  rows.append([str(c) if c is not None else "" for c in r])
 if not rows:
  print("  (empty sheet)");print();continue
 nc=max(len(r) for r in rows)
 for r in rows:
  r.extend([""]*(nc-len(r)))
 widths=[max(len(r[i]) for r in rows) for i in range(nc)]
 sep="-+-".join("-"*(w) for w in widths)
 for i,r in enumerate(rows):
  print(" | ".join(c.ljust(w) for c,w in zip(r,widths)))
  if i==0:print(sep)
 print()`
			cmd = exec.CommandContext(ctx, pythonCmd, "-c", script, itemPath)
		}
	case "plaintext":
		// CSV/TSV are already text — read directly, no conversion needed
		return "", fmt.Errorf("use text preview")
	default:
		return "", fmt.Errorf("unknown converter: %s", converter)
	}

	var stderr strings.Builder
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("document conversion failed: %w, stderr: %s", err, stderr.String())
	}

	content := string(output)
	if content == "" {
		return "", fmt.Errorf("empty conversion output")
	}

	// Truncate to preview height
	lines := strings.SplitN(content, "\n", previewHeight+1)
	if len(lines) > previewHeight {
		lines = lines[:previewHeight]
	}
	return strings.Join(lines, "\n"), nil
}
