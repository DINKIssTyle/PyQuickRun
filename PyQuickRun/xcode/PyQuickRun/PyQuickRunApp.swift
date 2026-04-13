// Created by DINKIssTyle on 2026. Copyright (C) 2026 DINKI'ssTyle. All rights reserved.

import SwiftUI

import UniformTypeIdentifiers

@main
struct PyQuickRunApp: App {
    @AppStorage("pythonPath") var pythonPath: String = "/usr/bin/python3"
    @AppStorage("useTerminal") var useTerminal: Bool = true
    @AppStorage("closeOnSuccess") var closeOnSuccess: Bool = false

    var body: some Scene {
        WindowGroup("PyQuickRun - Python Launcher") {
            ContentView(pythonPath: $pythonPath, useTerminal: $useTerminal, closeOnSuccess: $closeOnSuccess)
                .frame(minWidth: 500, maxWidth: 500, minHeight: 360, maxHeight: 360)
        }
        .windowResizability(.contentSize)
    }
}

struct ContentView: View {
    @Binding var pythonPath: String
    @Binding var useTerminal: Bool
    @Binding var closeOnSuccess: Bool
    
    @State private var statusMessage: String = "Ready to run."
    @State private var isTargeted: Bool = false
    @State private var isRunning: Bool = false
    
    // --- xcode option dialog states ---
    @State private var showingOptions = false
    @State private var pendingScriptURL: URL? = nil
    @State private var dialogUseTerminal: Bool = true
    @State private var dialogCloseOnSuccess: Bool = false
    @State private var dialogCategory: String = ""

    var body: some View {
        VStack(alignment: .leading, spacing: 20) {
            
            Spacer().frame(height: 10)

            // --- 설정 섹션 ---
            VStack(alignment: .leading, spacing: 10) {
                Text("Interpreter Path (uv or python):")
                    .font(.headline)
                
                HStack {
                    TextField("e.g., ~/pythons/default/.venv/bin/python", text: $pythonPath)
                        .textFieldStyle(RoundedBorderTextFieldStyle())
                    
                    Button("Browse") {
                        selectInterpreter()
                    }
                }
                
                VStack(alignment: .leading, spacing: 5) {
                    HStack {
                        Toggle("Run in Terminal window", isOn: $useTerminal)
                            .toggleStyle(.checkbox)
                        
                        Spacer()
                        
                        Text("Default: \(resolvePath(pythonPath))")
                            .font(.caption)
                            .foregroundColor(.secondary)
                            .lineLimit(1)
                            .truncationMode(.middle)
                            .frame(maxWidth: 220, alignment: .trailing)
                    }
                    
                    Toggle("Close window after successful execution", isOn: $closeOnSuccess)
                        .toggleStyle(.checkbox)
                }
            }

            Divider()

            // --- 드래그 앤 드롭 섹션 ---
            ZStack {
                RoundedRectangle(cornerRadius: 12)
                    .stroke(isTargeted ? Color.accentColor : Color.gray.opacity(0.3), lineWidth: 2)
                    .background(isTargeted ? Color.accentColor.opacity(0.05) : Color(NSColor.controlBackgroundColor))
                
                if isRunning {
                    ProgressView("Running Script...")
                } else {
                    VStack(spacing: 10) {
                        Image(systemName: "doc.text.fill")
                            .font(.system(size: 40))
                            .foregroundColor(isTargeted ? .accentColor : .gray)
                        
                        VStack(spacing: 2) {
                            Text("Drag & Drop .py file here")
                                .font(.headline)
                                .foregroundColor(.primary)
                            Text("or double-click file in Finder")
                                .font(.subheadline)
                                .foregroundColor(.secondary)
                        }
                    }
                }
            }
            .frame(height: 120)
            .onDrop(of: [.fileURL], isTargeted: $isTargeted) { providers in
                return handleDrop(providers: providers)
            }

            Spacer().frame(height: 15)
            
            // --- 상태 메시지바 & 카피라이트 ---
            HStack(alignment: .top) {
                Image(systemName: statusMessage.contains("Error") || statusMessage.contains("Failed") ? "exclamationmark.triangle.fill" : "info.circle.fill")
                    .foregroundColor(statusMessage.contains("Error") || statusMessage.contains("Failed") ? .red : .blue)
                    .padding(.top, 2)
                
                ScrollView {
                    Text(statusMessage)
                        .font(.caption)
                        .foregroundColor(.primary)
                        .multilineTextAlignment(.leading)
                        .frame(maxWidth: .infinity, alignment: .leading)
                }
                .frame(height: 40)
                
                Spacer(minLength: 10)
                
                Text("© 2026 DINKIssTyle")
                    .font(.system(size: 10))
                    .foregroundColor(.gray.opacity(0.6))
                    .padding(.top, 2)
                    .padding(.trailing, 5)
            }
            .padding(.bottom, 20)
        }
        .padding(25)
        .onOpenURL { url in
            executeScript(url: url)
        }
        .sheet(isPresented: $showingOptions) {
            OptionDialogView(
                useTerminal: $dialogUseTerminal,
                closeOnSuccess: $dialogCloseOnSuccess,
                category: $dialogCategory,
                onRunNow: {
                    showingOptions = false
                    if let url = pendingScriptURL {
                        performExecution(url: url, manualTerminal: dialogUseTerminal, manualClose: dialogCloseOnSuccess)
                    }
                },
                onSaveAndRun: {
                    showingOptions = false
                    if let url = pendingScriptURL {
                        saveAndRun(url: url, term: dialogUseTerminal, close: dialogCloseOnSuccess, category: dialogCategory)
                    }
                }
            )
        }
    }

