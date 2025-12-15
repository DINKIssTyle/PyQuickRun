import Foundation

struct ScriptItem: Identifiable, Hashable {
    let id = UUID()
    let name: String
    let path: String
    let category: String
    let iconPath: String?
    
    // Interpreter settings
    let interpDefault: String
    let interpMac: String
    let interpWin: String
    let interpUbuntu: String
    let terminal: Bool
    
    var displayName: String {
        return name
    }
}

// Add Equatable/Hashable conformance if needed (auto-synthesized is fine for now)
