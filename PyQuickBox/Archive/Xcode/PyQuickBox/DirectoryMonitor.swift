import Foundation

class DirectoryMonitor {
    private var monitors: [DispatchSourceFileSystemObject] = []
    private let queue = DispatchQueue(label: "com.pyquickbox.directorymonitor", attributes: .concurrent)
    
    // 감시 시작
    func startMonitoring(paths: [String], onChange: @escaping () -> Void) {
        stopMonitoring() // 기존 감시 중단
        
        for path in paths {
            let url = URL(fileURLWithPath: path)
            let fileDescriptor = open(url.path, O_EVTONLY)
            
            guard fileDescriptor != -1 else { continue }
            
            let source = DispatchSource.makeFileSystemObjectSource(
                fileDescriptor: fileDescriptor,
                eventMask: .write, // 파일 추가/삭제/수정 감지
                queue: queue
            )
            
            source.setEventHandler {
                // 변경 감지 시 메인 스레드에서 콜백 실행
                DispatchQueue.main.async {
                    onChange()
                }
            }
            
            source.setCancelHandler {
                close(fileDescriptor)
            }
            
            source.resume()
            monitors.append(source)
        }
    }
    
    // 감시 종료
    func stopMonitoring() {
        for source in monitors {
            source.cancel()
        }
        monitors.removeAll()
    }
}
