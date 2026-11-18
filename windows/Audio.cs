// SPDX-License-Identifier: GPL-2.0-only
// Native system-output audio only. Networking and trust live in the Go agent.
using System;
using System.IO;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using System.Threading;
using System.Diagnostics;
// Minimal WASAPI interop: system render-endpoint loopback, never the microphone.
[ComImport,Guid("BCDE0395-E52F-467C-8E3D-C4579291692E")] class DeviceEnumerator {}
[ComImport,Guid("A95664D2-9614-4F35-A746-DE8DB63617E6"),InterfaceType(ComInterfaceType.InterfaceIsIUnknown)] interface IMMDeviceEnumerator {
    [PreserveSig]int EnumAudioEndpoints(int flow,uint mask,out IntPtr collection);
    [PreserveSig]int GetDefaultAudioEndpoint(int flow,int role,out IMMDevice device);
    [PreserveSig]int GetDevice([MarshalAs(UnmanagedType.LPWStr)]string id,out IMMDevice device);
    [PreserveSig]int RegisterEndpointNotificationCallback(IntPtr callback);
    [PreserveSig]int UnregisterEndpointNotificationCallback(IntPtr callback);
}
[ComImport,Guid("D666063F-1587-4E43-81F1-B948E807363F"),InterfaceType(ComInterfaceType.InterfaceIsIUnknown)] interface IMMDevice {
    [PreserveSig]int Activate(ref Guid iid,uint clsctx,IntPtr parameters,[MarshalAs(UnmanagedType.IUnknown)]out object result);
    [PreserveSig]int OpenPropertyStore(uint access,out IntPtr properties);
    [PreserveSig]int GetId([MarshalAs(UnmanagedType.LPWStr)]out string id);
    [PreserveSig]int GetState(out uint state);
}
[ComImport,Guid("1CB9AD4C-DBFA-4C32-B178-C2F568A703B2"),InterfaceType(ComInterfaceType.InterfaceIsIUnknown)] interface IAudioClient {
    [PreserveSig]int Initialize(int mode,uint flags,long duration,long periodicity,IntPtr format,IntPtr session);
    [PreserveSig]int GetBufferSize(out uint frames);
    [PreserveSig]int GetStreamLatency(out long latency);
    [PreserveSig]int GetCurrentPadding(out uint frames);
    [PreserveSig]int IsFormatSupported(int mode,IntPtr format,out IntPtr closest);
    [PreserveSig]int GetMixFormat(out IntPtr format);
    [PreserveSig]int GetDevicePeriod(out long defaultPeriod,out long minimumPeriod);
    [PreserveSig]int Start(); [PreserveSig]int Stop(); [PreserveSig]int Reset();
    [PreserveSig]int SetEventHandle(IntPtr handle);
    [PreserveSig]int GetService(ref Guid iid,[MarshalAs(UnmanagedType.IUnknown)]out object result);
}
[ComImport,Guid("C8ADBD64-E71E-48A0-A4DE-185C395CD317"),InterfaceType(ComInterfaceType.InterfaceIsIUnknown)] interface IAudioCaptureClient {
    [PreserveSig]int GetBuffer(out IntPtr data,out uint frames,out uint flags,out ulong devicePosition,out ulong qpcPosition);
    [PreserveSig]int ReleaseBuffer(uint frames);
    [PreserveSig]int GetNextPacketSize(out uint frames);
}
sealed class Loopback : IDisposable {
    IMMDeviceEnumerator enumerator; IMMDevice device; IAudioClient client; IAudioCaptureClient capture;
    IntPtr mix; string deviceId; int channels,bits,blockAlign,tag; public int Rate {get; private set;}
    public static void Check(int hr) { if(hr<0)Marshal.ThrowExceptionForHR(hr); }
    public Loopback() {
        try {
            enumerator=(IMMDeviceEnumerator)new DeviceEnumerator(); Check(enumerator.GetDefaultAudioEndpoint(0,0,out device)); Check(device.GetId(out deviceId));
            Guid iid=new Guid("1CB9AD4C-DBFA-4C32-B178-C2F568A703B2"); object obj;
            Check(device.Activate(ref iid,23,IntPtr.Zero,out obj)); client=(IAudioClient)obj; Check(client.GetMixFormat(out mix));
            tag=(ushort)Marshal.ReadInt16(mix,0);channels=(ushort)Marshal.ReadInt16(mix,2);Rate=Marshal.ReadInt32(mix,4);
            blockAlign=(ushort)Marshal.ReadInt16(mix,12);bits=(ushort)Marshal.ReadInt16(mix,14);
            if(tag==0xfffe) { Guid sub=(Guid)Marshal.PtrToStructure(IntPtr.Add(mix,24),typeof(Guid));
                if(sub==new Guid("00000003-0000-0010-8000-00aa00389b71"))tag=3;
                else if(sub==new Guid("00000001-0000-0010-8000-00aa00389b71"))tag=1;
            }
            if(channels<1||channels>32||Rate<8000||Rate>192000||!((tag==3&&bits==32)||(tag==1&&(bits==16||bits==24||bits==32))))throw new NotSupportedException("Unsupported audio format");
            Check(client.Initialize(0,0x20000,1000000,0,mix,IntPtr.Zero));
            iid=new Guid("C8ADBD64-E71E-48A0-A4DE-185C395CD317");Check(client.GetService(ref iid,out obj));capture=(IAudioCaptureClient)obj;Check(client.Start());
        } catch { Dispose(); throw; }
    }
    public bool DeviceChanged() {
        IMMDevice current=null;try { Check(enumerator.GetDefaultAudioEndpoint(0,0,out current));string id;Check(current.GetId(out id));return id!=deviceId; }
        finally { if(current!=null)Marshal.ReleaseComObject(current); }
    }
    static float Sample(byte[] data,int p,int tag,int bits) {
        if(tag==3) { float f=BitConverter.ToSingle(data,p);return Single.IsNaN(f)||Single.IsInfinity(f)?0:f; }
        if(bits==16)return BitConverter.ToInt16(data,p)/32768f;
        if(bits==24) { int v=data[p]|(data[p+1]<<8)|(data[p+2]<<16);if((v&0x800000)!=0)v|=unchecked((int)0xff000000);return v/8388608f; }
        return BitConverter.ToInt32(data,p)/2147483648f;
    }
    public byte[] ReadPacket(out int count) {
        count=0;uint available;Check(capture.GetNextPacketSize(out available));if(available==0)return null;
        IntPtr data;uint frames,flags;ulong position,qpc;Check(capture.GetBuffer(out data,out frames,out flags,out position,out qpc));
        try {
            if(frames>Rate/2)throw new InvalidDataException("Capture packet too large");
            count=(int)frames;float[] stereo=new float[count*2];
            if((flags&2)==0) {
                byte[] raw=new byte[count*blockAlign];Marshal.Copy(data,raw,0,raw.Length);int step=bits/8;
                for(int f=0;f<count;f++) {
                    int p=f*blockAlign;float left=Sample(raw,p,tag,bits),right=channels==1?left:Sample(raw,p+step,tag,bits);
                    // Keep front L/R and add center/surround at reduced gain for multichannel endpoints.
                    if(channels>=3) { float center=Sample(raw,p+2*step,tag,bits)*0.7071f;left+=center;right+=center; }
                    for(int c=4;c<channels;c++) { float v=Sample(raw,p+c*step,tag,bits)*0.5f;if(c%2==0)left+=v;else right+=v; }
                    stereo[f*2]=Math.Max(-1,Math.Min(1,left));stereo[f*2+1]=Math.Max(-1,Math.Min(1,right));
                }
            }
            byte[] payload=new byte[1+stereo.Length*4];payload[0]=2;Buffer.BlockCopy(stereo,0,payload,1,payload.Length-1);return payload;
        } finally { Check(capture.ReleaseBuffer(frames)); }
    }
    public void Dispose() {
        if(client!=null)try {client.Stop();}catch{}
        if(capture!=null){Marshal.ReleaseComObject(capture);capture=null;}
        if(client!=null){Marshal.ReleaseComObject(client);client=null;}
        if(device!=null){Marshal.ReleaseComObject(device);device=null;}
        if(enumerator!=null){Marshal.ReleaseComObject(enumerator);enumerator=null;}
        if(mix!=IntPtr.Zero){Marshal.FreeCoTaskMem(mix);mix=IntPtr.Zero;}
    }
}

