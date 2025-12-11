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
        panel.allowsMultipleSelection = false
        panel.prompt = "Select Python"
        if panel.runModal() == .OK {
            self.pythonPath = panel.url?.path ?? ""
        }
    }

    func resolvePath(_ path: String) -> String {
        return (path as NSString).expandingTildeInPath
    }
    
    // [NEW] 헤더 파싱 (경로 + 터미널 강제 여부)
    // 반환값: (경로, 터미널강제여부)?
    func scanPqrHeader(url: URL) -> (String, Bool)? {
        do {
            let content = try String(contentsOf: url, encoding: .utf8)
            let lines = content.components(separatedBy: .newlines)
            
            // 상단 20줄만 검사
            for line in lines.prefix(20) {
                let trimmed = line.trimmingCharacters(in: .whitespaces)
                
                if trimmed.hasPrefix("#pqr mac") {
                    // "#pqr mac" 제거 후 남은 문자열
                    var remainder = trimmed.dropFirst("#pqr mac".count).trimmingCharacters(in: .whitespaces)
                    var forceTerminal = false
                    
                    // "terminal" 키워드 확인 (대소문자 무시)
                    if remainder.lowercased().hasPrefix("terminal") {
                        forceTerminal = true
                        // "terminal" 제거 후 남은 것이 진짜 경로
                        remainder = remainder.dropFirst("terminal".count).trimmingCharacters(in: .whitespaces)
                    }
                    
                    return (remainder, forceTerminal)
                }
            }
        } catch {
            print("Header scan failed: \(error)")
        }
        return nil
    }

    func executeScript(url: URL) {
        let scriptPath = url.path
        let directoryPath = url.deletingLastPathComponent().path
        
        var finalInterpreter = resolvePath(pythonPath)
        // 기본적으로는 체크박스 설정을 따름
        var shouldRunInTerminal = useTerminal
        
        // 1. 헤더 스캔
        if let (customPath, forceTerminal) = scanPqrHeader(url: url) {
            let resolvedCustom = resolvePath(customPath)
            if !resolvedCustom.isEmpty {
                finalInterpreter = resolvedCustom
                print("Custom interpreter: \(finalInterpreter)")
            }
            
            // [중요] 헤더에 terminal이 있으면 체크박스 무시하고 강제 True
            if forceTerminal {
                shouldRunInTerminal = true
                print("Force terminal mode active")
            }
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
