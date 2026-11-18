// SPDX-License-Identifier: GPL-2.0-only
import Foundation
import AVFoundation
import ScreenCaptureKit
import CoreMedia

enum AudioError: Error { case format, truncated, unsupported }
func report(_ message: String) { try? FileHandle.standardError.write(contentsOf: Data((message + "\n").utf8)) }
func die(_ error: Error) -> Never { report(String(describing: error)); exit(1) }
func uint32(_ data: Data, _ at: Int) -> UInt32 {
    data.withUnsafeBytes { UInt32(littleEndian: $0.loadUnaligned(fromByteOffset: at, as: UInt32.self)) }
}
func exact(_ count: Int) throws -> Data {
    var result = Data()
    while result.count < count {
        guard let more = try FileHandle.standardInput.read(upToCount: count-result.count), !more.isEmpty else { throw AudioError.truncated }
        result.append(more)
    }
    return result
}
func readFrame() throws -> Data {
    let n = Int(uint32(try exact(4),0))
    guard n > 0 && n <= 65537 else { throw AudioError.format }
    return try exact(n)
}
func writeFrame(_ data: Data) throws {
    var size = UInt32(data.count).littleEndian
    try withUnsafeBytes(of: &size) { try FileHandle.standardOutput.write(contentsOf: Data($0)) }
    try FileHandle.standardOutput.write(contentsOf: data)
}
func formatFrame(_ rate: UInt32) -> Data {
    var rate = rate.littleEndian
    var result = Data([1]); withUnsafeBytes(of:&rate){result.append(contentsOf:$0)};result.append(contentsOf:[2,0]);return result
}
func readRate(_ data: Data) throws -> Double {
    guard data.count == 7, data[0] == 1, data[5] == 2, data[6] == 0 else {throw AudioError.format}
    let rate = uint32(data,1);guard rate >= 8000 && rate <= 192000 else {throw AudioError.format};return Double(rate)
}
func values(_ data: Data) throws -> [Float] {
    guard data.count >= 9, data[0] == 2, (data.count-1)%8 == 0 else {throw AudioError.format}
    return stride(from:1,to:data.count,by:4).map { at in
        let value = Float(bitPattern:uint32(data,at));return value.isFinite ? min(1,max(-1,value)) : 0
    }
}
func sampleFrame(_ values: [Float]) -> Data {
    var result=Data([2]);values.withUnsafeBytes{result.append(contentsOf:$0)};return result
}

final class Output {
    var engine: AVAudioEngine?
    var player: AVAudioPlayerNode?
    var format: AVAudioFormat?
    var pending: AVAudioFrameCount = 0
    var generation=0
    var observer:NSObjectProtocol?
    let volume:Float, bufferMS:Int
    init(volume:Int,bufferMS:Int){self.volume=Float(volume)/100;self.bufferMS=bufferMS}
    func configure(_ rate:Double) throws {
        stop()
        guard let f=AVAudioFormat(standardFormatWithSampleRate:rate,channels:2) else {throw AudioError.format}
        let e=AVAudioEngine(),p=AVAudioPlayerNode();e.attach(p);e.connect(p,to:e.mainMixerNode,format:f)
        e.prepare();try e.start();p.volume=volume;p.play();engine=e;player=p;format=f
        observer=NotificationCenter.default.addObserver(forName:.AVAudioEngineConfigurationChange,object:e,queue:.main){[weak self] _ in
            guard let self=self else{return}
            do{try self.configure(rate)}catch{die(error)}
        }
    }
    func play(_ data:Data) throws {
        let samples=try values(data)
        guard let f=format,let p=player else{throw AudioError.format}
        let frames=AVAudioFrameCount(samples.count/2)
        if pending>AVAudioFrameCount(f.sampleRate*Double(bufferMS)/1000) {
            generation+=1;p.stop();pending=0;p.play()
        }
        guard let buffer=AVAudioPCMBuffer(pcmFormat:f,frameCapacity:frames),let channels=buffer.floatChannelData else{throw AudioError.format}
        buffer.frameLength=frames
        for i in 0..<Int(frames){channels[0][i]=samples[i*2];channels[1][i]=samples[i*2+1]}
        pending+=frames;let ticket=generation
        p.scheduleBuffer(buffer,completionCallbackType:.dataPlayedBack){[weak self] _ in DispatchQueue.main.async{
            guard let self=self,self.generation==ticket else{return};self.pending=self.pending>=frames ? self.pending-frames : 0
        }}
    }
    func stop(){generation+=1;if let o=observer{NotificationCenter.default.removeObserver(o)};observer=nil;player?.stop();engine?.stop();player=nil;engine=nil;format=nil;pending=0}
}