    // --- 기능 함수들 ---

    func handleDrop(providers: [NSItemProvider]) -> Bool {
        if let provider = providers.first(where: { $0.hasItemConformingToTypeIdentifier(UTType.fileURL.identifier) }) {
            provider.loadItem(forTypeIdentifier: UTType.fileURL.identifier, options: nil) { (urlData, error) in
                DispatchQueue.main.async {
                    if let urlData = urlData as? Data,
                       let url = URL(dataRepresentation: urlData, relativeTo: nil) {
                        if url.pathExtension.lowercased() == "py" {
                            executeScript(url: url)
                        } else {
                            statusMessage = "Error: Only .py files are supported."
                        }
                    } else if let url = urlData as? URL {
                         if url.pathExtension.lowercased() == "py" {
                            executeScript(url: url)
                        } else {
                            statusMessage = "Error: Only .py files are supported."
                        }
                    }
                }
            }
            return true
        }
        return false
    }

    func selectInterpreter() {
        let panel = NSOpenPanel()
        panel.canChooseFiles = true
        panel.canChooseDirectories = true // Enable folder selection
        panel.allowsMultipleSelection = false
        panel.prompt = "Select Python or Project Folder"
        
        if panel.runModal() == .OK, let url = panel.url {
            var finalPath = url.path
            
            // Check if it's a directory
            var isDir: ObjCBool = false
            if FileManager.default.fileExists(atPath: finalPath, isDirectory: &isDir) && isDir.boolValue {
                // Attempt to find venv python
                let candidates = [
                    url.appendingPathComponent(".venv/bin/python").path,
                    url.appendingPathComponent(".venv/bin/python3").path,
                    url.appendingPathComponent("venv/bin/python").path,
                    url.appendingPathComponent("venv/bin/python3").path,
                    url.appendingPathComponent("env/bin/python").path
                ]
                
                if let found = candidates.first(where: { FileManager.default.fileExists(atPath: $0) }) {
                    finalPath = found
                    self.statusMessage = "Auto-detected venv: \(finalPath)"
                } else {
                    self.statusMessage = "No standard virtualenv found in selected folder."
                    // If not found, we don't update pythonPath because it must be a binary/file usually.
                    return 
                }
            }
            
            self.pythonPath = finalPath
        }
    }

    func resolvePath(_ path: String) -> String {
        return (path as NSString).expandingTildeInPath
    }
    
    // [UPDATED] 헤더 파싱: 키-값 포맷 지원 및 존재 여부 반환
    func scanPqrHeader(url: URL) -> (interpreter: String?, terminalOverride: Bool?, category: String?, hasPqr: Bool) {
        do {
            let content = try String(contentsOf: url, encoding: .utf8)
            let lines = content.components(separatedBy: .newlines)

            // 상단 20줄만 검사
            for line in lines.prefix(20) {
                let trimmed = line.trimmingCharacters(in: .whitespaces)
                guard trimmed.lowercased().hasPrefix("#pqr") else { continue }

                // '#pqr' 이후 부분만 추출
                var remainder = trimmed.dropFirst(4) // remove '#pqr'
                remainder = Substring(remainder.trimmingCharacters(in: .whitespacesAndNewlines))

                let pairs = remainder.split(separator: ";").map { $0.trimmingCharacters(in: .whitespaces) }

                var dict: [String: String] = [:]
                for pair in pairs {
                    let parts = pair.split(separator: "=", maxSplits: 1).map { String($0).trimmingCharacters(in: .whitespaces) }
                    if parts.count == 2, !parts[0].isEmpty {
                        dict[parts[0].lowercased()] = parts[1]
                    }
                }

                let category = dict["cat"]
                let macPath = dict["mac"]
                let termString = dict["term"]?.lowercased()

                var terminalOverride: Bool? = nil
                if let termString = termString {
                    if ["true", "1", "yes", "y"].contains(termString) {
                        terminalOverride = true
                    } else if ["false", "0", "no", "n"].contains(termString) {
                        terminalOverride = false
                    }
                }

                return (interpreter: macPath, terminalOverride: terminalOverride, category: category, hasPqr: true)
            }
        } catch {
            print("Header scan failed: \(error)")
        }
        return (nil, nil, nil, false)
    }

