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
		pythonBin := pathEntry.Text
		useTerm := chkTerminal.Checked
		closeWin := chkClose.Checked
		workDir := filepath.Dir(scriptPath)

		// 헤더 파싱
		if file, err := os.Open(scriptPath); err == nil {
			scanner := bufio.NewScanner(file)
			for i := 0; i < 20 && scanner.Scan(); i++ {
				line := strings.TrimSpace(scanner.Text())
				if strings.HasPrefix(line, "#pqr linux") {
					opt := strings.TrimSpace(strings.TrimPrefix(line, "#pqr linux"))
					if strings.ToLower(opt) == "terminal" {
						useTerm = true
					}
				}
			}
			file.Close()
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
				statusLabel.SetText("Error: No supported terminal found.")
			}

		} else {
			cmd := exec.Command(pythonBin, scriptPath)
			cmd.Dir = workDir
			output, err := cmd.CombinedOutput()

			if err == nil {
				statusLabel.SetText("Success (Exit Code 0)")
				if closeWin {
					// 성공 시 1초 뒤 종료
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
		widget.NewLabelWithStyle("© 2025 DINKIssTyle", fyne.TextAlignTrailing, fyne.TextStyle{Italic: true}),
	)

	w.SetContent(container.NewPadded(content))

	// ==========================================
	// [추가된 핵심 로직] 더블 클릭(인자값) 처리
	// ==========================================
	if len(os.Args) > 1 {
		argPath := os.Args[1]
		// 파일이 실제 존재하고 .py 인지 확인
		if _, err := os.Stat(argPath); err == nil {
			if strings.HasSuffix(strings.ToLower(argPath), ".py") {
				// UI가 완전히 뜬 뒤 실행하기 위해 고루틴(별도 쓰레드) 사용
				go func() {
					// 0.2초 정도 대기 (UI 렌더링 확보)
					time.Sleep(200 * time.Millisecond)
					runScript(argPath)
				}()
			}
		}
	}

	w.ShowAndRun()
}
