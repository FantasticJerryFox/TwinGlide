# Render the authored geometric SVG design with the built-in Windows drawing API.
# No external image services or downloaded assets. GPL-2.0-only.
$ErrorActionPreference='Stop'
Add-Type -AssemblyName System.Drawing
$asset=Join-Path (Split-Path $PSScriptRoot -Parent) 'assets'
function Draw-Icon([int]$size) {
 $bitmap=New-Object System.Drawing.Bitmap($size,$size)
 $g=[Drawing.Graphics]::FromImage($bitmap)
 $g.SmoothingMode=[Drawing.Drawing2D.SmoothingMode]::AntiAlias
 $g.ScaleTransform($size/1024.0,$size/1024.0)
 $path=New-Object Drawing.Drawing2D.GraphicsPath
 $path.AddArc(0,0,448,448,180,90);$path.AddArc(576,0,448,448,270,90)
 $path.AddArc(576,576,448,448,0,90);$path.AddArc(0,576,448,448,90,90);$path.CloseFigure()
 $brush=New-Object Drawing.SolidBrush([Drawing.ColorTranslator]::FromHtml('#245bd6'))
 $g.FillPath($brush,$path)
 $pen=New-Object Drawing.Pen([Drawing.Color]::White,76)
 $pen.StartCap=$pen.EndCap=[Drawing.Drawing2D.LineCap]::Round
 $pen.LineJoin=[Drawing.Drawing2D.LineJoin]::Round
 $g.DrawBezier($pen,240,360,360,220,490,220,620,360)
 $g.DrawLine($pen,620,360,700,440);$g.DrawLine($pen,700,310,700,440);$g.DrawLine($pen,700,440,570,440)
 $pen.Color=[Drawing.ColorTranslator]::FromHtml('#a9e8ff')
 $g.DrawBezier($pen,784,664,664,804,534,804,404,664)
 $g.DrawLine($pen,404,664,324,584);$g.DrawLine($pen,324,714,324,584);$g.DrawLine($pen,324,584,454,584)
 $pen.Dispose();$brush.Dispose();$path.Dispose();$g.Dispose()
 return $bitmap
}
$large=Draw-Icon 1024
try{$large.Save((Join-Path $asset 'TwinGlide.png'),[Drawing.Imaging.ImageFormat]::Png)}finally{$large.Dispose()}
$sizes=@(16,24,32,48,64,128,256)
$images=@()
foreach($size in $sizes){
 $bitmap=Draw-Icon $size
 $stream=New-Object IO.MemoryStream
 try{$bitmap.Save($stream,[Drawing.Imaging.ImageFormat]::Png);$images+=,@($stream.ToArray())}finally{$bitmap.Dispose();$stream.Dispose()}
}
$output=[IO.File]::Create((Join-Path $asset 'TwinGlide.ico'));$writer=New-Object IO.BinaryWriter($output)
try{
 $writer.Write([uint16]0);$writer.Write([uint16]1);$writer.Write([uint16]$sizes.Count)
 $offset=6+16*$sizes.Count
 for($i=0;$i -lt $sizes.Count;$i++){
  $dimension=if($sizes[$i] -eq 256){0}else{$sizes[$i]}
  $writer.Write([byte]$dimension);$writer.Write([byte]$dimension);$writer.Write([byte]0);$writer.Write([byte]0)
  $writer.Write([uint16]1);$writer.Write([uint16]32);$writer.Write([uint32]$images[$i].Length);$writer.Write([uint32]$offset)
  $offset+=$images[$i].Length
 }
 foreach($bytes in $images){$writer.Write([byte[]]$bytes)}
}finally{$writer.Dispose();$output.Dispose()}
