// Created by DINKIssTyle on 2026. Copyright (C) 2026 DINKI'ssTyle. All rights reserved.

package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const AppName = "PyQuickRun"

func main() {
	// 앱 생성
	a := app.NewWithID("com.dinki.pyquickrun")
	w := a.NewWindow(AppName + " - Linux Native")
	w.Resize(fyne.NewSize(500, 400))
	w.SetFixedSize(true)

	// --- 설정 로드 ---
	prefs := a.Preferences()
	defaultPython := prefs.StringWithFallback("pythonPath", "/usr/bin/python3")

	// --- UI 컴포넌트 ---
	// --- UI 컴포넌트 ---
	statusLabel := widget.NewLabel("Ready to run.")
	statusLabel.Alignment = fyne.TextAlignCenter

	pathEntry := widget.NewEntry()
	pathEntry.SetText(defaultPython)
	pathEntry.PlaceHolder = "e.g. /usr/bin/python3 or ~/venv/bin/python"

	pathEntry.OnChanged = func(s string) {
		prefs.SetString("pythonPath", s)
	}

	browseBtn := widget.NewButtonWithIcon("Binary", theme.FileIcon(), func() {
		fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err == nil && reader != nil {
				pathEntry.SetText(reader.URI().Path())
			}
		}, w)
		fileDialog.Show()
	})

	projBtn := widget.NewButtonWithIcon("Project", theme.FolderIcon(), func() {
		folderDialog := dialog.NewFolderOpen(func(list fyne.ListableURI, err error) {
			if err == nil && list != nil {
				// autoDetect(list.Path()) - defined later or move definitions
			}
		}, w)
		folderDialog.Show()
	})

	chkTerminal := widget.NewCheck("Run in Terminal window", func(b bool) {
		prefs.SetBool("useTerminal", b)
	})
	chkTerminal.SetChecked(prefs.BoolWithFallback("useTerminal", false))

	chkClose := widget.NewCheck("Close window after successful execution", func(b bool) {
		prefs.SetBool("closeOnSuccess", b)
	})
	chkClose.SetChecked(prefs.BoolWithFallback("closeOnSuccess", false))

	// --- 실행 로직 ---
	var runScript func(string, *PqrHeader, *bool, *bool)

	saveAndRunGo := func(scriptPath string, terminal bool, category string) {
		file, err := os.ReadFile(scriptPath)
		if err != nil {
			return
		}
		lines := strings.Split(string(file), "\n")
		var tagParts []string
		tagParts = append(tagParts, fmt.Sprintf("term=%v", terminal))
		if strings.TrimSpace(category) != "" {
			tagParts = append(tagParts, fmt.Sprintf("cat=%s", category))
		}
		headerTag := "#pqr " + strings.Join(tagParts, "; ")

		insertIdx := 0
		if len(lines) > 0 && strings.HasPrefix(lines[0], "#!") {
			insertIdx = 1
		}

		newLines := make([]string, 0, len(lines)+1)
		newLines = append(newLines, lines[:insertIdx]...)
		newLines = append(newLines, headerTag)
		newLines = append(newLines, lines[insertIdx:]...)

		err = os.WriteFile(scriptPath, []byte(strings.Join(newLines, "\n")), 0644)
		if err != nil {
			statusLabel.SetText("Error saving header: " + err.Error())
			return
		}
		runScript(scriptPath, nil, nil, nil)
	}

	showOptionDialog := func(scriptPath string) {
		catEntry := widget.NewEntry()
		catEntry.PlaceHolder = "e.g. Utility, Tool, AI"

		termCheck := widget.NewCheck("Run in Terminal window", nil)
		termCheck.SetChecked(false)

		closeCheck := widget.NewCheck("Close window after successful execution", nil)
		closeCheck.SetChecked(prefs.BoolWithFallback("closeOnSuccess", false))

		headerIcon := widget.NewIcon(theme.QuestionIcon())
		headerIcon.SetResource(theme.QuestionIcon())

		form := container.NewVBox(
			container.NewCenter(container.NewPadded(widget.NewLabelWithStyle("No #pqr header found", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}))),
			widget.NewLabel("Category:"),
			catEntry,
			widget.NewSeparator(),
			widget.NewLabel("Next time this script will:"),
			termCheck,
			closeCheck,
			layout.NewSpacer(),
			widget.NewLabelWithStyle("Shortcuts: Run Now (Ctrl+D) / Save & Run (Ctrl+S)", fyne.TextAlignCenter, fyne.TextStyle{Italic: true}),
		)

		var dia dialog.Dialog

		runNow := func() {
			dia.Hide()
			useT := termCheck.Checked
			closeW := closeCheck.Checked
			runScript(scriptPath, nil, &useT, &closeW)
		}

		saveRun := func() {
			dia.Hide()
			saveAndRunGo(scriptPath, termCheck.Checked, catEntry.Text)
		}

		runBtn := widget.NewButton("Run Now (Ctrl+D)", runNow)
		saveBtn := widget.NewButton("Save & Run (Ctrl+S)", saveRun)
		saveBtn.Importance = widget.HighImportance

		dialogContent := container.NewPadded(container.NewVBox(
			container.NewCenter(widget.NewIcon(theme.HelpIcon())),
			widget.NewCard("", "", form),
			container.NewHBox(layout.NewSpacer(), runBtn, saveBtn, layout.NewSpacer()),
		))

		dia = dialog.NewCustom("Notice", "Cancel", dialogContent, w)
		dia.Resize(fyne.NewSize(450, 420))
		dia.Show()
	}

	runScript = func(scriptPath string, headerOverride *PqrHeader, terminalOverride *bool, closeOverride *bool) {
		pythonBin := pathEntry.Text
		useTerm := chkTerminal.Checked
		closeWin := chkClose.Checked
		workDir := filepath.Dir(scriptPath)

		var header PqrHeader
		if headerOverride != nil {
			header = *headerOverride
		} else {
			header = scanPqrHeaderGo(scriptPath)
		}

		if !header.HasPqr && terminalOverride == nil {
			showOptionDialog(scriptPath)
			return
		}

		if header.Interpreter != "" {
			pythonBin = header.Interpreter
		} else {
			// .venv 자동 감지
			venvCandidates := []string{
				filepath.Join(workDir, ".venv", "bin", "python"),
				filepath.Join(workDir, ".venv", "bin", "python3"),
			}
			for _, c := range venvCandidates {
				if _, err := os.Stat(c); err == nil {
					pythonBin = c
					statusLabel.SetText("Using local .venv: " + pythonBin)
					break
				}
			}
		}

		if header.TermOverride != nil {
			useTerm = *header.TermOverride
		}
		if terminalOverride != nil {
			useTerm = *terminalOverride
		}
		if closeOverride != nil {
			closeWin = *closeOverride
		}

		statusLabel.SetText("Running: " + filepath.Base(scriptPath))

		if useTerm {
			cmdStr := fmt.Sprintf("cd '%s' && '%s' '%s'; echo; echo 'Exit Code: $?'; read -p 'Press Enter to exit...'", workDir, pythonBin, scriptPath)

			terminals := [][]string{
				{"gnome-terminal", "--", "bash", "-c"},
				{"konsole", "-e", "bash", "-c"},
				{"xfce4-terminal", "-x", "bash", "-c"},
				{"xterm", "-e", "bash", "-c"},
			}

			launched := false
			for _, term := range terminals {
				if _, err := exec.LookPath(term[0]); err == nil {
					args := append(term[1:], cmdStr)
					exec.Command(term[0], args...).Start()
					statusLabel.SetText("Launched in " + term[0])
					launched = true
					if closeWin {
						w.Close()
					}
					break
				}
			}
			
			if !launched {
				appleScript := fmt.Sprintf(`tell application "Terminal" to do script "%s"`, cmdStr)
				if _, err := exec.LookPath("osascript"); err == nil {
					exec.Command("osascript", "-e", appleScript).Start()
					statusLabel.SetText("Launched in macOS Terminal")
					launched = true
					if closeWin {
						w.Close()
					}
				}
			}

			if !launched {
				statusLabel.SetText("Error: No supported terminal found.")
			}

		} else {
			cmd := exec.Command(pythonBin, scriptPath)
			cmd.Dir = workDir
			output, err := cmd.CombinedOutput()

			if err == nil {
				statusLabel.SetText("Success (Exit Code 0)")
				if closeWin {
					go func() {
						time.Sleep(time.Second)
						w.Close()
					}()
				}
			} else {
				statusLabel.SetText("Failed: " + err.Error())
				dialog.ShowInformation("Execution Error", string(output), w)
			}
		}
	}

	// --- 드래그 앤 드롭 ---
	w.SetOnDropped(func(pos fyne.Position, uris []fyne.URI) {
		if len(uris) > 0 {
			path := uris[0].Path()
			if fi, err := os.Stat(path); err == nil && fi.IsDir() {
				// Re-using autoDetect logic
				candidates := []string{
					filepath.Join(path, ".venv", "bin", "python"),
					filepath.Join(path, "venv", "bin", "python"),
				}
				for _, c := range candidates {
					if _, err := os.Stat(c); err == nil {
						pathEntry.SetText(c)
						statusLabel.SetText("Auto-selected: " + c)
						break
					}
				}
			} else if strings.HasSuffix(strings.ToLower(path), ".py") {
				runScript(path, nil, nil, nil)
			} else {
				statusLabel.SetText("Error: Only .py files or Project folders supported")
			}
		}
	})

	// --- 드롭 존 비주얼 개선 ---
	dropIcon := widget.NewIcon(theme.DocumentIcon())
	dropText := widget.NewLabelWithStyle("Drag & Drop .py file here", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	dropSubText := widget.NewLabelWithStyle("or drop anywhere in window", fyne.TextAlignCenter, fyne.TextStyle{Italic: true})

	dropContent := container.NewPadded(container.NewVBox(
		layout.NewSpacer(),
		container.NewCenter(dropIcon),
		dropText,
		dropSubText,
		layout.NewSpacer(),
	))
	dropCard := widget.NewCard("", "", dropContent)

	// --- 레이아웃 조립 ---
	mainContent := container.NewVBox(
		container.NewCenter(widget.NewLabelWithStyle(AppName, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})),
		widget.NewSeparator(),
		container.NewPadded(container.NewVBox(
			widget.NewLabel("Interpreter Path (uv or python):"),
			container.NewBorder(nil, nil, nil, container.NewHBox(browseBtn, projBtn), pathEntry),
			container.NewVBox(chkTerminal, chkClose),
		)),
		container.NewPadded(dropCard),
		layout.NewSpacer(),
		widget.NewSeparator(),
		statusLabel,
		container.NewHBox(layout.NewSpacer(), widget.NewLabelWithStyle("© 2026 DINKIssTyle", fyne.TextAlignTrailing, fyne.TextStyle{Italic: true})),
	)

	w.SetContent(container.NewPadded(mainContent))

	if len(os.Args) > 1 {
		argPath := os.Args[1]
		if _, err := os.Stat(argPath); err == nil {
			if strings.HasSuffix(strings.ToLower(argPath), ".py") {
				go func() {
					time.Sleep(200 * time.Millisecond)
					runScript(argPath, nil, nil, nil)
				}()
			}
		}
	}

	w.ShowAndRun()
}

type PqrHeader struct {
	Interpreter  string
	TermOverride *bool
	Category     string
	HasPqr       bool
}

func scanPqrHeaderGo(scriptPath string) PqrHeader {
	var header PqrHeader
	file, err := os.Open(scriptPath)
	if err != nil {
		return header
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for i := 0; i < 20 && scanner.Scan(); i++ {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(strings.ToLower(line), "#pqr") {
			header.HasPqr = true
			remainder := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(line), "#pqr"))
			parts := strings.Split(remainder, ";")
			for _, part := range parts {
				kv := strings.Split(strings.TrimSpace(part), "=")
				if len(kv) == 2 {
					key := strings.TrimSpace(kv[0])
					val := strings.TrimSpace(kv[1])
					if key == "linux" && val != "" {
						header.Interpreter = val
					} else if key == "cat" {
						header.Category = val
					} else if key == "term" {
						b := false
						if val == "true" || val == "1" || val == "yes" {
							b = true
						}
						header.TermOverride = &b
					}
				}
			}
			return header
		}
	}
	return header
}
