// SPDX-License-Identifier: GPL-2.0-only
using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Drawing;
using System.IO;
using System.Net;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using System.Web.Script.Serialization;
using System.Windows.Forms;

[assembly: System.Runtime.Versioning.TargetFramework(".NETFramework,Version=v4.7.2")]

static class Json {
 public static Dictionary<string,object> Map(object v){return v as Dictionary<string,object> ?? new Dictionary<string,object>();}
 public static object Get(Dictionary<string,object> d,string k){object v;return d.TryGetValue(k,out v)?v:null;}
 public static string Str(Dictionary<string,object> d,string k){return Convert.ToString(Get(d,k));}
 public static bool Yes(Dictionary<string,object> d,string k){return Get(d,k) is bool && (bool)d[k];}
 public static int Num(Dictionary<string,object> d,string k,int fallback){try{return Convert.ToInt32(d[k]);}catch{return fallback;}}
 public static Dictionary<string,object> Read(string s){return new JavaScriptSerializer().Deserialize<Dictionary<string,object>>(s);}
 public static string Write(object v){return new JavaScriptSerializer().Serialize(v);}
}
sealed class Choice {
 public string ID,Text;public Choice(string id,string text){ID=id;Text=text;}public override string ToString(){return Text;}
}
sealed class SettingsWindow : Form {
 readonly string endpointPath;readonly int agentPID;
 readonly Label connection=new Label(),engine=new Label(),audio=new Label(),feedback=new Label(),pairCode=new Label(),route=new Label();
 readonly FlowLayoutPanel nearby=new FlowLayoutPanel{FlowDirection=FlowDirection.TopDown,WrapContents=false,AutoScroll=true};
 readonly Label devicesHint=new Label(),pairedDevice=new Label();
 readonly TextBox address=new TextBox(),deviceName=new TextBox();
 readonly ComboBox source=new ComboBox(),side=new ComboBox(),returnKey=new ComboBox(),target=new ComboBox(),buffer=new ComboBox(),theme=new ComboBox();
 readonly CheckBox sharing=new CheckBox(),login=new CheckBox();
 readonly NumericUpDown delay=new NumericUpDown(),volume=new NumericUpDown();
 readonly Button confirm=new Button(),reject=new Button(),connect=new Button(),forget=new Button();
 readonly Panel pages=new Panel{Dock=DockStyle.Fill};
 readonly FlowLayoutPanel navigation=new FlowLayoutPanel{Dock=DockStyle.Fill,FlowDirection=FlowDirection.TopDown,WrapContents=false};
 readonly Label pageTitle=new Label(),localName=new Label();
 readonly Panel sidebar=new Panel{Dock=DockStyle.Fill,Tag="sidebar"};
 readonly Button saveAction=new Button();
 readonly ToolTip tips=new ToolTip();
 readonly Font titleFont=new Font("Microsoft YaHei UI",22,FontStyle.Bold),sectionFont=new Font("Microsoft YaHei UI",12,FontStyle.Bold),navFont=new Font("Microsoft YaHei UI",10,FontStyle.Bold);
 readonly string[] titles={"附近的设备","屏幕与键鼠","声音","个性化"};
 int activePage;bool isDark;
 readonly Panel devicesPage=new Panel(),screenPage=new Panel(),audioPage=new Panel(),generalPage=new Panel();
 readonly List<RadioButton> navigationButtons=new List<RadioButton>();
 readonly LayoutPreview preview=new LayoutPreview();
 readonly System.Windows.Forms.Timer timer=new System.Windows.Forms.Timer{Interval=1200};
 Dictionary<string,object> state;bool loading,updating,screenDirty,audioDirty,generalDirty,busy;
 string nearbyKey="",lastShared="",lastPrefs="",lastMessage="";
 public SettingsWindow(string path,int pid){
  endpointPath=path;agentPID=pid;Text="TwinGlide 双游";AutoScaleDimensions=new SizeF(96,96);AutoScaleMode=AutoScaleMode.Dpi;
  Font=new Font("Microsoft YaHei UI",10);ClientSize=new Size(1000,760);MinimumSize=new Size(840,660);StartPosition=FormStartPosition.CenterScreen;Icon=Icon.ExtractAssociatedIcon(Application.ExecutablePath);
  var root=new TableLayoutPanel{Dock=DockStyle.Fill,ColumnCount=2,RowCount=1,Margin=Padding.Empty};root.ColumnStyles.Add(new ColumnStyle(SizeType.Absolute,184));root.ColumnStyles.Add(new ColumnStyle(SizeType.Percent,100));root.RowStyles.Add(new RowStyle(SizeType.Percent,100));Controls.Add(root);
  sidebar.Margin=Padding.Empty;root.Controls.Add(sidebar,0,0);
  var rail=new TableLayoutPanel{Dock=DockStyle.Fill,Padding=new Padding(16,28,16,24),ColumnCount=1,RowCount=4,Margin=Padding.Empty};rail.RowStyles.Add(new RowStyle(SizeType.Absolute,88));rail.RowStyles.Add(new RowStyle(SizeType.Absolute,208));rail.RowStyles.Add(new RowStyle(SizeType.Percent,100));rail.RowStyles.Add(new RowStyle(SizeType.AutoSize));sidebar.Controls.Add(rail);
  var brand=PagePanel();brand.Controls.Add(new Label{Text="TwinGlide",Font=new Font(Font.FontFamily,17,FontStyle.Bold),AutoSize=true,Margin=new Padding(8,0,0,4)});brand.Controls.Add(new Label{Text="双游",AutoSize=true,Tag="muted",Margin=new Padding(8,0,0,0)});rail.Controls.Add(brand,0,0);navigation.Margin=Padding.Empty;rail.Controls.Add(navigation,0,1);
  var local=PagePanel();local.Padding=new Padding(8,0,0,0);Hint(local,"这台电脑");local.Controls[0].Margin=new Padding(0,0,0,8);localName.AutoSize=true;localName.MaximumSize=new Size(136,0);localName.Margin=new Padding(0);local.Controls.Add(localName);rail.Controls.Add(local,0,3);
  var main=new TableLayoutPanel{Dock=DockStyle.Fill,Padding=new Padding(32,28,32,16),ColumnCount=1,RowCount=3,Margin=Padding.Empty};main.RowStyles.Add(new RowStyle(SizeType.AutoSize));main.RowStyles.Add(new RowStyle(SizeType.Percent,100));main.RowStyles.Add(new RowStyle(SizeType.AutoSize));root.Controls.Add(main,1,0);
  var heading=PagePanel();pageTitle.Text=titles[0];pageTitle.Font=titleFont;pageTitle.AutoSize=true;pageTitle.Margin=new Padding(0,0,0,10);heading.Controls.Add(pageTitle);connection.Text="正在启动本机连接…";Hint(heading,connection);main.Controls.Add(heading,0,0);pages.Margin=Padding.Empty;main.Controls.Add(pages,0,1);
  var footer=new TableLayoutPanel{Dock=DockStyle.Fill,AutoSize=true,ColumnCount=2,MinimumSize=new Size(0,66),Padding=new Padding(0,16,0,0),Margin=Padding.Empty};footer.ColumnStyles.Add(new ColumnStyle(SizeType.Percent,100));footer.ColumnStyles.Add(new ColumnStyle(SizeType.AutoSize));feedback.AutoSize=true;feedback.Anchor=AnchorStyles.Left;feedback.Tag="muted";feedback.Margin=new Padding(0,0,20,0);footer.Controls.Add(feedback,0,0);footer.SizeChanged+=delegate{feedback.MaximumSize=new Size(Math.Max(180,footer.ClientSize.Width-(saveAction.Visible?184:0)),0);};StyleButton(saveAction,true);saveAction.Margin=Padding.Empty;saveAction.MinimumSize=new Size(152,36);saveAction.Anchor=AnchorStyles.Right;saveAction.Click+=async delegate{if(activePage==1)await SaveScreen();else if(activePage==2)await SaveAudio();else if(activePage==3)await SaveGeneral();};footer.Controls.Add(saveAction,1,0);main.Controls.Add(footer,0,2);
  var pageList=new[]{devicesPage,screenPage,audioPage,generalPage};
  for(int i=0;i<pageList.Length;i++){int index=i;var page=pageList[i];page.Dock=DockStyle.Fill;page.Margin=Padding.Empty;page.Visible=i==0;pages.Controls.Add(page);var nav=new RadioButton{Appearance=Appearance.Button,FlatStyle=FlatStyle.Flat,Text=titles[i],Width=152,Height=44,TextAlign=ContentAlignment.MiddleLeft,Padding=new Padding(12,0,0,0),Margin=new Padding(0,0,0,8),Checked=i==0,AccessibleRole=AccessibleRole.PageTab,AccessibleName=titles[i]};nav.FlatAppearance.BorderSize=0;nav.CheckedChanged+=delegate{if(nav.Checked){foreach(var other in pageList)other.Visible=other==page;page.BringToFront();activePage=index;pageTitle.Text=titles[index];UpdateActions();if(lastMessage=="")feedback.Text=index==0?"首次配对需要在两台电脑上确认。":index==3?"仅保存这台电脑的偏好。":"保存后会同步到另一台电脑。";}StyleNavigation();};navigationButtons.Add(nav);navigation.Controls.Add(nav);}
  var d=Page(devicesPage);Hint(d,"两台电脑打开双游，点击设备名称即可配对。首次双方确认，以后自动连接。");
  pairedDevice.Font=sectionFont;Hint(d,pairedDevice);pairedDevice.Tag=null;Hint(d,devicesHint);
  nearby.Height=164;nearby.Dock=DockStyle.Top;nearby.Margin=new Padding(0,0,0,16);d.Controls.Add(nearby);nearby.Resize+=delegate{foreach(Control c in nearby.Controls)c.Width=Math.Max(200,nearby.ClientSize.Width-SystemInformation.VerticalScrollBarWidth-2);};
  var advanced=Button("高级连接选项",delegate{});var options=PagePanel();options.Visible=false;
  advanced.Click+=delegate{options.Visible=!options.Visible;advanced.Text=options.Visible?"收起高级选项":"高级连接选项";};
  options.Controls.Add(Button("重新开放配对",async delegate{await Act("pairing",new{});}));
  Field(options,"另一台电脑的 IP",address);options.Controls.Add(Button("按 IP 连接",async delegate{await Act("manual",new{address=address.Text});}));
  pairCode.Font=titleFont;pairCode.AutoSize=true;pairCode.Margin=new Padding(0,16,0,8);d.Controls.Add(pairCode);d.SizeChanged+=delegate{pairCode.MaximumSize=new Size(Math.Max(200,d.ClientSize.Width),0);};
  var pairActions=Row();confirm.Text="两边一致，确认配对";StyleButton(confirm,true);confirm.Click+=async delegate{await Confirm(true);};reject.Text="不一致，取消";StyleButton(reject,false);reject.Click+=async delegate{await Confirm(false);};pairActions.Controls.AddRange(new Control[]{confirm,reject});d.Controls.Add(pairActions);
  d.Controls.Add(advanced);d.Controls.Add(options);
  forget.Text="移除已配对设备";StyleButton(forget,false);forget.Click+=async delegate{if(MessageBox.Show(this,"移除配对后，需要在两台电脑重新确认。","移除配对",MessageBoxButtons.OKCancel,MessageBoxIcon.Question)==DialogResult.OK){screenDirty=audioDirty=false;await Act("forget",new{});}};options.Controls.Add(forget);
  var s=Page(screenPage);Hint(s,"选择键鼠连接的电脑，再按桌面上的实际摆放设置屏幕方向。");
  Field(s,"键鼠连接在",source);Fill(side,new[]{new Choice("left","左侧"),new Choice("right","右侧"),new Choice("up","上方"),new Choice("down","下方")});Field(s,"另一台屏幕的位置",side);
  preview.Height=144;preview.Dock=DockStyle.Top;preview.Margin=new Padding(0,0,0,12);s.Controls.Add(preview);sharing.Text="启用键鼠跨屏";sharing.AutoSize=true;Field(s,"键鼠共享",sharing);
  delay.Maximum=1000;delay.Increment=50;Field(s,"边缘停留（毫秒）",delay);
  Fill(returnKey,new[]{new Choice("F10","Ctrl + Alt + Shift + F10"),new Choice("F11","Ctrl + Alt + Shift + F11"),new Choice("F12","Ctrl + Alt + Shift + F12")});Field(s,"立即回到键鼠所在电脑",returnKey);
  Hint(s,engine);
  var a=Page(audioPage);Hint(a,"把另一台电脑的系统声音送到所选电脑。每次只转发一个方向，原电脑也会继续播放。");
  Field(a,"声音输出到",target);Hint(a,route);route.Font=sectionFont;route.Tag=null;route.Margin=new Padding(0,12,0,24);
  volume.Maximum=100;volume.Value=100;volume.Increment=5;Field(a,"接收音量（%）",volume);
  Fill(buffer,new[]{new Choice("40","40 毫秒 · 低延迟"),new Choice("80","80 毫秒 · 均衡"),new Choice("160","160 毫秒 · 稳定")});Field(a,"声音缓冲",buffer);
  Hint(a,audio);Hint(a,"Mac 转发系统声音需要系统录音授权。双游只传输系统声音，不采集麦克风。");
  var g=Page(generalPage);Hint(g,"首次打开会获得一个随机昵称。你可以重新随机，也可以改成容易辨认的名字。");deviceName.MaxLength=40;Field(g,"这台电脑的昵称",deviceName);
  var shuffle=Button("换一个随机昵称",async delegate{try{var result=await Request("nickname",new{});deviceName.Text=Json.Str(result,"name");generalDirty=true;feedback.Text="昵称已生成，保存后生效";}catch(Exception e){feedback.Text=e.Message;}});Field(g,"",shuffle);shuffle.Anchor=AnchorStyles.Left;
  Fill(theme,new[]{new Choice("system","跟随系统"),new Choice("light","浅色"),new Choice("dark","深色")});Field(g,"外观",theme);login.Text="登录后自动启动双游";login.AutoSize=true;Field(g,"启动",login);
  Hint(g,"关闭窗口后会留在托盘。需要停止共享时，从托盘菜单选择“退出双游”。");
  foreach(Control c in new Control[]{source,side,returnKey,sharing,delay})Changed(c,delegate{if(!updating){screenDirty=true;UpdatePreview();}});
  foreach(Control c in new Control[]{target,volume,buffer})Changed(c,delegate{if(!updating){audioDirty=true;UpdatePreview();}});
  foreach(Control c in new Control[]{deviceName,theme,login})Changed(c,delegate{if(!updating)generalDirty=true;});
  FormClosing+=delegate(object sender,FormClosingEventArgs e){if(e.CloseReason==CloseReason.UserClosing){e.Cancel=true;Hide();}};
  UpdateActions();ApplyTheme("system");timer.Tick+=async delegate{await RefreshState();};timer.Start();Shown+=async delegate{await RefreshState();};
 }
 static TableLayoutPanel PagePanel(){var p=new TableLayoutPanel{Dock=DockStyle.Top,AutoSize=true,AutoSizeMode=AutoSizeMode.GrowAndShrink,ColumnCount=1,Margin=new Padding(0)};p.ColumnStyles.Add(new ColumnStyle(SizeType.Percent,100));return p;}
 static TableLayoutPanel Page(Panel page){page.Padding=Padding.Empty;var host=new Panel{Dock=DockStyle.Fill,AutoScroll=true,Margin=Padding.Empty};var p=PagePanel();p.Padding=new Padding(0,0,0,16);host.Controls.Add(p);page.Controls.Add(host);return p;}
 static FlowLayoutPanel Row(){return new FlowLayoutPanel{AutoSize=true,AutoSizeMode=AutoSizeMode.GrowAndShrink,Dock=DockStyle.Top,WrapContents=true,Margin=new Padding(0,0,0,8)};}
 static void Hint(TableLayoutPanel p,string text){Hint(p,new Label{Text=text});}
 static void Hint(TableLayoutPanel p,Label label){label.AutoSize=true;label.Tag="muted";label.MaximumSize=new Size(600,0);label.Margin=new Padding(0,0,0,24);p.Controls.Add(label);p.SizeChanged+=delegate{int w=Math.Max(120,p.ClientSize.Width-p.Padding.Horizontal);if(label.MaximumSize.Width!=w)label.MaximumSize=new Size(w,0);};}
 static void Field(TableLayoutPanel p,string text,Control c){var row=new TableLayoutPanel{ColumnCount=2,AutoSize=true,Dock=DockStyle.Top,MinimumSize=new Size(0,40),Margin=new Padding(0,0,0,8)};row.ColumnStyles.Add(new ColumnStyle(SizeType.Absolute,176));row.ColumnStyles.Add(new ColumnStyle(SizeType.Percent,100));row.Controls.Add(new Label{Text=text,AutoSize=true,Anchor=AnchorStyles.Left,Margin=new Padding(0,0,16,0)});c.Anchor=AnchorStyles.Left|AnchorStyles.Right;c.Margin=Padding.Empty;if(text!="")c.AccessibleName=text;if(c is ComboBox){((ComboBox)c).DropDownStyle=ComboBoxStyle.DropDownList;((ComboBox)c).FlatStyle=FlatStyle.Flat;((ComboBox)c).DropDownWidth=320;}if(c is TextBox)((TextBox)c).BorderStyle=BorderStyle.FixedSingle;if(c is NumericUpDown)((NumericUpDown)c).BorderStyle=BorderStyle.FixedSingle;row.Controls.Add(c);p.Controls.Add(row);}
 static void StyleButton(Button b,bool primary){b.AutoSize=true;b.AutoSizeMode=AutoSizeMode.GrowAndShrink;b.FlatStyle=FlatStyle.Flat;b.Padding=new Padding(16,5,16,5);b.MinimumSize=new Size(0,36);b.Margin=new Padding(0,0,8,16);b.Tag=primary?"primary":"button";b.UseVisualStyleBackColor=false;b.FlatAppearance.BorderSize=1;}
 static Button Button(string text,EventHandler action){var b=new Button{Text=text};StyleButton(b,false);b.Click+=action;return b;}
 void UpdateActions(){saveAction.Visible=activePage>0;saveAction.Text=activePage==1?"保存屏幕设置":activePage==2?"保存声音设置":"保存个性化设置";saveAction.Enabled=!busy&&(activePage==3||(state!=null&&Json.Map(Json.Get(state,"peer")).Count>0));}
 static void Changed(Control c,EventHandler f){if(c is ComboBox)((ComboBox)c).SelectedIndexChanged+=f;else if(c is CheckBox)((CheckBox)c).CheckedChanged+=f;else if(c is NumericUpDown)((NumericUpDown)c).ValueChanged+=f;else c.TextChanged+=f;}
 static void Fill(ComboBox c,Choice[] values){c.Items.Clear();c.Items.AddRange(values);}
 static string Selected(ComboBox c){var x=c.SelectedItem as Choice;return x==null?"":x.ID;}
 static void Select(ComboBox c,string value){for(int i=0;i<c.Items.Count;i++)if(((Choice)c.Items[i]).ID==value){c.SelectedIndex=i;return;}c.SelectedIndex=-1;}
 async Task<Dictionary<string,object>> Request(string path,object body){return await Task.Run(()=>{
   var ep=Json.Read(File.ReadAllText(endpointPath));Uri uri;
   if(Json.Num(ep,"pid",0)!=agentPID||!Uri.TryCreate(Json.Str(ep,"url"),UriKind.Absolute,out uri)||uri.Scheme!="http"||uri.Host!="127.0.0.1"||Json.Str(ep,"token").Length!=64)throw new Exception("本机连接信息无效，请重启双游。");
   var req=(HttpWebRequest)WebRequest.Create(new Uri(uri,"/api/"+path));req.Proxy=null;req.Timeout=4000;req.ReadWriteTimeout=4000;req.Headers["X-TwinGlide-Token"]=Json.Str(ep,"token");
   if(body!=null){req.Method="POST";req.ContentType="application/json";var bytes=Encoding.UTF8.GetBytes(Json.Write(body));using(var stream=req.GetRequestStream())stream.Write(bytes,0,bytes.Length);}
   try{using(var res=req.GetResponse())using(var reader=new StreamReader(res.GetResponseStream()))return Json.Read(reader.ReadToEnd());}
   catch(WebException ex){string message="无法连接本机后台，请重新打开双游。";if(ex.Response!=null)using(var reader=new StreamReader(ex.Response.GetResponseStream())){try{message=Json.Str(Json.Read(reader.ReadToEnd()),"error");}catch{}}throw new Exception(message,ex);}
  });}
 async Task Act(string path,object body){if(busy)return;busy=true;try{await Request(path,body);feedback.Text="操作已完成";await RefreshState();}catch(Exception ex){feedback.Text=ex.Message;}finally{busy=false;}}
 async Task Confirm(bool match){if(state==null)return;var p=Json.Map(Json.Get(state,"pending"));await Act("confirm",new{session=Json.Str(p,"session"),match=match});}
 Dictionary<string,object> Shared(){var s=new Dictionary<string,object>(Json.Map(Json.Get(state,"settings")));s.Remove("revision");s.Remove("actor");return s;}
 async Task SaveScreen(){if(state==null)return;var s=Shared();s["sourceID"]=Selected(source);s["side"]=Selected(side);s["enabled"]=sharing.Checked;s["edgeDelayMS"]=(int)delay.Value;s["returnKey"]=Selected(returnKey);await Save("settings",s,0);}
 async Task SaveAudio(){if(state==null||Selected(buffer)=="")return;var s=Shared();s["audioTarget"]=Selected(target);s["audioVolume"]=(int)volume.Value;s["audioBufferMS"]=Int32.Parse(Selected(buffer));await Save("settings",s,1);}
 async Task SaveGeneral(){await Save("preferences",new{name=deviceName.Text,theme=Selected(theme),startAtLogin=login.Checked},2);}
 async Task Save(string path,object body,int group){if(busy)return;busy=true;UpdateActions();try{await Request(path,body);if(group==0)screenDirty=false;if(group==1)audioDirty=false;if(group==2)generalDirty=false;lastShared=lastPrefs="";feedback.Text=group==2?"已保存这台电脑的偏好":"已保存，并同步到另一台电脑";await RefreshState();}catch(Exception ex){feedback.Text=ex.Message;}finally{busy=false;UpdateActions();}}
 async Task RefreshState(){if(loading||IsDisposed)return;loading=true;try{var value=await Request("state",null);if(!IsDisposed)Render(value);}catch(Exception ex){if(!IsDisposed)connection.Text=ex.Message;}finally{loading=false;}}
 void Render(Dictionary<string,object> s){
  state=s;updating=true;try{
   var peer=Json.Map(Json.Get(s,"peer"));var pending=Json.Map(Json.Get(s,"pending"));var shared=Json.Map(Json.Get(s,"settings"));var prefs=Json.Map(Json.Get(s,"preferences"));bool paired=peer.Count>0;
   localName.Text=Json.Str(s,"name");tips.SetToolTip(localName,localName.Text);connection.Text=Json.Yes(s,"connected")?"已连接 "+Json.Str(peer,"name"):paired?"等待 "+Json.Str(peer,"name")+" 上线":"正在寻找同一局域网中的电脑";
   if(Json.Yes(s,"noInput"))connection.Text+="（隔离测试）";screenPage.Enabled=audioPage.Enabled=paired;forget.Enabled=paired;connect.Enabled=!paired&&!Json.Yes(s,"dialing")&&pending.Count==0;
   nearby.Visible=!paired&&pending.Count==0;pairedDevice.Visible=paired;pairedDevice.Text=paired?Json.Str(peer,"name")+"  ·  "+Json.Str(peer,"os"):"";
   devicesHint.Text=paired?(Json.Yes(s,"connected")?"已配对 · 加密连接。可前往屏幕与键鼠、声音设置。":"已保存配对，另一台电脑上线后会自动连接。"):pending.Count>0?"请核对两台电脑显示的配对码。":"附近的设备 · 正在自动发现";
   pairCode.Visible=confirm.Parent.Visible=pending.Count>0;pairCode.Text=pending.Count==0?"":Json.Str(pending,"name")+"\n"+Json.Str(pending,"code");confirm.Visible=reject.Visible=pending.Count>0;confirm.Enabled=!Json.Yes(pending,"approved");confirm.Text=Json.Yes(pending,"approved")?"已确认，等待另一台电脑":"两边一致，确认配对";
   var items=new List<object>();var identities=new List<string>();var values=Json.Get(s,"nearby") as System.Collections.IEnumerable;if(values!=null)foreach(var value in values){items.Add(value);var n=Json.Map(value);identities.Add(Json.Str(n,"id")+"|"+Json.Str(n,"name")+"|"+Json.Str(n,"os"));}var list=items.ToArray();var key=Json.Write(identities);
   if(key!=nearbyKey){while(nearby.Controls.Count>0){var c=nearby.Controls[0];nearby.Controls.Remove(c);c.Dispose();}foreach(var item in list){var n=Json.Map(item);string peerID=Json.Str(n,"id");var b=Button(Json.Str(n,"name")+"\n"+Json.Str(n,"os")+" · 点击配对",async delegate{await Act("connect",new{id=peerID});});b.AutoSize=false;b.Height=72;b.Width=Math.Max(200,nearby.ClientSize.Width-SystemInformation.VerticalScrollBarWidth-2);b.TextAlign=ContentAlignment.MiddleLeft;b.Margin=new Padding(0,0,0,8);nearby.Controls.Add(b);}if(list.Length==0)nearby.Controls.Add(new Label{AutoSize=true,Tag="muted",Text="还没发现设备\n请确认另一台电脑已打开双游，并连接同一局域网。",MaximumSize=new Size(540,0),Margin=Padding.Empty,Padding=new Padding(0,8,0,0)});nearbyKey=key;ApplyTheme(Json.Str(prefs,"theme"));}
   foreach(Control c in nearby.Controls)c.Enabled=connect.Enabled;
   var choices=new[]{new Choice(Json.Str(s,"id"),Json.Str(s,"name")+"（这台）"),new Choice(Json.Str(peer,"id"),Json.Str(peer,"name"))};key=Json.Write(new object[]{shared,choices[0].Text,choices[1].Text});
   if(paired&&key!=lastShared){if(!screenDirty){Fill(source,choices);Select(source,Json.Str(shared,"sourceID"));Select(side,Json.Str(shared,"side"));sharing.Checked=Json.Yes(shared,"enabled");delay.Value=Json.Num(shared,"edgeDelayMS",0);Select(returnKey,Json.Str(shared,"returnKey"));}if(!audioDirty){Fill(target,new[]{new Choice("","各自播放 · 关闭转发"),choices[0],choices[1]});Select(target,Json.Str(shared,"audioTarget"));volume.Value=Json.Num(shared,"audioVolume",100);Select(buffer,Json.Str(shared,"audioBufferMS"));}lastShared=key;}
   key=Json.Write(new object[]{Json.Str(s,"name"),prefs});if(!generalDirty&&key!=lastPrefs){deviceName.Text=Json.Str(s,"name");Select(theme,Json.Str(prefs,"theme"));login.Checked=Json.Yes(prefs,"startAtLogin");ApplyTheme(Json.Str(prefs,"theme"));lastPrefs=key;}
   engine.Text=Json.Str(s,"engine");audio.Text=Json.Str(s,"audio");var message=Json.Str(s,"message");if(message!=lastMessage){if(message!=""||feedback.Text==lastMessage)feedback.Text=message;lastMessage=message;}UpdatePreview();UpdateActions();
  }finally{updating=false;}
 }
 void UpdatePreview(){if(state==null)return;var p=Json.Map(Json.Get(state,"peer"));bool local=Selected(source)==Json.Str(state,"id");preview.Source=local?Json.Str(state,"name"):Json.Str(p,"name");preview.Other=local?Json.Str(p,"name"):Json.Str(state,"name");preview.Side=Selected(side);preview.AccessibleDescription="键鼠来源："+preview.Source+"；另一台电脑："+preview.Other+"；位置："+side.Text;preview.Invalidate();var id=Selected(target);route.Text=id==""?"各自播放，不传输声音":id==Json.Str(state,"id")?Json.Str(p,"name")+" → "+Json.Str(state,"name"):Json.Str(state,"name")+" → "+Json.Str(p,"name");}
 void ApplyTheme(string value){isDark=value=="dark";if(value=="system")try{using(var k=Microsoft.Win32.Registry.CurrentUser.OpenSubKey(@"Software\Microsoft\Windows\CurrentVersion\Themes\Personalize"))isDark=Convert.ToInt32(k.GetValue("AppsUseLightTheme",1))==0;}catch{}var colors=new UiColors(isDark);PaintControls(this,colors,colors.Surface);preview.Colors=colors;preview.Invalidate();StyleNavigation();if(IsHandleCreated)NativeTheme.TitleBar(Handle,isDark);}
 protected override void OnHandleCreated(EventArgs e){base.OnHandleCreated(e);NativeTheme.TitleBar(Handle,isDark);}
 void StyleNavigation(){var c=new UiColors(isDark);foreach(var b in navigationButtons){b.BackColor=b.Checked?c.Selection:c.Sidebar;b.ForeColor=b.Checked?c.SelectedText:c.Text;b.Font=b.Checked?navFont:Font;b.FlatAppearance.CheckedBackColor=c.Selection;b.FlatAppearance.MouseOverBackColor=c.Selection;b.FlatAppearance.MouseDownBackColor=c.Selection;}}
 static void PaintControls(Control c,UiColors colors,Color background){string role=c.Tag as string;if(role=="sidebar")background=colors.Sidebar;c.BackColor=background;c.ForeColor=role=="muted"?colors.Muted:colors.Text;if(c is TextBox||c is ComboBox||c is NumericUpDown||c is ListBox)c.BackColor=colors.Field;var button=c as Button;if(button!=null){bool primary=role=="primary";button.BackColor=primary?colors.Accent:colors.Surface;button.ForeColor=primary?colors.OnAccent:colors.Text;button.FlatAppearance.BorderColor=primary?colors.Accent:colors.Border;button.FlatAppearance.MouseOverBackColor=primary?colors.AccentHover:colors.Selection;button.FlatAppearance.MouseDownBackColor=primary?colors.AccentHover:colors.Selection;}foreach(Control child in c.Controls)PaintControls(child,colors,c.BackColor);}
 protected override void Dispose(bool disposing){if(disposing){timer.Dispose();tips.Dispose();}base.Dispose(disposing);if(disposing){titleFont.Dispose();sectionFont.Dispose();navFont.Dispose();}}
}
static class NativeTheme {
 [System.Runtime.InteropServices.DllImport("dwmapi.dll")]static extern int DwmSetWindowAttribute(IntPtr window,int attribute,ref int value,int size);
 public static void TitleBar(IntPtr window,bool dark){try{int value=dark&&!SystemInformation.HighContrast?1:0;DwmSetWindowAttribute(window,20,ref value,4);}catch(DllNotFoundException){}catch(EntryPointNotFoundException){}}
}
sealed class UiColors {
 public Color Surface,Sidebar,Text,Muted,Border,Accent,AccentHover,OnAccent,Selection,SelectedText,Field;
 public UiColors(bool dark){if(SystemInformation.HighContrast){Surface=Field=SystemColors.Window;Sidebar=SystemColors.Control;Text=Muted=SystemColors.WindowText;Border=SystemColors.WindowText;Accent=AccentHover=Selection=SystemColors.Highlight;OnAccent=SelectedText=SystemColors.HighlightText;return;}Surface=ColorTranslator.FromHtml(dark?"#20252E":"#FFFFFF");Sidebar=ColorTranslator.FromHtml(dark?"#191E26":"#F3F5F8");Text=ColorTranslator.FromHtml(dark?"#EFF3FA":"#202B3A");Muted=ColorTranslator.FromHtml(dark?"#AFBACC":"#586579");Border=ColorTranslator.FromHtml(dark?"#4C586B":"#DCE2EB");Accent=ColorTranslator.FromHtml("#225DD2");AccentHover=ColorTranslator.FromHtml("#194FB8");OnAccent=Color.White;Selection=ColorTranslator.FromHtml(dark?"#2A3A58":"#E8EFFD");SelectedText=ColorTranslator.FromHtml(dark?"#C8DCFF":"#194FB8");Field=ColorTranslator.FromHtml(dark?"#2A313D":"#FFFFFF");}
}
sealed class LayoutPreview : Control {
 public string Source="",Other="",Side="right";
 public UiColors Colors=new UiColors(false);
 public LayoutPreview(){DoubleBuffered=true;AccessibleName="屏幕相对位置预览";}
 protected override void OnPaint(PaintEventArgs e){base.OnPaint(e);var g=e.Graphics;g.SmoothingMode=System.Drawing.Drawing2D.SmoothingMode.AntiAlias;float scale=DeviceDpi/96f;bool vertical=Side=="up"||Side=="down";int gap=(int)(32*scale),w=Math.Min((int)(224*scale),(Width-gap-24)/2),h=(int)(64*scale);int cy=(Height-h)/2;var a=new Rectangle(Width/2-w-gap/2,cy,w,h);var b=new Rectangle(Width/2+gap/2,cy,w,h);if(vertical){h=(int)(54*scale);a=new Rectangle((Width-w)/2,(int)(8*scale),w,h);b=new Rectangle((Width-w)/2,Height-h-(int)(8*scale),w,h);}if(Side=="left"||Side=="up"){var swap=a;a=b;b=swap;}using(var pen=new Pen(Colors.Border,scale)){g.DrawLine(pen,a.Left+a.Width/2,a.Top+a.Height/2,b.Left+b.Width/2,b.Top+b.Height/2);}DrawMonitor(g,a,Source,"键鼠来源",true,scale);DrawMonitor(g,b,Other,"另一台电脑",false,scale);}
 void DrawMonitor(Graphics g,Rectangle r,string name,string caption,bool selected,float scale){using(var fill=new SolidBrush(selected?Colors.Selection:Colors.Field))g.FillRectangle(fill,r);using(var pen=new Pen(selected?Colors.Accent:Colors.Border,selected?2*scale:scale))g.DrawRectangle(pen,r);int pad=(int)(8*scale),half=r.Height/2;TextRenderer.DrawText(g,String.IsNullOrEmpty(name)?caption:name,Font,new Rectangle(r.X+pad,r.Y+(int)(5*scale),r.Width-2*pad,half),Colors.Text,TextFormatFlags.HorizontalCenter|TextFormatFlags.VerticalCenter|TextFormatFlags.EndEllipsis|TextFormatFlags.SingleLine);TextRenderer.DrawText(g,caption,Font,new Rectangle(r.X+pad,r.Y+half,r.Width-2*pad,half-(int)(3*scale)),Colors.Muted,TextFormatFlags.HorizontalCenter|TextFormatFlags.VerticalCenter|TextFormatFlags.SingleLine);}
}
sealed class TwinGlideTray : ApplicationContext {
 readonly NotifyIcon icon;readonly Process agent;readonly SettingsWindow window;readonly System.Windows.Forms.Timer timer;readonly EventWaitHandle showEvent;bool closing;
 public TwinGlideTray(EventWaitHandle signal){
  showEvent=signal;string root=AppDomain.CurrentDomain.BaseDirectory,data=Environment.GetEnvironmentVariable("TWINGLIDE_DATA_DIR");
  var start=new ProcessStartInfo(Path.Combine(root,"twinglide-agent.exe"),"--parent-stdin"+(Environment.GetEnvironmentVariable("TWINGLIDE_TEST_MODE")=="1"?" --no-input":"")){UseShellExecute=false,CreateNoWindow=true,RedirectStandardInput=true};
  if(!String.IsNullOrEmpty(data))start.Arguments+=" --data-dir \""+data+"\"";agent=Process.Start(start);
  window=new SettingsWindow(Path.Combine(String.IsNullOrEmpty(data)?Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),"TwinGlide"):data,"endpoint.json"),agent.Id);
  icon=new NotifyIcon{Text="TwinGlide 双游",Icon=Icon.ExtractAssociatedIcon(Application.ExecutablePath),Visible=true};var menu=new ContextMenuStrip();menu.Items.Add("打开双游设置",null,delegate{Open();});menu.Items.Add("退出双游",null,delegate{ExitThread();});icon.ContextMenuStrip=menu;icon.DoubleClick+=delegate{Open();};
  timer=new System.Windows.Forms.Timer{Interval=600};timer.Tick+=delegate{if(agent.HasExited){if(!closing)MessageBox.Show("双游后台已退出，请重新打开应用。","TwinGlide");ExitThread();}else if(showEvent.WaitOne(0))Open();};timer.Start();if(!Environment.CommandLine.Contains("--background"))Open();
 }
 void Open(){window.Show();if(window.WindowState==FormWindowState.Minimized)window.WindowState=FormWindowState.Normal;window.Activate();}
 protected override void ExitThreadCore(){closing=true;timer.Dispose();window.Dispose();icon.Visible=false;icon.Dispose();try{agent.StandardInput.Close();if(!agent.WaitForExit(7000))agent.Kill();}catch{}agent.Dispose();base.ExitThreadCore();}
 [STAThread]static void Main(){bool first;using(var mutex=new Mutex(true,"Local\\TwinGlideDesktop",out first))using(var signal=new EventWaitHandle(false,EventResetMode.AutoReset,"Local\\TwinGlideShow")){if(!first){signal.Set();return;}Application.EnableVisualStyles();Application.SetCompatibleTextRenderingDefault(false);try{Application.Run(new TwinGlideTray(signal));}catch(Exception e){MessageBox.Show(e.Message,"TwinGlide");}}}
}
