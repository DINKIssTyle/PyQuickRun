import Foundation
import AppKit

class ScriptRunner {
    static func runScript(item: ScriptItem, pythonPath: String) {
        // Determine interpreter
        var interpreter = item.interpMac
        if interpreter.isEmpty { interpreter = item.interpDefault }
        if interpreter.isEmpty { interpreter = pythonPath }
        if interpreter.isEmpty { interpreter = "/usr/bin/python3" } // Fallback
        
        print("Run Code: \(item.name) / Python: \(interpreter)")
        
        if item.terminal {
            runInTerminal(interpreter: interpreter, scriptPath: item.path)
        } else {
            runProcess(interpreter: interpreter, scriptPath: item.path)
        }
    }
    
    private static func runProcess(interpreter: String, scriptPath: String) {
        let task = Process()
        task.executableURL = URL(fileURLWithPath: interpreter)
        task.arguments = [scriptPath]
        
        // Environment
        var env = ProcessInfo.processInfo.environment
        env["PYTHONUNBUFFERED"] = "1"
        task.environment = env
        
        // Standard Output (Optional: Redirect to a log view eventually)
        // task.standardOutput = Pipe()
        
        do {
            try task.run()
        } catch {
            print("Error running script: \(error)")
            // Show alert?
        }
    }
    
    private static func runInTerminal(interpreter: String, scriptPath: String) {
        let scriptSource = "tell application \"Terminal\" to do script \"\(interpreter) \(scriptPath)\""
        if let appleScript = NSAppleScript(source: scriptSource) {
            var error: NSDictionary?
            appleScript.executeAndReturnError(&error)
            if let error = error {
                print("AppleScript Error: \(error)")
            }
        }
    }
    
    static func openFileLocation(path: String) {
        let fileURL = URL(fileURLWithPath: path)
        NSWorkspace.shared.activateFileViewerSelecting([fileURL])
    }
}
