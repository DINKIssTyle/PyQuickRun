import SwiftUI
import Combine

struct ContentView: View {
    @StateObject var viewModel = LauncherViewModel()
    @State private var showSettings = false
    @State private var isSearchActive = false
    @FocusState private var isSearchFocused: Bool
    
    // [핵심] 속성창을 띄우기 위해 선택된 스크립트 (이게 nil이 아니면 시트가 열림)
    @State private var selectedScriptForProps: LauncherScriptItem? = nil
    
    var body: some View {
        NavigationSplitView {
            // [왼쪽] 사이드바 영역
            List(selection: $viewModel.selectedCategory) {
                Section {
                    NavigationLink(value: "All") {
                        Label("모든 앱 (All Apps)", systemImage: "square.grid.2x2")
                    }
                } header: {
                    Text("Library")
                }
                
                Section {
                    ForEach(viewModel.categories, id: \.self) { category in
                        NavigationLink(value: category) {
                            Label(category, systemImage: "folder")
                        }
                    }
                } header: {
                    Text("Categories")
                }
            }
            .listStyle(SidebarListStyle())
            .navigationSplitViewColumnWidth(min: 200, ideal: 250)
            
        } detail: {
            // [오른쪽] 메인 컨텐츠 영역
            ZStack {
                Color(NSColor.controlBackgroundColor)
                    .ignoresSafeArea()
                
                ScrollView {
                    LazyVGrid(
                        columns: [GridItem(.adaptive(minimum: viewModel.iconSize + 20), spacing: 20, alignment: .top)],
                        spacing: 30
                    ) {
                        ForEach(viewModel.filteredScripts) { (script: LauncherScriptItem) in
                            // 아이콘 셀 (더블 클릭 및 애니메이션 포함)
                            FinderIconCell(
                                script: script,
                                size: viewModel.iconSize,
                                fontSize: viewModel.labelFontSize,
                                onDoubleTap: {
                                    viewModel.runScript(script)
                                }
                            )
                            // 우클릭 컨텍스트 메뉴
                            .contextMenu {
                                // 1. 실행
                                Button(action: {
                                    viewModel.runScript(script)
                                }) {
                                    Label("실행 (Run)", systemImage: "play.fill")
                                }
                                
                                Divider()
                                
                                // 2. 파일 위치 열기
                                Button(action: {
                                    viewModel.openFileLocation(script.path)
                                }) {
                                    Label("파일 위치 열기 (Show in Finder)", systemImage: "folder")
                                }
                                
                                // 3. 속성 (Properties) -> 시트 트리거
                                Button(action: {
                                    selectedScriptForProps = script
                                }) {
                                    Label("속성 (Properties)", systemImage: "slider.horizontal.3")
                                }
                            }
                        }
                    }
                    .padding(30)
                }
            }
            // 상단 툴바
            .toolbar {
                ToolbarItem(placement: .primaryAction) {
                    HStack {
                        if isSearchActive {
                            HStack {
                                Image(systemName: "magnifyingglass").foregroundColor(.secondary)
                                TextField("Search...", text: $viewModel.searchText)
                                    .textFieldStyle(PlainTextFieldStyle())
                                    .focused($isSearchFocused)
                                    .frame(width: 200)
                                
                                Button(action: {
                                    withAnimation(.spring()) {
                                        viewModel.searchText = ""
                                        isSearchActive = false
                                        isSearchFocused = false
                                    }
                                }) {
                                    Image(systemName: "xmark.circle.fill").foregroundColor(.gray)
                                }
                                .buttonStyle(PlainButtonStyle())
                            }
                            .padding(6)
                            .background(Color(NSColor.controlBackgroundColor))
                            .cornerRadius(8)
                            .overlay(RoundedRectangle(cornerRadius: 8).stroke(Color.gray.opacity(0.2)))
                            .transition(.move(edge: .trailing).combined(with: .opacity))
                        } else {
                            Button(action: {
                                withAnimation(.spring()) {
                                    isSearchActive = true
                                    isSearchFocused = true
                                }
                            }) {
                                Image(systemName: "magnifyingglass")
                            }
                        }
                    }
                }
                
                ToolbarItem(placement: .automatic) {
                    HStack {
                        Image(systemName: "photo").font(.caption)
                        Slider(value: $viewModel.iconSize, in: 60...200)
                            .frame(width: 120)
                            .controlSize(.mini)
                    }
                }
                
                ToolbarItem(placement: .automatic) {
                    Button(action: { showSettings.toggle() }) {
                        Image(systemName: "gearshape")
                    }
                }
            }
        }
        // [설정 시트]
        .sheet(isPresented: $showSettings) {
            SettingsView(viewModel: viewModel)
        }
        // [속성 시트] - 데이터(item)가 있을 때만 열리도록 함 (빈 화면 방지)
        .sheet(item: $selectedScriptForProps) { (script: LauncherScriptItem) in
            PropertiesView(script: script) {
                // 저장 후 리스트 새로고침
                viewModel.refreshScripts()
            }
        }
        .onAppear {
            viewModel.refreshScripts()
            viewModel.selectedCategory = "All"
        }
    }
}

// [디자인] 튕기는 애니메이션이 적용된 아이콘 셀
struct FinderIconCell: View {
    let script: LauncherScriptItem
    let size: CGFloat
    let fontSize: Double
    var onDoubleTap: () -> Void // 더블 클릭 시 실행할 클로저
    
    @State private var isHovered = false
    @State private var isBouncing = false // 애니메이션 상태
    
    var body: some View {
        VStack(spacing: 8) {
            if let img = script.image {
                Image(nsImage: img)
                    .resizable()
                    .aspectRatio(contentMode: .fit)
                    .frame(width: size, height: size)
                    .shadow(color: .black.opacity(0.2), radius: 4, x: 0, y: 2)
                    // [효과] 더블 클릭 시 튕김
                    .scaleEffect(isBouncing ? 1.2 : 1.0)
                    .animation(.spring(response: 0.3, dampingFraction: 0.6), value: isBouncing)
            }
            
            Text(script.name)
                .font(.system(size: CGFloat(fontSize)))
                .fontWeight(isHovered ? .semibold : .medium)
                .foregroundColor(isHovered ? .white : .primary)
                .multilineTextAlignment(.center)
                .lineLimit(2)
                .padding(.horizontal, 8)
                .padding(.vertical, 4)
                .background(
                    RoundedRectangle(cornerRadius: 6)
                        .fill(isHovered ? Color.blue : Color.clear)
                )
                .frame(width: size + 20)
        }
        .padding(10)
        .background(
            RoundedRectangle(cornerRadius: 12)
                .fill(isHovered ? Color.gray.opacity(0.1) : Color.clear)
        )
        .contentShape(Rectangle()) // 투명 영역도 클릭 가능하게
        .onHover { hovering in
            withAnimation(.easeInOut(duration: 0.15)) {
                isHovered = hovering
            }
        }
        // [동작] 더블 클릭 제스처
        .onTapGesture(count: 2) {
            // 1. 애니메이션 시작
            isBouncing = true
            
            // 2. 잠시 후 복귀
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.1) {
                isBouncing = false
            }
            
            // 3. 실행 로직 호출
            onDoubleTap()
        }
    }
}

