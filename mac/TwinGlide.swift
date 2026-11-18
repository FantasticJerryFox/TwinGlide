// SPDX-License-Identifier: GPL-2.0-only
import Cocoa

if CommandLine.arguments.contains("--request-accessibility") {
    let options = [kAXTrustedCheckOptionPrompt.takeUnretainedValue() as String: true] as CFDictionary
    exit(AXIsProcessTrustedWithOptions(options) ? 0 : 2)
}
final class LayoutPreview: NSView {
    var source = "", other = "", side = "right"
    override var intrinsicContentSize: NSSize { NSSize(width: 640, height: 112) }
    override func draw(_ dirtyRect: NSRect) {
        let w: CGFloat = 190, h: CGFloat = 36
        var a = NSRect(x: bounds.midX-w-12, y: 38, width: w, height: h)
        var b = NSRect(x: bounds.midX+12, y: 38, width: w, height: h)
        if side == "up" || side == "down" {
            a = NSRect(x: bounds.midX-w/2,y: 62,width: w,height: h)
            b = NSRect(x: bounds.midX-w/2,y: 8,width: w,height: h)
        }
        if side == "left" || side == "up" { swap(&a,&b) }
        NSColor.controlAccentColor.setStroke()
        for rect in [a,b] { let p=NSBezierPath(roundedRect:rect,xRadius:4,yRadius:4);p.lineWidth=2;p.stroke() }
        let style=NSMutableParagraphStyle();style.alignment = .center;style.lineBreakMode = .byTruncatingTail
        for (text,rect) in [(source,a),(other,b)] {
            (text as NSString).draw(in:rect.insetBy(dx:8,dy:9),withAttributes:[.font:NSFont.systemFont(ofSize:13),.foregroundColor:NSColor.labelColor,.paragraphStyle:style])
        }
    }
}
final class Delegate: NSObject, NSApplicationDelegate, NSTextFieldDelegate {
    var status: NSStatusItem!, window: NSWindow!
    var agent: Process?, pipe: Pipe?, timer: Timer?
    var closing=false, loading=false, busy=false, updating=false
    var dirty=Set<Int>(), state=[String:Any](), sharedKey="", preferencesKey="", lastMessage=""
    let connection=NSTextField(wrappingLabelWithString:"正在启动本机连接…")
    let feedback=NSTextField(wrappingLabelWithString:"")
    let engine=NSTextField(wrappingLabelWithString:""), audio=NSTextField(wrappingLabelWithString:""), code=NSTextField(wrappingLabelWithString:""), route=NSTextField(wrappingLabelWithString:"")
    let nearby=NSPopUpButton(),source=NSPopUpButton(),side=NSPopUpButton(),returnKey=NSPopUpButton(),target=NSPopUpButton(),buffer=NSPopUpButton(),theme=NSPopUpButton()
    let address=NSTextField(),deviceName=NSTextField(),delay=NSTextField(string:"0"),volume=NSTextField(string:"100")
    let sharing=NSButton(checkboxWithTitle:"启用键鼠跨屏",target:nil,action:nil)
    let login=NSButton(checkboxWithTitle:"登录后自动启动双游",target:nil,action:nil)
    let preview=LayoutPreview(),tabs=NSTabView()
    var confirm:NSButton!,reject:NSButton!,pair:NSButton!,forget:NSButton!
    var screenControls=[NSControl](),audioControls=[NSControl]()
    let nearbyScroll=NSScrollView(),nearbyDocument=NSView(),discoveryHint=NSTextField(wrappingLabelWithString:"正在自动发现附近设备…")
    var advanced:NSStackView!,advancedButton:NSButton!,nearbyKey=""
    let support=ProcessInfo.processInfo.environment["TWINGLIDE_DATA_DIR"].map{URL(fileURLWithPath:$0)}
        ?? FileManager.default.homeDirectoryForCurrentUser.appendingPathComponent("Library/Application Support/TwinGlide")
    func applicationDidFinishLaunching(_ notification:Notification){
        if let other=NSRunningApplication.runningApplications(withBundleIdentifier:"org.twinglide.desktop").first(where:{$0.processIdentifier != ProcessInfo.processInfo.processIdentifier}) {
            other.activate(options:.activateIgnoringOtherApps);NSApp.terminate(nil);return
        }
        status=NSStatusBar.system.statusItem(withLength:NSStatusItem.squareLength)
        status.button?.image=NSImage(systemSymbolName:"arrow.left.arrow.right",accessibilityDescription:"TwinGlide 双游")
        let menu=NSMenu()
        for (title,action) in [("打开双游设置",#selector(show)),("本地网络授权",#selector(networkPermission)),("辅助功能授权",#selector(accessibility)),("系统声音授权",#selector(audioPermission)),("退出双游",#selector(quit))] {
            let item=NSMenuItem(title:title,action:action,keyEquivalent:"");item.target=self;menu.addItem(item)
        }
        status.menu=menu
        buildWindow()
        let p=Process();p.executableURL=Bundle.main.bundleURL.appendingPathComponent("Contents/MacOS/twinglide-agent")
        p.arguments=["--parent-stdin","--data-dir",support.path]
        if ProcessInfo.processInfo.environment["TWINGLIDE_TEST_MODE"]=="1"{p.arguments?.append("--no-input")}
        pipe=Pipe();p.standardInput=pipe
        p.terminationHandler={ [weak self] _ in DispatchQueue.main.async {
            guard let self=self,!self.closing else{return}
            let alert=NSAlert();alert.messageText="双游后台已退出";alert.informativeText="请重新打开应用；若持续发生，请检查端口和数据目录。";alert.runModal();NSApp.terminate(nil)
        }}
        do{try p.run();agent=p}catch{feedback.stringValue=error.localizedDescription;return}
        timer=Timer.scheduledTimer(withTimeInterval:1.2,repeats:true){[weak self] _ in self?.refresh()}
        refresh();if !CommandLine.arguments.contains("--background"){show()}
    }
    func label(_ text:String)->NSTextField{let l=NSTextField(wrappingLabelWithString:text);l.font = .systemFont(ofSize:13);return l}
    func stack(_ views:[NSView],vertical:Bool=true)->NSStackView{
        let s=NSStackView(views:views);s.orientation=vertical ? .vertical : .horizontal;s.alignment=vertical ? .leading : .centerY;s.spacing=14
        if vertical {for v in views{v.translatesAutoresizingMaskIntoConstraints=false;v.widthAnchor.constraint(equalTo:s.widthAnchor).isActive=true}}
        return s
    }
    func button(_ title:String,_ action:String)->NSButton{let b=NSButton(title:title,target:self,action:#selector(action(_:)));b.identifier=NSUserInterfaceItemIdentifier(action);b.bezelStyle = .rounded;return b}
    func field(_ title:String,_ control:NSControl)->NSView{let l=label(title);l.widthAnchor.constraint(equalToConstant:205).isActive=true;control.setAccessibilityLabel(title);let s=stack([l,control],vertical:false);control.setContentHuggingPriority(.defaultLow,for:.horizontal);return s}
    func page(_ title:String,_ views:[NSView]){
        let item=NSTabViewItem(identifier:title);item.label=title;let host=NSView();let content=stack(views);host.addSubview(content);content.translatesAutoresizingMaskIntoConstraints=false
        NSLayoutConstraint.activate([content.leadingAnchor.constraint(equalTo:host.leadingAnchor,constant:22),content.trailingAnchor.constraint(equalTo:host.trailingAnchor,constant:-22),content.topAnchor.constraint(equalTo:host.topAnchor,constant:20),content.bottomAnchor.constraint(lessThanOrEqualTo:host.bottomAnchor,constant:-20)])
        item.view=host;tabs.addTabViewItem(item)
    }
    func options(_ popup:NSPopUpButton,_ values:[(String,String)],selected:String=""){
        popup.removeAllItems();for (id,title) in values{popup.addItem(withTitle:title);popup.lastItem?.representedObject=id}
        select(popup,selected)
    }
    func select(_ popup:NSPopUpButton,_ id:String){if let item=popup.itemArray.first(where:{$0.representedObject as? String == id}){popup.select(item)}}
    func selected(_ p:NSPopUpButton)->String{p.selectedItem?.representedObject as? String ?? ""}
    func buildWindow(){
        window=NSWindow(contentRect:NSRect(x:0,y:0,width:780,height:720),styleMask:[.titled,.closable,.miniaturizable,.resizable],backing:.buffered,defer:false)
        window.title="TwinGlide 双游";window.minSize=NSSize(width:740,height:710);window.center();window.isReleasedWhenClosed=false
        let heading=label("TwinGlide  双游");heading.font = .systemFont(ofSize:26,weight:.semibold)
        let body=stack([heading,connection,tabs,feedback]);let host=NSView();window.contentView=host;host.addSubview(body);body.translatesAutoresizingMaskIntoConstraints=false
        NSLayoutConstraint.activate([body.leadingAnchor.constraint(equalTo:host.leadingAnchor,constant:26),body.trailingAnchor.constraint(equalTo:host.trailingAnchor,constant:-26),body.topAnchor.constraint(equalTo:host.topAnchor,constant:24),body.bottomAnchor.constraint(equalTo:host.bottomAnchor,constant:-24)])
        tabs.setContentHuggingPriority(.defaultLow,for:.vertical);tabs.setContentCompressionResistancePriority(.defaultLow,for:.vertical)
        feedback.heightAnchor.constraint(greaterThanOrEqualToConstant:32).isActive=true
        pair=button("配对所选设备","pair");forget=button("移除已配对设备","forget");confirm=button("两边一致，确认配对","confirm");reject=button("不一致，取消","reject")
        code.font = .monospacedSystemFont(ofSize:22,weight:.semibold)
        address.placeholderString="另一台电脑的 IPv4 地址";address.setAccessibilityLabel("另一台电脑的 IPv4 地址")
        nearbyScroll.hasVerticalScroller=true;nearbyScroll.drawsBackground=false;nearbyScroll.documentView=nearbyDocument;nearbyScroll.heightAnchor.constraint(equalToConstant:216).isActive=true
        advancedButton=button("高级连接选项","advanced")
        advanced=stack([button("重新开放配对","allow"),stack([address,button("按 IP 连接","manual")],vertical:false),forget]);advanced.isHidden=true
        page("设备与配对",[label("两台电脑打开双游，点击下面的设备名称。首次双方确认，以后自动连接。Windows 与 Mac、两台 Windows 都可以使用。"),discoveryHint,nearbyScroll,code,stack([confirm,reject],vertical:false),stack([advancedButton,button("本地网络授权","network")],vertical:false),advanced])
        options(side,[("left","左侧"),("right","右侧"),("up","上方"),("down","下方")])
        options(returnKey,[("F10","Control + Option + Shift + F10"),("F11","Control + Option + Shift + F11"),("F12","Control + Option + Shift + F12")])
        let screenSave=button("保存屏幕设置","screen")
        page("屏幕与键鼠",[label("选择键盘鼠标连接在哪台电脑，再设置另一台屏幕相对它的位置。"),field("键鼠连接在",source),field("另一台屏幕的位置",side),preview,sharing,field("边缘停留（0–1000 毫秒）",delay),field("立即回到键鼠所在电脑",returnKey),screenSave,engine])
        options(buffer,[("40","40 毫秒 · 低延迟"),("80","80 毫秒 · 均衡"),("160","160 毫秒 · 稳定")])
        let audioSave=button("保存声音设置","audio")
        page("声音",[label("把另一台电脑的系统声音送到所选电脑。每次只转发一个方向，原电脑也会继续播放。"),field("声音输出到",target),route,field("接收音量（0–100%）",volume),field("声音缓冲",buffer),audioSave,audio,label("Mac 转发系统声音需要系统录音授权。不采集麦克风。"),button("打开系统声音授权","permission")])
        options(theme,[("system","跟随系统"),("light","浅色"),("dark","深色")])
        page("个性化",[label("首次打开会获得一个随机昵称。你可以重新随机，也可以改成容易辨认的名字。"),field("这台电脑的昵称",deviceName),button("换一个随机昵称","nickname"),field("外观",theme),login,button("保存个性化设置","general"),button("打开辅助功能授权","accessibility"),label("关闭窗口后仍留在菜单栏；从菜单选择“退出双游”即可停止共享。")])
        screenControls=[source,side,sharing,delay,returnKey,screenSave];audioControls=[target,volume,buffer,audioSave]
        for (group,controls) in [(0,Array(screenControls.dropLast())),(1,Array(audioControls.dropLast())),(2,[deviceName,theme,login] as [NSControl])]{
            for control in controls{control.tag=group;control.target=self;control.action = #selector(changed(_:));if let text=control as? NSTextField{text.delegate=self}}
        }
    }
    @objc func changed(_ sender:NSControl){if !updating{dirty.insert(sender.tag);updatePreview()}}
    func controlTextDidChange(_ notification:Notification){if let c=notification.object as? NSControl{changed(c)}}
    func request(_ path:String,_ body:[String:Any]?=nil,completion:@escaping(Result<[String:Any],Error>)->Void){
        do{
            let raw=try Data(contentsOf:support.appendingPathComponent("endpoint.json"))
            guard let ep=try JSONSerialization.jsonObject(with:raw) as? [String:Any],ep["pid"] as? Int == Int(agent?.processIdentifier ?? 0),let address=ep["url"] as? String,let url=URL(string:address+"/api/"+path),url.scheme=="http",url.host=="127.0.0.1",let token=ep["token"] as? String,token.count==64 else{throw NSError(domain:"TwinGlide",code:1,userInfo:[NSLocalizedDescriptionKey:"本机连接信息无效，请重启双游。"])}
            var req=URLRequest(url:url,timeoutInterval:4);req.setValue(token,forHTTPHeaderField:"X-TwinGlide-Token")
            if let body=body{req.httpMethod="POST";req.setValue("application/json",forHTTPHeaderField:"Content-Type");req.httpBody=try JSONSerialization.data(withJSONObject:body)}
            URLSession.shared.dataTask(with:req){data,response,error in
                var result:Result<[String:Any],Error>
                if let error=error{result = .failure(error)}
                else{let object=(data.flatMap{try? JSONSerialization.jsonObject(with:$0)} as? [String:Any]) ?? [:]
                    if (response as? HTTPURLResponse)?.statusCode == 200{result = .success(object)}
                    else{result = .failure(NSError(domain:"TwinGlide",code:2,userInfo:[NSLocalizedDescriptionKey:object["error"] as? String ?? "操作失败，请重新打开双游。"]))}
                }
                DispatchQueue.main.async{completion(result)}
            }.resume()
        }catch{completion(.failure(error))}
    }
    func refresh(){guard !loading,!closing else{return};loading=true;request("state"){[weak self] result in
        guard let self=self else{return};self.loading=false
        switch result{case .success(let s):self.render(s);case .failure(let e):self.connection.stringValue="正在连接本机后台… "+e.localizedDescription}
    }}
    func act(_ path:String,_ body:[String:Any]=[:],saved:Int?=nil){
        guard !busy else{return};busy=true;request(path,body){[weak self] result in guard let self=self else{return};self.busy=false
            switch result{case .success:
                if let saved=saved{self.dirty.remove(saved);self.sharedKey="";self.preferencesKey=""}
                self.feedback.stringValue=saved == 2 ? "已保存这台电脑的偏好":"操作已完成";self.refresh()
            case .failure(let e):self.feedback.stringValue=e.localizedDescription}
        }
    }
    @objc func action(_ sender:NSButton){
        var shared=state["settings"] as? [String:Any] ?? [:];shared.removeValue(forKey:"revision");shared.removeValue(forKey:"actor")
        switch sender.identifier?.rawValue{
        case "allow":act("pairing")
        case "pair":act("connect",["id":selected(nearby)])
        case "manual":act("manual",["address":address.stringValue])
        case "confirm","reject":if let p=state["pending"] as? [String:Any],let session=p["session"] as? String{act("confirm",["session":session,"match":sender.identifier?.rawValue=="confirm"])}
        case "forget":
            let alert=NSAlert();alert.messageText="移除已配对设备？";alert.informativeText="两台电脑需要重新确认，才能再次连接。";alert.addButton(withTitle:"取消");alert.addButton(withTitle:"移除配对")
            alert.beginSheetModal(for:window){[weak self] response in if response == .alertSecondButtonReturn{self?.dirty=[];self?.act("forget")}}
        case "screen":
            guard let value=Int(delay.stringValue),(0...1000).contains(value) else{feedback.stringValue="边缘停留请输入 0–1000 的整数。";return}
            shared["sourceID"]=selected(source);shared["side"]=selected(side);shared["enabled"]=sharing.state == .on;shared["edgeDelayMS"]=value;shared["returnKey"]=selected(returnKey);act("settings",shared,saved:0)
        case "audio":
            guard let value=Int(volume.stringValue),(0...100).contains(value) else{feedback.stringValue="音量请输入 0–100 的整数。";return}
            shared["audioTarget"]=selected(target);shared["audioVolume"]=value;shared["audioBufferMS"]=Int(selected(buffer)) ?? 80;act("settings",shared,saved:1)
        case "general":act("preferences",["name":deviceName.stringValue,"theme":selected(theme),"startAtLogin":login.state == .on],saved:2)
        case "nickname":request("nickname",[:]){[weak self] result in guard let self=self else{return};switch result{case .success(let v):self.deviceName.stringValue=v["name"] as? String ?? "";self.dirty.insert(2);self.feedback.stringValue="昵称已生成，保存后生效";case .failure(let e):self.feedback.stringValue=e.localizedDescription}}
        case "advanced":advanced.isHidden.toggle();advancedButton.title=advanced.isHidden ? "高级连接选项":"收起高级选项"
        case "permission":audioPermission()
        case "network":networkPermission()
        case "accessibility":accessibility()
        default:if let action=sender.identifier?.rawValue,action.hasPrefix("connect:"){act("connect",["id":String(action.dropFirst(8))])}
        }
    }
    func render(_ s:[String:Any]){
        state=s;updating=true;defer{updating=false}
        let name=s["name"] as? String ?? "",id=s["id"] as? String ?? "",p=s["peer"] as? [String:Any],peerName=p?["name"] as? String ?? "",peerID=p?["id"] as? String ?? ""
        connection.stringValue=name+"  ·  "+((s["connected"] as? Bool == true) ? "已连接 "+peerName : p == nil ? "正在寻找另一台电脑":"等待 "+peerName+" 上线")
        if s["noInput"] as? Bool == true{connection.stringValue+="（隔离测试）"}
        for c in screenControls+audioControls{c.isEnabled=p != nil};forget.isEnabled=p != nil
        let pending=s["pending"] as? [String:Any];pair.isEnabled=p == nil && pending == nil && s["dialing"] as? Bool != true
        code.stringValue=pending == nil ? "" : (pending?["name"] as? String ?? "")+"\n"+(pending?["code"] as? String ?? "")
        confirm.isHidden=pending == nil;reject.isHidden=pending == nil;confirm.isEnabled=pending?["approved"] as? Bool != true
        confirm.title=pending?["approved"] as? Bool == true ? "已确认，等待另一台电脑":"两边一致，确认配对"
        nearbyScroll.isHidden=p != nil || pending != nil
        discoveryHint.stringValue=p != nil ? peerName+" · "+(p?["os"] as? String ?? "")+"\n"+((s["connected"] as? Bool == true) ? "已配对 · 加密连接。可前往屏幕与键鼠、声音设置。":"已保存配对，另一台电脑上线后会自动连接。") : pending != nil ? "请核对两台电脑显示的配对码。":"附近的设备 · 正在自动发现"
        let devices=s["nearby"] as? [[String:Any]] ?? []
        let nk=devices.map{($0["id"] as? String ?? "")+($0["name"] as? String ?? "")}.joined(separator:"|")
        if nk != nearbyKey || nearbyDocument.subviews.isEmpty{
            for v in nearbyDocument.subviews{v.removeFromSuperview()}
            let width=max(600,nearbyScroll.contentSize.width),height=max(210,CGFloat(devices.count)*78)
            nearbyDocument.frame=NSRect(x:0,y:0,width:width,height:height);nearbyDocument.autoresizingMask=[.width]
            for (i,n) in devices.enumerated(){
                let b=button((n["name"] as? String ?? "")+"  ·  "+(n["os"] as? String ?? "")+"    点击配对","connect:"+(n["id"] as? String ?? ""))
                b.bezelStyle = .regularSquare;b.frame=NSRect(x:0,y:height-CGFloat(i+1)*78+8,width:width-16,height:64);b.autoresizingMask=[.width];b.font = .systemFont(ofSize:15,weight:.medium);nearbyDocument.addSubview(b)
            }
            if devices.isEmpty{let hint=label("还没发现设备。\n请确认另一台电脑已打开双游，并连接同一局域网。");hint.frame=NSRect(x:0,y:130,width:width-16,height:64);hint.autoresizingMask=[.width];nearbyDocument.addSubview(hint)}
            nearbyKey=nk
        }
        for b in nearbyDocument.subviews.compactMap({$0 as? NSButton}){b.isEnabled=pair.isEnabled}
        let shared=s["settings"] as? [String:Any] ?? [:],prefs=s["preferences"] as? [String:Any] ?? [:]
        let key=String(describing:shared)+name+peerName,choices=[(id,name+"（这台）"),(peerID,peerName)]
        if p != nil && sharedKey != key{
            if !dirty.contains(0){options(source,choices,selected:shared["sourceID"] as? String ?? "");select(side,shared["side"] as? String ?? "right");sharing.state=shared["enabled"] as? Bool == true ? .on:.off;delay.integerValue=shared["edgeDelayMS"] as? Int ?? 0;select(returnKey,shared["returnKey"] as? String ?? "F12")}
            if !dirty.contains(1){options(target,[("","各自播放 · 关闭转发")]+choices,selected:shared["audioTarget"] as? String ?? "");volume.integerValue=shared["audioVolume"] as? Int ?? 100;select(buffer,String(shared["audioBufferMS"] as? Int ?? 80))}
            sharedKey=key
        }
        let pk=String(describing:prefs)+name
        if !dirty.contains(2) && preferencesKey != pk{
            deviceName.stringValue=name;select(theme,prefs["theme"] as? String ?? "system");login.state=prefs["startAtLogin"] as? Bool == true ? .on:.off
            let t=prefs["theme"] as? String ?? "system";window.appearance=t=="dark" ? NSAppearance(named:.darkAqua):t=="light" ? NSAppearance(named:.aqua):nil;preferencesKey=pk
        }
        engine.stringValue=s["engine"] as? String ?? "";audio.stringValue=s["audio"] as? String ?? ""
        let message=s["message"] as? String ?? ""
        if message != lastMessage {if !message.isEmpty || feedback.stringValue == lastMessage {feedback.stringValue=message};lastMessage=message}
        updatePreview()
    }
    func updatePreview(){
        let name=state["name"] as? String ?? "",id=state["id"] as? String ?? "",p=state["peer"] as? [String:Any] ?? [:],peer=p["name"] as? String ?? ""
        let local=selected(source)==id;preview.source=local ? name:peer;preview.other=local ? peer:name;preview.side=selected(side);preview.needsDisplay=true
        let t=selected(target);route.stringValue=t.isEmpty ? "各自播放，不传输声音":t==id ? peer+" → "+name:name+" → "+peer
    }
    @objc func show(){NSApp.activate(ignoringOtherApps:true);window.makeKeyAndOrderFront(nil)}
    func applicationShouldHandleReopen(_ sender:NSApplication,hasVisibleWindows flag:Bool)->Bool{show();return true}
    @objc func accessibility(){let options=[kAXTrustedCheckOptionPrompt.takeUnretainedValue() as String:true] as CFDictionary;_ = AXIsProcessTrustedWithOptions(options);if let url=URL(string:"x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility"){NSWorkspace.shared.open(url)}}
    @objc func audioPermission(){if let url=URL(string:"x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture"){NSWorkspace.shared.open(url)}}
    @objc func networkPermission(){
        feedback.stringValue="在系统设置 → 隐私与安全性 → 本地网络中，允许 TwinGlide 与 TwinGlide Input。更改后请退出并重新打开双游。"
        if let url=URL(string:"x-apple.systempreferences:com.apple.settings.PrivacySecurity.extension"){NSWorkspace.shared.open(url)}
    }
    @objc func quit(){NSApp.terminate(nil)}
    func applicationWillTerminate(_ notification:Notification){
        closing=true;timer?.invalidate();try? pipe?.fileHandleForWriting.close()
        if let p=agent{let until=Date().addingTimeInterval(7);while p.isRunning && Date()<until{Thread.sleep(forTimeInterval:0.05)};if p.isRunning{p.terminate()}}
    }
}
let app=NSApplication.shared
app.setActivationPolicy(.accessory)
let delegate=Delegate();app.delegate=delegate;app.run()
