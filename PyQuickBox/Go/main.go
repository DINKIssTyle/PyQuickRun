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
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
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
	uiScale := myApp.Preferences().FloatWithFallback(KeyUIScale, 0.9)
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

	// 1) 설정 불러오기 + 테마 적용
	launcher.loadPreferences()
	launcher.applyTheme(launcher.ThemeMode)

	// 2) UI 구성
	launcher.setupUI()

	// 3) 초기 스캔 (한 번만)
	launcher.refreshScripts()

	// 4) Process CLI args (Windows/Linux)
	if len(os.Args) > 1 {
		for _, arg := range os.Args[1:] {
			if strings.HasSuffix(arg, ".py") {
				launcher.runScriptFromPath(arg)
			}
		}
	}

	// 5) 드래그 앤 드롭 핸들러
	myWindow.SetOnDropped(func(pos fyne.Position, uris []fyne.URI) {
		launcher.handleDrops(uris)
	})

	myWindow.Resize(fyne.NewSize(800, 600))
	myWindow.ShowAndRun()
}

// --------------------
// 폴더 변경 통합 처리
// --------------------
func (l *LauncherApp) onFoldersChanged() {
	l.savePreferences()
	l.refreshScripts()
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
				dialog.ShowConfirm("Add Folder",
					fmt.Sprintf("Add '%s' to registered folders?", filepath.Base(path)),
					func(ok bool) {
						if ok {
							l.RegisteredFolders = append(l.RegisteredFolders, path)
							l.onFoldersChanged()
						}
					},
					l.Window,
				)
			}
		} else {
			// 파일 드롭: .py 확인
			if filepath.Ext(path) == ".py" {
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

		// 1) New Format: #pqr key=val; key=val
		if strings.Contains(line, "=") {
			content := strings.TrimSpace(strings.TrimPrefix(line, "#pqr"))
			parts := strings.Split(content, ";")
			for _, part := range parts {
				kv := strings.SplitN(part, "=", 2)
				if len(kv) != 2 {
					continue
				}
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
					terminal = (strings.ToLower(val) == "true")
				case "def":
					interpDefault = val
				}
			}
			continue
		}

		// 2) Legacy Format: #pqr key "val"
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
	// 1) Sidebar (좌측)
	l.Sidebar = widget.NewList(
		func() int {
			return 1 + len(l.Categories) // All Apps + Categories
		},
		func() fyne.CanvasObject {
			icon := canvas.NewImageFromResource(theme.FolderIcon())
			icon.FillMode = canvas.ImageFillContain

			label := canvas.NewText("Template", theme.ForegroundColor())
			label.Alignment = fyne.TextAlignLeading

			borderLayout := container.NewBorder(nil, nil, icon, nil, label)
			return container.NewPadded(borderLayout)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			padded := o.(*fyne.Container)
			border := padded.Objects[0].(*fyne.Container)

			var icon *canvas.Image
			var label *canvas.Text
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

			scaledSize := float32(l.FontSize * 1.5)
			icon.SetMinSize(fyne.NewSize(scaledSize, scaledSize))

			label.TextSize = l.FontSize
			label.Color = theme.ForegroundColor()

			if i == 0 {
				icon.Resource = theme.GridIcon()
				label.Text = "All Apps"
				label.TextStyle = fyne.TextStyle{Bold: true}
			} else {
				catIndex := i - 1
				if catIndex >= 0 && catIndex < len(l.Categories) {
					icon.Resource = theme.FolderIcon()
					label.Text = l.Categories[catIndex]
					label.TextStyle = fyne.TextStyle{}
				}
			}
			label.Refresh()
			icon.Refresh()
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

	// 2) Top Bar (우측 상단)
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
	searchSpacer := canvas.NewRectangle(color.Transparent)
	searchSpacer.SetMinSize(fyne.NewSize(200, 34))
	searchContainer := container.NewStack(searchSpacer, l.SearchEntry)

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
	sliderContainer := container.NewGridWrap(fyne.NewSize(150, 34), iconSlider)

	settingsBtn := widget.NewButtonWithIcon("", theme.SettingsIcon(), func() {
		l.showSettingsDialog()
	})

	// ✅ Refresh 버튼 추가 (수동 갱신)
	refreshBtn := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() {
		l.refreshScripts()
	})

	themeBtn := NewThemeButton(l)

	topRightControls := container.NewHBox(
		sliderContainer,
		themeBtn,
		refreshBtn,
		settingsBtn,
	)

	topLeftControls := container.NewHBox(toggleBtn, searchContainer)

	l.TopBar = container.NewBorder(nil, nil, topLeftControls, topRightControls)

	// 3) Main Content
	l.ContentBox = container.NewVBox()
	scrollArea := container.NewVScroll(l.ContentBox)
	l.MainContent = container.NewPadded(scrollArea)

	// 4) 초기 상태
	l.CurrentCategory = "All"
	l.SidebarVisible = true
	l.Sidebar.Select(0)

	l.refreshLayout()
}

func (l *LauncherApp) refreshLayout() {
	var bodyContent fyne.CanvasObject

	if l.SidebarVisible {
		split := container.NewHSplit(l.Sidebar, l.MainContent)
		split.Offset = 0.2
		bodyContent = split
	} else {
		bodyContent = l.MainContent
	}

	finalLayout := container.NewBorder(container.NewPadded(l.TopBar), nil, nil, nil, bodyContent)
	l.Window.SetContent(finalLayout)
}

// --- 그리드 UI 갱신 ---
func (l *LauncherApp) updateGridUI() {
	l.ContentBox.Objects = nil

	var displayScripts []ScriptItem
	if l.CurrentCategory == "All" || l.CurrentCategory == "" {
		for _, scripts := range l.Scripts {
			displayScripts = append(displayScripts, scripts...)
		}
	} else {
		displayScripts = l.Scripts[l.CurrentCategory]
	}

	sort.Slice(displayScripts, func(i, j int) bool {
		return strings.ToLower(displayScripts[i].Name) < strings.ToLower(displayScripts[j].Name)
	})

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

	textHeight := float32(l.FontSize) * 2.8
	itemWidth := l.IconSize + 40
	itemHeight := l.IconSize + textHeight + 10

	itemSize := fyne.NewSize(itemWidth, itemHeight)
	grid := container.NewGridWrap(itemSize)

	for _, script := range filteredScripts {
		sw := NewScriptWidget(script, l)
		grid.Add(sw)
	}

	l.ContentBox.Add(grid)
	l.ContentBox.Refresh()
}

// --- 로직: 스크립트 스캔 (수동 Refresh / 폴더 변경시에만 호출) ---
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
			if filepath.Ext(file.Name()) != ".py" {
				continue
			}

			fullPath := filepath.Join(folder, file.Name())
			fileName := strings.TrimSuffix(file.Name(), ".py")

			var iconPath string
			specificIcon := filepath.Join(iconFolder, fileName+".png")
			defaultIcon := filepath.Join(iconFolder, "default.png")

			if _, err := os.Stat(specificIcon); err == nil {
				iconPath = specificIcon
			} else if _, err := os.Stat(defaultIcon); err == nil {
				iconPath = defaultIcon
			}

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

	var sortedCats []string
	for k := range newCategories {
		sortedCats = append(sortedCats, k)
	}
	sort.Strings(sortedCats)

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

	l.Sidebar.Refresh()
	l.updateGridUI()
}

