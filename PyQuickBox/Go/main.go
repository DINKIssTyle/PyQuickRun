package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"image/color"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop" // Mouse interaction
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/fsnotify/fsnotify"
)

// --- 데이터 모델 ---
type ScriptItem struct {
	Name          string
	Path          string
	Category      string
	IconPath      string
	InterpDefault string // #pqr ... (Legacy or fallback)
	InterpMac     string // #pqr mac
	InterpWin     string // #pqr win
	InterpUbuntu  string // #pqr ubuntu
	Terminal      bool   // #pqr terminal true
}

// --- 앱 설정 키 ---
const (
	KeyRegisteredFolders = "RegisteredFolders"
	KeyPythonPath        = "PythonPath"
	KeyIconSize          = "IconSize"
	KeyFontSize          = "FontSize"
	KeyUIScale           = "UIScale"
	KeyThemeMode         = "ThemeMode" // "dark", "light", "system"
)

const (
	AppName      = "PyQuickBox"
	AppVersion   = "1.0.0"
	AppCopyright = "© 2025 DINKI'ssTyle"
)

// --- 메인 구조체 ---
type LauncherApp struct {
	App        fyne.App
	Window     fyne.Window
	ContentBox *fyne.Container // 메인 스크롤 영역

	// 데이터
	Scripts           map[string][]ScriptItem // 카테고리별 스크립트
	Categories        []string
	RegisteredFolders []string

	// Settings Window
	SettingsWindow fyne.Window

	// 설정
	DefaultPythonPath string
	IconSize          float32
	FontSize          float32
	UIScale           float32
	ThemeMode         string // "dark", "light", "system"

	// 검색
	SearchText  string
	SearchEntry *widget.Entry

	// 파일 감지
	Watcher *fsnotify.Watcher

	// UI State
	CurrentCategory string
	Sidebar         *widget.List
	SidebarVisible  bool
	MainContent     *fyne.Container // 우측 컨텐츠 영역 참조 유지
	TopBar          *fyne.Container
}

func main() {
	myApp := app.NewWithID("com.dinkisstyle.pyquickbox")

	// Apply Global UI Scale from Preferences (Default 0.9)
	uiScale := myApp.Preferences().FloatWithFallback("UIScale", 0.9)
	os.Setenv("FYNE_SCALE", fmt.Sprintf("%f", uiScale))

	myApp.SetIcon(resourceIconPng)
	myWindow := myApp.NewWindow(AppName)

	launcher := &LauncherApp{
		App:      myApp,
		Window:   myWindow,
		Scripts:  make(map[string][]ScriptItem),
		IconSize: 80, // 기본값
		FontSize: 12, // 기본값
		UIScale:  float32(uiScale),
	}

	// 1. 설정 불러오기
	launcher.loadPreferences()
	launcher.applyTheme(launcher.ThemeMode)

	// --- File Association & Open Handling ---
	// macOS "Open With" / Docker Drag
	/*
		if desk, ok := myApp.(desktop.App); ok {
			desk.SetOnOpened(func(uc fyne.URIReadCloser) {
				if uc == nil {
					return
				}
				path := uc.URI().Path()
				launcher.runScriptFromPath(path)
			})
		}
	*/

	// 2. 파일 감지기 시작
	watcher, err := fsnotify.NewWatcher()
	if err == nil {
		launcher.Watcher = watcher
		go launcher.watchFolders()
	}

	// 3. UI 구성
	launcher.setupUI()

	// 4. 초기 스캔
	launcher.refreshScripts()

	// Process CLI args (Windows/Linux)
	if len(os.Args) > 1 {
		// execute in goroutine to allow UI execution?
		// Actually runScriptFromPath uses exec.Command which is mostly async or blocking?
		// runScript returns *exec.Cmd, it starts it.
		// Let's iterate args.
		for _, arg := range os.Args[1:] {
			if strings.HasSuffix(arg, ".py") {
				launcher.runScriptFromPath(arg)
			}
		}
	}

	// 5. 드래그 앤 드롭 핸들러
	myWindow.SetOnDropped(func(pos fyne.Position, uris []fyne.URI) {
		launcher.handleDrops(uris)
	})

	myWindow.Resize(fyne.NewSize(800, 600))
	myWindow.ShowAndRun()
}

// --- Drag & Drop Handler ---
func (l *LauncherApp) handleDrops(uris []fyne.URI) {
	for _, uri := range uris {
		path := uri.Path()
		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		if info.IsDir() {
			// 폴더 드롭: 등록 여부 확인
			exists := false
			for _, f := range l.RegisteredFolders {
				if f == path {
					exists = true
					break
				}
			}
			if !exists {
				dialog.ShowConfirm("Add Folder", fmt.Sprintf("Add '%s' to registered folders?", filepath.Base(path)), func(ok bool) {
					if ok {
						l.RegisteredFolders = append(l.RegisteredFolders, path)
						l.savePreferences()
						l.refreshScripts()
						// 설정 다이얼로그가 열려있다면 갱신이 필요하겠지만, 여기서는 메인 UI 갱신이면 충분
					}
				}, l.Window)
			}
		} else {
			// 파일 드롭: .py 확인
			if filepath.Ext(path) == ".py" {
				// 임시 ScriptItem 생성 및 실행
				// 파싱하여 기존 헤더 설정 확인
				cat, iMac, iWin, iUbu, term, iDef := l.parseHeader(path)

				item := ScriptItem{
					Name:          strings.TrimSuffix(filepath.Base(path), ".py"),
					Path:          path,
					Category:      cat,
					InterpMac:     iMac,
					InterpWin:     iWin,
					InterpUbuntu:  iUbu,
					Terminal:      term,
					InterpDefault: iDef,
				}
				l.runScript(item)
			}
		}
	}
}

