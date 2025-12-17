import SwiftUI

struct PropertiesView: View {
    let item: ScriptItem
    @ObservedObject var viewModel: LauncherViewModel
    @Environment(\.dismiss) var dismiss
    
    @State private var category: String
    @State private var macPath: String
    @State private var winPath: String
    @State private var ubuntuPath: String
    @State private var terminal: Bool
    
    init(item: ScriptItem, viewModel: LauncherViewModel) {
        self.item = item
        self.viewModel = viewModel
        _category = State(initialValue: item.category)
        _macPath = State(initialValue: item.interpMac)
        _winPath = State(initialValue: item.interpWin)
        _ubuntuPath = State(initialValue: item.interpUbuntu)
        _terminal = State(initialValue: item.terminal)
    }
    
    var body: some View {
        VStack(spacing: 20) {
            Text("Properties")
                .font(.headline)
            
            Text("Script Path: \(item.path)")
                .font(.caption)
                .textSelection(.enabled)
                .lineLimit(2)
            
            Form {
                TextField("Category", text: $category)
                
                Section("Interpreters") {
                    PathPicker(label: "Mac (python/sh)", path: $macPath)
                    PathPicker(label: "Windows (python/exe)", path: $winPath)
                    PathPicker(label: "Ubuntu (python/sh)", path: $ubuntuPath)
                }
                
                Toggle("Run in Terminal", isOn: $terminal)
            }
            .formStyle(.grouped)
            
            HStack {
                Button("Cancel") { dismiss() }
                Button("Save") {
                    viewModel.updateProperties(for: item, category: category, mac: macPath, win: winPath, ubuntu: ubuntuPath, terminal: terminal)
                    dismiss()
                }
                .keyboardShortcut(.defaultAction)
            }
            .padding()
        }
        .frame(minWidth: 400, minHeight: 400)
        .padding()
    }
}

struct PathPicker: View {
    let label: String
    @Binding var path: String
    
    var body: some View {
        HStack {
            TextField(label, text: $path)
            Button("Browse") {
                let panel = NSOpenPanel()
                panel.allowsMultipleSelection = false
                panel.canChooseDirectories = false
                panel.canChooseFiles = true
                if panel.runModal() == .OK {
                    if let url = panel.url {
                        path = url.path
                    }
                }
            }
        }
    }
}
