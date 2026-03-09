//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// PowerShell C# COM script for Windows audio (shared by set/get)
const volumePS1 = `$code=@'
using System.Runtime.InteropServices;
[Guid("5CDF2C82-841E-4546-9722-0CF74078229A"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
interface IAudioEndpointVolume {
    int f(); int g(); int h(); int i();
    int SetMasterVolumeLevelScalar(float fLevel, System.Guid pguidEventContext);
    int j();
    int GetMasterVolumeLevelScalar(out float pfLevel);
    int k(); int l(); int m(); int n();
    int SetMute([MarshalAs(UnmanagedType.Bool)] bool bMute, System.Guid pguidEventContext);
    int GetMute(out bool pbMute);
}
[Guid("D666063F-1587-4E43-81F1-B948E807363F"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
interface IMMDevice {
    int Activate(ref System.Guid id, int clsCtx, int activationParams, out IAudioEndpointVolume aev);
}
[Guid("A95664D2-9614-4F35-A746-DE8DB63617E6"), InterfaceType(ComInterfaceType.InterfaceIsIUnknown)]
interface IMMDeviceEnumerator {
    int f();
    int GetDefaultAudioEndpoint(int dataFlow, int role, out IMMDevice endpoint);
}
[ComImport, Guid("BCDE0395-E52F-467C-8E3D-C4579291692E")] class MMDeviceEnumeratorComObject { }
public class Audio {
    static IAudioEndpointVolume Vol() {
        var enumerator = new MMDeviceEnumeratorComObject() as IMMDeviceEnumerator;
        IMMDevice dev = null;
        Marshal.ThrowExceptionForHR(enumerator.GetDefaultAudioEndpoint(0, 1, out dev));
        IAudioEndpointVolume epv = null;
        var epvid = typeof(IAudioEndpointVolume).GUID;
        Marshal.ThrowExceptionForHR(dev.Activate(ref epvid, 23, 0, out epv));
        return epv;
    }
    public static float Volume {
        get { float v = -1; Marshal.ThrowExceptionForHR(Vol().GetMasterVolumeLevelScalar(out v)); return v; }
        set { Marshal.ThrowExceptionForHR(Vol().SetMasterVolumeLevelScalar(value, System.Guid.Empty)); }
    }
}
'@
Add-Type -TypeDefinition $code
`

func init() {
	getVolumeLevelImpl = getVolumeLevelWindows
}

func setVolumeLevel(level int) error {
	level = clamp(level, 0, 100)
	scalar := float64(level) / 100.0
	script := volumePS1 + fmt.Sprintf("[Audio]::Volume = %g", scalar)
	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	prepareCmd(cmd)
	return cmd.Run()
}

func getVolumeLevelWindows() int {
	script := volumePS1 + "[Math]::Round([Audio]::Volume * 100)"
	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	prepareCmd(cmd)
	out, err := cmd.Output()
	if err != nil {
		return -1
	}
	v, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil || v < 0 || v > 100 {
		return -1
	}
	return clamp(v, 0, 100)
}
