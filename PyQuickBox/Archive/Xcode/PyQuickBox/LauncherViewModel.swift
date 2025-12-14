import SwiftUI
import Combine
import AppKit

// 데이터 모델 수정 (터미널 옵션 추가)
struct LauncherScriptItem: Identifiable, Codable {
    var id = UUID()
    let name: String
    let path: String
    let category: String
    let image: NSImage?
    let interpreterPath: String?
    let useTerminal: Bool // [추가] 터미널 실행 여부
    
    // Codable 제외 설정 (NSImage는 Codable 아님)
    enum CodingKeys: String, CodingKey {
        case id, name, path, category, interpreterPath, useTerminal
    }
    
    init(name: String, path: String, category: String, iconPath: String?, interpreterPath: String?, useTerminal: Bool = false) {
        self.name = name
        self.path = path
        self.category = category
        self.interpreterPath = interpreterPath
        self.useTerminal = useTerminal
        
        if let iconPath = iconPath, let img = NSImage(contentsOfFile: iconPath) {
            self.image = img
        } else {
            self.image = NSImage(systemSymbolName: "doc.text", accessibilityDescription: nil)
        }
    }
    
    // 디코딩용 (단순화)
    init(from decoder: Decoder) throws {
        fatalError("Not implemented")
    }
    func encode(to encoder: Encoder) throws {}
}

class LauncherViewModel: ObservableObject {
    @Published var groupedScripts: [String: [LauncherScriptItem]] = [:]
    @Published var categories: [String] = []
    
    // 설정값들
    @AppStorage("iconSize") var iconSize: Double = 80.0
    @AppStorage("labelFontSize") var labelFontSize: Double = 12.0
    @AppStorage("defaultInterpreterPath") var defaultInterpreterPath: String = "/usr/bin/python3"
    @AppStorage("registeredFolders") var registeredFoldersData: Data = Data()
    
    var registeredFolders: [String] {
        get {
            if let decoded = try? JSONDecoder().decode([String].self, from: registeredFoldersData) {
                return decoded
            }
            return []
        }
        set {
            if let encoded = try? JSONEncoder().encode(newValue) {
                registeredFoldersData = encoded
            }
        }
    }

    @Published var searchText: String = ""
    @Published var selectedCategory: String? = "All"
    
    // ... (필터링 로직은 기존과 동일하므로 생략 가능, 필요시 추가) ...
    var filteredScripts: [LauncherScriptItem] {
        let category = selectedCategory ?? "All"
        let allScripts = groupedScripts.values.flatMap { $0 }
        
        let targetScripts: [LauncherScriptItem]
        if category == "All" {
            targetScripts = allScripts
        } else {
            targetScripts = groupedScripts[category] ?? []
        }
        
        if searchText.isEmpty {
            return targetScripts.sorted { $0.name < $1.name }
        } else {
            return targetScripts.filter { $0.name.localizedCaseInsensitiveContains(searchText) }
        }
    }
    
    // MARK: - 스크립트 실행 (터미널 로직 추가됨)
    func runScript(_ script: LauncherScriptItem) {
        let python = (script.interpreterPath != nil && !script.interpreterPath!.isEmpty) ? script.interpreterPath! : defaultInterpreterPath
        
        print("🚀 실행 요청: \(script.name)")
        print("   - Python: \(python)")
        print("   - Terminal: \(script.useTerminal)")
        
        // [옵션 1] 터미널에서 실행
        if script.useTerminal {
            // AppleScript를 사용하여 터미널 앱을 열고 명령어를 실행합니다.
            let command = "\(python) '\(script.path)'"
            let appleScript = """
            tell application "Terminal"
                activate
                do script "\(command)"
            end tell
            """
            
            var error: NSDictionary?
            if let scriptObject = NSAppleScript(source: appleScript) {
                scriptObject.executeAndReturnError(&error)
                if let error = error {
                    print("❌ 터미널 실행 실패: \(error)")
                }
            }
            return
        }
        
        // [옵션 2] 백그라운드 실행 (기존 로직)
        let task = Process()
        task.executableURL = URL(fileURLWithPath: python)
        task.arguments = [script.path]
        
        // 환경변수 설정 (로그가 바로 보이도록)
        var env = ProcessInfo.processInfo.environment
        env["PYTHONUNBUFFERED"] = "1"
        task.environment = env
        
        do {
            try task.run()
        } catch {
            print("❌ 실행 실패: \(error)")
        }
    }
    
