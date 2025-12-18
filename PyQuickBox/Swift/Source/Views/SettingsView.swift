import SwiftUI

struct SettingsView: View {
    @ObservedObject var viewModel: LauncherViewModel
    @Environment(\.dismiss) var dismiss
    
    var body: some View {
        NavigationStack {
            Form {
                Section("Execution") {
                    HStack {
                        TextField("Interpreter Path", text: $viewModel.pythonPath)
                        
                        // 1. Binary Selection
                        Button(action: {
                            let panel = NSOpenPanel()
                            panel.allowsMultipleSelection = false
                            panel.canChooseFiles = true
                            panel.canChooseDirectories = false
                            if panel.runModal() == .OK, let url = panel.url {
                                viewModel.pythonPath = url.path
                            }
                        }) {
                            Image(systemName: "doc.text")
                        }
                        .help("Select Interpreter Binary")
                        
                        // 2. Project Folder Auto-Detection
                        Button(action: {
                            let panel = NSOpenPanel()
                            panel.allowsMultipleSelection = false
                            panel.canChooseFiles = false
                            panel.canChooseDirectories = true // Folder Mode
                            panel.prompt = "Select Project Folder"
                            
                            if panel.runModal() == .OK, let url = panel.url {
                                let path = url.path
                                var foundPath: String?
                                
                                // Python venv check
                                let candidates = [
                                    url.appendingPathComponent(".venv/bin/python").path,
                                    url.appendingPathComponent(".venv/bin/python3").path,
                                    url.appendingPathComponent("venv/bin/python").path,
                                    url.appendingPathComponent("venv/bin/python3").path,
                                    url.appendingPathComponent("env/bin/python").path
                                ]
                                foundPath = candidates.first(where: { FileManager.default.fileExists(atPath: $0) })
                                
                                // Go Project check
                                if foundPath == nil && FileManager.default.fileExists(atPath: url.appendingPathComponent("go.mod").path) {
                                    foundPath = findBinary(name: "go")
                                }
                                
                                // Swift Project check
                                if foundPath == nil && FileManager.default.fileExists(atPath: url.appendingPathComponent("Package.swift").path) {
                                    foundPath = findBinary(name: "swift")
                                }
                                
                                if let found = foundPath {
                                    viewModel.pythonPath = found
                                }
                            }
                        }) {
                            Image(systemName: "folder")
                        }
                        .help("Select Project Root (Auto-detect)")
                    }
                }
                
                Section("Registered Folders") {
                    List {
                        ForEach(viewModel.getRegisteredFolders(), id: \.self) { folder in
                            HStack {
                                Image(systemName: "folder")
                                Text(folder)
                            }
                        }
                        .onDelete { indexSet in
                            indexSet.forEach { viewModel.removeFolder(at: $0) }
                        }
                    }
                    .frame(minHeight: 100)
                    
                    Button(action: addFolder) {
                        Label("Add Folder", systemImage: "plus")
                    }
                }
            }
            .formStyle(.grouped)
            .navigationTitle("Settings")
            .toolbar {
                ToolbarItem(placement: .confirmationAction) {
                    Button("Done") {
                        dismiss()
                    }
                }
            }
            .frame(width: 500, height: 400)
        }
    }
    
    private func findBinary(name: String) -> String? {
        let candidates = [
            "/usr/bin/\(name)",
            "/usr/local/bin/\(name)",
            "/opt/homebrew/bin/\(name)",
            "/usr/local/go/bin/\(name)"
        ]
        return candidates.first(where: { FileManager.default.fileExists(atPath: $0) })
    }

    private func addFolder() {
        let panel = NSOpenPanel()
        panel.canChooseDirectories = true
        panel.canChooseFiles = false
        panel.allowsMultipleSelection = false
        if panel.runModal() == .OK {
            if let url = panel.url {
                viewModel.addFolder(url.path)
            }
        }
    }
}
