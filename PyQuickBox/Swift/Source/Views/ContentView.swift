import SwiftUI

struct ContentView: View {
    @ObservedObject var viewModel: LauncherViewModel
    @State private var showingSettings = false
    
    var body: some View {
        NavigationSplitView {
            List(selection: $viewModel.currentCategory) {
                NavigationLink(value: "All") {
                    Label("All Apps", systemImage: "square.grid.2x2")
                }
                
                Section("Categories") {
                    ForEach(viewModel.categories, id: \.self) { category in
                        NavigationLink(value: category) {
                            Label(category, systemImage: "folder")
                        }
                    }
                }
            }
            .listStyle(.sidebar)
            .navigationSplitViewColumnWidth(min: 180, ideal: 200)
        } detail: {
            VStack {
                ScrollView {
                    LazyVGrid(columns: [GridItem(.adaptive(minimum: viewModel.iconSize + 40))], spacing: 20) {
                        ForEach(viewModel.filteredScripts) { item in
                            ScriptGridItem(item: item, viewModel: viewModel)
                        }
                    }
                    .padding()
                }
            }
            .navigationTitle(viewModel.currentCategory)
            .searchable(text: $viewModel.searchText)
            .onChange(of: viewModel.currentCategory) { _ in
                viewModel.updateFilteredScripts()
            }
            .toolbar {
                ToolbarItemGroup(placement: .navigation) {
                    Slider(value: $viewModel.iconSize, in: 32...200) {
                        Text("Icon Size")
                    }
                    .frame(width: 120)
                    
                    Button(action: { viewModel.refreshScripts() }) {
                        Label("Refresh", systemImage: "arrow.clockwise")
                    }
                    
                    Button(action: { showingSettings = true }) {
                        Label("Settings", systemImage: "gear")
                    }
                }
                
                ToolbarItem(placement: .principal) {
                    Spacer()
                }
            }
            .sheet(isPresented: $showingSettings) {
                SettingsView(viewModel: viewModel)
            }
        }
        .onDrop(of: [.fileURL], isTargeted: nil) { providers in
            // Handle folder drops?
            for provider in providers {
                provider.loadItem(forTypeIdentifier: "public.file-url", options: nil) { (item, error) in
                    if let data = item as? Data, let url = URL(dataRepresentation: data, relativeTo: nil) {
                        DispatchQueue.main.async {
                            // Check if folder
                            var isDir: ObjCBool = false
                            if FileManager.default.fileExists(atPath: url.path, isDirectory: &isDir) {
                                if isDir.boolValue {
                                    viewModel.addFolder(url.path)
                                }
                            }
                        }
                    }
                }
            }
            return true
        }
    }
}
