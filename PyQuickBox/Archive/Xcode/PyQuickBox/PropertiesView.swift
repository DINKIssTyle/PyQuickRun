import SwiftUI
import Foundation // [필수] FileManager, Date 처리를 위해 추가

struct PropertiesView: View {
    // [중요] 만약 앞서 ScriptItem 이름을 LauncherScriptItem 등으로 바꿨다면 여기도 바꿔야 합니다.
    let script: LauncherScriptItem
    
    @Environment(\.dismiss) var dismiss
    var onSave: () -> Void
    
    // 파일 정보
    @State private var fileSize: String = ""
    @State private var creationDate: String = ""
    @State private var modificationDate: String = ""
    
    // 옵션
    @State private var category: String = ""
    @State private var interpreterPath: String = ""
    @State private var useTerminal: Bool = false
    
    // 변경 감지용
    @State private var initialCategory: String = ""
    @State private var initialInterpreter: String = ""
    @State private var initialUseTerminal: Bool = false
    
    var hasChanges: Bool {
        category != initialCategory ||
        interpreterPath != initialInterpreter ||
        useTerminal != initialUseTerminal
    }
    
    var body: some View {
        VStack(spacing: 20) {
            Text("\(script.name) 속성")
                .font(.headline)
                .padding(.top)
            
            TabView {
                // [탭 1] 일반 정보
                Form {
                    Section(header: Text("파일 정보")) {
                        LabeledContent("이름", value: script.name + ".py")
                        LabeledContent("위치", value: script.path)
                        LabeledContent("크기", value: fileSize)
                        LabeledContent("생성일", value: creationDate)
                        LabeledContent("수정일", value: modificationDate)
                    }
                }
                .tabItem { Label("일반", systemImage: "info.circle") }
                
                // [탭 2] 옵션 설정
                Form {
                    Section(header: Text("QuickBox 설정 (#pqr)")) {
                        TextField("카테고리 (Category)", text: $category)
                            .help("예: #pqr cat \"Tool\"")
                        
                        VStack(alignment: .leading) {
                            Text("실행 파이썬 경로 (Interpreter Path)")
                            HStack {
                                TextField("/usr/bin/python3 등...", text: $interpreterPath)
                                    .textFieldStyle(RoundedBorderTextFieldStyle())
                                
                                Button("찾기") {
                                    let panel = NSOpenPanel()
                                    panel.canChooseFiles = true
                                    panel.allowsMultipleSelection = false
                                    if panel.runModal() == .OK, let url = panel.url {
                                        interpreterPath = url.path
                                    }
                                }
                            }
                        }
                        
                        Toggle("터미널 창에서 실행 (Run in Terminal)", isOn: $useTerminal)
                    }
                }
                .tabItem { Label("옵션", systemImage: "slider.horizontal.3") }
            }
            .padding()
            
            Divider()
            
            HStack {
                Button("취소") { dismiss() }
                .keyboardShortcut(.cancelAction)
                
                Spacer()
                
                Button("변경사항 저장") { saveChanges() }
                .disabled(!hasChanges)
                .keyboardShortcut(.defaultAction)
            }
            .padding()
        }
        .frame(width: 500, height: 450)
        .onAppear(perform: loadData)
    }
    
    func loadData() {
        // 1. 메타데이터 읽기
        let fm = FileManager.default
        if let attrs = try? fm.attributesOfItem(atPath: script.path) {
            let size = attrs[FileAttributeKey.size] as? Int64 ?? 0
            fileSize = ByteCountFormatter.string(fromByteCount: size, countStyle: .file)
            
            let formatter = DateFormatter()
            formatter.dateStyle = .medium
            formatter.timeStyle = .short
            if let c = attrs[FileAttributeKey.creationDate] as? Date { creationDate = formatter.string(from: c) }
            if let m = attrs[FileAttributeKey.modificationDate] as? Date { modificationDate = formatter.string(from: m) }
        }
        
        // 2. 파일 내용 파싱 (줄 단위)
        if let content = try? String(contentsOfFile: script.path, encoding: String.Encoding.utf8) {
            let lines = content.components(separatedBy: CharacterSet.newlines)
            
            for line in lines.prefix(20) { // 상단 20줄만 검사
                let trimmed = line.trimmingCharacters(in: CharacterSet.whitespaces)
                if !trimmed.hasPrefix("#pqr") { continue }
                
                // (1) 카테고리: #pqr cat "Tool"
                if trimmed.contains(" cat ") {
                    if let firstQuote = trimmed.firstIndex(of: "\""),
                       let lastQuote = trimmed.lastIndex(of: "\""),
                       firstQuote != lastQuote {
                        category = String(trimmed[trimmed.index(after: firstQuote)..<lastQuote])
                    }
                }
                
                // (2) 경로: #pqr mac /path/to/python
                else if trimmed.contains(" mac ") {
                    let components = trimmed.components(separatedBy: " mac ")
                    if components.count > 1 {
                        interpreterPath = components[1].trimmingCharacters(in: CharacterSet.whitespaces)
                    }
                }
                
                // (3) 터미널: #pqr terminal true
                else if trimmed.contains("terminal true") {
                    useTerminal = true
                }
            }
            
            // 파일에 경로 없으면 뷰모델값(기본값) 사용
            if interpreterPath.isEmpty { interpreterPath = script.interpreterPath ?? "" }
            
            // 파일에 카테고리 없으면 뷰모델값 사용
            if category.isEmpty && script.category != "Uncategorized" { category = script.category }
        }
        
        initialCategory = category
        initialInterpreter = interpreterPath
        initialUseTerminal = useTerminal
    }
    
    func saveChanges() {
        guard let content = try? String(contentsOfFile: script.path, encoding: String.Encoding.utf8) else { return }
        var lines = content.components(separatedBy: CharacterSet.newlines)
        
        // [수정 핵심] 기존 #pqr 태그 "모두" 제거 (중복 방지)
        lines = lines.filter { line in
            let trimmed = line.trimmingCharacters(in: CharacterSet.whitespaces)
            return !trimmed.lowercased().hasPrefix("#pqr")
        }
        
        // 새 태그 생성
        var newTags: [String] = []
        
        // 1. 카테고리 태그 (#pqr cat "Name")
        if !category.isEmpty {
            newTags.append("#pqr cat \"\(category)\"")
        }
        
        // 2. 맥 경로 태그 (#pqr mac Path)
        if !interpreterPath.isEmpty {
            newTags.append("#pqr mac \(interpreterPath)")
        }
        
        // 3. 터미널 태그
        if useTerminal {
            newTags.append("#pqr terminal true")
        }
        
        // 상단에 삽입
        lines.insert(contentsOf: newTags, at: 0)
        
        // 저장
        let newContent = lines.joined(separator: "\n")
        try? newContent.write(toFile: script.path, atomically: true, encoding: String.Encoding.utf8)
        
        onSave()
        dismiss()
    }
}