@available(macOS 13.0,*)
final class Capture:NSObject,SCStreamOutput,SCStreamDelegate {
    var stream:SCStream?
    var heartbeat:DispatchSourceTimer?
    let queue=DispatchQueue(label:"org.twinglide.audio.capture")
    func start() async throws {
        let available=try await SCShareableContent.excludingDesktopWindows(false,onScreenWindowsOnly:true)
        guard let display=available.displays.first else{throw AudioError.unsupported}
        let filter=SCContentFilter(display:display,excludingApplications:[],exceptingWindows:[])
        let config=SCStreamConfiguration()
        config.width=2;config.height=2;config.minimumFrameInterval=CMTime(value:1,timescale:1);config.showsCursor=false
        config.capturesAudio=true;config.sampleRate=48000;config.channelCount=2;config.excludesCurrentProcessAudio=true
        let s=SCStream(filter:filter,configuration:config,delegate:self)
        // Only an audio output is registered: no screen frames or microphone are retained.
        try s.addStreamOutput(self,type:.audio,sampleHandlerQueue:queue)
        try writeFrame(formatFrame(48000));stream=s;try await s.startCapture()
        let timer=DispatchSource.makeTimerSource(queue:queue);timer.schedule(deadline:.now()+0.5,repeating:0.5)
        timer.setEventHandler{do{try writeFrame(Data([3]))}catch{die(error)}};timer.resume();heartbeat=timer
    }
    func stream(_ stream:SCStream,didStopWithError error:Error){die(error)}
    func stream(_ stream:SCStream,didOutputSampleBuffer sampleBuffer:CMSampleBuffer,of type:SCStreamOutputType){
        guard type == .audio,CMSampleBufferDataIsReady(sampleBuffer) else{return}
        do{try emit(sampleBuffer)}catch{die(error)}
    }
    func emit(_ sample:CMSampleBuffer) throws {
        guard let description=CMSampleBufferGetFormatDescription(sample),
              let pointer=CMAudioFormatDescriptionGetStreamBasicDescription(description) else{throw AudioError.format}
        let f=pointer.pointee
        guard f.mSampleRate==48000,f.mChannelsPerFrame==2,f.mBitsPerChannel==32,
              f.mFormatID==kAudioFormatLinearPCM,f.mFormatFlags & kAudioFormatFlagIsFloat != 0 else{throw AudioError.unsupported}
        var needed=0
        _=CMSampleBufferGetAudioBufferListWithRetainedBlockBuffer(sample,bufferListSizeNeededOut:&needed,bufferListOut:nil,bufferListSize:0,blockBufferAllocator:nil,blockBufferMemoryAllocator:nil,flags:0,blockBufferOut:nil)
        guard needed>0 && needed<4096 else{throw AudioError.format}
        let memory=UnsafeMutableRawPointer.allocate(byteCount:needed,alignment:16);defer{memory.deallocate()}
        let list=memory.bindMemory(to:AudioBufferList.self,capacity:1);var block:CMBlockBuffer?
        guard CMSampleBufferGetAudioBufferListWithRetainedBlockBuffer(sample,bufferListSizeNeededOut:nil,bufferListOut:list,bufferListSize:needed,blockBufferAllocator:nil,blockBufferMemoryAllocator:nil,flags:0,blockBufferOut:&block)==noErr else{throw AudioError.format}
        let buffers=UnsafeMutableAudioBufferListPointer(list);let count=CMSampleBufferGetNumSamples(sample)
        guard count>0 && count<=48000, buffers.count==1 || buffers.count==2 else{throw AudioError.format}
        let planar=buffers.count==2
        for b in buffers{guard b.mData != nil,Int(b.mDataByteSize)>=count*(planar ? 4:8) else{throw AudioError.format}}
        for start in stride(from:0,to:count,by:4096){
            let n=min(4096,count-start);var samples=[Float](repeating:0,count:n*2)
            for i in 0..<n {for c in 0..<2 {
                let buffer=buffers[planar ? c:0];let at=planar ? start+i:(start+i)*2+c
                let v=buffer.mData!.load(fromByteOffset:at*4,as:Float.self)
                samples[i*2+c]=v.isFinite ? min(1,max(-1,v)):0
            }}
            try writeFrame(sampleFrame(samples))
        }
        withExtendedLifetime(block){}
    }
}

