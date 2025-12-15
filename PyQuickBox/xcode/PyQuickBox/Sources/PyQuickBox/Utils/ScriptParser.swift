import Foundation

class ScriptParser {
    static func parseHeader(at path: String) -> (category: String, mac: String, win: String, ubuntu: String, terminal: Bool, def: String) {
        var category = "Uncategorized"
        var interpMac = ""
        var interpWin = ""
        var interpUbuntu = ""
        var terminal = false
        var interpDefault = ""

        guard let content = try? String(contentsOfFile: path, encoding: .utf8) else {
            return (category, interpMac, interpWin, interpUbuntu, terminal, interpDefault)
        }

        let lines = content.components(separatedBy: .newlines)
        for line in lines {
            let trimmed = line.trimmingCharacters(in: .whitespaces)
            guard trimmed.hasPrefix("#pqr") else { continue }
            
            // Format 1: #pqr key=val; key=val
            if trimmed.contains("=") {
                let content = trimmed.dropFirst(4).trimmingCharacters(in: .whitespaces) // Remove "#pqr"
                let parts = content.components(separatedBy: ";")
                for part in parts {
                    let kv = part.components(separatedBy: "=")
                    guard kv.count >= 2 else { continue }
                    let key = kv[0].trimmingCharacters(in: .whitespaces).lowercased()
                    let val = kv[1].trimmingCharacters(in: .whitespaces)
                    
                    switch key {
                    case "cat": category = val
                    case "mac": interpMac = val
                    case "win": interpWin = val
                    case "linux", "ubuntu": interpUbuntu = val
                    case "term": terminal = (val.lowercased() == "true")
                    case "def": interpDefault = val
                    default: break
                    }
                }
                continue // Prioritize new format if found on line
            }
            
            // Format 2: Legacy #pqr cat "val"
            // Simple regex replacement or string manipulation
            if trimmed.hasPrefix("#pqr cat") {
                category = extractLegacyValue(from: trimmed, prefix: "#pqr cat") ?? category
            } else if trimmed.hasPrefix("#pqr mac") {
                interpMac = extractLegacyValue(from: trimmed, prefix: "#pqr mac") ?? interpMac
            } else if trimmed.hasPrefix("#pqr win") {
                interpWin = extractLegacyValue(from: trimmed, prefix: "#pqr win") ?? interpWin
            } else if trimmed.hasPrefix("#pqr ubuntu") {
                interpUbuntu = extractLegacyValue(from: trimmed, prefix: "#pqr ubuntu") ?? interpUbuntu
            } else if trimmed.hasPrefix("#pqr terminal") {
                if trimmed.lowercased().contains("true") { terminal = true }
            }
        }
        
        return (category, interpMac, interpWin, interpUbuntu, terminal, interpDefault)
    }
    
    // Extracts value from: #pqr key "value"
    private static func extractLegacyValue(from line: String, prefix: String) -> String? {
        // Find first quote
        guard let firstQuote = line.firstIndex(of: "\"") else { return nil }
        let afterFirst = line[line.index(after: firstQuote)...]
        guard let lastQuote = afterFirst.lastIndex(of: "\"") else { return nil }
        return String(afterFirst[..<lastQuote])
    }
    
    static func updateScriptMetadata(path: String, category: String, mac: String, win: String, ubuntu: String, terminal: Bool) {
        guard let content = try? String(contentsOfFile: path, encoding: .utf8) else { return }
        
        let lines = content.components(separatedBy: .newlines)
        var newLines: [String] = []
        var pqrFound = false
        
        let newPqr = "#pqr cat=\(category); mac=\(mac); win=\(win); linux=\(ubuntu); term=\(terminal)"
        
        for line in lines {
            let trimmed = line.trimmingCharacters(in: .whitespaces)
            if trimmed.hasPrefix("#pqr") {
                if !pqrFound {
                    newLines.append(newPqr)
                    pqrFound = true
                }
                // Skip other existing pqr lines to deduplicate/update
            } else {
                newLines.append(line)
            }
        }
        
        if !pqrFound {
            // Insert at top, preserving shebang if present
            if let first = newLines.first, first.trimmingCharacters(in: .whitespaces).hasPrefix("#!") {
                newLines.insert(newPqr, at: 1)
            } else {
                newLines.insert(newPqr, at: 0)
            }
        }
        
        let newContent = newLines.joined(separator: "\n")
        try? newContent.write(toFile: path, atomically: true, encoding: .utf8)
    }
}
