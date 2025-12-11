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
	// "fyne.io/fyne/v2/storage" <- 사용하지 않아 에러 발생하므로 삭제함
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

	// 상태 메시지
	statusLabel := widget.NewLabel("Ready to run.")
	statusLabel.Alignment = fyne.TextAlignCenter

	// 파이썬 경로 입력
	pathEntry := widget.NewEntry()
	pathEntry.SetText(defaultPython)
	pathEntry.PlaceHolder = "/usr/bin/python3"

	// 경로 저장 로직
	pathEntry.OnChanged = func(s string) {
		prefs.SetString("pythonPath", s)
	}

	// 파일 찾기 버튼
	browseBtn := widget.NewButtonWithIcon("Browse", theme.FolderOpenIcon(), func() {
		fileDialog := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err == nil && reader != nil {
				pathEntry.SetText(reader.URI().Path())
			}
		}, w)
		fileDialog.Show()
	})

	// 옵션 체크박스
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

		// 헤더 파싱 (#pqr linux terminal 등)
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
			// 터미널 실행 (gnome-terminal 등 감지)
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
			// 백그라운드 실행
			cmd := exec.Command(pythonBin, scriptPath)
			cmd.Dir = workDir
			output, err := cmd.CombinedOutput()

			if err == nil {
				statusLabel.SetText("Success (Exit Code 0)")
				if closeWin {
					time.Sleep(time.Second)
					w.Close()
				}
			} else {
				statusLabel.SetText("Failed: " + err.Error())
				// 에러 내용을 다이얼로그로 표시
				dialog.ShowInformation("Execution Error", string(output), w)
			}
		}
	}

	// --- 드래그 앤 드롭 영역 (Custom Widget) ---
	w.SetOnDropped(func(pos fyne.Position, uris []fyne.URI) {
		if len(uris) > 0 {
			path := uris[0].Path()
			if strings.HasSuffix(strings.ToLower(path), ".py") {
				runScript(path)
			} else {
				statusLabel.SetText("Error: Only .py files supported")
			}
		}
	})

	// 드래그 안내 UI 구성 (수정됨: 최신 아이콘 사용)
	dropIcon := widget.NewIcon(theme.UploadIcon()) 
	dropText := widget.NewLabel("Drag & Drop .py file here\n(or Drop anywhere in window)")
	dropText.Alignment = fyne.TextAlignCenter

	dropZone := container.NewVBox(
		layout.NewSpacer(),
		dropIcon,
		dropText,
		layout.NewSpacer(),
	)

	// 카드 스타일 배경 프레임
	cardFrame := container.NewPadded(
		container.NewPadded(dropZone),
	)

	// --- 전체 레이아웃 조립 ---
	content := container.NewVBox(
		widget.NewLabelWithStyle(AppName, fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),

		widget.NewLabel("Interpreter Path:"),
		container.NewBorder(nil, nil, nil, browseBtn, pathEntry),

		chkTerminal,
		chkClose,

		layout.NewSpacer(),
		widget.NewCard("", "", cardFrame),
		layout.NewSpacer(),

		widget.NewSeparator(),
		statusLabel,
		// 수정됨: TextAlignTrailing (오른쪽 정렬) 사용
		widget.NewLabelWithStyle("© 2025 DINKIssTyle", fyne.TextAlignTrailing, fyne.TextStyle{Italic: true}),
	)

	w.SetContent(container.NewPadded(content))
	w.ShowAndRun()
}