[ComImport,Guid("F294ACFC-3146-4483-A7BF-ADDCA7C260E2"),InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
interface IAudioRenderClient {
 [PreserveSig]int GetBuffer(uint frames,out IntPtr data);
 [PreserveSig]int ReleaseBuffer(uint frames,uint flags);
}
sealed class Playback:IDisposable {
 IMMDeviceEnumerator enumerator;IMMDevice device;IAudioClient client;IAudioRenderClient render;
 string deviceId;uint capacity;IntPtr format;
 public Playback(int rate,int bufferMS){
  try{
   enumerator=(IMMDeviceEnumerator)new DeviceEnumerator();Loopback.Check(enumerator.GetDefaultAudioEndpoint(0,0,out device));Loopback.Check(device.GetId(out deviceId));
   Guid iid=new Guid("1CB9AD4C-DBFA-4C32-B178-C2F568A703B2");object obj;
   Loopback.Check(device.Activate(ref iid,23,IntPtr.Zero,out obj));client=(IAudioClient)obj;
   byte[] bytes=new byte[18];Array.Copy(BitConverter.GetBytes((short)3),0,bytes,0,2);Array.Copy(BitConverter.GetBytes((short)2),0,bytes,2,2);
   Array.Copy(BitConverter.GetBytes(rate),0,bytes,4,4);Array.Copy(BitConverter.GetBytes(rate*8),0,bytes,8,4);
   bytes[12]=8;bytes[14]=32;format=Marshal.AllocCoTaskMem(18);Marshal.Copy(bytes,0,format,18);
   // Shared-mode Windows audio engine converts float/stereo/sample rate as needed.
   Loopback.Check(client.Initialize(0,0x88000000,Math.Max(40,bufferMS)*10000L,0,format,IntPtr.Zero));
   Loopback.Check(client.GetBufferSize(out capacity));
   iid=new Guid("F294ACFC-3146-4483-A7BF-ADDCA7C260E2");Loopback.Check(client.GetService(ref iid,out obj));render=(IAudioRenderClient)obj;
   Loopback.Check(client.Start());
  }catch{Dispose();throw;}
 }
 public void Pump(Queue<float> samples,float volume){
  uint padding;Loopback.Check(client.GetCurrentPadding(out padding));
  if(padding>capacity)throw new IOException("Invalid audio buffer padding");
  uint frames=Math.Min(capacity-padding,(uint)(samples.Count/2));if(frames==0)return;
  float[] values=new float[frames*2];for(int i=0;i<values.Length;i++)values[i]=Math.Max(-1,Math.Min(1,samples.Dequeue()*volume));
  IntPtr data;Loopback.Check(render.GetBuffer(frames,out data));
  try{Marshal.Copy(values,0,data,values.Length);}finally{Loopback.Check(render.ReleaseBuffer(frames,0));}
 }
 public bool DeviceChanged(){
  IMMDevice next=null;try{Loopback.Check(enumerator.GetDefaultAudioEndpoint(0,0,out next));string id;Loopback.Check(next.GetId(out id));return id!=deviceId;}
  finally{if(next!=null)Marshal.ReleaseComObject(next);}
 }
 public void Dispose(){
  if(client!=null)try{client.Stop();}catch{}
  if(render!=null){Marshal.ReleaseComObject(render);render=null;}
  if(client!=null){Marshal.ReleaseComObject(client);client=null;}
  if(device!=null){Marshal.ReleaseComObject(device);device=null;}
  if(enumerator!=null){Marshal.ReleaseComObject(enumerator);enumerator=null;}
  if(format!=IntPtr.Zero){Marshal.FreeCoTaskMem(format);format=IntPtr.Zero;}
 }
}
static class AudioWire {
 public static byte[] Exact(Stream stream,int n){byte[] b=new byte[n];int at=0;while(at<n){int got=stream.Read(b,at,n-at);if(got==0)throw new EndOfStreamException();at+=got;}return b;}
 public static byte[] Read(Stream stream){int n=BitConverter.ToInt32(Exact(stream,4),0);if(n<1||n>65537)throw new InvalidDataException("Audio frame size");return Exact(stream,n);}
 public static void Write(Stream stream,byte[] b){stream.Write(BitConverter.GetBytes(b.Length),0,4);stream.Write(b,0,b.Length);stream.Flush();}
 public static byte[] Format(int rate){byte[] b=new byte[7];b[0]=1;b[5]=2;Array.Copy(BitConverter.GetBytes(rate),0,b,1,4);return b;}
 public static int Rate(byte[] b){if(b.Length!=7||b[0]!=1||b[5]!=2||b[6]!=0)throw new InvalidDataException("Audio format");int r=BitConverter.ToInt32(b,1);if(r<8000||r>192000)throw new InvalidDataException("Audio rate");return r;}
 public static float[] Samples(byte[] b){if(b.Length<9||b[0]!=2||(b.Length-1)%8!=0)throw new InvalidDataException("Audio samples");float[] f=new float[(b.Length-1)/4];Buffer.BlockCopy(b,1,f,0,b.Length-1);for(int i=0;i<f.Length;i++)if(Single.IsNaN(f[i])||Single.IsInfinity(f[i]))f[i]=0;return f;}
}
static class AudioProgram {
 static void Capture(){
  var output=Console.OpenStandardOutput();
  using(var source=new Loopback()){
   AudioWire.Write(output,AudioWire.Format(source.Rate));DateTime check=DateTime.UtcNow;
   while(true){
    int frames;byte[] b=source.ReadPacket(out frames);
    if(b==null)Thread.Sleep(5);
    else for(int offset=1;offset<b.Length;offset+=32768){int n=Math.Min(32768,b.Length-offset);byte[] part=new byte[n+1];part[0]=2;Buffer.BlockCopy(b,offset,part,1,n);AudioWire.Write(output,part);}
    if((DateTime.UtcNow-check).TotalMilliseconds>=500){if(source.DeviceChanged())throw new IOException("System output changed; reconnecting");AudioWire.Write(output,new byte[]{3});check=DateTime.UtcNow;}
   }
  }
 }
 static void Play(int volume,int bufferMS,bool verify){
  var input=Console.OpenStandardInput();int rate=AudioWire.Rate(AudioWire.Read(input));
  if(verify){
   long total=0;while(true){byte[] b=AudioWire.Read(input);if(b.Length==1&&b[0]==3)continue;float[] f=AudioWire.Samples(b);
    for(int i=0;i<f.Length;i++)if(f[i]!=(i%2==0?0.125f:-0.125f))throw new InvalidDataException("Synthetic sample mismatch");
    total+=f.Length/2;if(total%48000==0)Console.Error.WriteLine("Synthetic frames verified: "+total);
   }
  }
  var samples=new Queue<float>();object gate=new object();Exception failure=null;bool finished=false;
  Thread reader=new Thread(delegate(){
   try{while(true){byte[] b=AudioWire.Read(input);if(b.Length==1&&b[0]==3)continue;float[] f=AudioWire.Samples(b);
    lock(gate){int max=rate*bufferMS/1000*2;foreach(float v in f)samples.Enqueue(v);while(samples.Count>max)samples.Dequeue();}
   }}catch(EndOfStreamException){}catch(Exception e){lock(gate)failure=e;}finally{lock(gate)finished=true;}
  });reader.IsBackground=true;reader.Start();
  using(var sink=new Playback(rate,bufferMS)){
   DateTime check=DateTime.UtcNow;
   while(true){lock(gate){if(failure!=null)throw failure;sink.Pump(samples,volume/100f);if(finished&&samples.Count==0)return;}
    if((DateTime.UtcNow-check).TotalSeconds>=1){if(sink.DeviceChanged())throw new IOException("System output changed; reconnecting");check=DateTime.UtcNow;}
    Thread.Sleep(3);
   }
  }
 }
 static void Fixture(){
  var output=Console.OpenStandardOutput();AudioWire.Write(output,AudioWire.Format(48000));
  float[] samples=new float[1920];for(int i=0;i<samples.Length;i++)samples[i]=i%2==0?0.125f:-0.125f;
  byte[] b=new byte[1+samples.Length*4];b[0]=2;Buffer.BlockCopy(samples,0,b,1,b.Length-1);
  while(true){AudioWire.Write(output,b);Thread.Sleep(20);}
 }
 [MTAThread] static int Main(string[] args){
  try{
   string mode=args.Length==0?"":args[0];
   if(mode=="--self-test"){var m=new MemoryStream();AudioWire.Write(m,AudioWire.Format(48000));m.Position=0;if(AudioWire.Rate(AudioWire.Read(m))!=48000)throw new Exception("Format round trip failed");Console.WriteLine("Audio framing self-test passed; no audio device opened");return 0;}
   if(args.Length>3){
    var parent=Process.GetProcessById(Int32.Parse(args[3]));
    if(!parent.ProcessName.Equals("twinglide-agent",StringComparison.OrdinalIgnoreCase))throw new ArgumentException("Unexpected parent process");
    var watcher=new Thread(delegate(){try{parent.WaitForExit();}finally{parent.Dispose();Environment.Exit(0);}});watcher.IsBackground=true;watcher.Start();
   }else if(mode=="--capture"||mode=="--play"){throw new ArgumentException("Audio must be started by TwinGlide");}
   int volume=args.Length>1?Int32.Parse(args[1]):100,bufferMS=args.Length>2?Int32.Parse(args[2]):80;
   if(volume<0||volume>100||(bufferMS!=40&&bufferMS!=80&&bufferMS!=160))throw new ArgumentException("Audio preferences");
   if(mode=="--capture")Capture();else if(mode=="--play"||mode=="--verify")Play(volume,bufferMS,mode=="--verify");else if(mode=="--fixture")Fixture();else throw new ArgumentException("Unknown mode");
   return 0;
  }catch(EndOfStreamException){return 0;}catch(Exception e){Console.Error.WriteLine(e.Message);return 1;}
 }
}
