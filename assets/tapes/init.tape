# lagoon init — real capture of the setup wizard
Output assets/init.gif

Set Shell bash
Set FontSize 22
Set Width 1100
Set Height 720
Set Padding 24
Set Theme "Catppuccin Mocha"
Set TypingSpeed 50ms

Hide
Type "mkdir -p /tmp/initdemo && cd /tmp/initdemo && rm -f lagoon.toml && clear"
Enter
Show

Sleep 800ms
Type "lagoon init"
Sleep 500ms
Enter
Sleep 2500ms

# intent: dev workspace (first option)
Enter
Sleep 1200ms

# preset: python (second option)
Down
Sleep 600ms
Enter
Sleep 2s

# network access: keep default (on for python preset)
Enter
Sleep 1500ms

# add more packages: keep "use preset only"
Enter
Sleep 2500ms

# preview card is shown — confirm write
Left
Sleep 600ms
Enter
Sleep 3s
