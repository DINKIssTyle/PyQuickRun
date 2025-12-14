import Foundation
import AppKit

struct ScriptItem: Identifiable, Hashable {
    let id = UUID()
    let name: String
    let path: String
    let category: String
    let iconPath: String?
    let interpreterPath: String? // 실행 시 필요하다면 저장
    
    // 화면에 보여줄 이미지 로드
    var image: NSImage? {
        if let iconPath = iconPath, let img = NSImage(contentsOfFile: iconPath) {
            return img
        }
        // 기본 아이콘 (파이썬 로고 등, 여기서는 시스템 아이콘 사용)
        return NSImage(systemSymbolName: "doc.text.fill", accessibilityDescription: nil)
    }
}
