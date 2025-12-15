// swift-tools-version: 5.9
// The swift-tools-version declares the minimum version of Swift required to build this package.

import PackageDescription

let package = Package(
    name: "PyQuickBox",
    platforms: [
        .macOS(.v13)
    ],
    products: [
        .executable(
            name: "PyQuickBox",
            targets: ["PyQuickBox"]),
    ],
    targets: [
        .executableTarget(
            name: "PyQuickBox",
            path: "Sources/PyQuickBox",
            resources: []
        ),
    ]
)
