import Foundation
import Combine
import SwiftUI

class LauncherViewModel: ObservableObject {
    // published properties for UI
    @Published var categories: [String] = []
    @Published var scripts: [String: [ScriptItem]] = [:] // key: Category
    @Published var currentCategory: String = "All"
    @Published var searchText: String = ""
    @Published var filteredScripts: [ScriptItem] = []
    
    // Settings
    @AppStorage("RegisteredFolders") var registeredFoldersJson: String = "[]"
    @AppStorage("PythonPath") var pythonPath: String = "/usr/bin/python3"
    @AppStorage("IconSize") var iconSize: Double = 80.0
    
    private var registeredFolders: [String] {
        get {
            guard let data = registeredFoldersJson.data(using: .utf8) else { return [] }
            return (try? JSONDecoder().decode([String].self, from: data)) ?? []
        }
        set {
            guard let data = try? JSONEncoder().encode(newValue) else { return }
            registeredFoldersJson = String(data: data, encoding: .utf8) ?? "[]"
        }
    }
    
    private var cancellables = Set<AnyCancellable>()

    init() {
        $searchText
            .debounce(for: .milliseconds(200), scheduler: RunLoop.main)
            .removeDuplicates()
            .sink { [weak self] _ in
                self?.updateFilteredScripts()
            }
            .store(in: &cancellables)
            
        refreshScripts()
    }
    
    // MARK: - Script Management
    
    func refreshScripts() {
        var newScripts: [String: [ScriptItem]] = [:]
        var newCategories: Set<String> = []
        
        let folders = registeredFolders
        
        for folder in folders {
            let folderURL = URL(fileURLWithPath: folder)
            let iconFolder = folderURL.appendingPathComponent("icon")
            
            // Enumerate files
            guard let fileURLs = try? FileManager.default.contentsOfDirectory(at: folderURL, includingPropertiesForKeys: nil) else { continue }
            
            for fileURL in fileURLs {
                if fileURL.pathExtension == "py" {
                    let fullPath = fileURL.path
                    let fileName = fileURL.deletingPathExtension().lastPathComponent
                    
                    // Icon check
                    var iconPath: String?
                    let specificIcon = iconFolder.appendingPathComponent("\(fileName).png")
                    let defaultIcon = iconFolder.appendingPathComponent("default.png")
                    
                    if FileManager.default.fileExists(atPath: specificIcon.path) {
                        iconPath = specificIcon.path
                    } else if FileManager.default.fileExists(atPath: defaultIcon.path) {
                        iconPath = defaultIcon.path
                    }
                    
                    let meta = ScriptParser.parseHeader(at: fullPath)
                    
                    let item = ScriptItem(
                        name: fileName,
                        path: fullPath,
                        category: meta.category,
                        iconPath: iconPath,
                        interpDefault: meta.def,
                        interpMac: meta.mac,
                        interpWin: meta.win,
                        interpUbuntu: meta.ubuntu,
                        terminal: meta.terminal
                    )
                    
                    newScripts[meta.category, default: []].append(item)
                    newCategories.insert(meta.category)
                }
            }
        }
        
        var sortedCats = newCategories.sorted()
        // Ensure "Uncategorized" is last if present
        if let idx = sortedCats.firstIndex(of: "Uncategorized") {
            sortedCats.remove(at: idx)
            sortedCats.append("Uncategorized")
        }
        
        DispatchQueue.main.async {
            self.scripts = newScripts
            self.categories = sortedCats
            self.updateFilteredScripts()
        }
    }
    
    func updateFilteredScripts() {
        var allDisplay: [ScriptItem] = []
        
        if currentCategory == "All" {
            for (_, items) in scripts {
                allDisplay.append(contentsOf: items)
            }
        } else {
            allDisplay = scripts[currentCategory] ?? []
        }
        
        allDisplay.sort { $0.name.localizedCaseInsensitiveCompare($1.name) == .orderedAscending }
        
        if !searchText.isEmpty {
            allDisplay = allDisplay.filter { $0.name.localizedCaseInsensitiveContains(searchText) }
        }
        
        self.filteredScripts = allDisplay
    }
    
    // MARK: - Actions
    
    func runScript(_ item: ScriptItem) {
        ScriptRunner.runScript(item: item, pythonPath: pythonPath)
    }
    
    func openLocation(_ item: ScriptItem) {
        ScriptRunner.openFileLocation(path: item.path)
    }
    
    // MARK: - Settings Actions
    
    func getRegisteredFolders() -> [String] {
        return registeredFolders
    }
    
    func addFolder(_ path: String) {
        var current = registeredFolders
        if !current.contains(path) {
            current.append(path)
            registeredFolders = current
            refreshScripts()
        }
    }
    
    func removeFolder(at index: Int) {
        var current = registeredFolders
        current.remove(at: index)
        registeredFolders = current
        refreshScripts()
    }
    
    func updateProperties(for item: ScriptItem, category: String, mac: String, win: String, ubuntu: String, terminal: Bool) {
        ScriptParser.updateScriptMetadata(path: item.path, category: category, mac: mac, win: win, ubuntu: ubuntu, terminal: terminal)
        refreshScripts()
    }
}
