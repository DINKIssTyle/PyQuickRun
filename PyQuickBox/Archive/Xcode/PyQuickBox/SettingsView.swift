import SwiftUI
import AppKit

struct SettingsView: View {
    @ObservedObject var viewModel: LauncherViewModel
    @Environment(\.dismiss) private var dismiss
    
    var body: some View {
        VStack(spacing: 20) {
            Text("환경 설정")
                .font(.title3)
                .fontWeight(.bold)
                .padding(.top, 10)
            
            // 1. 화면 표시 설정 (신규 추가)
            GroupBox(label: Text("화면 표시 (Display)")) {
                HStack {
                    Text("아이콘 이름 크기 (Label Size):")
                    Text("\(Int(viewModel.labelFontSize))pt")
                        .fontWeight(.bold)
                        .frame(width: 40)
                    
                    Slider(value: $viewModel.labelFontSize, in: 9...24, step: 1)
                }
                .padding(8)
            }
            
            // 2. 파이썬 경로 설정
            GroupBox(label: Text("기본 파이썬 경로 (Python Interpreter)")) {
                HStack(alignment: .firstTextBaseline) {
                    TextField("/Users/...", text: $viewModel.defaultInterpreterPath)
                        .textFieldStyle(RoundedBorderTextFieldStyle())
                    
                    Button("찾기 (Browse)") {
                        let panel = NSOpenPanel()
                        panel.canChooseFiles = true
                        panel.canChooseDirectories = false
                        panel.allowsMultipleSelection = false
                        if panel.runModal() == .OK, let url = panel.url {
                            viewModel.defaultInterpreterPath = url.path
                        }
                    }
                }
                .padding(8)
            }
            
            // 3. 폴더 관리
            GroupBox(label: Text("등록된 폴더 관리 (Folders)")) {
                VStack(spacing: 10) {
                    List {
                        ForEach(viewModel.registeredFolders, id: \.self) { folder in
                            HStack {
                                Image(systemName: "folder.fill")
                                    .foregroundColor(.blue)
                                Text(folder)
                                    .font(.system(size: 13))
                                    .lineLimit(1)
                                    .truncationMode(.middle)
                                
                                Spacer()
                                
                                Button(action: {
                                    viewModel.removePath(folder)
                                }) {
                                    Image(systemName: "trash")
                                        .foregroundColor(.red)
                                }
                                .buttonStyle(PlainButtonStyle())
                            }
                            .padding(.vertical, 4)
                        }
                    }
                    .listStyle(InsetListStyle())
                    .frame(height: 150) // 높이 조절
                    .background(Color(NSColor.controlBackgroundColor))
                    .cornerRadius(6)
                    
                    HStack {
                        Spacer()
                        Button(action: { viewModel.addFolder() }) {
                            Label("폴더 추가", systemImage: "plus")
                        }
                    }
                }
                .padding(8)
            }
            
            Spacer()
            
            HStack {
                Spacer()
                Button("완료") {
                    viewModel.refreshScripts()
                    dismiss()
                }
                .keyboardShortcut(.defaultAction)
                .controlSize(.large)
            }
        }
        .padding(20)
        .frame(width: 550, height: 600) // 창 높이 늘림
    }
}