// parseHeader는 스크립트 파일의 #pqr 주석을 파싱하여 메타데이터를 추출합니다.
// parseHeader는 스크립트 파일의 #pqr 주석을 파싱하여 메타데이터를 추출합니다.
func (l *LauncherApp) parseHeader(filePath string) (category, interpMac, interpWin, interpUbuntu string, terminal bool, interpDefault string) {
	file, err := os.Open(filePath)
	if err != nil {
		return "Uncategorized", "", "", "", false, ""
	}
	defer file.Close()

	category = "Uncategorized" // Default

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "#pqr") {
			continue
		}

		// 1. Try New Format: #pqr key=val; key=val
		// Heuristic: Must contain '='
		if strings.Contains(line, "=") {
			content := strings.TrimSpace(strings.TrimPrefix(line, "#pqr"))
			parts := strings.Split(content, ";")
			for _, part := range parts {
				kv := strings.SplitN(part, "=", 2)
				if len(kv) == 2 {
					key := strings.TrimSpace(kv[0])
					val := strings.TrimSpace(kv[1])
					switch strings.ToLower(key) {
					case "cat":
						category = val
					case "mac":
						interpMac = val
					case "win":
						interpWin = val
					case "linux", "ubuntu":
						interpUbuntu = val
					case "term":
						if strings.ToLower(val) == "true" {
							terminal = true
						}
					case "def":
						interpDefault = val
					}
				}
			}
			continue // Assuming new format supersedes or is exclusive per line
		}

		// 2. Try Legacy Format: #pqr key "val"
		if strings.HasPrefix(line, "#pqr cat") {
			re := regexp.MustCompile(`#pqr\s+cat\s+"([^"]+)"`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				category = matches[1]
			}
		} else if strings.HasPrefix(line, "#pqr mac") {
			re := regexp.MustCompile(`#pqr\s+mac\s+"([^"]+)"`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				interpMac = matches[1]
			}
		} else if strings.HasPrefix(line, "#pqr win") {
			re := regexp.MustCompile(`#pqr\s+win\s+"([^"]+)"`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				interpWin = matches[1]
			}
		} else if strings.HasPrefix(line, "#pqr ubuntu") {
			re := regexp.MustCompile(`#pqr\s+ubuntu\s+"([^"]+)"`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				interpUbuntu = matches[1]
			}
		} else if strings.HasPrefix(line, "#pqr terminal") {
			if strings.Contains(line, "true") {
				terminal = true
			}
		}
	}
	return
}

// 테마 적용
func (l *LauncherApp) applyTheme(mode string) {
	l.ThemeMode = mode
	switch mode {
	case "dark":
		l.App.Settings().SetTheme(theme.DarkTheme())
	case "light":
		l.App.Settings().SetTheme(theme.LightTheme())
	default: // system
		l.App.Settings().SetTheme(theme.DefaultTheme())
	}
	l.savePreferences()
}

// --- 커스텀 버튼 (테마 메뉴용) ---
type ThemeButton struct {
	widget.Button
	l *LauncherApp
}

func NewThemeButton(l *LauncherApp) *ThemeButton {
	b := &ThemeButton{l: l}
	b.Icon = theme.ColorPaletteIcon()
	b.Importance = widget.MediumImportance
	b.ExtendBaseWidget(b)
	return b
}

func (b *ThemeButton) Tapped(e *fyne.PointEvent) {
	menu := fyne.NewMenu("Theme",
		fyne.NewMenuItem("Dark Mode", func() { b.l.applyTheme("dark") }),
		fyne.NewMenuItem("Light Mode", func() { b.l.applyTheme("light") }),
		fyne.NewMenuItem("System Default", func() { b.l.applyTheme("system") }),
	)
	widget.ShowPopUpMenuAtPosition(menu, b.l.Window.Canvas(), e.AbsolutePosition)
}

// --- UI 구성 ---
func (l *LauncherApp) setupUI() {
	// 1. Sidebar (좌측)
	l.Sidebar = widget.NewList(
		func() int {
			// All Apps + Categories
			return 1 + len(l.Categories)
		},
		func() fyne.CanvasObject {
			// Template Item
			// Use canvas.Image instead of widget.Icon to support SetMinSize
			icon := canvas.NewImageFromResource(theme.FolderIcon())
			icon.FillMode = canvas.ImageFillContain

			// Use canvas.Text for dynamic sizing
			label := canvas.NewText("Template", theme.ForegroundColor())
			label.Alignment = fyne.TextAlignLeading

			// Layout: Border (Left=Icon, Center=Label)
			borderLayout := container.NewBorder(nil, nil, icon, nil, label)

			// Add Padding
			return container.NewPadded(borderLayout)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			// Object Hierarchy: Padded -> Border -> (Icon, Label)
			padded := o.(*fyne.Container)
			border := padded.Objects[0].(*fyne.Container)

			var icon *canvas.Image
			var label *canvas.Text

			// Find components
			for _, obj := range border.Objects {
				if img, ok := obj.(*canvas.Image); ok {
					icon = img
				} else if lbl, ok := obj.(*canvas.Text); ok {
					label = lbl
				}
			}

			if icon == nil || label == nil {
				return
			}

			// Dynamic Sizing
			// Base icon size on Font Size (e.g. 1.5x)
			scaledSize := float32(l.FontSize * 1.5)
			icon.SetMinSize(fyne.NewSize(scaledSize, scaledSize))

			// Update Label
			label.TextSize = l.FontSize
			label.Color = theme.ForegroundColor() // Ensure theme update

			if i == 0 { // Item: All Apps
				icon.Resource = theme.GridIcon()
				label.Text = "All Apps"
				label.TextStyle = fyne.TextStyle{Bold: true}
			} else {
				// Categories
				catIndex := i - 1
				if catIndex >= 0 && catIndex < len(l.Categories) {
					icon.Resource = theme.FolderIcon()
					label.Text = l.Categories[catIndex]
					label.TextStyle = fyne.TextStyle{}
				}
			}
			label.Refresh()
			icon.Refresh() // Important to render new resource/size
		},
	)

	l.Sidebar.OnSelected = func(id widget.ListItemID) {
		if id == 0 {
			l.CurrentCategory = "All"
		} else {
			catIndex := id - 1
			if catIndex >= 0 && catIndex < len(l.Categories) {
				l.CurrentCategory = l.Categories[catIndex]
			}
		}
		l.updateGridUI()
	}

	// 2. Top Bar (우측 상단)
	// 사이드바 토글 버튼 (아이콘: Menu)
	toggleBtn := widget.NewButtonWithIcon("", theme.MenuIcon(), func() {
		l.SidebarVisible = !l.SidebarVisible
		l.refreshLayout()
	})

	l.SearchEntry = widget.NewEntry()
	l.SearchEntry.SetPlaceHolder("Search...")
	l.SearchEntry.OnChanged = func(s string) {
		l.SearchText = s
		l.updateGridUI()
	}
	// 검색창 크기 고정 (GridWrap 대신 Stack+Spacer 사용)
	// GridWrap이 의도치 않은 스크롤바/여백을 만들 수 있으므로 수정
	searchSpacer := canvas.NewRectangle(color.Transparent)
	searchSpacer.SetMinSize(fyne.NewSize(200, 34))
	searchContainer := container.NewStack(searchSpacer, l.SearchEntry)

	// 슬라이더 (최소값 32로 변경)
	iconSlider := widget.NewSlider(32, 200)
	iconSlider.Value = float64(l.IconSize)
	var debounceTimer *time.Timer
	iconSlider.OnChanged = func(f float64) {
		if debounceTimer != nil {
			debounceTimer.Stop()
		}
		l.IconSize = float32(f)
		l.App.Preferences().SetFloat(KeyIconSize, float64(l.IconSize))

		debounceTimer = time.AfterFunc(150*time.Millisecond, func() {
			l.updateGridUI()
		})
	}
	// 슬라이더 크기 고정
	sliderContainer := container.NewGridWrap(fyne.NewSize(150, 34), iconSlider)

	settingsBtn := widget.NewButtonWithIcon("", theme.SettingsIcon(), func() {
		l.showSettingsDialog()
	})

	// Theme Switcher Button
	themeBtn := NewThemeButton(l)
	// Button size: standard icon button
	// ThemeButton extends Button, so we just use it directly

	topRightControls := container.NewHBox(
		// widget.NewIcon(theme.GridIcon()) Removed as requested
		sliderContainer,
		themeBtn,
		settingsBtn,
	)

	// titleLabel Removed
	// Search Bar moved to Left
	topLeftControls := container.NewHBox(toggleBtn, searchContainer)

	l.TopBar = container.NewBorder(nil, nil, topLeftControls, topRightControls)

	// 3. Main Content (우측 -> Center Grid)
	l.ContentBox = container.NewVBox()
	scrollArea := container.NewVScroll(l.ContentBox)

	// Previously l.MainContent wrapped TopBar. Now it only holds the Grid content.
	// We wrap it in Padded for consistency
	l.MainContent = container.NewPadded(scrollArea)

	// 4. 초기 상태 설정 및 레이아웃 적용
	l.CurrentCategory = "All"
	l.SidebarVisible = true
	l.Sidebar.Select(0)

	l.refreshLayout()
}

