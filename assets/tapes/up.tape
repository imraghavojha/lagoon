# lagoon up — real capture of the live service dashboard
Output assets/up.gif

Set Shell bash
Set FontSize 22
Set Width 1100
Set Height 720
Set Padding 24
Set Theme "Catppuccin Mocha"
Set TypingSpeed 50ms

Hide
Type "cd ~/code/myapp"
Enter
Type "(sleep 8; curl -s localhost:8000 >/dev/null; sleep 1; curl -s localhost:8000/app.py >/dev/null) >/dev/null 2>&1 &"
Enter
Type "clear"
Enter
Show

Sleep 800ms
Type "lagoon up"
Sleep 500ms
Enter
Sleep 12s

Screenshot assets/up.png

Sleep 2s
Type "q"
Sleep 2500ms