    // MARK: - 파일 스캔 및 파싱 (형식 변경됨)
    func refreshScripts() {
        DispatchQueue.global(qos: .userInitiated).async {
            var newGrouped: [String: [LauncherScriptItem]] = [:]
            var newCategories: Set<String> = []
            let fileManager = FileManager.default
            
            for folderPath in self.registeredFolders {
                guard let items = try? fileManager.contentsOfDirectory(atPath: folderPath) else { continue }
                
                for item in items where item.hasSuffix(".py") {
                    let fullPath = (folderPath as NSString).appendingPathComponent(item)
                    let fileName = (item as NSString).deletingPathExtension
                    
                    // 아이콘 경로 설정
                    let iconFolder = (folderPath as NSString).appendingPathComponent("icon")
                    let specificIcon = (iconFolder as NSString).appendingPathComponent(fileName + ".png")
                    let defaultIcon = (iconFolder as NSString).appendingPathComponent("default.png")
                    
                    var finalIconPath: String? = nil
                    if fileManager.fileExists(atPath: specificIcon) { finalIconPath = specificIcon }
                    else if fileManager.fileExists(atPath: defaultIcon) { finalIconPath = defaultIcon }
                    
                    // [파싱 로직 호출]
                    let (cat, interp, isTerm) = self.parsePyFileHeader(path: fullPath)
                    
                    let scriptItem = LauncherScriptItem(
                        name: fileName,
                        path: fullPath,
                        category: cat,
                        iconPath: finalIconPath,
                        interpreterPath: interp,
                        useTerminal: isTerm // 터미널 옵션 전달
                    )
                    
                    if newGrouped[cat] == nil { newGrouped[cat] = [] }
                    newGrouped[cat]?.append(scriptItem)
                    newCategories.insert(cat)
                }
            }
            
            let sortedCategories = Array(newCategories).sorted { lhs, rhs in
                if lhs == "Uncategorized" { return false }
                if rhs == "Uncategorized" { return true }
                return lhs < rhs
            }
            
            DispatchQueue.main.async {
                self.groupedScripts = newGrouped
                self.categories = sortedCategories
            }
        }
    }
    
    // MARK: - 헤더 파싱 로직 (업데이트됨)
    func parsePyFileHeader(path: String) -> (String, String?, Bool) {
        var category = "Uncategorized"
        var interpreter: String? = nil
        var useTerminal = false
        
        guard let content = try? String(contentsOfFile: path, encoding: .utf8) else {
            return (category, interpreter, useTerminal)
        }
        
        let lines = content.components(separatedBy: .newlines)
        // 상단 20줄만 검사
        for line in lines.prefix(20) {
            let trimmed = line.trimmingCharacters(in: .whitespaces)
            if !trimmed.hasPrefix("#pqr") { continue }
            
            // 1. 카테고리: #pqr cat "Tool"
            if trimmed.contains(" cat ") {
                if let firstQuote = trimmed.firstIndex(of: "\""),
                   let lastQuote = trimmed.lastIndex(of: "\""),
                   firstQuote != lastQuote {
                    category = String(trimmed[trimmed.index(after: firstQuote)..<lastQuote])
                }
            }
            // 2. 맥 경로: #pqr mac /path/to/python
            else if trimmed.contains(" mac ") {
                let components = trimmed.components(separatedBy: " mac ")
                if components.count > 1 {
                    interpreter = components[1].trimmingCharacters(in: .whitespaces)
                }
            }
            // 3. 터미널: #pqr terminal true
            else if trimmed.contains("terminal true") {
                useTerminal = true
            }
        }
        
        return (category, interpreter, useTerminal)
    }
    
    // 기타 함수들 (폴더 추가/삭제, 열기 등)은 기존 코드 유지...
    func addFolder() {
        let panel = NSOpenPanel()
        panel.canChooseDirectories = true
        panel.canChooseFiles = false
        panel.allowsMultipleSelection = false
        if panel.runModal() == .OK, let url = panel.url {
            var folders = registeredFolders
            if !folders.contains(url.path) {
                folders.append(url.path)
                registeredFolders = folders
                refreshScripts()
            }
        }
    }
    
    func removePath(_ path: String) {
        var folders = registeredFolders
        folders.removeAll { $0 == path }
        registeredFolders = folders
        refreshScripts()
    }
    
    func openFileLocation(_ path: String) {
        let url = URL(fileURLWithPath: path)
        NSWorkspace.shared.activateFileViewerSelecting([url])
    }
    
    func editScript(_ path: String) {
        // 기존 editScript 로직 (필요시)
    }
}
