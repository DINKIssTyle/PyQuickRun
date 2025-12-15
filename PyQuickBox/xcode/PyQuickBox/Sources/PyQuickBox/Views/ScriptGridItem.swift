import SwiftUI

struct ScriptGridItem: View {
    let item: ScriptItem
    @ObservedObject var viewModel: LauncherViewModel
    
    @State private var isHovering = false
    @State private var showProperties = false
    @State private var isExecuting = false
    
    var body: some View {
        VStack {
            if let iconPath = item.iconPath, let nsImage = NSImage(contentsOfFile: iconPath) {
                Image(nsImage: nsImage)
                    .resizable()
                    .aspectRatio(contentMode: .fit)
                    .frame(width: viewModel.iconSize, height: viewModel.iconSize)
            } else {
                Image(systemName: "doc.text")
                    .resizable()
                    .aspectRatio(contentMode: .fit)
                    .foregroundStyle(.secondary)
                    .frame(width: viewModel.iconSize, height: viewModel.iconSize)
            }
            
            Text(item.displayName)
                .font(.system(size: 12))
                .lineLimit(2)
                .multilineTextAlignment(.center)
                .foregroundStyle(.primary)
        }
        .padding()
        .background(
            RoundedRectangle(cornerRadius: 8)
                .fill(isExecuting ? Color.accentColor.opacity(0.3) : (isHovering ? Color.accentColor.opacity(0.1) : Color.clear))
        )
        .scaleEffect(isExecuting ? 0.95 : 1.0)
        .contentShape(Rectangle()) // Make entire area clickable
        .onHover { hover in
            withAnimation(.easeInOut(duration: 0.2)) {
                isHovering = hover
            }
        }
        .onTapGesture(count: 2) {
            withAnimation(.spring(response: 0.3, dampingFraction: 0.6)) {
                isExecuting = true
            }
            
            viewModel.runScript(item)
            
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.2) {
                withAnimation {
                    isExecuting = false
                }
            }
        }
        .contextMenu {
            Button("Run") {
                viewModel.runScript(item)
            }
            Button("Show in Finder") {
                viewModel.openLocation(item)
            }
            Divider()
            Button("Properties") {
                showProperties = true
            }
        }
        .sheet(isPresented: $showProperties) {
            PropertiesView(item: item, viewModel: viewModel)
        }
    }
}