// --- 로직: 실행 ---
func (l *LauncherApp) runScript(s ScriptItem) *exec.Cmd {
	var python string

	switch runtime.GOOS {
	case "darwin":
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

	if python == "" {
		python = s.InterpDefault
	}
	if python == "" {
		python = l.DefaultPythonPath
	}
	if python == "" {
		if runtime.GOOS == "windows" {
			python = "python"
		} else {
			python = "/usr/bin/python3"
		}
	}

	fmt.Printf("Run Code: %s / Python: %s\n", s.Name, python)

	var cmd *exec.Cmd
	if s.Terminal {
		cmd = l.createTerminalCommand(python, s.Path)
	} else {
		cmd = exec.Command(python, s.Path)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "PYTHONUNBUFFERED=1")

	go func() {
		if err := cmd.Run(); err != nil {
			fmt.Printf("Error running script: %v\n", err)
			dialog.ShowError(err, l.Window)
		}
	}()
	return cmd
}

func (l *LauncherApp) createTerminalCommand(python, scriptPath string) *exec.Cmd {
	switch runtime.GOOS {
	case "darwin":
		script := fmt.Sprintf(`tell application "Terminal" to do script "%s %s"`, python, scriptPath)
		return exec.Command("osascript", "-e", script)
	case "windows":
		return exec.Command("cmd", "/C", "start", "cmd", "/k", python, scriptPath)
	case "linux":
		return exec.Command("x-terminal-emulator", "-e", fmt.Sprintf("%s %s", python, scriptPath))
	default:
		return exec.Command(python, scriptPath)
	}
}

// 파일 위치 열기
func (l *LauncherApp) openFileLocation(s ScriptItem) {
	dir := filepath.Dir(s.Path)
	switch runtime.GOOS {
	case "darwin":
		_ = exec.Command("open", dir).Start()
	case "windows":
		_ = exec.Command("explorer", dir).Start()
	case "linux":
		l.openFileLocationLinux(dir)
	}
}

func (l *LauncherApp) openFileLocationLinux(dir string) {
	fileManagers := []string{"nautilus", "dolphin", "nemo", "caja", "thunar", "pcmanfm"}
	for _, fm := range fileManagers {
		path, err := exec.LookPath(fm)
		if err == nil && path != "" {
			_ = exec.Command(fm, dir).Start()
			return
		}
	}
	_ = exec.Command("xdg-open", dir).Start()
}

// --- 임의 경로 스크립트 실행 ---
func (l *LauncherApp) runScriptFromPath(path string) {
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
	cmd := l.runScript(item)
	if cmd != nil {
		fmt.Println("Launched external file:", path)
	}
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
		_ = json.Unmarshal([]byte(foldersJson), &l.RegisteredFolders)
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
	if l.SettingsWindow != nil {
		l.SettingsWindow.Show()
		l.SettingsWindow.RequestFocus()
		return
	}

	w := l.App.NewWindow("Settings")
	l.SettingsWindow = w

	pythonEntry := widget.NewEntry()
	pythonEntry.SetText(l.DefaultPythonPath)

	// --- Auto Detect Logic ---
	autoDetect := func(dir string) {
		candidates := []string{
			filepath.Join(dir, ".venv", "bin", "python"),
			filepath.Join(dir, ".venv", "bin", "python3"),
			filepath.Join(dir, "venv", "bin", "python"),
			filepath.Join(dir, "venv", "bin", "python3"),
			filepath.Join(dir, "env", "bin", "python"),
		}
		// Windows specific candidates
		if runtime.GOOS == "windows" {
			candidates = append(candidates,
				filepath.Join(dir, ".venv", "Scripts", "python.exe"),
				filepath.Join(dir, "venv", "Scripts", "python.exe"),
				filepath.Join(dir, "env", "Scripts", "python.exe"),
			)
		}

		found := ""
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				found = c
				break
			}
		}

		// Go Project Detection
		if found == "" {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				if path, err := exec.LookPath("go"); err == nil {
					found = path
				}
			}
		}

		// Swift Project Detection
		if found == "" {
			if _, err := os.Stat(filepath.Join(dir, "Package.swift")); err == nil {
				if path, err := exec.LookPath("swift"); err == nil {
					found = path
				}
			}
		}

		if found != "" {
			pythonEntry.SetText(found)
		} else {
			dialog.ShowInformation("No Environment Found", "Could not find virtualenv, go.mod, or Package.swift in:\n"+dir, w)
		}
	}

	pythonBtn := widget.NewButtonWithIcon("", theme.FileIcon(), func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err == nil && reader != nil {
				pythonEntry.SetText(reader.URI().Path())
			}
		}, w)
	})

	projBtn := widget.NewButtonWithIcon("", theme.FolderIcon(), func() {
		dialog.ShowFolderOpen(func(list fyne.ListableURI, err error) {
			if err == nil && list != nil {
				autoDetect(list.Path())
			}
		}, w)
	})

	// Layout for Interpreter Path
	interpContainer := container.NewBorder(nil, nil, nil, container.NewHBox(pythonBtn, projBtn), pythonEntry)

	fontSlider := widget.NewSlider(10, 24)
	fontSlider.Step = 1
	fontSlider.Value = float64(l.FontSize)
	fontLabel := widget.NewLabel(fmt.Sprintf("%.0f", fontSlider.Value))
	fontSlider.OnChanged = func(f float64) {
		l.FontSize = float32(f)
		fontLabel.SetText(fmt.Sprintf("%.0f", f))
	}
	fontContainer := container.NewBorder(nil, nil, nil, fontLabel, fontSlider)

	// 폴더 리스트
	var folderList *widget.List
	folderList = widget.NewList(
		func() int { return len(l.RegisteredFolders) },
		func() fyne.CanvasObject {
			return container.NewBorder(nil, nil, nil,
				widget.NewButtonWithIcon("", theme.DeleteIcon(), nil),
				widget.NewLabel("template"),
			)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			c := o.(*fyne.Container)
			label := c.Objects[0].(*widget.Label)
			btn := c.Objects[1].(*widget.Button)

			folder := l.RegisteredFolders[i]
			label.SetText(folder)

			btn.OnTapped = func() {
				idx := -1
				for k, v := range l.RegisteredFolders {
					if v == folder {
						idx = k
						break
					}
				}
				if idx != -1 {
					l.RegisteredFolders = append(l.RegisteredFolders[:idx], l.RegisteredFolders[idx+1:]...)
					l.onFoldersChanged()
					folderList.Refresh()
				}
			}
		},
	)

	folderScroll := container.NewVScroll(folderList)
	folderScroll.SetMinSize(fyne.NewSize(0, 200))

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
					l.onFoldersChanged()
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

	settingsForm := container.NewGridWithColumns(2,
		widget.NewLabel("Interpreter Path:"), interpContainer,
		widget.NewLabel("UI Font Size:"), fontContainer,
	)

	settingsItems := []fyne.CanvasObject{
		settingsForm,
		widget.NewSeparator(),

		widget.NewLabelWithStyle("UI Scale (Restart Required):", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		scaleContainer,
		widget.NewSeparator(),

		widget.NewLabelWithStyle("Label Font Size:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		fontContainer,
		widget.NewSeparator(),

		widget.NewLabelWithStyle("Registered Folders:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		addFolderBtn,
		folderScroll,
	}

	mainContent := container.NewVBox(settingsItems...)
	scrollContainer := container.NewVScroll(container.NewPadded(mainContent))

	copyrightLabel := container.NewVBox(
		widget.NewSeparator(),
		widget.NewLabelWithStyle(fmt.Sprintf("Version %s | %s", AppVersion, AppCopyright),
			fyne.TextAlignCenter, fyne.TextStyle{}),
	)

	finalLayout := container.NewBorder(nil, copyrightLabel, nil, nil, scrollContainer)

	w.SetContent(finalLayout)
	w.Resize(fyne.NewSize(500, 600))

	w.SetOnClosed(func() {
		l.DefaultPythonPath = pythonEntry.Text
		l.savePreferences()
		// 설정창 닫힐 때는 굳이 refresh를 강제할 필요는 없지만,
		// python path가 바뀌었을 수 있으니 유지하겠습니다.
		l.refreshScripts()
		l.SettingsWindow = nil
	})

	w.Show()
}

// 속성 다이얼로그 표시
func (l *LauncherApp) showPropertiesDialog(s ScriptItem) {
	desc := widget.NewLabel("Script Path: " + s.Path)
	desc.Wrapping = fyne.TextWrapBreak

	catEntry := widget.NewEntry()
	catEntry.SetText(s.Category)

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
			d.Show()
		})

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

	buttons := container.NewHBox(layout.NewSpacer(), saveBtn, cancelBtn, closeBtn, layout.NewSpacer())

	title := widget.NewLabelWithStyle("Properties", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	mainContainer := container.NewVBox(
		title,
		widget.NewSeparator(),
		container.NewPadded(content),
		widget.NewSeparator(),
		buttons,
	)

	finalContent := container.NewPadded(mainContainer)

	// Min width 확보용 스페이서
	spacer := canvas.NewRectangle(color.Transparent)
	spacer.SetMinSize(fyne.NewSize(500, 0))
	mainContainer.Add(spacer)

	popup = widget.NewModalPopUp(finalContent, l.Window.Canvas())
	popup.Show()
}

// 메타데이터 업데이트 (파일 쓰기)
func (l *LauncherApp) updateScriptMetadata(s ScriptItem, cat, mac, win, ubuntu string, term bool) {
	input, err := ioutil.ReadFile(s.Path)
	if err != nil {
		dialog.ShowError(err, l.Window)
		return
	}

	lines := strings.Split(string(input), "\n")
	var newLines []string
	pqrFound := false

	newPqr := fmt.Sprintf("#pqr cat=%s; mac=%s; win=%s; linux=%s; term=%t", cat, mac, win, ubuntu, term)

	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "#pqr") {
			if !pqrFound {
				newLines = append(newLines, newPqr)
				pqrFound = true
			}
		} else {
			newLines = append(newLines, line)
		}
	}

	if !pqrFound {
		if len(newLines) > 0 && strings.HasPrefix(strings.TrimSpace(newLines[0]), "#!") {
			newLines = append(newLines[:1], newLines[0:]...)
			newLines[1] = newPqr
		} else {
			newLines = append([]string{newPqr}, newLines...)
		}
	}

	output := strings.Join(newLines, "\n")
	if err := ioutil.WriteFile(s.Path, []byte(output), 0644); err != nil {
		dialog.ShowError(err, l.Window)
	}
}

