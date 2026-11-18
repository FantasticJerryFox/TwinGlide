// SPDX-License-Identifier: GPL-2.0-only
// Keep Deskflow's macOS server IME from translating physical punctuation before
// Windows receives it. No keyboard hook, key logging, or accessibility access.
import Cocoa
import Carbon
import Darwin

let supportRoot = URL(fileURLWithPath: ProcessInfo.processInfo.environment["TWINGLIDE_DATA_DIR"] ?? FileManager.default.homeDirectoryForCurrentUser.appendingPathComponent("Library/Application Support/TwinGlide").path)
let inputRoot = supportRoot.appendingPathComponent("input")
let stateFile = inputRoot.appendingPathComponent("input-source-state.json")
let logFile = supportRoot.appendingPathComponent("logs/input-source.log")
func sourceID(_ source: TISInputSource) -> String {
    guard let p = TISGetInputSourceProperty(source, kTISPropertyInputSourceID) else { return "" }
    return Unmanaged<CFString>.fromOpaque(p).takeUnretainedValue() as String
}
func currentSource() -> TISInputSource? { TISCopyCurrentKeyboardInputSource()?.takeRetainedValue() }
func sourceNamed(_ id: String) -> TISInputSource? {
    let filter = [kTISPropertyInputSourceID as String: id] as CFDictionary
    // Recovery must also find an input source temporarily disabled by us.
    let list = TISCreateInputSourceList(filter, true).takeRetainedValue() as NSArray
    return list.firstObject.map { $0 as! TISInputSource }
}
func sourceFlag(_ source: TISInputSource, _ property: CFString) -> Bool {
    guard let p = TISGetInputSourceProperty(source, property) else { return false }
    return CFBooleanGetValue(Unmanaged<CFBoolean>.fromOpaque(p).takeUnretainedValue())
}
func enabledNonASCIISources() -> [String] {
    let list = TISCreateInputSourceList(nil, false).takeRetainedValue() as NSArray
    return list.compactMap { value in
        let source = value as! TISInputSource
        guard let p = TISGetInputSourceProperty(source, kTISPropertyInputSourceCategory),
              Unmanaged<CFString>.fromOpaque(p).takeUnretainedValue() == kTISCategoryKeyboardInputSource else { return nil }
        // Disabling a single SCIM input mode reports success but the enabled
        // parent immediately makes it selectable again. Isolate the input
        // method itself; re-enabling it preserves the user's enabled modes.
        guard let kind = TISGetInputSourceProperty(source, kTISPropertyInputSourceType),
              Unmanaged<CFString>.fromOpaque(kind).takeUnretainedValue() != kTISTypeKeyboardInputMode else { return nil }
        guard sourceFlag(source, kTISPropertyInputSourceIsEnabled),
              sourceFlag(source, kTISPropertyInputSourceIsEnableCapable),
              !sourceFlag(source, kTISPropertyInputSourceIsASCIICapable) else { return nil }
        return sourceID(source)
    }
}
func record(_ message: String) {
    let data = Data((ISO8601DateFormatter().string(from: Date()) + " " + message + "\n").utf8)
    if !FileManager.default.fileExists(atPath: logFile.path) { FileManager.default.createFile(atPath: logFile.path, contents: nil) }
    if let file = try? FileHandle(forWritingTo: logFile) {
        if file.seekToEndOfFile() > 256 * 1024 { file.truncateFile(atOffset: 0) }
        file.write(data); try? file.close()
    }
}
struct SavedSource: Codable {
    let original: String; let temporary: String; let temporarilyDisabled: [String]
    init(original: String, temporary: String, temporarilyDisabled: [String] = []) {
        self.original = original; self.temporary = temporary; self.temporarilyDisabled = temporarilyDisabled
    }
    enum CodingKeys: String, CodingKey { case original, temporary, temporarilyDisabled }
    init(from decoder: Decoder) throws {
        let c = try decoder.container(keyedBy: CodingKeys.self)
        original = try c.decode(String.self, forKey: .original)
        temporary = try c.decode(String.self, forKey: .temporary)
        temporarilyDisabled = try c.decodeIfPresent([String].self, forKey: .temporarilyDisabled) ?? []
    }
}
// Extract only connection/screen events from the existing INFO log.
func remoteState(_ line: String, previous: Bool) -> Bool {
    if line.contains("INFO: entering screen") || line.contains("INFO: entering computer") || line.contains("disconnected") || line.contains("waiting for clients") || line.contains("started server") || line.contains("stopping server") { return false }
    if line.contains("INFO: leaving screen") || line.contains("INFO: leaving computer") { return true }
    return previous
}
final class LogReader {
    var offset: UInt64 = 0; var inode: UInt64 = 0; var partial = ""
    let url = supportRoot.appendingPathComponent("logs/keyboard.log")
    func read(_ previous: Bool) -> Bool {
        guard let attrs = try? FileManager.default.attributesOfItem(atPath: url.path), let size = attrs[.size] as? UInt64, let id = attrs[.systemFileNumber] as? UInt64, let h = try? FileHandle(forReadingFrom: url) else { return previous }
        defer { try? h.close() }
        var state = previous
        if inode != id || size < offset { inode = id; offset = size > 65536 ? size - 65536 : 0; partial = ""; state = false }
        if size == offset { return state }
        h.seek(toFileOffset: offset)
        let data = h.readDataToEndOfFile(); offset += UInt64(data.count)
        let lines = (partial + String(decoding: data, as: UTF8.self)).components(separatedBy: "\n")
        partial = lines.last ?? ""
        for line in lines.dropLast() { state = remoteState(line, previous: state) }
        return state
    }
}
func inputCorePID() -> pid_t? {
    guard let value = ProcessInfo.processInfo.environment["TWINGLIDE_INPUT_PID"], let pid = Int32(value), pid > 0,
          let expected = ProcessInfo.processInfo.environment["TWINGLIDE_INPUT_PATH"] else { return nil }
    var path = [CChar](repeating: 0, count: 4 * Int(MAXPATHLEN))
    guard proc_pidpath(pid, &path, UInt32(path.count)) > 0, String(cString: path) == expected else { return nil }
    return pid
}
final class SourceGuard {
    var saved: SavedSource?; var remote = false; var pid: pid_t?; let reader = LogReader()
    var preparedRemote = false
    init() { if let data = try? Data(contentsOf: stateFile) { saved = try? JSONDecoder().decode(SavedSource.self, from: data) } }
    func persist(_ state: SavedSource) -> Bool {
        do {
            try JSONEncoder().encode(state).write(to: stateFile, options: .atomic)
            chmod(stateFile.path, 0o600); saved = state; return true
        } catch { return false }
    }
    @discardableResult func prepareRemote() -> Bool {
        guard let current = currentSource(), let ascii = TISCopyCurrentASCIICapableKeyboardInputSource()?.takeRetainedValue() else { return false }
        let from = sourceID(current), to = sourceID(ascii)
        guard !from.isEmpty && !to.isEmpty else { return false }
        let old = saved
        let disabled = Array(Set((old?.temporarilyDisabled ?? []) + enabledNonASCIISources())).sorted()
        // Save the full restoration plan before changing system input sources.
        guard persist(SavedSource(original: old?.original ?? from, temporary: to, temporarilyDisabled: disabled)) else { return false }
        guard from == to || TISSelectInputSource(ascii) == noErr else { return false }
        var ready = true
        for id in disabled {
            guard let source = sourceNamed(id) else { ready = false; continue }
            if sourceFlag(source, kTISPropertyInputSourceIsEnabled) {
                if TISDisableInputSource(source) != noErr { ready = false }
            }
        }
        record("remote layout " + to + "; restore " + (saved?.original ?? from) + "; isolated input modes " + String(disabled.count))
        preparedRemote = ready
        return ready
    }
    func restore() {
        preparedRemote = false
        guard let old = saved else { return }
        // Re-enable before looking up/selecting the original input source.
        for id in old.temporarilyDisabled {
            guard let source = sourceNamed(id), TISEnableInputSource(source) == noErr else { return }
        }
        guard let current = currentSource() else { return }
        // An application/user may already have selected another input source.
        if sourceID(current) == old.temporary {
            guard let original = sourceNamed(old.original), TISSelectInputSource(original) == noErr else { return }
            record("restored " + old.original)
        }
        saved = nil; try? FileManager.default.removeItem(at: stateFile)
    }
    func tick() {
        let core = inputCorePID()
        guard core != nil else { restore(); pid = nil; remote = false; return }
        if pid != nil && pid != core { restore(); remote = false }
        pid = core; remote = reader.read(remote)
        guard remote else { restore(); return }
        if !preparedRemote { _ = prepareRemote() }
        guard let current = currentSource(), let ascii = TISCopyCurrentASCIICapableKeyboardInputSource()?.takeRetainedValue() else { return }
        let from = sourceID(current), to = sourceID(ascii)
        guard !from.isEmpty && !to.isEmpty && from != to else { return }
        guard saved != nil else { return }
        if TISSelectInputSource(ascii) == noErr { record("remote layout " + to + "; restore " + (saved?.original ?? from)) }
    }
}
if CommandLine.arguments.contains("--self-test") {
    precondition(remoteState("[now] INFO: leaving screen", previous: false))
    precondition(remoteState("[now] INFO: leaving computer", previous: false))
    precondition(!remoteState("[now] INFO: entering computer", previous: true))
    precondition(!remoteState("[now] INFO: entering screen", previous: true))
    precondition(!remoteState("[now] INFO: client tg-example disconnected", previous: true))
    precondition(!remoteState("[now] NOTE: started server, waiting for clients", previous: true))
    precondition(remoteState("[now] INFO: clipboard was updated", previous: true))
    let data = try JSONEncoder().encode(SavedSource(original: "ime", temporary: "ABC"))
    let decoded = try JSONDecoder().decode(SavedSource.self, from: data)
    precondition(decoded.original == "ime")
    let legacy = try JSONDecoder().decode(SavedSource.self, from: Data("{\"original\":\"pinyin\",\"temporary\":\"ABC\"}".utf8))
    precondition(legacy.temporarilyDisabled.isEmpty)
    let recovery = SavedSource(original: "pinyin", temporary: "ABC", temporarilyDisabled: ["pinyin"])
    let recovered = try JSONDecoder().decode(SavedSource.self, from: JSONEncoder().encode(recovery))
    precondition(recovered.temporarilyDisabled == ["pinyin"])
    print("input source guard self-test passed"); exit(0)
}
if CommandLine.arguments.contains("--probe") {
    print("current=" + (currentSource().map(sourceID) ?? "unknown"))
    print("caps=" + String(CGEventSource.flagsState(.combinedSessionState).contains(.maskAlphaShift)))
    print("non_ascii_enabled=" + enabledNonASCIISources().joined(separator: ","))
    print("core=" + String(inputCorePID() ?? 0)); exit(0)
}
try FileManager.default.createDirectory(at: inputRoot, withIntermediateDirectories: true)
let lock = open(inputRoot.appendingPathComponent("input-source.lock").path, O_CREAT | O_RDWR, 0o600)
guard lock >= 0 && flock(lock, LOCK_EX | LOCK_NB) == 0 else { exit(CommandLine.arguments.contains("--exercise") ? 1 : 0) }
let guarder = SourceGuard()
if CommandLine.arguments.contains("--restore") { guarder.restore(); exit(0) }
if CommandLine.arguments.contains("--exercise") {
    guarder.restore()
    let original = currentSource().map(sourceID)
    let enabled = enabledNonASCIISources()
    let isolated = guarder.prepareRemote() && enabledNonASCIISources().isEmpty
    // A fresh instance must recover the on-disk restoration plan, as it would
    // after an unexpected process exit with an input method still disabled.
    let recovery = SourceGuard()
    recovery.restore()
    let restored = original == currentSource().map(sourceID) && enabled == enabledNonASCIISources() && recovery.saved == nil
    print("input_modes_isolated=\(isolated) persisted_recovery=\(restored)")
    exit(isolated && restored ? 0 : 1)
}
var signals: [DispatchSourceSignal] = []
for number in [SIGTERM, SIGINT] {
    signal(number, SIG_IGN)
    let source = DispatchSource.makeSignalSource(signal: number, queue: .main)
    source.setEventHandler { guarder.restore(); exit(0) }; source.resume(); signals.append(source)
}
record("started")
guarder.tick()
let timer = Timer.scheduledTimer(withTimeInterval: 0.08, repeats: true) { _ in if let parent = ProcessInfo.processInfo.environment["TWINGLIDE_AGENT_PID"], Int32(parent) != getppid() { guarder.restore(); exit(0) }; guarder.tick() }
RunLoop.main.run()
