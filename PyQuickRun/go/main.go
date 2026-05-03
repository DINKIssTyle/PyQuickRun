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
	pathEntry.PlaceHolder = "/usr/bin/python3"

	pathEntry.OnChanged = func(s string) {
		prefs.SetString("pythonPath", s)
	}

	// --- 자동 감지 로직 ---
	autoDetect := func(dir string) {
		candidates := []string{
			filepath.Join(dir, ".venv", "bin", "python"),
			filepath.Join(dir, ".venv", "bin", "python3"),
			filepath.Join(dir, "venv", "bin", "python"),
			filepath.Join(dir, "venv", "bin", "python3"),
			filepath.Join(dir, "env", "bin", "python"), // Some use 'env'
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
				autoDetect(list.Path())
			}
		}, w)
		folderDialog.Show()
	})

	chkTerminal := widget.NewCheck("Run in Terminal window", func(b bool) {
		prefs.SetBool("useTerminal", b)
	})
	chkTerminal.SetChecked(prefs.BoolWithFallback("useTerminal", false))

	chkClose := widget.NewCheck("Close window after success", func(b bool) {
		prefs.SetBool("closeOnSuccess", b)
	})
	chkClose.SetChecked(prefs.BoolWithFallback("closeOnSuccess", false))

	// --- 실행 로직 ---
	runScript := func(scriptPath string) {
		// Ensure absolute path for consistency
		if abs, err := filepath.Abs(scriptPath); err == nil {
			scriptPath = abs
		}

		pythonBin := pathEntry.Text
		useTerm := chkTerminal.Checked
		closeWin := chkClose.Checked
		scriptDir := filepath.Dir(scriptPath)
		workDir := scriptDir // Default to script's directory

		// Header Parsing (#pqr or #qpr)
		foundInterpreter := ""
		sourceMsg := "Default"

		if file, err := os.Open(scriptPath); err == nil {
			scanner := bufio.NewScanner(file)
			for i := 0; i < 20 && scanner.Scan(); i++ {
				line := strings.TrimSpace(scanner.Text())
				lowerLine := strings.ToLower(line)
				if strings.HasPrefix(lowerLine, "#pqr") || strings.HasPrefix(lowerLine, "#qpr") {
					remainder := strings.TrimSpace(line[4:])
					parts := strings.Split(remainder, ";")
					for _, part := range parts {
						kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
						if len(kv) == 2 {
							key := strings.ToLower(strings.TrimSpace(kv[0]))
							val := strings.TrimSpace(kv[1])
							if key == "linux" && val != "" {
								foundInterpreter = val
								sourceMsg = "#qpr"
							} else if key == "term" {
								lowerVal := strings.ToLower(val)
								if lowerVal == "true" || lowerVal == "1" || lowerVal == "yes" {
									useTerm = true
								} else if lowerVal == "false" || lowerVal == "0" || lowerVal == "no" {
									useTerm = false
								}
							}
						}
					}
				}
			}
			file.Close()
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
					// Check for python binary in this venv
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

		// Determine if we are using a venv (either from #qpr or auto-detected)
		// We re-verify if the chosen pythonBin is part of a venv to set environment variables
		venvDir := ""
		absBin, _ := filepath.Abs(pythonBin)
		binDir := filepath.Dir(absBin)
		if _, err := os.Stat(filepath.Join(filepath.Dir(binDir), "pyvenv.cfg")); err == nil {
			venvDir = filepath.Dir(binDir)
			// If we are in a venv, we might want the project root as workDir
			if projectRoot != "" {
				workDir = projectRoot
			}
		}

		// Resolve relative interpreter path in #pqr (if any)
		if foundInterpreter != "" && !filepath.IsAbs(foundInterpreter) {
			pythonBin = filepath.Join(scriptDir, foundInterpreter)
		}

		statusLabel.SetText(fmt.Sprintf("Running %s via %s", filepath.Base(scriptPath), sourceMsg))

		if useTerm {
			// Construct command with environment variables and proper quoting
			envPrefix := ""
			if venvDir != "" {
				envPrefix = fmt.Sprintf("export VIRTUAL_ENV=%s; export PATH=%s:$PATH; ", shellQuote(venvDir), shellQuote(binDir))
			}
			// Use absolute path for scriptPath to avoid ambiguity when changing dir
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
				// Fallback to x-terminal-emulator
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
			// Inject environment variables for direct execution
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
			// 폴더인지 확인
			if fi, err := os.Stat(path); err == nil && fi.IsDir() {
				autoDetect(path)
			} else if strings.HasSuffix(strings.ToLower(path), ".py") {
				runScript(path)
			} else {
				statusLabel.SetText("Error: Only .py files or Project folders supported")
			}
		}
	})

	dropIcon := widget.NewIcon(theme.UploadIcon())
	dropText := widget.NewLabel("Drag & Drop .py file here\n(or Drop anywhere in window)")
	dropText.Alignment = fyne.TextAlignCenter

	dropZone := container.NewVBox(
		layout.NewSpacer(),
		dropIcon,
		dropText,
		layout.NewSpacer(),
	)

	cardFrame := container.NewPadded(container.NewPadded(dropZone))

	// --- 레이아웃 조립 ---
	content := container.NewVBox(
		widget.NewLabelWithStyle(AppName, fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		widget.NewLabel("Interpreter Path:"),
		container.NewBorder(nil, nil, nil, container.NewHBox(browseBtn, projBtn), pathEntry),
		chkTerminal,
		chkClose,
		layout.NewSpacer(),
		widget.NewCard("", "", cardFrame),
		layout.NewSpacer(),
		widget.NewSeparator(),
		statusLabel,
		widget.NewLabelWithStyle("© 2026 DINKIssTyle", fyne.TextAlignTrailing, fyne.TextStyle{Italic: true}),
	)

	w.SetContent(container.NewPadded(content))

	// ==========================================
	// [추가된 핵심 로직] 더블 클릭(인자값) 처리
	// ==========================================
	if len(os.Args) > 1 {
		arg := os.Args[1]
		targetPath := ""

		// URI(file://) 형태인지 확인
		if strings.HasPrefix(arg, "file://") {
			if u, err := url.Parse(arg); err == nil {
				targetPath = u.Path
			}
		} else {
			targetPath = arg
		}

		if targetPath != "" {
			// 실제 절대 경로로 변환
			if abs, err := filepath.Abs(targetPath); err == nil {
				targetPath = abs
			}

			// 파일이 실제 존재하고 .py 인지 확인
			if info, err := os.Stat(targetPath); err == nil && !info.IsDir() {
				if strings.HasSuffix(strings.ToLower(targetPath), ".py") {
					// UI가 완전히 뜬 뒤 실행하기 위해 고루틴(별도 쓰레드) 사용
					go func() {
						// 0.2초 정도 대기 (UI 렌더링 확보)
						time.Sleep(200 * time.Millisecond)
						runScript(targetPath)
					}()
				}
			}
		}
	}

	w.ShowAndRun()
}

// shellQuote returns a shell-escaped version of the string.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