// --- 커스텀 위젯: ScriptWidget ---
type ScriptWidget struct {
	widget.BaseWidget
	item ScriptItem
	app  *LauncherApp

	lastTap time.Time

	background *canvas.Rectangle
	icon       *canvas.Image
}

func NewScriptWidget(item ScriptItem, app *LauncherApp) *ScriptWidget {
	w := &ScriptWidget{item: item, app: app}
	w.ExtendBaseWidget(w)
	return w
}

func (w *ScriptWidget) CreateRenderer() fyne.WidgetRenderer {
	w.background = canvas.NewRectangle(color.Transparent)
	w.background.CornerRadius = 8

	if w.item.IconPath != "" {
		w.icon = canvas.NewImageFromFile(w.item.IconPath)
	} else {
		w.icon = canvas.NewImageFromResource(theme.FileIcon())
	}
	w.icon.FillMode = canvas.ImageFillContain
	w.icon.SetMinSize(fyne.NewSize(w.app.IconSize, w.app.IconSize))

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

type scriptWidgetRenderer struct {
	w          *ScriptWidget
	content    *fyne.Container
	labelTexts []*canvas.Text
}

func (r *scriptWidgetRenderer) Layout(s fyne.Size) { r.content.Resize(s) }
func (r *scriptWidgetRenderer) MinSize() fyne.Size { return r.content.MinSize() }
func (r *scriptWidgetRenderer) Destroy()           {}
func (r *scriptWidgetRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.content}
}
func (r *scriptWidgetRenderer) Refresh() {
	fg := theme.ForegroundColor()
	for _, txt := range r.labelTexts {
		txt.Color = fg
		txt.TextSize = r.w.app.FontSize
	}
	r.content.Refresh()
}