// 레이아웃 갱신 (사이드바 토글 처리 + TopBar Fixed)
func (l *LauncherApp) refreshLayout() {
	var bodyContent fyne.CanvasObject

	if l.SidebarVisible {
		// Split Layout: Sidebar | Grid
		split := container.NewHSplit(l.Sidebar, l.MainContent)
		split.Offset = 0.2 // 사이드바 비율 조정
		bodyContent = split
	} else {
		// Only Grid
		bodyContent = l.MainContent
	}

	// TopBar is fixed at the top
	// content := Top + Body
	finalLayout := container.NewBorder(container.NewPadded(l.TopBar), nil, nil, nil, bodyContent)
	l.Window.SetContent(finalLayout)
}

// --- 그리드 UI 갱신 (핵심) ---
func (l *LauncherApp) updateGridUI() {
	l.ContentBox.Objects = nil // 기존 내용 초기화

	// 표시할 스크립트 목록 수집
	var displayScripts []ScriptItem

	if l.CurrentCategory == "All" || l.CurrentCategory == "" {
		// 모든 카테고리 보기
		for _, scripts := range l.Scripts {
			displayScripts = append(displayScripts, scripts...)
		}
	} else {
		// 특정 카테고리 보기
		displayScripts = l.Scripts[l.CurrentCategory]
	}

	// 정렬 (이름순)
	sort.Slice(displayScripts, func(i, j int) bool {
		return strings.ToLower(displayScripts[i].Name) < strings.ToLower(displayScripts[j].Name)
	})

	// 검색어 필터링
	var filteredScripts []ScriptItem
	if l.SearchText == "" {
		filteredScripts = displayScripts
	} else {
		for _, s := range displayScripts {
			if strings.Contains(strings.ToLower(s.Name), strings.ToLower(l.SearchText)) {
				filteredScripts = append(filteredScripts, s)
			}
		}
	}

	if len(filteredScripts) == 0 {
		l.ContentBox.Refresh()
		return
	}

	// 그리드 생성 (섹션 헤더 없이)
	// 텍스트 높이 계산
	// 2줄만 허용하므로 높이를 최적화 (Font Size * 2.8)
	// 약 2줄 높이 + 약간의 여유
	textHeight := float32(l.FontSize) * 2.8

	// 아이콘 간격 넓히기: 아이콘 크기 + 40 (좌우 여백)
	itemWidth := l.IconSize + 40
	itemHeight := l.IconSize + textHeight + 10 // 아이콘 + 텍스트 + 여백 (줄임)

	itemSize := fyne.NewSize(itemWidth, itemHeight)
	grid := container.NewGridWrap(itemSize)

	for _, script := range filteredScripts {
		sw := NewScriptWidget(script, l)
		grid.Add(sw)
	}

	l.ContentBox.Add(grid)
	l.ContentBox.Refresh()
}