    func executeScript(url: URL) {
        let header = scanPqrHeader(url: url)
        
        if !header.hasPqr {
            // #pqr 헤더가 없는 경우 옵션창 표시
            self.pendingScriptURL = url
            self.dialogUseTerminal = false // Default to unchecked
            self.dialogCloseOnSuccess = self.closeOnSuccess
            self.dialogCategory = "" // 초기화
            self.showingOptions = true
        } else {
            performExecution(url: url, header: header)
        }
    }

    func saveAndRun(url: URL, term: Bool, close: Bool, category: String) {
        do {
            let content = try String(contentsOf: url, encoding: .utf8)
            var lines = content.components(separatedBy: .newlines)
            
            // #pqr 태그 조립 (term, cat 포함)
            var tagParts = ["term=\(term)"]
            if !category.trimmingCharacters(in: .whitespaces).isEmpty {
                tagParts.append("cat=\(category)")
            }
            let headerTag = "#pqr " + tagParts.joined(separator: "; ")
            
            // Shebang이 있으면 그 다음에 삽입, 없으면 제일 위에 삽입
            if let firstLine = lines.first, firstLine.hasPrefix("#!") {
                lines.insert(headerTag, at: 1)
            } else {
                lines.insert(headerTag, at: 0)
            }
            
            let newContent = lines.joined(separator: "\n")
            try newContent.write(to: url, atomically: true, encoding: .utf8)
            
            // 저장 후 즉시 실행 (이제 헤더가 있으므로 바로 실행됨)
            executeScript(url: url)
        } catch {
            statusMessage = "Error saving header: \(error.localizedDescription)"
        }
    }

    func performExecution(url: URL, header: (interpreter: String?, terminalOverride: Bool?, category: String?, hasPqr: Bool)? = nil, manualTerminal: Bool? = nil, manualClose: Bool? = nil) {
        let scriptPath = url.path
        let directoryPath = url.deletingLastPathComponent().path
        
        var finalInterpreter = resolvePath(pythonPath)
        var shouldRunInTerminal = manualTerminal ?? useTerminal
        let shouldCloseOnSuccess = manualClose ?? closeOnSuccess
        
        // 1. 헤더 혹은 .venv 감지
        if let h = header, let customPath = h.interpreter, !customPath.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            let resolvedCustom = resolvePath(customPath)
            if !resolvedCustom.isEmpty {
                finalInterpreter = resolvedCustom
            }
        } else {
            let venvCandidates = [
                url.deletingLastPathComponent().appendingPathComponent(".venv/bin/python").path,
                url.deletingLastPathComponent().appendingPathComponent(".venv/bin/python3").path
            ]
            
            if let foundVenv = venvCandidates.first(where: { FileManager.default.fileExists(atPath: $0) }) {
                finalInterpreter = foundVenv
                statusMessage = "Using local .venv: \(finalInterpreter)"
            }
        }

        // 헤더의 term 우선순위가 가장 높음
        if let h = header, let termOverride = h.terminalOverride {
            shouldRunInTerminal = termOverride
        }
        
        if !FileManager.default.fileExists(atPath: finalInterpreter) {
            statusMessage = "Error: Interpreter not found!\nPath: \(finalInterpreter)"
            return
        }