// Hoverable
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
	if time.Since(w.lastTap) < 500*time.Millisecond {
		w.animateLaunch()
		w.app.runScript(w.item)
		w.lastTap = time.Time{}
	} else {
		w.lastTap = time.Now()
	}
}

func (w *ScriptWidget) animateLaunch() {
	fade := fyne.NewAnimation(200*time.Millisecond, func(v float32) {
		if v < 0.5 {
			w.icon.Translucency = float64(v)
		} else {
			w.icon.Translucency = float64(1 - v)
		}
		w.icon.Refresh()

		if v < 0.5 {
			w.background.FillColor = theme.SelectionColor()
		} else {
			w.background.FillColor = theme.HoverColor()
		}
		w.background.Refresh()
	})
	fade.Start()
}

// 텍스트 래핑
func wrapSmart(text string, size float32, maxWidth float32) []string {
	if text == "" {
		return []string{}
	}

	style := fyne.TextStyle{}
	var lines []string
	var currentLine string

	words := strings.Fields(text)
	for _, word := range words {
		if fyne.MeasureText(word, size, style).Width > maxWidth {
			if currentLine != "" {
				lines = append(lines, currentLine)
				currentLine = ""
			}
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
				currentLine = chunk
			}
		} else {
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

	if len(lines) > 2 {
		lines = lines[:2]
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
