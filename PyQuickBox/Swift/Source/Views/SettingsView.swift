import SwiftUI

struct SettingsView: View {
    @ObservedObject var viewModel: LauncherViewModel
    @Environment(\.dismiss) var dismiss
    
    var body: some View {
        NavigationStack {
            Form {
                Section("Execution") {
                    HStack {
                        TextField("Default Python Path", text: $viewModel.pythonPath)
                        Button("Browse") {
                            let panel = NSOpenPanel()
                            panel.allowsMultipleSelection = false
                            panel.canChooseFiles = true
                            if panel.runModal() == .OK {
                                if let url = panel.url {
                                    viewModel.pythonPath = url.path
                                }
                            }
                        }
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
