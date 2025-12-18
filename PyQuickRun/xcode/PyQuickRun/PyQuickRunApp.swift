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
                
                Text("© 2025 DINKIssTyle")
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
    
    // [UPDATED] 헤더 파싱: 키-값 포맷 지원
    // 형식: #pqr cat=Tool; mac=/path/to/python; win=...; linux=...; term=false
    // 반환값: (인터프리터경로, 터미널강제옵션(있을때만), 카테고리)
    func scanPqrHeader(url: URL) -> (interpreter: String?, terminalOverride: Bool?, category: String?) {
        do {
            let content = try String(contentsOf: url, encoding: .utf8)
            let lines = content.components(separatedBy: .newlines)

            // 상단 20줄만 검사
            for line in lines.prefix(20) {
                let trimmed = line.trimmingCharacters(in: .whitespaces)
                guard trimmed.lowercased().hasPrefix("#pqr") else { continue }

                // '#pqr' 이후 부분만 추출
                var remainder = trimmed.dropFirst(4) // remove '#pqr'
                // 앞 공백과 콜론/대시 등 제거
                remainder = Substring(remainder.trimmingCharacters(in: .whitespacesAndNewlines))

                // 세미콜론으로 분리된 key=value 쌍 파싱
                let pairs = remainder.split(separator: ";").map { $0.trimmingCharacters(in: .whitespaces) }

                var dict: [String: String] = [:]
                for pair in pairs {
                    let parts = pair.split(separator: "=", maxSplits: 1).map { String($0).trimmingCharacters(in: .whitespaces) }
                    if parts.count == 2, !parts[0].isEmpty {
                        dict[parts[0].lowercased()] = parts[1]
                    }
                }

                // 키 추출
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

                // 현재는 macOS 전용이므로 mac 키를 우선 사용
                return (interpreter: macPath, terminalOverride: terminalOverride, category: category)
            }
        } catch {
            print("Header scan failed: \(error)")
        }
        return (nil, nil, nil)
    }

    func executeScript(url: URL) {
        let scriptPath = url.path
        let directoryPath = url.deletingLastPathComponent().path
        
        var finalInterpreter = resolvePath(pythonPath)
        // 기본적으로는 체크박스 설정을 따름
        var shouldRunInTerminal = useTerminal
        
        // 1. 헤더 스캔 (새 포맷)
        let header = scanPqrHeader(url: url)
        if let customPath = header.interpreter, !customPath.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            let resolvedCustom = resolvePath(customPath)
            if !resolvedCustom.isEmpty {
                finalInterpreter = resolvedCustom
                print("Custom interpreter: \(finalInterpreter)")
            }
        }
        // term이 명시되면 체크박스 무시하고 강제 적용
        if let termOverride = header.terminalOverride {
            shouldRunInTerminal = termOverride
            print("Terminal override from header: \(termOverride)")
        }
        
        // 경로 검증
        if !FileManager.default.fileExists(atPath: finalInterpreter) {
            statusMessage = "Error: Interpreter not found!\nPath: \(finalInterpreter)"
            return
        }

        // 결정된 모드로 실행
        if shouldRunInTerminal {
            runInTerminal(interpreter: finalInterpreter, scriptPath: scriptPath, directoryPath: directoryPath)
        } else {
            runInBackground(interpreter: finalInterpreter, scriptPath: scriptPath, directoryPath: directoryPath)
        }
    }

    // A. 터미널 실행
    func runInTerminal(interpreter: String, scriptPath: String, directoryPath: String) {
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

    // B. 백그라운드 실행
    func runInBackground(interpreter: String, scriptPath: String, directoryPath: String) {
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
                        
                        if self.closeOnSuccess {
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