let arguments=CommandLine.arguments
let mode=arguments.count>1 ? arguments[1]:""
let volume=arguments.count>2 ? Int(arguments[2]) ?? -1:100
let bufferMS=arguments.count>3 ? Int(arguments[3]) ?? -1:80
guard (0...100).contains(volume),[40,80,160].contains(bufferMS) else{die(AudioError.format)}
var capture:Capture?
var fixtureTimer:DispatchSourceTimer?
var output:Output?
var parentWatch:DispatchSourceTimer?
if mode != "--self-test" {
    if arguments.count>4,let parent=Int32(arguments[4]),parent>1 {
        guard getppid()==parent else{die(AudioError.unsupported)}
        parentWatch=DispatchSource.makeTimerSource(queue:DispatchQueue(label:"org.twinglide.audio.parent"))
        parentWatch?.schedule(deadline:.now(),repeating:0.2)
        parentWatch?.setEventHandler{if getppid() != parent{exit(0)}};parentWatch?.resume()
    } else if mode=="--capture" || mode=="--play"{die(AudioError.unsupported)}
}
if mode=="--self-test" {
    do {
        guard try readRate(formatFrame(48000))==48000,try values(sampleFrame([0.125,-0.125])) == [0.125,-0.125] else{throw AudioError.format}
        print("Audio framing self-test passed; no audio device opened");exit(0)
    }catch{die(error)}
} else if mode=="--capture" {
    capture=Capture();Task{do{try await capture!.start()}catch{report("Allow TwinGlide system-audio/screen recording permission in System Settings.");die(error)}}
} else if mode=="--fixture" {
    do{try writeFrame(formatFrame(48000))}catch{die(error)}
    let samples=(0..<1920).map{Float($0%2==0 ? 0.125:-0.125)}
    let packet=sampleFrame(samples)
    fixtureTimer=DispatchSource.makeTimerSource(queue:DispatchQueue(label:"org.twinglide.audio.fixture"))
    fixtureTimer?.schedule(deadline:.now(),repeating:0.02);fixtureTimer?.setEventHandler{do{try writeFrame(packet)}catch{die(error)}};fixtureTimer?.resume()
} else if mode=="--play" || mode=="--verify" {
    if mode=="--play"{output=Output(volume:volume,bufferMS:bufferMS)}
    DispatchQueue.global().async{
        do {
            let rate=try readRate(readFrame())
            if mode=="--play"{try DispatchQueue.main.sync{try output!.configure(rate)}}
            var total=0
            while true {
                let packet=try readFrame();if packet==Data([3]){continue}
                if mode=="--verify" {
                    let samples=try values(packet)
                    for (i,v) in samples.enumerated(){if v != (i%2==0 ? Float(0.125):Float(-0.125)){throw AudioError.format}}
                    total+=samples.count/2;if total%48000==0{report("Synthetic frames verified: \(total)")}
                } else {try DispatchQueue.main.sync{try output!.play(packet)}}
            }
        }catch AudioError.truncated{exit(0)}catch{die(error)}
    }
} else {die(AudioError.unsupported)}
RunLoop.main.run()