// 검색 필터링
func (l *LauncherApp) filterScripts(category string) []ScriptItem {
	scripts := l.Scripts[category]
	if l.SearchText == "" {
		return scripts
	}

	// 카테고리 이름이 매칭되면 전체 표시
	if strings.Contains(strings.ToLower(category), strings.ToLower(l.SearchText)) {
		return scripts
	}

	// 파일명 매칭 확인
	var filtered []ScriptItem
	for _, s := range scripts {
		if strings.Contains(strings.ToLower(s.Name), strings.ToLower(l.SearchText)) {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

// --- 로직: 스크립트 스캔 ---
func (l *LauncherApp) refreshScripts() {
	newScripts := make(map[string][]ScriptItem)
	newCategories := make(map[string]bool)

	for _, folder := range l.RegisteredFolders {
		files, err := ioutil.ReadDir(folder)
		if err != nil {
			continue
		}

		iconFolder := filepath.Join(folder, "icon")

		for _, file := range files {
			if filepath.Ext(file.Name()) == ".py" {
				fullPath := filepath.Join(folder, file.Name())
				fileName := strings.TrimSuffix(file.Name(), ".py")

				// 아이콘 찾기
				var iconPath string
				specificIcon := filepath.Join(iconFolder, fileName+".png")
				defaultIcon := filepath.Join(iconFolder, "default.png")

				if _, err := os.Stat(specificIcon); err == nil {
					iconPath = specificIcon
				} else if _, err := os.Stat(defaultIcon); err == nil {
					iconPath = defaultIcon
				}

				// 파싱
				cat, iMac, iWin, iUbu, term, iDef := l.parseHeader(fullPath)

				item := ScriptItem{
					Name:          fileName,
					Path:          fullPath,
					Category:      cat,
					IconPath:      iconPath,
					InterpMac:     iMac,
					InterpWin:     iWin,
					InterpUbuntu:  iUbu,
					Terminal:      term,
					InterpDefault: iDef,
				}

				newScripts[cat] = append(newScripts[cat], item)
				newCategories[cat] = true
			}
		}
	}

	// 카테고리 정렬
	var sortedCats []string
	for k := range newCategories {
		sortedCats = append(sortedCats, k)
	}
	sort.Strings(sortedCats)

	// Uncategorized 맨 뒤로
	finalCats := []string{}
	hasUncat := false
	for _, c := range sortedCats {
		if c == "Uncategorized" {
			hasUncat = true
		} else {
			finalCats = append(finalCats, c)
		}
	}
	if hasUncat {
		finalCats = append(finalCats, "Uncategorized")
	}

	l.Scripts = newScripts
	l.Categories = finalCats

	// UI 갱신은 메인 스레드에서
	l.Sidebar.Refresh() // 사이드바 갱신
	l.updateGridUI()

	// 감시 폴더 업데이트
	if l.Watcher != nil {
		for _, f := range l.RegisteredFolders {
			l.Watcher.Add(f)
		}
	}
}

// --- 로직: 실행 ---
func (l *LauncherApp) runScript(s ScriptItem) *exec.Cmd {
	var python string

	// OS별 인터프리터 선택
	switch runtime.GOOS {
	case "darwin": // Mac
		if s.InterpMac != "" {
			python = s.InterpMac
		}
	case "windows":
		if s.InterpWin != "" {
			python = s.InterpWin
		}
	case "linux":
		if s.InterpUbuntu != "" {
			python = s.InterpUbuntu
		}
	}

	// 1순위: OS 전용, 2순위: Default(Legacy), 3순위: 앱 설정 기본값
	if python == "" {
		python = s.InterpDefault
	}
	if python == "" {
		python = l.DefaultPythonPath
	}
	// 마지막 보루
	if python == "" {
		if runtime.GOOS == "windows" {
			python = "python"
		} else {
			python = "/usr/bin/python3"
		}
	}

	fmt.Printf("Run Code: %s / Path: %s\n", s.Name, python)

	// 터미널 실행 여부 (단순 구현: 터미널을 열어서 실행하는 것은 OS별로 복잡하므로,
	// 여기서는 터미널 플래그가 있으면 xterm 등을 사용하는 식으로 확장이 가능하나,
	// 일단 로그만 찍고 기본 실행으로 유지하되, 필요시 확장)
	// Mac의 경우 'open -a Terminal script' 식이나
	// Windows의 경우 'cmd /k ...' 식의 처리가 필요.
	// 사용자 요청은 단순히 "Terminal 창 켤지 여부" 이므로
	// 간단히 구현 시도:

	var cmd *exec.Cmd

	if s.Terminal {
		cmd = l.createTerminalCommand(python, s.Path)
	} else {
		cmd = exec.Command(python, s.Path)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "PYTHONUNBUFFERED=1")

	go func() {
		err := cmd.Run()
		if err != nil {
			fmt.Printf("Error running script: %v\n", err)
			dialog.ShowError(err, l.Window)
		}
	}()
	return cmd
}

func (l *LauncherApp) createTerminalCommand(python, scriptPath string) *exec.Cmd {
	switch runtime.GOOS {
	case "darwin":
		// Mac: osascript를 사용하여 터미널 열기 등은 복잡하므로,
		// 여기서는 'open' 명령어로 터미널에서 실행되도록 유도하거나
		// 단순히 user choice에 따라 xterm 등을 호출.
		// 가장 호환성 높은 방법: Terminal.app에 스크립트를 던짐.
		// 하지만 python 인터프리터를 지정해서 열기는 까다로움.
		// 대안: 새 창을 띄우는 open -a Terminal 사용 (인자 전달의 어려움 있음)
		// 여기서는 "open"을 사용하여 기본 연결된 프로그램으로 열거나,
		// apple script로 do script ... 수행.

		// 간단한 접근:
		script := fmt.Sprintf(`tell application "Terminal" to do script "%s %s"`, python, scriptPath)
		return exec.Command("osascript", "-e", script)

	case "windows":
		// cmd /k "python script.py"
		return exec.Command("cmd", "/C", "start", "cmd", "/k", python, scriptPath)
	case "linux":
		// x-terminal-emulator or gnome-terminal
		return exec.Command("x-terminal-emulator", "-e", fmt.Sprintf("%s %s", python, scriptPath))
	default:
		return exec.Command(python, scriptPath)
	}
}

// 파일 위치 열기
// 파일 위치 열기
func (l *LauncherApp) openFileLocation(s ScriptItem) {
	dir := filepath.Dir(s.Path)
	switch runtime.GOOS {
	case "darwin":
		exec.Command("open", dir).Start()
	case "windows":
		exec.Command("explorer", dir).Start()
	case "linux":
		l.openFileLocationLinux(dir)
	}
}

func (l *LauncherApp) openFileLocationLinux(dir string) {
	// 1. Try common file managers explicitly
	// VS Code often hijacks xdg-open, so we prefer specific file managers.
	fileManagers := []string{
		"nautilus", // GNOME
		"dolphin",  // KDE
		"nemo",     // Cinnamon
		"caja",     // MATE
		"thunar",   // XFCE
		"pcmanfm",  // LXDE
	}

	for _, fm := range fileManagers {
		path, err := exec.LookPath(fm)
		if err == nil && path != "" {
			exec.Command(fm, dir).Start()
		}
	}

	// 2. Fallback to xdg-open if nothing else works
	exec.Command("xdg-open", dir).Start()
}

// --- 임의 경로 스크립트 실행 ---
func (l *LauncherApp) runScriptFromPath(path string) {
	// 1. Parse Metadata
	cat, iMac, iWin, iUbu, term, iDef := l.parseHeader(path)

	// 2. Create Temp ScriptItem
	item := ScriptItem{
		Name:          strings.TrimSuffix(filepath.Base(path), ".py"),
		Path:          path,
		Category:      cat,
		InterpMac:     iMac,
		InterpWin:     iWin,
		InterpUbuntu:  iUbu,
		Terminal:      term,
		InterpDefault: iDef,
	}

	// 3. Run
	cmd := l.runScript(item)
	if cmd != nil {
		// Log or Notify?
		fmt.Println("Launched external file:", path)
	}
}

// --- Desktop Entry Helpers (Linux only) ---
func (l *LauncherApp) createDesktopShortcut() {
	if runtime.GOOS != "linux" {
		return
	}

	homeDir, _ := os.UserHomeDir()
	appDir := filepath.Join(homeDir, ".local", "share", "applications")
	os.MkdirAll(appDir, 0755)

	exePath, err := os.Executable()
	if err != nil {
		dialog.ShowError(err, l.Window)
		return
	}
	exePath, _ = filepath.Abs(exePath)

	// Icon handling: We need to extract the icon to a file since .desktop needs a path
	iconPath := filepath.Join(homeDir, ".local", "share", "icons", "PyQuickBox.png")
	os.MkdirAll(filepath.Dir(iconPath), 0755)

	// Save bundled icon to file
	// Note: resourceIconPng is bundled via fyne bundle
	err = ioutil.WriteFile(iconPath, resourceIconPng.Content(), 0644)
	if err != nil {
		fmt.Println("Warning: Could not write icon file:", err)
	}

	desktopContent := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=PyQuickBox
Comment=Python Script Launcher
Exec=%s %%f
Icon=%s
Terminal=false
Categories=Utility;Development;
MimeType=text/x-python;
`, exePath, iconPath)

	desktopPath := filepath.Join(appDir, "PyQuickBox.desktop")
	err = ioutil.WriteFile(desktopPath, []byte(desktopContent), 0644)
	if err != nil {
		dialog.ShowError(err, l.Window)
	} else {
		// Update DB
		exec.Command("update-desktop-database", appDir).Run()
		dialog.ShowInformation("Success", "Desktop shortcut & File association created!", l.Window)
	}
}

func (l *LauncherApp) removeDesktopShortcut() {
	if runtime.GOOS != "linux" {
		return
	}
	homeDir, _ := os.UserHomeDir()
	desktopPath := filepath.Join(homeDir, ".local", "share", "applications", "PyQuickBox.desktop")

	err := os.Remove(desktopPath)
	if err != nil && !os.IsNotExist(err) {
		dialog.ShowError(err, l.Window)
	} else {
		dialog.ShowInformation("Success", "Desktop shortcut removed!", l.Window)
	}
}

// --- Windows Registry Helpers ---
func (l *LauncherApp) registerWindowsAssociation() {
	if runtime.GOOS != "windows" {
		return
	}

	exePath, err := os.Executable()
	if err != nil {
		dialog.ShowError(err, l.Window)
		return
	}
	exePath, _ = filepath.Abs(exePath)

	// Format: "C:\Path\To\App.exe" "%1"
	cmdVal := fmt.Sprintf(`"%s" "%%1"`, exePath)

	// 1. HKCU\Software\Classes\.py (Associate .py with ProgID)
	// Note: Use a custom ProgID to avoid messing with system python default unrecoverably?
	// Better: PyQuickBox.PythonScript

	// We use "reg" command for simplicity
	cmds := [][]string{
		{"add", `HKCU\Software\Classes\.py`, "/ve", "/d", "PyQuickBox.PythonScript", "/f"},
		{"add", `HKCU\Software\Classes\PyQuickBox.PythonScript`, "/ve", "/d", "Python Script", "/f"},
		{"add", `HKCU\Software\Classes\PyQuickBox.PythonScript\shell\open\command`, "/ve", "/d", cmdVal, "/f"},
	}

	for _, args := range cmds {
		err := exec.Command("reg", args...).Run()
		if err != nil {
			dialog.ShowError(fmt.Errorf("Registry Error: %v\nArgs: %v", err, args), l.Window)
			return
		}
	}

	dialog.ShowInformation("Success", "Registered .py file association!\n(You may need to restart Explorer or choose PyQuickBox from 'Open With')", l.Window)
}

// --- 설정 및 데이터 관리 ---
func (l *LauncherApp) loadPreferences() {
	l.DefaultPythonPath = l.App.Preferences().StringWithFallback(KeyPythonPath, "/usr/bin/python3")
	l.IconSize = float32(l.App.Preferences().FloatWithFallback(KeyIconSize, 80))
	l.FontSize = float32(l.App.Preferences().FloatWithFallback(KeyFontSize, 12))
	l.UIScale = float32(l.App.Preferences().FloatWithFallback(KeyUIScale, 0.9))
	l.ThemeMode = l.App.Preferences().StringWithFallback(KeyThemeMode, "system")

	foldersJson := l.App.Preferences().String(KeyRegisteredFolders)
	if foldersJson != "" {
		json.Unmarshal([]byte(foldersJson), &l.RegisteredFolders)
	}
}

func (l *LauncherApp) savePreferences() {
	l.App.Preferences().SetString(KeyPythonPath, l.DefaultPythonPath)
	l.App.Preferences().SetFloat(KeyIconSize, float64(l.IconSize))
	l.App.Preferences().SetFloat(KeyFontSize, float64(l.FontSize))
	l.App.Preferences().SetFloat(KeyUIScale, float64(l.UIScale))
	l.App.Preferences().SetString(KeyThemeMode, l.ThemeMode)

	data, _ := json.Marshal(l.RegisteredFolders)
	l.App.Preferences().SetString(KeyRegisteredFolders, string(data))
}

// 설정 다이얼로그 (새 창)
func (l *LauncherApp) showSettingsDialog() {
	// 이미 열려있으면 포커스
	if l.SettingsWindow != nil {
		l.SettingsWindow.Show()
		l.SettingsWindow.RequestFocus()
		return
	}

	w := l.App.NewWindow("Settings")
	l.SettingsWindow = w

	// 파이썬 경로
	pythonEntry := widget.NewEntry()
	pythonEntry.SetText(l.DefaultPythonPath)

	pythonBtn := widget.NewButton("Browse", func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err == nil && reader != nil {
				pythonEntry.SetText(reader.URI().Path())
			}
		}, w)
	})

	// 폰트 크기 조절
	fontSlider := widget.NewSlider(10, 24) // 10 ~ 24
	fontSlider.Step = 1
	fontSlider.Value = float64(l.FontSize)
	fontLabel := widget.NewLabel(fmt.Sprintf("%.0f", fontSlider.Value))
	fontSlider.OnChanged = func(f float64) {
		l.FontSize = float32(f)
		fontLabel.SetText(fmt.Sprintf("%.0f", f))
	}

	fontContainer := container.NewBorder(nil, nil, nil, fontLabel, fontSlider)

	// Linux Desktop Shortcut Buttons
	var linuxShortcutBox *fyne.Container
	if runtime.GOOS == "linux" {
		createBtn := widget.NewButton("Create Shortcut & Associate .py", func() {
			l.createDesktopShortcut()
		})
		removeBtn := widget.NewButton("Remove Shortcut", func() {
			l.removeDesktopShortcut()
		})
		linuxShortcutBox = container.NewVBox(
			widget.NewLabelWithStyle("Desktop Integration:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			container.NewGridWithColumns(2, createBtn, removeBtn),
			widget.NewSeparator(),
		)
	}

	// Windows Association Button
	var winAssocBox *fyne.Container
	if runtime.GOOS == "windows" {
		regBtn := widget.NewButton("Register .py Association", func() {
			l.registerWindowsAssociation()
		})
		winAssocBox = container.NewVBox(
			widget.NewLabelWithStyle("Windows Integration:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			regBtn,
			widget.NewSeparator(),
		)
	}

	// 폴더 리스트
	var folderList *widget.List
	folderList = widget.NewList(
		func() int { return len(l.RegisteredFolders) },
		func() fyne.CanvasObject {
			return container.NewBorder(nil, nil, nil, widget.NewButtonWithIcon("", theme.DeleteIcon(), nil), widget.NewLabel("template"))
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			c := o.(*fyne.Container)
			label := c.Objects[0].(*widget.Label)
			btn := c.Objects[1].(*widget.Button)

			folder := l.RegisteredFolders[i]
			label.SetText(folder)

			btn.OnTapped = func() {
				// 삭제 로직: 인덱스 i를 직접 사용하면 슬라이스 변경 시 패닉 발생 가능
				// 값(folder)으로 현재 인덱스를 다시 찾아서 삭제
				idx := -1
				for k, v := range l.RegisteredFolders {
					if v == folder {
						idx = k
						break
					}
				}

				if idx != -1 {
					l.RegisteredFolders = append(l.RegisteredFolders[:idx], l.RegisteredFolders[idx+1:]...)
					l.savePreferences()
					l.refreshScripts()
					// 리스트 갱신
					folderList.Refresh()
				}
			}
		},
	)
	folderScroll := container.NewVScroll(folderList)
	folderScroll.SetMinSize(fyne.NewSize(0, 200)) // 리스트 최소 높이

	addFolderBtn := widget.NewButtonWithIcon("Add Folder", theme.ContentAddIcon(), func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err == nil && uri != nil {
				path := uri.Path()
				exists := false
				for _, f := range l.RegisteredFolders {
					if f == path {
						exists = true
						break
					}
				}
				if !exists {
					l.RegisteredFolders = append(l.RegisteredFolders, path)
					l.savePreferences()
					l.refreshScripts()
					folderList.Refresh()
				}
			}
		}, w)
	})

	// Scale Control
	scaleSlider := widget.NewSlider(0.5, 1.25)
	scaleSlider.Step = 0.05
	scaleSlider.Value = float64(l.UIScale)
	scaleLabel := widget.NewLabel(fmt.Sprintf("%.2f", scaleSlider.Value))
	scaleSlider.OnChanged = func(f float64) {
		l.UIScale = float32(f)
		scaleLabel.SetText(fmt.Sprintf("%.2f", f))
	}
	scaleContainer := container.NewBorder(nil, nil, nil, scaleLabel, scaleSlider)

	// 다이얼로그 내용 구성
	settingsItems := []fyne.CanvasObject{
		widget.NewLabelWithStyle("Default Python Path:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewBorder(nil, nil, nil, pythonBtn, pythonEntry),
		widget.NewSeparator(),

		widget.NewLabelWithStyle("UI Scale (Restart Required):", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		scaleContainer,
		widget.NewSeparator(),

		widget.NewLabelWithStyle("Label Font Size:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		fontContainer,
		widget.NewSeparator(),
	}

	if linuxShortcutBox != nil {
		settingsItems = append(settingsItems, linuxShortcutBox)
	}
	if winAssocBox != nil {
		settingsItems = append(settingsItems, winAssocBox)
	}

	settingsItems = append(settingsItems,
		widget.NewLabelWithStyle("Registered Folders:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		addFolderBtn,
		folderScroll,
	)

	// 전체 내용을 스크롤 가능하게 (VBox -> VScroll)
	mainContent := container.NewVBox(settingsItems...)
	scrollContainer := container.NewVScroll(container.NewPadded(mainContent))

	// Copyright Label (Docked at Bottom)
	copyrightLabel := container.NewVBox(
		widget.NewSeparator(),
		widget.NewLabelWithStyle(fmt.Sprintf("Version %s | %s", AppVersion, AppCopyright), fyne.TextAlignCenter, fyne.TextStyle{}),
	)

	// Layout: Scrollable Center + Fixed Bottom
	finalLayout := container.NewBorder(nil, copyrightLabel, nil, nil, scrollContainer)

	w.SetContent(finalLayout)
	w.Resize(fyne.NewSize(500, 600))

	w.SetOnClosed(func() {
		l.DefaultPythonPath = pythonEntry.Text
		l.savePreferences()
		l.refreshScripts()
		l.SettingsWindow = nil // 참조 해제
	})

	w.Show()
}

// 속성 다이얼로그 표시
func (l *LauncherApp) showPropertiesDialog(s ScriptItem) {
	desc := widget.NewLabel("Script Path: " + s.Path)
	desc.Wrapping = fyne.TextWrapBreak

	catEntry := widget.NewEntry()
	catEntry.SetText(s.Category)

	// OS별 인터프리터 설정
	// Helper to create browse row
	createBrowseRow := func(placeholder string, initial string) (*widget.Entry, *fyne.Container) {
		entry := widget.NewEntry()
		entry.SetPlaceHolder(placeholder)
		entry.SetText(initial)

		btn := widget.NewButton("Browse", func() {
			d := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
				if err == nil && reader != nil {
					entry.SetText(reader.URI().Path())
				}
			}, l.Window)
			// d.ShowHiddenFiles = true // Fyne v2.x might not expose this directly on FileOpen struct easily pre-2.5?
			// Actually recent Fyne versions support this if the driver allows, usually standard dialogs
			// For internal dialog, we might need a custom one, but let's try standard first.
			// User specifically asked for "hidden files".
			// Let's assume standard behavior or try to hit "Show" for hidden if API exists.
			d.Show()
		})

		// Layout: Entry takes all space, Button on right
		row := container.NewBorder(nil, nil, nil, btn, entry)
		return entry, row
	}

	macEntry, macRow := createBrowseRow("Path to python/sh (Mac)", s.InterpMac)
	winEntry, winRow := createBrowseRow("Path to python/exe (Windows)", s.InterpWin)
	ubuEntry, ubuRow := createBrowseRow("Path to python/sh (Ubuntu)", s.InterpUbuntu)

	termCheck := widget.NewCheck("Run in Terminal", nil)
	termCheck.Checked = s.Terminal

	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "Category", Widget: catEntry},
			{Text: "Mac", Widget: macRow},
			{Text: "Win", Widget: winRow},
			{Text: "Ubuntu", Widget: ubuRow},
			{Text: "Option", Widget: termCheck},
		},
	}

	content := container.NewVBox(desc, form)

	// 버튼 추가 (저장/취소/닫기)
	var popup *widget.PopUp

	saveBtn := widget.NewButton("Save", func() {
		l.updateScriptMetadata(s, catEntry.Text, macEntry.Text, winEntry.Text, ubuEntry.Text, termCheck.Checked)
		l.refreshScripts()
		if popup != nil {
			popup.Hide()
		}
	})
	cancelBtn := widget.NewButton("Cancel", func() {
		if popup != nil {
			popup.Hide()
		}
	})
	closeBtn := widget.NewButton("Close", func() {
		if popup != nil {
			popup.Hide()
		}
	})

	// Buttons Row
	buttons := container.NewHBox(layout.NewSpacer(), saveBtn, cancelBtn, closeBtn, layout.NewSpacer())

	// Main Layout
	// Title
	title := widget.NewLabelWithStyle("Properties", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	mainContainer := container.NewVBox(
		title,
		widget.NewSeparator(),
		container.NewPadded(content),
		widget.NewSeparator(),
		buttons,
	)

	// Adding some padding around the whole dialog content
	finalContent := container.NewPadded(mainContainer)

	// Widen: Enforce minimum width using a container that requests it?
	// Or explicitly resize the popup content if possible.
	// container.NewGridWrap(size, content) forces a size.
	// Or just ensure the entries are wide enough?
	// Let's wrap finalContent in a container that enforces min width.
	// A hacky way is a transparent rect with min size in a stack?
	// Or just let's set a MinSize on the popup content?
	// Actually, `widget.NewModalPopUp` sizes validly.
	// If inputs are too narrow, the window is too narrow.
	// We can put the content in a container with a MinWidth? Use a specialized layout?
	// Let's use a container that has a minimum width.
	// Actually, just resizing the content object before showing? No, layout re-calcs.

	// Best approach: Add a wide spacer or ensure text fields desire more width.
	// Mac/Win/Ubu fields are `NewEntry`. They expand.
	// If we set a MinSize on the mainContainer?
	// We can't set MinSize on VBox directly.
	// Use a utility container that enforces min size.
	// Or simpler: Resize the CanvasObject? No.

	// Let's add an invisible spacer of desired width to the VBox.
	spacer := canvas.NewRectangle(color.Transparent)
	spacer.SetMinSize(fyne.NewSize(500, 0)) // 500px width
	mainContainer.Add(spacer)

	popup = widget.NewModalPopUp(finalContent, l.Window.Canvas())
	popup.Show()
}

// 메타데이터 업데이트 (파일 쓰기)
// 기존 파일을 읽어서 #pqr 라인을 찾아서 수정하거나, 없으면 맨 위에 추가
func (l *LauncherApp) updateScriptMetadata(s ScriptItem, cat, mac, win, ubuntu string, term bool) {
	input, err := ioutil.ReadFile(s.Path)
	if err != nil {
		dialog.ShowError(err, l.Window)
		return
	}

	lines := strings.Split(string(input), "\n")
	var newLines []string
	pqrFound := false

	// Construct new pqr line (using 'linux' key for ubuntu value)
	newPqr := fmt.Sprintf("#pqr cat=%s; mac=%s; win=%s; linux=%s; term=%t", cat, mac, win, ubuntu, term)

	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "#pqr") {
			// Replace existing pqr line
			if !pqrFound {
				newLines = append(newLines, newPqr)
				pqrFound = true
			}
			// If duplicate pqr lines exist, ignore subsequent ones (we replaced the first)
		} else {
			newLines = append(newLines, line)
		}
	}

	if !pqrFound {
		// Insert at top
		// Check for shebang
		if len(newLines) > 0 && strings.HasPrefix(strings.TrimSpace(newLines[0]), "#!") {
			// Insert after shebang
			newLines = append(newLines[:1], newLines[0:]...)
			newLines[1] = newPqr
		} else {
			// Prepend
			newLines = append([]string{newPqr}, newLines...)
		}
	}

	output := strings.Join(newLines, "\n")
	err = ioutil.WriteFile(s.Path, []byte(output), 0644)
	if err != nil {
		dialog.ShowError(err, l.Window)
	}
}

// 파일 감시
func (l *LauncherApp) watchFolders() {
	for {
		select {
		case event, ok := <-l.Watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create || event.Op&fsnotify.Remove == fsnotify.Remove {
				time.Sleep(500 * time.Millisecond)
				l.refreshScripts()
			}
		case <-l.Watcher.Errors:
			return
		}
	}
}

// --- 커스텀 위젯: ScriptWidget ---
type ScriptWidget struct {
	widget.BaseWidget
	item ScriptItem
	app  *LauncherApp

	lastTap time.Time

	// UI Elements for manipulation
	background *canvas.Rectangle
	icon       *canvas.Image
}

func NewScriptWidget(item ScriptItem, app *LauncherApp) *ScriptWidget {
	w := &ScriptWidget{item: item, app: app}
	w.ExtendBaseWidget(w)
	return w
}

func (w *ScriptWidget) CreateRenderer() fyne.WidgetRenderer {
	// 배경
	w.background = canvas.NewRectangle(color.Transparent)
	w.background.CornerRadius = 8

	// 아이콘
	if w.item.IconPath != "" {
		w.icon = canvas.NewImageFromFile(w.item.IconPath)
	} else {
		w.icon = canvas.NewImageFromResource(theme.FileIcon())
	}
	w.icon.FillMode = canvas.ImageFillContain
	w.icon.SetMinSize(fyne.NewSize(w.app.IconSize, w.app.IconSize))

	// 텍스트 라인 생성
	lines := wrapSmart(w.item.Name, w.app.FontSize, w.app.IconSize+30)

	var labelTexts []*canvas.Text
	labelVBox := container.NewVBox()
	for _, line := range lines {
		txt := canvas.NewText(line, theme.ForegroundColor())
		txt.TextSize = w.app.FontSize
		txt.Alignment = fyne.TextAlignCenter
		labelVBox.Add(txt)
		labelTexts = append(labelTexts, txt)
	}

	iconContainer := container.NewGridWrap(fyne.NewSize(w.app.IconSize, w.app.IconSize), w.icon)

	mainLayout := container.NewBorder(
		container.NewCenter(iconContainer),
		nil, nil, nil,
		labelVBox,
	)

	content := container.NewMax(w.background, container.NewPadded(mainLayout))

	return &scriptWidgetRenderer{
		w:          w,
		content:    content,
		labelTexts: labelTexts,
	}
}

// --- Renderer Implementation ---
type scriptWidgetRenderer struct {
	w          *ScriptWidget
	content    *fyne.Container
	labelTexts []*canvas.Text
}

func (r *scriptWidgetRenderer) Layout(s fyne.Size) {
	r.content.Resize(s)
}

func (r *scriptWidgetRenderer) MinSize() fyne.Size {
	return r.content.MinSize()
}

func (r *scriptWidgetRenderer) Refresh() {
	// 테마 변경 시 색상 업데이트
	fg := theme.ForegroundColor()
	for _, txt := range r.labelTexts {
		txt.Color = fg
		txt.TextSize = r.w.app.FontSize // 폰트 크기 변경도 반영
	}
	r.content.Refresh()
}

func (r *scriptWidgetRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.content}
}

func (r *scriptWidgetRenderer) Destroy() {}

// Hoverable 인터페이스 구현
func (w *ScriptWidget) MouseIn(*desktop.MouseEvent) {
	w.background.FillColor = theme.HoverColor()
	w.background.Refresh()
}

func (w *ScriptWidget) MouseOut() {
	w.background.FillColor = color.Transparent
	w.background.Refresh()
}

func (w *ScriptWidget) MouseMoved(*desktop.MouseEvent) {}

func (w *ScriptWidget) Tapped(e *fyne.PointEvent) {
	// 더블 클릭 감지 (500ms)
	if time.Since(w.lastTap) < 500*time.Millisecond {
		w.animateLaunch()
		w.app.runScript(w.item)
		w.lastTap = time.Time{} // 초기화
	} else {
		w.lastTap = time.Now()
	}
}

func (w *ScriptWidget) animateLaunch() {
	// 펄스 애니메이션 (작아졌다 커짐)

	// Simple scale animation: Fyne doesn't support direct scale transform on all objects easily without custom layout,
	// but we can animate opacity or simple sizing if layout permits.
	// For immediate visual feedback, let's flash the background and fade the icon slightly.

	fade := fyne.NewAnimation(200*time.Millisecond, func(v float32) {
		// v goes 0 -> 1
		// Opacity: 1 -> 0.5 -> 1
		if v < 0.5 {
			w.icon.Translucency = float64(v) // 0 -> 0.5 (fadout)
		} else {
			w.icon.Translucency = float64(1 - v) // 0.5 -> 0 (fadein)
		}
		w.icon.Refresh()

		// Background flash
		if v < 0.5 {
			w.background.FillColor = theme.SelectionColor()
		} else {
			w.background.FillColor = theme.HoverColor() // Return to hover state
		}
		w.background.Refresh()
	})
	fade.Start()
}

// 텍스트 래핑 헬퍼 함수 (개선됨: 긴 단어 자르기 포함)
func wrapSmart(text string, size float32, maxWidth float32) []string {
	if text == "" {
		return []string{}
	}

	style := fyne.TextStyle{}
	var lines []string
	var currentLine string

	// 1. 이미 줄바꿈이 있는 경우 처리? (일단 무시하고 one block으로 봄 or split)
	// 단순화를 위해 전체를 run array로 변환하여 처리 (Character Wrap)
	// 단어 단위 보존을 위해 먼저 Fields로 나누고, 너무 긴 단어는 쪼갭니다.

	words := strings.Fields(text)
	for _, word := range words {
		// 단어 자체가 maxWidth보다 긴 경우: 강제로 쪼개야 함
		if fyne.MeasureText(word, size, style).Width > maxWidth {
			// 현재 라인 비우고 시작
			if currentLine != "" {
				lines = append(lines, currentLine)
				currentLine = ""
			}

			// 글자 단위로 쪼개서 넣기
			runes := []rune(word)
			chunk := ""
			for _, r := range runes {
				testChunk := chunk + string(r)
				if fyne.MeasureText(testChunk, size, style).Width <= maxWidth {
					chunk = testChunk
				} else {
					lines = append(lines, chunk)
					chunk = string(r)
				}
			}
			if chunk != "" {
				currentLine = chunk // 마지막 조각을 현재 라인으로
			}
		} else {
			// 일반 단어 처리
			testLine := word
			if currentLine != "" {
				testLine = currentLine + " " + word
			}

			if fyne.MeasureText(testLine, size, style).Width <= maxWidth {
				currentLine = testLine
			} else {
				lines = append(lines, currentLine)
				currentLine = word
			}
		}
	}
	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	// 최대 2줄 제한
	if len(lines) > 2 {
		lines = lines[:2]
		// lines[1] += "..." // 생략 표시 (선택사항)
	}

	return lines
}

func (w *ScriptWidget) TappedSecondary(e *fyne.PointEvent) {
	menu := fyne.NewMenu("",
		fyne.NewMenuItem("Run", func() { w.app.runScript(w.item) }),
		fyne.NewMenuItem("Open Location", func() { w.app.openFileLocation(w.item) }),
		fyne.NewMenuItem("Properties", func() { w.app.showPropertiesDialog(w.item) }),
	)

	widget.ShowPopUpMenuAtPosition(menu, w.app.Window.Canvas(), e.AbsolutePosition)
}
