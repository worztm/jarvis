# JARVIS voice: Windows SAPI5 synthesis, female voice preferred.
# Protocol (stdin lines):
#   SAY|<text>      cancel current speech, speak text async
#   CANCEL|         stop speaking now (barge-in)
#   VOICE?|         report selected voice name -> VOICE|<name>
#   QUIT|           shutdown
# stdout:
#   VOICE|<name>    voice armed
#   ERR|<msg>
$ErrorActionPreference = 'Stop'
try { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8 } catch {}
function Out-Line([string]$s) { [Console]::Out.WriteLine($s); [Console]::Out.Flush() }

try { Add-Type -AssemblyName System.Speech } catch {
    Out-Line "ERR|System.Speech unavailable: $($_.Exception.Message)"; exit 1
}

$synth = New-Object System.Speech.Synthesis.SpeechSynthesizer

# Female English voice preference: ask SAPI directly by gender (no name guessing).
$female = $synth.GetInstalledVoices() |
    ForEach-Object { $_.VoiceInfo } |
    Where-Object { $_.Gender -eq 'Female' -and $_.Culture.Name -like 'en*' } |
    Select-Object -First 1
if (-not $female) {
    $female = $synth.GetInstalledVoices() |
        ForEach-Object { $_.VoiceInfo } |
        Where-Object { $_.Gender -eq 'Female' } | Select-Object -First 1
}
if (-not $female) {
    $female = $synth.GetInstalledVoices() |
        ForEach-Object { $_.VoiceInfo } |
        Where-Object { $_.Culture.Name -like 'en*' } | Select-Object -First 1
}
if ($female) {
    $synth.SelectVoice($female.Name)
    $synth.Rate = 1
    $synth.Volume = 100
    Out-Line "VOICE|$($female.Name)"
} else {
    Out-Line "VOICE|<system default>"
}

while ($true) {
    $line = [Console]::In.ReadLine()
    if ($null -eq $line) { break }          # parent died -> exit

    if ($line.StartsWith('QUIT')) { break }

    if ($line.StartsWith('CANCEL')) {
        try { $synth.SpeakAsyncCancelAll() } catch {}
        continue
    }

    if ($line.StartsWith('VOICE?')) {
        Out-Line "VOICE|$($synth.Voice.Name)"
        continue
    }

    if ($line.StartsWith('SAY')) {
        $text = $line.Substring(4)
        if ($text.Trim()) {
            try {
                $synth.SpeakAsyncCancelAll()   # barge-in: new command kills old speech
                $null = $synth.SpeakAsync($text)
            } catch {
                Out-Line "ERR|speak failed: $($_.Exception.Message)"
            }
        }
        continue
    }
}
$synth.Dispose()