        if shouldRunInTerminal {
            runInTerminal(interpreter: finalInterpreter, scriptPath: scriptPath, directoryPath: directoryPath, closeOnSuccess: shouldCloseOnSuccess)
        } else {
            runInBackground(interpreter: finalInterpreter, scriptPath: scriptPath, directoryPath: directoryPath, closeOnSuccess: shouldCloseOnSuccess)
        }
    }

    func runInTerminal(interpreter: String, scriptPath: String, directoryPath: String, closeOnSuccess: Bool) {
        let command = "cd '\(directoryPath)' && '\(interpreter)' '\(scriptPath)' && echo Exit status: $? && exit 1"
        statusMessage = "Launching in Terminal...\nUsing: \(interpreter)"
        
        let appleScriptSource = """
        tell application "Terminal"
            activate
            do script "\(command)"
        end tell
        """
        
        var error: NSDictionary?
        if let scriptObject = NSAppleScript(source: appleScriptSource) {
            scriptObject.executeAndReturnError(&error)
            if error == nil {
                statusMessage = "Launched in Terminal successfully."
                if closeOnSuccess {
                    DispatchQueue.main.asyncAfter(deadline: .now() + 0.5) {
                        NSApplication.shared.terminate(nil)
                    }
                }
            }
        }
        if let error = error {
            let msg = error["NSAppleScriptErrorMessage"] as? String ?? "Unknown Error"
            statusMessage = "Error: \(msg)"
        }
    }

    func runInBackground(interpreter: String, scriptPath: String, directoryPath: String, closeOnSuccess: Bool) {
        isRunning = true
        statusMessage = "Running...\nUsing: \(interpreter)"

        DispatchQueue.global(qos: .userInitiated).async {
            let task = Process()
            task.executableURL = URL(fileURLWithPath: interpreter)
            task.arguments = [scriptPath]
            task.currentDirectoryURL = URL(fileURLWithPath: directoryPath)
            
            let outputPipe = Pipe()
            let errorPipe = Pipe()
            task.standardOutput = outputPipe
            task.standardError = errorPipe
            
            do {
                try task.run()
                
                let outputData = outputPipe.fileHandleForReading.readDataToEndOfFile()
                let errorData = errorPipe.fileHandleForReading.readDataToEndOfFile()
                
                task.waitUntilExit()
                
                let output = String(data: outputData, encoding: .utf8) ?? ""
                let errorMsg = String(data: errorData, encoding: .utf8) ?? ""
                
                DispatchQueue.main.async {
                    self.isRunning = false
                    if task.terminationStatus == 0 {
                        let displayMsg = output.isEmpty ? "Success (No Output)" : output
                        self.statusMessage = "Success:\n\(displayMsg)"
                        
                        if closeOnSuccess {
                             DispatchQueue.main.asyncAfter(deadline: .now() + 1.0) {
                                NSApplication.shared.terminate(nil)
                            }
                        }
                    } else {
                        let displayMsg = errorMsg.isEmpty ? "Exit Code \(task.terminationStatus)" : errorMsg
                        self.statusMessage = "Failed:\n\(displayMsg)"
                    }
                }
            } catch {
                DispatchQueue.main.async {
                    self.isRunning = false
                    self.statusMessage = "Error: \(error.localizedDescription)"
                }
            }
        }
    }
}

// --- xcode option dialog view ---
struct OptionDialogView: View {
    @Binding var useTerminal: Bool
    @Binding var closeOnSuccess: Bool
    @Binding var category: String
    var onRunNow: () -> Void
    var onSaveAndRun: () -> Void
    
    var body: some View {
        VStack(spacing: 25) {
            VStack(spacing: 12) {
                Image(systemName: "questionmark.circle.fill")
                    .font(.system(size: 44))
                    .foregroundColor(.accentColor)
                
                Text("No #pqr header found")
                    .font(.title2.bold())
            }
            
            VStack(alignment: .leading, spacing: 14) {
                VStack(alignment: .leading, spacing: 8) {
                    Text("Category:")
                        .font(.subheadline)
                        .foregroundColor(.secondary)
                    TextField("e.g. Utility, Tool, AI", text: $category)
                        .textFieldStyle(RoundedBorderTextFieldStyle())
                }

                Divider().padding(.vertical, 5)

                Text("Next time this script will:")
                    .font(.subheadline)
                    .foregroundColor(.secondary)
                
                VStack(alignment: .leading, spacing: 8) {
                    Toggle("Run in Terminal window", isOn: $useTerminal)
                        .toggleStyle(.checkbox)
                    
                    Toggle("Close window after successful execution", isOn: $closeOnSuccess)
                        .toggleStyle(.checkbox)
                }
            }
            .padding(20)
            .background(Color(NSColor.controlBackgroundColor))
            .cornerRadius(12)
            .overlay(
                RoundedRectangle(cornerRadius: 12)
                    .stroke(Color.gray.opacity(0.1), lineWidth: 1)
            )

            HStack(spacing: 15) {
                Button(action: onRunNow) {
                    Text("Run Now (⌘D)")
                        .frame(minWidth: 140, minHeight: 30)
                }
                .keyboardShortcut("d", modifiers: .command)
                
                Button(action: onSaveAndRun) {
                    Text("Save & Run (⌘S)")
                        .frame(minWidth: 140, minHeight: 30)
                }
                .buttonStyle(.borderedProminent)
                .keyboardShortcut("s", modifiers: .command)
            }
        }
        .padding(30)
        .frame(width: 420)
    }
}
