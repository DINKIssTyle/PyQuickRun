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

	"net/url"

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
				// autoDetect logic moved here or defined locally
				autoDetect(list.Path(), pathEntry, statusLabel, w)
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
		if abs, err := filepath.Abs(scriptPath); err == nil {
			scriptPath = abs
		}

		pythonBin := pathEntry.Text
		useTerm := chkTerminal.Checked
		closeWin := chkClose.Checked
		scriptDir := filepath.Dir(scriptPath)
		workDir := scriptDir
		sourceMsg := "Default"

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

		foundInterpreter := ""
		if header.Interpreter != "" {
			foundInterpreter = header.Interpreter
			pythonBin = foundInterpreter
			sourceMsg = "#qpr"
		}

		// Search for .venv (Upward)
		projectRoot := ""
		tempDir := scriptDir
		for i := 0; i < 5; i++ {
			candidates := []string{
				filepath.Join(tempDir, ".venv"),
				filepath.Join(tempDir, "venv"),
			}
			for _, c := range candidates {
				if info, err := os.Stat(c); err == nil && info.IsDir() {
					binCandidates := []string{
						filepath.Join(c, "bin", "python"),
						filepath.Join(c, "bin", "python3"),
					}
					for _, bc := range binCandidates {
						if binfo, berr := os.Stat(bc); berr == nil && !binfo.IsDir() {
							if foundInterpreter == "" {
								pythonBin = bc
								sourceMsg = "Auto(.venv)"
							}
							projectRoot = tempDir
							break
						}
					}
				}
				if projectRoot != "" {
					break
				}
			}
			if projectRoot != "" {
				break
			}
			parent := filepath.Dir(tempDir)
			if parent == tempDir {
				break
			}
			tempDir = parent
		}

		venvDir := ""
		absBin, _ := filepath.Abs(pythonBin)
		binDir := filepath.Dir(absBin)
		if _, err := os.Stat(filepath.Join(filepath.Dir(binDir), "pyvenv.cfg")); err == nil {
			venvDir = filepath.Dir(binDir)
			if projectRoot != "" {
				workDir = projectRoot
			}
		}

		if foundInterpreter != "" && !filepath.IsAbs(foundInterpreter) {
			pythonBin = filepath.Join(scriptDir, foundInterpreter)
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

		statusLabel.SetText(fmt.Sprintf("Running %s via %s", filepath.Base(scriptPath), sourceMsg))

		if useTerm {
			envPrefix := ""
			if venvDir != "" {
				envPrefix = fmt.Sprintf("export VIRTUAL_ENV=%s; export PATH=%s:$PATH; ", shellQuote(venvDir), shellQuote(binDir))
			}
			cmdStr := fmt.Sprintf("%scd %s && %s %s; echo; echo 'Exit Code: $?'; read -p 'Press Enter to exit...'",
				envPrefix, shellQuote(workDir), shellQuote(pythonBin), shellQuote(scriptPath))

			terminals := [][]string{
				{"ptyxis", "--", "bash", "-c"},
				{"kgx", "--", "bash", "-c"},
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
				if _, err := exec.LookPath("x-terminal-emulator"); err == nil {
					exec.Command("x-terminal-emulator", "-e", "bash", "-c", cmdStr).Start()
					statusLabel.SetText("Launched in x-terminal-emulator")
					launched = true
					if closeWin {
						w.Close()
					}
				} else {
					statusLabel.SetText("Error: No supported terminal found.")
				}
			}

		} else {
			cmd := exec.Command(pythonBin, scriptPath)
			cmd.Dir = workDir
			cmd.Env = os.Environ()
			if venvDir != "" {
				cmd.Env = append(cmd.Env, "VIRTUAL_ENV="+venvDir)
				pathFound := false
				for i, env := range cmd.Env {
					if strings.HasPrefix(strings.ToUpper(env), "PATH=") {
						cmd.Env[i] = "PATH=" + binDir + ":" + env[5:]
						pathFound = true
						break
					}
				}
				if !pathFound {
					cmd.Env = append(cmd.Env, "PATH="+binDir+":"+os.Getenv("PATH"))
				}
			}

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
				autoDetect(path, pathEntry, statusLabel, w)
			} else if strings.HasSuffix(strings.ToLower(path), ".py") {
				runScript(path, nil, nil, nil)
			} else {
				statusLabel.SetText("Error: Only .py files or Project folders supported")
			}
		}
	})

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
		arg := os.Args[1]
		targetPath := ""
		if strings.HasPrefix(arg, "file://") {
			if u, err := url.Parse(arg); err == nil {
				targetPath = u.Path
			}
		} else {
			targetPath = arg
		}

		if targetPath != "" {
			if abs, err := filepath.Abs(targetPath); err == nil {
				targetPath = abs
			}
			if info, err := os.Stat(targetPath); err == nil && !info.IsDir() {
				if strings.HasSuffix(strings.ToLower(targetPath), ".py") {
					go func() {
						time.Sleep(200 * time.Millisecond)
						runScript(targetPath, nil, nil, nil)
					}()
				}
			}
		}
	}

	w.ShowAndRun()
}

// autoDetect logic
func autoDetect(dir string, pathEntry *widget.Entry, statusLabel *widget.Label, w fyne.Window) {
	candidates := []string{
		filepath.Join(dir, ".venv", "bin", "python"),
		filepath.Join(dir, ".venv", "bin", "python3"),
		filepath.Join(dir, "venv", "bin", "python"),
		filepath.Join(dir, "venv", "bin", "python3"),
		filepath.Join(dir, "env", "bin", "python"),
	}

	found := ""
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			found = c
			break
		}
	}

	if found != "" {
		pathEntry.SetText(found)
		statusLabel.SetText("Auto-selected: " + found)
	} else {
		statusLabel.SetText("No venv found in: " + filepath.Base(dir))
		dialog.ShowInformation("No Venv Found", "Could not find standard virtualenv (bin/python) in:\n"+dir, w)
	}
}

// shellQuote returns a shell-escaped version of the string.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
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
