import SwiftUI

@main
struct PyQuickBoxApp: App {
    @StateObject private var viewModel = LauncherViewModel()
    
    var body: some Scene {
        WindowGroup {
            ContentView(viewModel: viewModel)
                .frame(minWidth: 800, minHeight: 600)
        }
        .windowStyle(.hiddenTitleBar) // Custom or hidden title bar if needed
        .commands {
            // Add custom menu commands if needed
            CommandGroup(replacing: .newItem) { } // Disable New Window
        }
        
        Settings {
            SettingsView(viewModel: viewModel)
        }
    }
}
